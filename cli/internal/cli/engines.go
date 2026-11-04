package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/engine/claude"
	"github.com/joaoprofile/gofi/cli/internal/engine/codex"
	"github.com/joaoprofile/gofi/cli/internal/engine/copilot"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// EnvCodex and EnvCopilot point at the executables of those hosts, as
// EnvEngine does at Claude Code's; empty, they are looked up on PATH.
const (
	EnvCodex   = "GOFI_CODEX"
	EnvCopilot = "GOFI_COPILOT"
)

// agentEngine opens the agent sessions of the host the project runs on —
// ai.host, as gofi init set it: Claude Code, Codex or Copilot. Every agent
// turn of gofi ask and gofi run goes through it, so the loop is the same
// whichever host does the work. mode and tools are Claude Code's permission
// mode and allowed tools; the other hosts run in their own sandbox.
func agentEngine(cfg *config.GofiConfig, root, mode string, tools []string) (func(model string) engine.Session, error) {
	open, err := agentSessions(cfg, root, mode, tools, false)
	if err != nil {
		return nil, err
	}
	return func(model string) engine.Session { return open(model, nil) }, nil
}

// agentSessions is agentEngine with the tools each session may use: a role's
// grant from its contract (0002, D15), or nil for the defaults given here.
// Claude Code gets exactly the grant, and every tool that edits a file or
// runs a command outside it refused; Codex and Copilot grant per sandbox,
// not per tool, and run the role in theirs.
//
// A turn of the loop is isolated: it runs on the project alone, with gofi's
// MCP server only, and its system prompt says no one reads its text.
func agentSessions(cfg *config.GofiConfig, root, mode string, tools []string, loopTurn bool) (func(model string, grant []string) engine.Session, error) {
	h, ok := host.Get(cfg.AI.Host)
	if !ok {
		return nil, fmt.Errorf("ai.host %q — %s", cfg.AI.Host, strings.Join(host.IDs(), ", "))
	}
	switch h.ID {
	case host.Codex.ID:
		exe, err := hostExecutable(h, EnvCodex, codex.DefaultExecutable)
		if err != nil {
			return nil, err
		}
		return func(model string, grant []string) engine.Session {
			sandbox, config := codexTools(grant)
			return skillTurns{codex.New(codex.Options{Executable: exe, Dir: root, Model: model, Sandbox: sandbox, Config: config})}
		}, nil
	case host.Copilot.ID:
		exe, err := hostExecutable(h, EnvCopilot, copilot.DefaultExecutable)
		if err != nil {
			return nil, err
		}
		return func(model string, grant []string) engine.Session {
			return skillTurns{copilot.New(copilot.Options{Executable: exe, Dir: root, Model: model, Deny: copilotDenied(grant)})}
		}, nil
	}
	if err := requireClaudeCode(os.Getenv(EnvEngine)); err != nil {
		return nil, err
	}
	return func(model string, grant []string) engine.Session {
		allowed, denied := tools, []string(nil)
		if grant != nil {
			allowed, denied = claudeTools(grant)
		}
		// The contract is the person's: no turn of the loop writes it, and
		// the engine refuses before the integrity stage would (0002, D12).
		denied = append(denied, protectedWrites...)
		o := claude.Options{Executable: os.Getenv(EnvEngine), Dir: root, Model: model,
			PermissionMode: mode, AllowedTools: allowed, DisallowedTools: denied}
		if loopTurn {
			o.SystemPrompt, o.Isolated, o.MCPConfig = loopSystemPrompt, true, host.MCPConfig()
		}
		return claude.New(o)
	}, nil
}

// loopSystemPrompt is what a turn of the loop is told beyond its brief: the
// chat's "ask in text" would lose the question.
const loopSystemPrompt = "You are one turn of gofi's delivery loop, not a conversation: no one reads your text. " +
	"Do what the prompt asks with the tools you have and end the turn. " +
	"A decision you lack goes to the elicitation file the prompt names, never to the text of your answer."

// claudeGrants are Claude Code's own tools for each tool a contract grants;
// gofi's are in gofiGrants.
var claudeGrants = map[string][]string{
	"read":   {"Read"},
	"edit":   {"Edit", "MultiEdit", "NotebookEdit"},
	"write":  {"Write"},
	"search": {"Glob", "Grep"},
	"shell":  {"Bash"},
}

// gofiGrants are the tools of gofi's MCP server, by their name on it, for
// each tool a contract grants: the same on every host.
var gofiGrants = map[string][]string{
	"gofi":    {"find", "show", "path", "impact", "index_status", "index"},
	"verify":  {"verify"},
	"verdict": {"verdict_write"},
	"memory":  {"memory_write"},
}

// gofiServerTools are every tool gofi's MCP server serves.
var gofiServerTools = []string{"find", "show", "path", "impact", "intake", "index_status", "index", "verify", "verdict_write", "memory_write"}

// grantedGofi are the server's tools a grant names, in the server's order.
func grantedGofi(grant []string) []string {
	var named []string
	for _, g := range grant {
		named = append(named, gofiGrants[g]...)
	}
	var out []string
	for _, t := range gofiServerTools {
		if slices.Contains(named, t) {
			out = append(out, t)
		}
	}
	return out
}

// changes reports whether a grant lets the role change the project.
func changes(grant []string) bool {
	return slices.Contains(grant, "edit") || slices.Contains(grant, "write") || slices.Contains(grant, "shell")
}

// codexTools turns a contract's grant into Codex's sandbox and the -c
// overrides of gofi's server. Codex edits through its shell, so it has no
// tool to refuse one by one: a role that changes nothing runs read-only.
// The server is given whole — Codex reads the project's config only for a
// trusted project — with the tools the role is granted, approved: a headless
// exec cancels a call it would ask about (openai/codex#24135). A nil grant —
// a skill's turn — keeps the sandbox and every tool.
func codexTools(grant []string) (sandbox string, config []string) {
	server := "mcp_servers." + host.ServerName
	config = []string{server + `.command="gofi"`, server + `.args=["mcp"]`, server + `.default_tools_approval_mode="approve"`}
	if grant == nil {
		return "", config
	}
	if !changes(grant) {
		sandbox = "read-only"
	}
	tools := grantedGofi(grant)
	if len(tools) == 0 {
		return sandbox, append(config, server+".enabled=false")
	}
	return sandbox, append(config, server+`.enabled_tools=["`+strings.Join(tools, `", "`)+`"]`)
}

// copilotDenied turns a contract's grant into what Copilot refuses: a deny
// wins over the --allow-all-tools a headless run needs. Copilot's write is
// every edit, so edit and write are granted together; reading and searching
// it never asks about. Its write(path) matches a file's name, not a folder,
// so the contract is not refused here: the integrity stage refuses an edit
// of it (0002, D12). A nil grant — a skill's turn — refuses nothing.
func copilotDenied(grant []string) []string {
	if grant == nil {
		return nil
	}
	var deny []string
	if !slices.Contains(grant, "shell") {
		deny = append(deny, "shell")
	}
	if !slices.Contains(grant, "edit") && !slices.Contains(grant, "write") {
		deny = append(deny, "write")
	}
	granted := grantedGofi(grant)
	if len(granted) == 0 {
		return append(deny, host.ServerName)
	}
	for _, t := range gofiServerTools {
		if !slices.Contains(granted, t) {
			deny = append(deny, host.ServerName+"("+t+")")
		}
	}
	return deny
}

// protectedWrites are the writes no turn of the loop makes: the contract and
// the discovery change only by the person.
var protectedWrites = []string{
	"Edit(specs/**)", "MultiEdit(specs/**)", "Write(specs/**)",
	"Edit(prd/**)", "MultiEdit(prd/**)", "Write(prd/**)",
}

// gated are the tools that change the project or reach past it, refused to
// a role whose contract does not grant them — and the subagents: a turn is
// one session, and work handed to a subagent in the background is lost when
// the turn ends.
var gated = []string{"Bash", "BashOutput", "KillShell", "Edit", "MultiEdit", "NotebookEdit", "Write", "Task", "Agent", "WebFetch", "WebSearch"}

// claudeTools turns a contract's grant into Claude Code's allowed tools and
// the gated ones it refuses.
func claudeTools(grant []string) (allowed, denied []string) {
	for _, g := range grant {
		allowed = append(allowed, claudeGrants[g]...)
		for _, t := range gofiGrants[g] {
			allowed = append(allowed, guard.MCPPrefix+t)
		}
	}
	for _, t := range gated {
		if !slices.Contains(allowed, t) {
			denied = append(denied, t)
		}
	}
	return allowed, denied
}

// hostExecutable is the host's CLI, refused before any turn when it is not
// installed: a run that cannot reach its engine should not start.
func hostExecutable(h host.Host, env, name string) (string, error) {
	exe := os.Getenv(env)
	if exe == "" {
		exe = name
	}
	if _, err := exec.LookPath(exe); err != nil {
		return "", fmt.Errorf("%s", i18n.T("engine.missing", h.Label, exe))
	}
	return exe, nil
}

// skillTurns has a host with no slash commands follow a role's skill: a turn
// that invokes /<role> is sent as "follow the role in <home>/skills/<role>/
// SKILL.md", the path the host discovers its skills under. Claude Code runs
// /<role> itself.
type skillTurns struct{ engine.Session }

func (s skillTurns) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	t.Prompt = asSkill(t.Prompt)
	return s.Session.Send(ctx, t)
}

// asSkill rewrites a leading /<role> into the instruction to follow it.
func asSkill(prompt string) string {
	if !strings.HasPrefix(prompt, "/") {
		return prompt
	}
	role, rest, _ := strings.Cut(prompt[1:], " ")
	if role == "" || strings.ContainsAny(role, "/\n") {
		return prompt
	}
	return fmt.Sprintf("Siga o papel descrito em %s.\n\n%s", layout.Skills().Path(role, "SKILL.md"), strings.TrimLeft(rest, " "))
}

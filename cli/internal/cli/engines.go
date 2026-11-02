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
	open, err := agentSessions(cfg, root, mode, tools)
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
func agentSessions(cfg *config.GofiConfig, root, mode string, tools []string) (func(model string, grant []string) engine.Session, error) {
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
		return func(model string, _ []string) engine.Session {
			return skillTurns{codex.New(codex.Options{Executable: exe, Dir: root, Model: model})}
		}, nil
	case host.Copilot.ID:
		exe, err := hostExecutable(h, EnvCopilot, copilot.DefaultExecutable)
		if err != nil {
			return nil, err
		}
		return func(model string, _ []string) engine.Session {
			return skillTurns{copilot.New(copilot.Options{Executable: exe, Dir: root, Model: model})}
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
		return claude.New(claude.Options{Executable: os.Getenv(EnvEngine), Dir: root, Model: model,
			PermissionMode: mode, AllowedTools: allowed, DisallowedTools: denied})
	}, nil
}

// claudeGrants are Claude Code's tools for each tool a contract grants.
var claudeGrants = map[string][]string{
	"read":    {"Read"},
	"edit":    {"Edit", "MultiEdit", "NotebookEdit"},
	"write":   {"Write"},
	"search":  {"Glob", "Grep"},
	"gofi":    {guard.MCPPrefix + "find", guard.MCPPrefix + "show", guard.MCPPrefix + "path", guard.MCPPrefix + "index_status", guard.MCPPrefix + "index"},
	"verify":  {guard.MCPPrefix + "verify"},
	"verdict": {guard.MCPPrefix + "verdict_write"},
	"memory":  {guard.MCPPrefix + "memory_write"},
	"shell":   {"Bash"},
}

// gated are the tools that change the project or reach past it: refused to
// a role whose contract does not grant them.
var gated = []string{"Bash", "Edit", "MultiEdit", "NotebookEdit", "Write"}

// claudeTools turns a contract's grant into Claude Code's allowed tools and
// the gated ones it refuses.
func claudeTools(grant []string) (allowed, denied []string) {
	for _, g := range grant {
		allowed = append(allowed, claudeGrants[g]...)
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

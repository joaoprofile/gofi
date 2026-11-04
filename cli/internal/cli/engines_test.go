//go:build !windows

package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// A /<role> turn is sent to a host with no slash commands as the role's
// skill to follow; anything else goes as it is.
func TestASkillTurnBecomesTheRoleToFollow(t *testing.T) {
	got := asSkill("/gofi-eng Implemente a mudança")
	want := "Siga o papel descrito em " + layout.Skills().Path("gofi-eng", "SKILL.md") + ".\n\nImplemente a mudança"
	if got != want {
		t.Errorf("got %q", got)
	}
	for _, p := range []string{"Implemente", "/tmp/x é um caminho", "/"} {
		if asSkill(p) != p {
			t.Errorf("%q rewritten to %q", p, asSkill(p))
		}
	}
}

// The agents work on the project's host: with ai.host codex, a turn goes to
// the codex CLI, its /<role> as the skill to follow.
func TestTheAgentsRunOnTheProjectsHost(t *testing.T) {
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	exe := filepath.Join(dir, "codex")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > " + args + "\n" +
		`echo '{"type":"thread.started","thread_id":"th"}'; echo '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'` + "\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvCodex, exe)
	cfg := &config.GofiConfig{AI: config.AI{Host: "codex"}}
	open, err := agentEngine(cfg, dir, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ch, err := open("").Send(context.Background(), engine.Turn{Prompt: "/gofi-eng Implemente"})
	if err != nil {
		t.Fatal(err)
	}
	var last engine.Event
	for e := range ch {
		last = e
	}
	if d, ok := last.(engine.Done); !ok || d.IsError {
		t.Fatalf("last event %+v", last)
	}
	got, _ := os.ReadFile(args)
	if !strings.HasPrefix(string(got), "exec --json") || !strings.Contains(string(got), "Siga o papel descrito em") {
		t.Errorf("codex got: %s", got)
	}

	// Not installed: refused before any turn, saying which host and CLI.
	t.Setenv(EnvCopilot, filepath.Join(dir, "copilot-que-nao-existe"))
	cfg.AI.Host = "copilot"
	if _, err := agentEngine(cfg, dir, "", nil); err == nil || !strings.Contains(err.Error(), "GitHub Copilot") {
		t.Errorf("a missing copilot: %v", err)
	}
}

// A role's grant is what it may do on every host: Codex runs a role that
// changes nothing read-only, with only its tools of gofi's server; Copilot
// refuses what the role is not granted.
func TestARolesGrantHoldsOnCodexAndCopilot(t *testing.T) {
	audit := []string{"read", "search", "gofi", "verdict"}
	sandbox, config := codexTools(audit)
	if sandbox != "read-only" {
		t.Errorf("audit sandbox = %q", sandbox)
	}
	want := `mcp_servers.gofi.enabled_tools=["find", "show", "path", "impact", "index_status", "index", "verdict_write"]`
	if !slices.Contains(config, want) || !slices.Contains(config, `mcp_servers.gofi.command="gofi"`) {
		t.Errorf("audit config = %v", config)
	}
	if sandbox, _ := codexTools([]string{"read", "edit", "shell", "verify"}); sandbox != "" {
		t.Errorf("a role that edits runs in %q", sandbox)
	}
	if _, config := codexTools([]string{"read"}); !slices.Contains(config, "mcp_servers.gofi.enabled=false") {
		t.Errorf("no gofi tool granted: %v", config)
	}
	if sandbox, config := codexTools(nil); sandbox != "" || slices.ContainsFunc(config, func(c string) bool { return strings.Contains(c, "enabled") }) {
		t.Errorf("a skill's turn is limited: %q %v", sandbox, config)
	}

	deny := copilotDenied(audit)
	for _, d := range []string{"shell", "write", "gofi(verify)", "gofi(memory_write)", "gofi(intake)"} {
		if !slices.Contains(deny, d) {
			t.Errorf("audit on Copilot does not refuse %s: %v", d, deny)
		}
	}
	if slices.Contains(deny, "gofi(verdict_write)") || slices.Contains(deny, "gofi(find)") {
		t.Errorf("audit on Copilot refuses a granted tool: %v", deny)
	}
	if d := copilotDenied([]string{"read", "edit"}); !slices.Equal(d, []string{"shell", "gofi"}) {
		t.Errorf("edit, no gofi: %v", d)
	}
	if copilotDenied(nil) != nil {
		t.Error("a skill's turn refuses tools on Copilot")
	}
}

// A turn of the loop reads the sections of AGENTS.md it works by, in its
// system prompt, and Claude Code is told to leave the whole file out.
func TestALoopTurnReadsTheSectionsOfAGENTSItWorksBy(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# AGENTS.md\n\nintro\n\n## Leis comuns a todas as skills\n\nlei 1\n\n## Skills disponíveis\n\n| /gofi-pd |\n\n## Projeto\n\n- nome: x\n")
	common, settings := loopMemory(root)
	if !strings.Contains(common, "lei 1") || !strings.Contains(common, "nome: x") || strings.Contains(common, "/gofi-pd") || strings.Contains(common, "intro") {
		t.Errorf("common:\n%s", common)
	}
	abs, _ := filepath.Abs(filepath.Join(root, "AGENTS.md"))
	if !strings.Contains(settings, `"claudeMdExcludes":["`+abs+`"]`) {
		t.Errorf("settings %s", settings)
	}
	if c, s := loopMemory(t.TempDir()); c != "" || s != "" {
		t.Errorf("no AGENTS.md: %q %q", c, s)
	}
}

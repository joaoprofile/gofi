//go:build !windows

package cli

import (
	"context"
	"os"
	"path/filepath"
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

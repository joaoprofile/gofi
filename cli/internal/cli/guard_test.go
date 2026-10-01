package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

func readSettings(t *testing.T, root string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, claudeSettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// The team's own hooks, permissions and keys survive; running it again
// changes nothing; turning the guard off takes out only gofi's entries.
func TestAgentSettingsMergeWithTheTeams(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, claudeSettingsFile), `{
  "model": "opus",
  "permissions": {"allow": ["Bash(make test)"]},
  "hooks": {"PreToolUse": [{"matcher": "Edit", "hooks": [{"type": "command", "command": "lint.sh"}]}]}
}`)
	if changed, err := installAgentSettings(root, config.AI{Guard: config.GuardWarn}); err != nil || !changed {
		t.Fatalf("install: changed=%v err=%v", changed, err)
	}
	if changed, err := installAgentSettings(root, config.AI{Guard: config.GuardWarn}); err != nil || changed {
		t.Fatalf("second install rewrote the file: changed=%v err=%v", changed, err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, claudeSettingsFile))
	for _, want := range []string{`"model": "opus"`, `"Bash(make test)"`, `"lint.sh"`, `"mcp__gofi__find"`, `"gofi hook guard"`, `"UserPromptSubmit"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("settings lack %s:\n%s", want, raw)
		}
	}

	// The guard off takes out its entries only: the intake's prompt hook,
	// on by default, stays.
	if _, err := installAgentSettings(root, config.AI{Guard: config.GuardOff}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(root, claudeSettingsFile))
	if strings.Contains(string(raw), guardCommand) || !strings.Contains(string(raw), intakeCommand) {
		t.Errorf("guard off should leave only the intake's hook:\n%s", raw)
	}
	// With both off, nothing of gofi's is left on the prompt.
	if _, err := installAgentSettings(root, config.AI{Guard: config.GuardOff, Intake: config.IntakeOff}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(root, claudeSettingsFile))
	if strings.Contains(string(raw), "UserPromptSubmit") {
		t.Errorf("gofi's prompt hooks left after both off:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"lint.sh"`) {
		t.Errorf("the team's hook went with the guard's:\n%s", raw)
	}
}

// The hook as Claude Code runs it: JSON in on stdin, a decision out on stdout,
// the mode read from the project the session is in.
func TestHookGuardAgainstAProject(t *testing.T) {
	root := configuredProject(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	run := func(in string) string {
		var out bytes.Buffer
		runHookGuard(strings.NewReader(in), &out)
		return out.String()
	}
	ev := func(event, tool, input string) string {
		return `{"session_id":"abc","hook_event_name":"` + event + `","tool_name":"` + tool + `","tool_input":` + input + `,"cwd":"` + filepath.ToSlash(root) + `"}`
	}
	run(ev("UserPromptSubmit", "", "{}"))
	if out := run(ev("PreToolUse", "Grep", `{"pattern":"Bill"}`)); !strings.Contains(out, "additionalContext") {
		t.Fatalf("warn mode said nothing about a grep before any query: %q", out)
	}

	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Guard = config.GuardEnforce
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	run(ev("UserPromptSubmit", "", "{}"))
	if out := run(ev("PreToolUse", "Grep", `{"pattern":"Bill"}`)); !strings.Contains(out, `"deny"`) {
		t.Fatalf("enforce did not refuse: %q", out)
	}
	run(ev("PreToolUse", "mcp__gofi__find", `{"query":"bill"}`))
	if out := run(ev("PreToolUse", "Grep", `{"pattern":"Bill"}`)); out != "" {
		t.Errorf("a grep after a query was refused: %q", out)
	}
	if out := run("not json"); out != "" {
		t.Errorf("bad input must pass silently: %q", out)
	}
}

func TestGuardCommandSetsTheMode(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newGuardCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"enforce"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil || cfg.AI.Guard != config.GuardEnforce {
		t.Fatalf("mode not saved: %v %v", cfg.AI.Guard, err)
	}
	if s := readSettings(t, root); s["hooks"] == nil {
		t.Error("hook not installed")
	}
	cmd = newGuardCmd()
	cmd.SetArgs([]string{"loud"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil {
		t.Error("an unknown mode was accepted")
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/guard"
)

// intakeProject is a configured project with the shipped contracts and one
// context, pricing, that has a spec.
func intakeProject(t *testing.T) string {
	t.Helper()
	root := configuredProject(t)
	skills, _ := filepath.Glob("../../../ai/skills/*/contract.yaml")
	for _, s := range skills {
		b, _ := os.ReadFile(s)
		writeFile(t, filepath.Join(root, ".claude/skills", filepath.Base(filepath.Dir(s)), "contract.yaml"), string(b))
	}
	writeFile(t, filepath.Join(root, "specs", "pricing", "sdd-pricing.md"),
		"---\ntipo: spec\nformato: sdd\ncontexto: pricing\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [preco]\n---\n# SDD — pricing\n\n## 1. Envio de preços\n\nEm lote.\n")
	// gofi init builds the document index; the hook reads it.
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	return root
}

func hookIntake(t *testing.T, root, prompt string) string {
	t.Helper()
	return hookIntakeIn(t, root, prompt, "")
}

// hookIntakeIn is hookIntake inside a session whose transcript is at path.
func hookIntakeIn(t *testing.T, root, prompt, transcript string) string {
	t.Helper()
	in, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": prompt, "cwd": root, "transcript_path": transcript})
	var out bytes.Buffer
	runHookIntake(bytes.NewReader(in), &out)
	return out.String()
}

// A task typed straight into Claude Code reaches the agent with its plan.
func TestIntakeHookHandsThePlan(t *testing.T) {
	root := intakeProject(t)
	start := time.Now()
	out := hookIntake(t, root, "altere o envio de pricing, adicionando um rate limit")
	t.Logf("hook took %v", time.Since(start))
	var r struct {
		Hook struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not the hook's JSON: %v\n%s", err, out)
	}
	if r.Hook.Event != guard.EventPrompt {
		t.Errorf("event = %q", r.Hook.Event)
	}
	for _, want := range []string{"/gofi-spec (deep) → /gofi-eng", "contexto pricing", "leia: specs/pricing/sdd-pricing.md"} {
		if !strings.Contains(r.Hook.Context, want) {
			t.Errorf("context lacks %q:\n%s", want, r.Hook.Context)
		}
	}
}

// Quiet on a command, when turned off, on a conducted turn, outside a
// project, and on input it cannot read.
func TestIntakeHookStaysQuiet(t *testing.T) {
	root := intakeProject(t)
	if out := hookIntake(t, root, "/gofi-eng implemente o cancelamento"); out != "" {
		t.Errorf("a skill invoked outright got a hint: %s", out)
	}
	if out := hookIntake(t, root, "!git status"); out != "" {
		t.Errorf("a shell command got a hint: %s", out)
	}
	if out := hookIntake(t, t.TempDir(), "altere o envio de pricing"); out != "" {
		t.Errorf("outside a project: %s", out)
	}
	var out bytes.Buffer
	runHookIntake(strings.NewReader("not json"), &out)
	if out.Len() != 0 {
		t.Errorf("unreadable input: %s", out.String())
	}
	t.Setenv(EnvConducting, "1")
	if out := hookIntake(t, root, "altere o envio de pricing, adicionando um rate limit"); out != "" {
		t.Errorf("a conducted turn got a hint: %s", out)
	}
	t.Setenv(EnvConducting, "")
	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Intake = config.IntakeOff
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	if out := hookIntake(t, root, "altere o envio de pricing, adicionando um rate limit"); out != "" {
		t.Errorf("ai.intake off: %s", out)
	}
}

// The prompt hook is installed beside the guard, taken off by ai.intake: off,
// and the team's own hooks survive both.
func TestIntakeHookInstallation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, claudeSettingsFile),
		`{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"./team-hook.sh"}]}]}}`)
	if _, err := installAgentSettings(root, config.AI{}); err != nil {
		t.Fatal(err)
	}
	s, _ := loadClaudeSettings(root)
	if !s.hasIntake() || !s.hasGuard() {
		t.Fatalf("default settings lack a hook: intake=%v guard=%v", s.hasIntake(), s.hasGuard())
	}
	if _, err := installAgentSettings(root, config.AI{Intake: config.IntakeOff}); err != nil {
		t.Fatal(err)
	}
	s, _ = loadClaudeSettings(root)
	if s.hasIntake() || !s.hasGuard() {
		t.Errorf("ai.intake off: intake=%v guard=%v", s.hasIntake(), s.hasGuard())
	}
	b, _ := os.ReadFile(filepath.Join(root, claudeSettingsFile))
	if !strings.Contains(string(b), "./team-hook.sh") {
		t.Errorf("the team's hook was dropped:\n%s", b)
	}
}

func setIntakeMode(t *testing.T, root, mode string) {
	t.Helper()
	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Intake = mode
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
}

// In gate mode a vague prompt is held back before any model reads it, with
// its questions; a prompt with nothing open goes on with its plan.
func TestIntakeGateHoldsAVaguePrompt(t *testing.T) {
	root := intakeProject(t)
	setIntakeMode(t, root, config.IntakeGate)
	var r struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	out := hookIntake(t, root, "ajuste aquilo")
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not the hook's JSON: %v\n%s", err, out)
	}
	if r.Decision != "block" || !strings.Contains(r.Reason, "Em qual contexto?") || !strings.Contains(r.Reason, "gofi ask") {
		t.Errorf("reply = %+v\n%s", r, out)
	}
	out = hookIntake(t, root, "altere o envio de pricing, adicionando um rate limit")
	if strings.Contains(out, `"decision"`) || !strings.Contains(out, "additionalContext") {
		t.Errorf("a clear prompt was held back: %s", out)
	}
	// In hint mode the same vague prompt goes on, with its questions for the agent.
	setIntakeMode(t, root, config.IntakeHint)
	if out := hookIntake(t, root, "ajuste aquilo"); strings.Contains(out, `"decision"`) {
		t.Errorf("hint held a prompt back: %s", out)
	}
}

// A session on a deep model is told when the plan needs less; one on the
// tier the plan needs is not.
func TestIntakeHookTellsWhenTheSessionIsDeeperThanThePlan(t *testing.T) {
	root := intakeProject(t)
	transcript := filepath.Join(t.TempDir(), "s.jsonl")
	writeFile(t, transcript, `{"type":"assistant","message":{"model":"claude-opus-5-5","content":[]}}`+"\n")
	var r struct {
		System string `json:"systemMessage"`
	}
	fix := "corrija o arredondamento no envio de pricing"
	_ = json.Unmarshal([]byte(hookIntakeIn(t, root, fix, transcript)), &r)
	if !strings.Contains(r.System, "claude-opus-5-5") || !strings.Contains(r.System, "/model sonnet") {
		t.Errorf("no tip for a fix on opus: %q", r.System)
	}
	r.System = ""
	_ = json.Unmarshal([]byte(hookIntakeIn(t, root, "altere o envio de pricing, adicionando um rate limit", transcript)), &r)
	if r.System != "" {
		t.Errorf("a plan with a deep phase got a tip: %q", r.System)
	}
	writeFile(t, transcript, `{"type":"assistant","message":{"model":"claude-sonnet-5-5","content":[]}}`+"\n")
	r.System = ""
	_ = json.Unmarshal([]byte(hookIntakeIn(t, root, fix, transcript)), &r)
	if r.System != "" {
		t.Errorf("a session on sonnet got a tip: %q", r.System)
	}
}

// A question about the project gets its pointers from the hook, not a plan.
func TestIntakeHookPointsAQuestionAtItsSections(t *testing.T) {
	root := intakeProject(t)
	setIntakeMode(t, root, config.IntakeHint)
	out := hookIntake(t, root, "explique como funciona o envio de pricing")
	if !strings.Contains(out, "pergunta sobre o projeto") || !strings.Contains(out, "specs/pricing/sdd-pricing.md") || strings.Contains(out, `"decision"`) {
		t.Errorf("hook = %s", out)
	}
}

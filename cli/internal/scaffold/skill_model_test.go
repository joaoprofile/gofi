package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/joaoprofile/gofi/cli/internal/host"
)

func tieredSkills() fstest.MapFS {
	return fstest.MapFS{
		"ai/skills/gofi-status/SKILL.md":      {Data: []byte("---\nname: gofi-status\ndescription: Status.\n---\n# /gofi-status\n")},
		"ai/skills/gofi-status/contract.yaml": {Data: []byte("tier: light\n")},
		"ai/skills/gofi-pd/SKILL.md":          {Data: []byte("---\nname: gofi-pd\ndescription: PRD.\n---\n# /gofi-pd\n")},
		"ai/skills/gofi-pd/contract.yaml":     {Data: []byte("tier: deep\n")},
		"ai/skills/gofi-old/SKILL.md":         {Data: []byte("# /gofi-old — no contract\n")},
	}
}

func modelLine(t *testing.T, root, skill string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".claude/skills", skill, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(frontmatterOf(t, b), "\n") {
		if strings.HasPrefix(l, "model:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "model:"))
		}
	}
	return ""
}

func withSkillModels(t *testing.T, h host.Host, tiers map[string]string) {
	t.Helper()
	SetSkillModels(h, tiers)
	t.Cleanup(func() { skillModel = nil })
}

// On Claude Code each skill runs on the model its tier maps to; the
// project's ai.tiers wins; a skill without a contract names none.
func TestSkillsNameTheModelOfTheirTier(t *testing.T) {
	withSkillModels(t, host.ClaudeCode, map[string]string{"deep": "claude-opus-5-5"})
	root := t.TempDir()
	if _, err := InstallSkillsContent(tieredSkills(), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	for skill, want := range map[string]string{"gofi-status": "haiku", "gofi-pd": "claude-opus-5-5", "gofi-old": ""} {
		if got := modelLine(t, root, skill); got != want {
			t.Errorf("%s: model = %q, want %q", skill, got, want)
		}
	}
	// What was just installed is what the next plan computes: no churn.
	plan, err := PlanSkillsUpdate(tieredSkills(), ".", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 0 {
		t.Errorf("a fresh install plans changes: %+v", plan)
	}
	// A new tier mapping is a change the update plans.
	withSkillModels(t, host.ClaudeCode, map[string]string{"deep": "fable"})
	if plan, _ := PlanSkillsUpdate(tieredSkills(), ".", root); len(plan) != 1 {
		t.Errorf("remapping a tier should plan one change, got %+v", plan)
	}
}

// The open standard has no model field, and an unknown key there is an
// error: other hosts get none.
func TestOtherHostsGetNoModel(t *testing.T) {
	withSkillModels(t, host.Codex, map[string]string{"light": "gpt-mini"})
	root := t.TempDir()
	if _, err := InstallSkillsContent(tieredSkills(), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	for _, skill := range []string{"gofi-status", "gofi-pd"} {
		if got := modelLine(t, root, skill); got != "" {
			t.Errorf("%s got model %q on Codex", skill, got)
		}
	}
}

// A contract that does not parse is a broken release, not a skill without a
// model.
func TestBrokenContractStopsTheInstall(t *testing.T) {
	withSkillModels(t, host.ClaudeCode, nil)
	fsys := tieredSkills()
	fsys["ai/skills/gofi-pd/contract.yaml"] = &fstest.MapFile{Data: []byte("tier: huge\n")}
	if _, err := InstallSkillsContent(fsys, ".", t.TempDir(), InstallNew); err == nil || !strings.Contains(err.Error(), "gofi-pd") {
		t.Fatalf("expected an error naming the skill, got %v", err)
	}
}

// Claude Code rejects a SKILL.md with a frontmatter key it does not know, so
// every skill gofi ships, installed for it, carries only the keys it accepts.
func TestShippedSkillsCarryOnlyKnownKeys(t *testing.T) {
	withSkillModels(t, host.ClaudeCode, nil)
	root := t.TempDir()
	if _, err := InstallSkillsContent(os.DirFS("../../.."), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"name": true, "description": true, "model": true}
	entries, _ := os.ReadDir(filepath.Join(root, ".claude/skills"))
	if len(entries) == 0 {
		t.Fatal("no skill installed")
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(root, ".claude/skills", e.Name(), "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(frontmatterOf(t, b), "\n") {
			k, _, ok := strings.Cut(l, ":")
			if ok && !strings.HasPrefix(l, " ") && !allowed[k] {
				t.Errorf("%s: frontmatter key %q would be rejected", e.Name(), k)
			}
		}
		if modelLine(t, root, e.Name()) == "" {
			t.Errorf("%s: no model — its contract.yaml is missing", e.Name())
		}
	}
}

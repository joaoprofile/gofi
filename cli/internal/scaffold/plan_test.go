package scaffold

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestPlanAgentsUpdate_NewProject(t *testing.T) {
	dir := t.TempDir()
	plan, err := PlanAgentsUpdate(fixtureFS(), ".", dir, sampleData())
	if err != nil {
		t.Fatalf("PlanAgentsUpdate: %v", err)
	}
	for _, c := range plan {
		if c.Kind != ChangeNew {
			t.Errorf("expected all entries to be 'new' on empty project, got %s for %s", c.Kind, c.RelPath)
		}
	}
	wantPaths := []string{
		".claude/CLAUDE.md",
		".claude/skills/gofi-pd.md",
		".claude/skills/gofi-spec.md",
		".claude/skills/gofi-eng.md",
		".claude/skills/gofi-qa.md",
		".claude/templates/sdd-template.md",
		".claude/templates/prd-template.md",
	}
	got := planPaths(plan)
	sort.Strings(wantPaths)
	for _, w := range wantPaths {
		if !contains(got, w) {
			t.Errorf("expected plan to include %s, got %v", w, got)
		}
	}
}

func TestPlanAgentsUpdate_OmitsUnchanged(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallAgentsContent(fixtureFS(), ".", dir, sampleData(), InstallNew); err != nil {
		t.Fatalf("seed: %v", err)
	}

	plan, err := PlanAgentsUpdate(fixtureFS(), ".", dir, sampleData())
	if err != nil {
		t.Fatalf("PlanAgentsUpdate: %v", err)
	}
	if len(plan) != 0 {
		t.Errorf("expected empty plan when project matches source, got: %v", planPaths(plan))
	}
}

func TestPlanAgentsUpdate_DetectsModifications(t *testing.T) {
	dir := t.TempDir()
	if _, err := InstallAgentsContent(fixtureFS(), ".", dir, sampleData(), InstallNew); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tampered := filepath.Join(dir, ".claude/skills/gofi-eng.md")
	if err := os.WriteFile(tampered, []byte("# locally edited"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	missing := filepath.Join(dir, ".claude/CLAUDE.md")
	if err := os.Remove(missing); err != nil {
		t.Fatalf("remove: %v", err)
	}

	plan, err := PlanAgentsUpdate(fixtureFS(), ".", dir, sampleData())
	if err != nil {
		t.Fatalf("PlanAgentsUpdate: %v", err)
	}

	kindByPath := map[string]ChangeKind{}
	for _, c := range plan {
		kindByPath[c.RelPath] = c.Kind
	}
	if k := kindByPath[".claude/CLAUDE.md"]; k != ChangeNew {
		t.Errorf("CLAUDE.md should be 'new' after deletion, got %q", k)
	}
	if k := kindByPath[".claude/skills/gofi-eng.md"]; k != ChangeModified {
		t.Errorf("gofi-eng.md should be 'modified' after local edit, got %q", k)
	}
	for _, untouched := range []string{
		".claude/skills/gofi-pd.md",
		".claude/templates/sdd-template.md",
	} {
		if _, found := kindByPath[untouched]; found {
			t.Errorf("%s should not appear in plan (unchanged)", untouched)
		}
	}
}

func planPaths(p []Change) []string {
	out := make([]string, len(p))
	for i, c := range p {
		out[i] = c.RelPath
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

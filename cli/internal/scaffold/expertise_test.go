package scaffold

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func expertiseSource(body string) fstest.MapFS {
	return fstest.MapFS{
		"ai/expertise/messaging-kafka/PACK.md":      {Data: []byte("---\npack: messaging-kafka\n---\n# Kafka\n")},
		"ai/expertise/messaging-kafka/consumers.md": {Data: []byte(body)},
	}
}

// Packs are gofi's: installed, refreshed, and a team's edit kept unless forced.
func TestExpertiseInstallsUpdatesAndKeepsEdits(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallExpertiseContent(expertiseSource("# Consumers v1\n"), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, ".claude", "expertise", "messaging-kafka", "consumers.md")
	mustContain(t, file, "v1")

	plan, err := PlanExpertiseUpdate(expertiseSource("# Consumers v2\n"), ".", root)
	if err != nil || len(plan) != 1 || plan[0].Kind != ChangeModified {
		t.Fatalf("plan = %+v (%v)", plan, err)
	}
	if _, err := InstallExpertiseContent(expertiseSource("# Consumers v2\n"), ".", root, InstallUpdate); err != nil {
		t.Fatal(err)
	}
	mustContain(t, file, "v2")

	if err := os.WriteFile(file, []byte("# nossa versão\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if plan, _ := PlanExpertiseUpdate(expertiseSource("# Consumers v3\n"), ".", root); len(plan) != 1 || plan[0].Kind != ChangeKept {
		t.Errorf("edited file not planned as kept: %+v", plan)
	}
	_, _ = InstallExpertiseContent(expertiseSource("# Consumers v3\n"), ".", root, InstallUpdate)
	mustContain(t, file, "nossa versão")
	_, _ = InstallExpertiseContent(expertiseSource("# Consumers v3\n"), ".", root, InstallReset)
	mustContain(t, file, "v3")
	backups, _ := filepath.Glob(filepath.Join(root, ".gofi", "backup", "*", ".claude", "expertise", "messaging-kafka", "consumers.md"))
	if len(backups) == 0 {
		t.Error("--force replaced the team's edit without a backup")
	}
	if b, _ := os.ReadFile(backups[0]); !strings.Contains(string(b), "nossa versão") {
		t.Errorf("backup = %s", b)
	}
}

// Copies older versions seeded into knowledge/ go once the pack carries them —
// but only the untouched ones: an edited copy, or one gofi has no record of,
// is the team's.
func TestRetireSeededKnowledge(t *testing.T) {
	root := t.TempDir()
	put := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put(".claude/knowledge/shared/ddd-principles.md", "seeded\n")
	put(".claude/knowledge/shared/clean-code.md", "edited\n")
	put(".claude/knowledge/shared/id-types.md", "unrecorded\n")
	put(".claude/knowledge/shared/diagram-conventions.md", "seeded\n")
	put(".claude/knowledge/shared/glossario.md", "the team's\n")
	m := Manifest{
		".claude/knowledge/shared/ddd-principles.md":      hashBytes([]byte("seeded\n")),
		".claude/knowledge/shared/clean-code.md":          hashBytes([]byte("seeded\n")),
		".claude/knowledge/shared/diagram-conventions.md": hashBytes([]byte("seeded\n")),
	}
	if err := m.merge(root); err != nil {
		t.Fatal(err)
	}
	put(".claude/expertise/ddd-architecture/ddd-principles.md", "pack\n")
	put(".claude/expertise/ddd-architecture/clean-code.md", "pack\n")
	put(".claude/expertise/ddd-architecture/id-types.md", "pack\n")
	// diagrams/conventions.md is not installed: its copy must stay for now.

	retire, kept := PlanSeededKnowledge(root)
	if want := []string{".claude/knowledge/shared/ddd-principles.md", ".claude/knowledge/shared/diagram-conventions.md"}; !slices.Equal(retire, want) {
		t.Errorf("retire = %v, want %v", retire, want)
	}
	if want := []string{".claude/knowledge/shared/clean-code.md", ".claude/knowledge/shared/id-types.md"}; !slices.Equal(kept, want) {
		t.Errorf("kept = %v, want %v", kept, want)
	}

	removed, err := RetireSeededKnowledge(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/knowledge/shared/ddd-principles.md"}; !slices.Equal(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
	for _, still := range []string{"clean-code.md", "id-types.md", "diagram-conventions.md", "glossario.md"} {
		if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/shared", still)); err != nil {
			t.Errorf("%s must stay: %v", still, err)
		}
	}
	backups, _ := filepath.Glob(filepath.Join(root, ".gofi/backup/*/.claude/knowledge/shared/ddd-principles.md"))
	if len(backups) != 1 {
		t.Errorf("the removed copy is not backed up: %v", backups)
	}
}

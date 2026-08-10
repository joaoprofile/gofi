package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// upstreamAfterInit is the tarball a project meets on its next `gofi update`:
// every managed area moved upstream, not only the skills.
func upstreamAfterInit() fstest.MapFS {
	return fstest.MapFS{
		"ai/skills/gofi-pd.md":                   {Data: []byte("new pd skill")},
		"ai/claude/CLAUDE.md":                    {Data: []byte("# CLAUDE — updated")},
		"ai/templates/sdd-template.md":           {Data: []byte("new spec template")},
		"ai/knowledge/shared/memory-protocol.md": {Data: []byte("# memory protocol v2")},
		"ai/knowledge/shared/brand-new.md":       {Data: []byte("added upstream")},
	}
}

// The whole point of the skills-only update: once a project exists, everything
// but .claude/skills/ is configured by hand, so an update must not carry a
// single upstream byte into CLAUDE.md, templates/ or knowledge/ — not even a
// file the project never received.
func TestInstallSkillsContent_WritesNothingOutsideSkills(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())

	if _, err := InstallSkillsContent(upstreamAfterInit(), ".", root, InstallUpdate); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}

	mustContain(t, filepath.Join(root, ".claude/skills/gofi-pd/SKILL.md"), "new pd skill")
	mustContain(t, filepath.Join(root, ".claude/CLAUDE.md"), "# CLAUDE")
	mustContain(t, filepath.Join(root, ".claude/templates/sdd-template.md"), "# SDD")
	mustContain(t, filepath.Join(root, ".claude/knowledge/shared/memory-protocol.md"), "# memory protocol")
	if b, err := os.ReadFile(filepath.Join(root, ".claude/CLAUDE.md")); err == nil && strings.Contains(string(b), "updated") {
		t.Error("CLAUDE.md was refreshed; the update must leave it alone")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/shared/brand-new.md")); err == nil {
		t.Error("a knowledge file the project never had still arrived; the update must not seed it")
	}
	// The skills the new tarball no longer ships are the project's to keep.
	mustExist(t, root, ".claude/skills/gofi-eng/SKILL.md")
}

// A skill the team adapted survives, exactly as before — the narrower scope
// must not cost the guarantee that made updates safe in the first place.
func TestInstallSkillsContent_KeepsEditedSkill(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())

	edited := filepath.Join(root, ".claude/skills/gofi-pd/SKILL.md")
	if err := os.WriteFile(edited, []byte("our own pd skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallSkillsContent(upstreamAfterInit(), ".", root, InstallUpdate); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}
	mustContain(t, edited, "our own pd skill")

	// --force is the way back to upstream, and the replaced content is kept.
	if _, err := InstallSkillsContent(upstreamAfterInit(), ".", root, InstallReset); err != nil {
		t.Fatalf("InstallSkillsContent (reset): %v", err)
	}
	mustContain(t, edited, "new pd skill")
}

func TestPlanSkillsUpdate_ListsOnlySkills(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())

	plan, err := PlanSkillsUpdate(upstreamAfterInit(), ".", root)
	if err != nil {
		t.Fatalf("PlanSkillsUpdate: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("plan = %v, want only the changed skill", planPaths(plan))
	}
	if plan[0].RelPath != filepath.Join(".claude", "skills", "gofi-pd", "SKILL.md") {
		t.Errorf("plan[0] = %s, want the gofi-pd skill", plan[0].RelPath)
	}
	if plan[0].Kind != ChangeModified {
		t.Errorf("plan[0].Kind = %s, want %s", plan[0].Kind, ChangeModified)
	}
}

// PreservedFilesIn backs the "kept as-is" list the plan prints, which must
// name only what the update was going to write.
func TestPreservedFilesIn_SkillsOnly(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())

	if err := os.WriteFile(filepath.Join(root, ".claude/skills/gofi-pd/SKILL.md"), []byte("ours"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude/CLAUDE.md"), []byte("ours too"), 0o644); err != nil {
		t.Fatal(err)
	}

	kept := PreservedFilesIn(root, []string{SkillsDir})
	if len(kept) != 1 || kept[0] != ".claude/skills/gofi-pd/SKILL.md" {
		t.Errorf("PreservedFilesIn = %v, want only the edited skill", kept)
	}
}

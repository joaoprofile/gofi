package scaffold

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

func roleTree(engVisibility string) fstest.MapFS {
	return fstest.MapFS{
		"ai/skills/gofi-eng/SKILL.md":                {Data: []byte("# /gofi-eng — Context Engineer\n\nCorpo.\n")},
		"ai/skills/gofi-eng/contract.yaml":           {Data: []byte("tier: standard\nvisibility: " + engVisibility + "\nstage: implement\n")},
		"ai/skills/gofi-eng/reference/rules-core.md": {Data: []byte("regras\n")},
		"ai/skills/gofi-pd/SKILL.md":                 {Data: []byte("# /gofi-pd — Discovery\n\nCorpo.\n")},
		"ai/skills/gofi-pd/contract.yaml":            {Data: []byte("tier: deep\n")},
	}
}

// An internal role installs where the host does not look for skills; a user
// one stays a slash command (0002, D19).
func TestAnInternalRoleIsNotASkill(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallSkillsContent(roleTree("internal"), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		layout.Roles().Path("gofi-eng", "SKILL.md"),
		layout.Roles().Path("gofi-eng", "reference", "rules-core.md"),
		layout.Skills().Path("gofi-pd", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(root, want)); err != nil {
			t.Errorf("%s not installed", want)
		}
	}
	if _, err := os.Stat(layout.Skills().Abs(root, "gofi-eng")); err == nil {
		t.Error("an internal role was installed as a slash command")
	}
}

// Updating a project from before the move moves the role, not copies it:
// what nobody edited leaves skills/, an edited file stays for the audit.
func TestAnUpdateMovesARoleThatBecameInternal(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallSkillsContent(roleTree("user"), ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	edited := layout.Skills().Abs(root, "gofi-eng", "reference", "rules-core.md")
	if err := os.WriteFile(edited, []byte("regras da casa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallSkillsContent(roleTree("internal"), ".", root, InstallUpdate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(layout.Roles().Abs(root, "gofi-eng", "SKILL.md")); err != nil {
		t.Error("the role was not installed in its new place")
	}
	if _, err := os.Stat(layout.Skills().Abs(root, "gofi-eng", "SKILL.md")); err == nil {
		t.Error("the unedited SKILL.md was left as a slash command")
	}
	if b, err := os.ReadFile(edited); err != nil || string(b) != "regras da casa\n" {
		t.Error("the team's edit was lost")
	}
	if _, tracked := LoadManifest(root)[layout.Skills().Path("gofi-eng", "SKILL.md")]; tracked {
		t.Error("the manifest still records the retired file")
	}
}

// A skill gofi stopped shipping leaves the project on update, unless the
// team edited it.
func TestAnUpdateRetiresASkillNoLongerShipped(t *testing.T) {
	root := t.TempDir()
	old := roleTree("user")
	old["ai/skills/gofi-full/SKILL.md"] = &fstest.MapFile{Data: []byte("# /gofi-full — Orchestrator\n\nCorpo.\n")}
	if _, err := InstallSkillsContent(old, ".", root, InstallNew); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallSkillsContent(roleTree("user"), ".", root, InstallUpdate); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(layout.Skills().Abs(root, "gofi-full")); err == nil {
		t.Error("the retired skill is still a slash command")
	}
}

package scaffold

import (
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// folderSkillsFS is the shape ai/skills/ carries: one folder per skill, holding
// SKILL.md and whatever resources it bundles.
func folderSkillsFS() fstest.MapFS {
	return fstest.MapFS{
		"ai/skills/gofi-eng/SKILL.md":           {Data: []byte("# /gofi-eng — Context Engineer")},
		"ai/skills/gofi-eng/references/rbac.md": {Data: []byte("rbac reference")},
		"ai/skills/gofi-qa/SKILL.md":            {Data: []byte("# /gofi-qa — Quality Auditor")},
		"ai/skills/gofi-qa/scripts/.gitkeep":    {Data: []byte("")},
		"ai/skills/notes.txt":                   {Data: []byte("not a skill")},
		"ai/skills/gofi-orphan/README.md":       {Data: []byte("a folder without a manifest is not a skill")},
	}
}

func TestInstallSkillsContent_FolderLayout(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallSkillsContent(folderSkillsFS(), ".", root, InstallNew); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}

	mustExist(t, root,
		".claude/skills/gofi-eng/SKILL.md",
		".claude/skills/gofi-qa/SKILL.md",
	)
	mustContain(t, filepath.Join(root, ".claude/skills/gofi-eng/SKILL.md"), "name: gofi-eng")
	// A skill folder is a bundle: what sits beside SKILL.md travels with it.
	mustContain(t, filepath.Join(root, ".claude/skills/gofi-eng/references/rbac.md"), "rbac reference")
	// .gitkeep is scaffolding for git, not content for the project.
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/gofi-qa/scripts/.gitkeep")); err == nil {
		t.Error("a .gitkeep was installed into the project")
	}
	// Everything else under ai/skills/ is not a skill.
	for _, p := range []string{".claude/skills/notes", ".claude/skills/gofi-orphan"} {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			t.Errorf("%s was installed as a skill", p)
		}
	}
}

// The flat ai/skills/<name>.md is what tarballs pinned before the migration
// ship, and `gofi update` still has to install from those refs.
func TestInstallSkillsContent_FlatLayoutStillReads(t *testing.T) {
	root := t.TempDir()
	flat := fstest.MapFS{"ai/skills/gofi-eng.md": {Data: []byte("# /gofi-eng — Context Engineer")}}
	if _, err := InstallSkillsContent(flat, ".", root, InstallNew); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}
	mustContain(t, filepath.Join(root, ".claude/skills/gofi-eng/SKILL.md"), "name: gofi-eng")
}

// A tree caught mid-migration carries both shapes for the same skill; the
// folder is the one that counts, and the skill must not be installed twice.
func TestListSkillNames_PrefersFolderOverFlatFile(t *testing.T) {
	both := fstest.MapFS{
		"ai/skills/gofi-eng/SKILL.md": {Data: []byte("folder body")},
		"ai/skills/gofi-eng.md":       {Data: []byte("flat body")},
	}
	names, err := listSkillNames(both, ".")
	if err != nil {
		t.Fatalf("listSkillNames: %v", err)
	}
	if len(names) != 1 || names[0] != "gofi-eng" {
		t.Fatalf("listSkillNames = %v, want [gofi-eng]", names)
	}

	root := t.TempDir()
	if _, err := InstallSkillsContent(both, ".", root, InstallNew); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}
	mustContain(t, filepath.Join(root, ".claude/skills/gofi-eng/SKILL.md"), "folder body")
}

func TestPlanSkillsUpdate_ListsBundledResources(t *testing.T) {
	root := t.TempDir()
	plan, err := PlanSkillsUpdate(folderSkillsFS(), ".", root)
	if err != nil {
		t.Fatalf("PlanSkillsUpdate: %v", err)
	}
	got := planPaths(plan)
	for _, want := range []string{
		filepath.Join(".claude", "skills", "gofi-eng", "SKILL.md"),
		filepath.Join(".claude", "skills", "gofi-eng", "references", "rbac.md"),
	} {
		if !contains(got, want) {
			t.Errorf("plan is missing %s, got %v", want, got)
		}
	}
}

// The fixtures above are in-memory, so a green suite says nothing about the
// real ai/ tree — which is exactly where the folder layout can be got wrong by
// hand. This installs the repo's own skills into a throwaway dir and checks
// they arrive invocable.
func TestInstallSkillsContent_RealTree(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	if _, err := os.Stat(filepath.Join(repoRoot, "ai", "skills")); err != nil {
		t.Skip("ai/skills/ is not reachable from here")
	}

	root := t.TempDir()
	if _, err := InstallSkillsContent(os.DirFS(repoRoot), ".", root, InstallNew); err != nil {
		t.Fatalf("InstallSkillsContent: %v", err)
	}
	// The entry commands are slash commands; every other role is internal
	// and runs from .claude/gofi/roles/, out of the host's reach (0002, D19).
	entry := map[string]bool{"gofi-pd": true, "gofi-spec": true, "gofi-status": true, "gofi-run": true}
	for _, agent := range allSkills {
		installed := filepath.Join(root, ".claude", skillRelPath(agent))
		if !entry[agent] {
			if _, err := os.Stat(installed); err == nil {
				t.Errorf("internal role %s is a slash command", agent)
			}
			installed = layout.Roles().Abs(root, agent, "SKILL.md")
		}
		body, err := os.ReadFile(installed)
		if err != nil {
			t.Errorf("skill %s did not install: %v", agent, err)
			continue
		}
		front := frontmatterOf(t, body)
		if !strings.Contains(front, "name: "+agent) {
			t.Errorf("skill %s installed without its name:\n%s", agent, front)
		}
		if !strings.Contains(front, "description:") {
			t.Errorf("skill %s installed without a description:\n%s", agent, front)
		}
	}
}

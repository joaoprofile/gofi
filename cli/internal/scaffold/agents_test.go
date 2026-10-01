package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

var upstream = []byte("# AGENTS — release\n")

func TestAgentsReplacesAnUneditedLegacyFile(t *testing.T) {
	root := t.TempDir()
	put(t, root, LegacyInstructions, "# CLAUDE — as installed\n")
	if err := (Manifest{LegacyInstructions: hashBytes([]byte("# CLAUDE — as installed\n"))}).merge(root); err != nil {
		t.Fatal(err)
	}
	p, err := PlanAgents(root, upstream, false)
	if err != nil {
		t.Fatal(err)
	}
	if !p.RemoveLegacy || p.LegacyEdited || p.Note != "new" {
		t.Fatalf("plan = %+v", p)
	}
	if err := ApplyAgents(root, p); err != nil {
		t.Fatal(err)
	}
	if exists(root, LegacyInstructions) {
		t.Error("the legacy file is still blocking AGENTS.md")
	}
	if b, _ := os.ReadFile(filepath.Join(root, AgentsFile)); string(b) != string(upstream) {
		t.Errorf("AGENTS.md = %q", b)
	}
	// Recorded: the next update knows the file is the release's.
	if again, _ := PlanAgents(root, []byte("# AGENTS — next release\n"), false); again.Note != "updated" {
		t.Errorf("an unedited AGENTS.md was not refreshed: %+v", again)
	}
}

// The team's instructions survive the rename word for word.
func TestAgentsCarriesTheTeamsEdits(t *testing.T) {
	root := t.TempDir()
	put(t, root, LegacyInstructions, "# CLAUDE\n\nnossa regra\n")
	p, err := PlanAgents(root, upstream, false)
	if err != nil {
		t.Fatal(err)
	}
	if !p.LegacyEdited || !strings.HasPrefix(p.Note, "moved") {
		t.Fatalf("plan = %+v", p)
	}
	if err := ApplyAgents(root, p); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, AgentsFile)); !strings.Contains(string(b), "nossa regra") {
		t.Errorf("the team's text was lost: %q", b)
	}
	if exists(root, LegacyInstructions) {
		t.Error("the legacy file was left")
	}
	backups, _ := filepath.Glob(filepath.Join(BackupDir(root), "*", ".claude", "CLAUDE.md"))
	if len(backups) == 0 {
		t.Error("no backup of the removed file")
	}
	// Carried text is the team's: the next update keeps it.
	if again, _ := PlanAgents(root, upstream, false); again.Write != nil {
		t.Errorf("the carried file would be overwritten: %+v", again)
	}
}

func TestAgentsKeepsAnEditedFileUnlessForced(t *testing.T) {
	root := t.TempDir()
	put(t, root, AgentsFile, "# ours\n")
	put(t, root, "CLAUDE.md", "@AGENTS.md\n")
	p, _ := PlanAgents(root, upstream, false)
	if p.Write != nil || !strings.HasPrefix(p.Note, "kept (edited") {
		t.Errorf("plan = %+v", p)
	}
	if len(p.Blocking) != 1 || p.Blocking[0] != "CLAUDE.md" {
		t.Errorf("blocking = %v", p.Blocking)
	}
	if forced, _ := PlanAgents(root, upstream, true); forced.Note != "updated" {
		t.Errorf("force = %+v", forced)
	}
	if same, _ := PlanAgents(root, []byte("# ours\n"), false); !same.Empty() || same.Note != "unchanged" {
		t.Errorf("identical file: %+v", same)
	}
}

var withBlockUpstream = []byte("# AGENTS — release\n\n## Projeto\n\n" + ProjectBegin + "\n<!-- escreva aqui -->\n" + ProjectEnd + "\n")

// The team writes in the block; an update replaces gofi's text and leaves the
// block exactly as it was — and editing the block is not editing the file.
func TestAgentsBlockSurvivesUpdates(t *testing.T) {
	root := t.TempDir()
	p, _ := PlanAgents(root, withBlockUpstream, false)
	if err := ApplyAgents(root, p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, AgentsFile))
	team := strings.Replace(string(b), "<!-- escreva aqui -->", "Rodar os testes com `make test`.", 1)
	put(t, root, AgentsFile, team)

	next := []byte(strings.Replace(string(withBlockUpstream), "release", "next release", 1))
	p, _ = PlanAgents(root, next, false)
	if p.Note != "updated" {
		t.Fatalf("an edit inside the block blocked the update: %+v", p)
	}
	if err := ApplyAgents(root, p); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, AgentsFile))
	if !strings.Contains(string(got), "next release") || !strings.Contains(string(got), "make test") {
		t.Errorf("AGENTS.md = %s", got)
	}
}

// Editing gofi's part is the team's call: kept, or replaced under --force with
// the block still in place.
func TestAgentsEditOutsideTheBlock(t *testing.T) {
	root := t.TempDir()
	p, _ := PlanAgents(root, withBlockUpstream, false)
	_ = ApplyAgents(root, p)
	b, _ := os.ReadFile(filepath.Join(root, AgentsFile))
	edited := strings.Replace(string(b), "release", "nossa versão", 1)
	edited = strings.Replace(edited, "<!-- escreva aqui -->", "bloco do time", 1)
	put(t, root, AgentsFile, edited)

	if p, _ := PlanAgents(root, withBlockUpstream, false); p.Write != nil {
		t.Errorf("an edit to gofi's text was overwritten without --force: %+v", p)
	}
	p, _ = PlanAgents(root, withBlockUpstream, true)
	if !strings.Contains(string(p.Write), "bloco do time") || strings.Contains(string(p.Write), "nossa versão") {
		t.Errorf("--force = %s", p.Write)
	}
}

// An adopted repository's own AGENTS.md becomes the project's block.
func TestAdoptKeepsAnExistingAgentsFile(t *testing.T) {
	got := string(adopt(withBlockUpstream, []byte("# Nosso repo\n\nUse pnpm.\n")))
	if !strings.Contains(got, "AGENTS — release") || !strings.Contains(got, "Use pnpm.") {
		t.Fatalf("adopted = %s", got)
	}
	block, ok := projectBlock([]byte(got))
	if !ok || !strings.Contains(string(block), "Use pnpm.") {
		t.Errorf("the existing text is not inside the block: %s", got)
	}
}

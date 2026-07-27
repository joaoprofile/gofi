package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontmatterOf returns the YAML block at the top of a rendered skill, or ""
// when there is none — which is itself the failure Claude Code punishes.
func frontmatterOf(t *testing.T, rendered []byte) string {
	t.Helper()
	text := string(rendered)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("rendered skill has no frontmatter:\n%s", text)
	}
	end := strings.Index(text[4:], "\n---")
	if end == -1 {
		t.Fatalf("frontmatter is not closed:\n%s", text)
	}
	return text[4 : 4+end]
}

// TestRenderSkill_SynthesisesFrontmatter is the whole point of skill.go:
// Claude Code ignores a SKILL.md without `name` and `description`, silently, so
// the slash command simply does not exist. The gofi skills ship without
// frontmatter, so the installer has to add it.
func TestRenderSkill_SynthesisesFrontmatter(t *testing.T) {
	body := []byte("# /gofi-eng — Context Engineer\n\n## Identidade\n\nVocê é o gofi-eng.\n")

	front := frontmatterOf(t, renderSkill("gofi-eng", body))

	if !strings.Contains(front, "name: gofi-eng") {
		t.Errorf("frontmatter is missing the name:\n%s", front)
	}
	// The role after the dash is the honest one-line summary of the skill.
	if !strings.Contains(front, "Context Engineer") {
		t.Errorf("description should carry the role from the heading:\n%s", front)
	}
	if !strings.Contains(front, "/gofi-eng") {
		t.Errorf("description should name the slash command:\n%s", front)
	}
	// The body must survive intact — this step adds metadata, it does not edit
	// the skill.
	if !strings.Contains(string(renderSkill("gofi-eng", body)), "## Identidade") {
		t.Error("skill body was altered")
	}
}

func TestRenderSkill_KeepsExistingFrontmatter(t *testing.T) {
	body := []byte("---\nname: whatever\ndescription: Curada à mão.\nmodel: opus\n---\n\n# Título\n\nCorpo.\n")

	front := frontmatterOf(t, renderSkill("gofi-qa", body))

	// A curated description beats anything derived here.
	if !strings.Contains(front, "description: Curada à mão.") {
		t.Errorf("hand-written description was replaced:\n%s", front)
	}
	// Unknown keys are none of our business.
	if !strings.Contains(front, "model: opus") {
		t.Errorf("unrelated frontmatter key was dropped:\n%s", front)
	}
	// The folder decides the slash command, so a stale name must be corrected
	// or the two would disagree about what to invoke.
	if !strings.Contains(front, "name: gofi-qa") || strings.Contains(front, "name: whatever") {
		t.Errorf("name was not aligned with the skill folder:\n%s", front)
	}
}

func TestRenderSkill_FillsOnlyTheMissingKey(t *testing.T) {
	body := []byte("---\ndescription: Só a descrição.\n---\n\n# /gofi-pd — Discovery\n")

	front := frontmatterOf(t, renderSkill("gofi-pd", body))

	if !strings.Contains(front, "name: gofi-pd") {
		t.Errorf("missing name was not added:\n%s", front)
	}
	if strings.Count(front, "description:") != 1 {
		t.Errorf("description was duplicated:\n%s", front)
	}
}

// A description containing `:` would break the YAML if emitted bare.
func TestRenderSkill_QuotesRiskyDescriptions(t *testing.T) {
	body := []byte("# /gofi-doc — Documentation: contratos e QA\n")

	front := frontmatterOf(t, renderSkill("gofi-doc", body))

	line := ""
	for _, l := range strings.Split(front, "\n") {
		if strings.HasPrefix(l, "description:") {
			line = l
		}
	}
	if !strings.Contains(line, `"`) {
		t.Errorf("description with a colon must be quoted, got: %s", line)
	}
}

func TestRenderSkill_NoHeadingStillDescribes(t *testing.T) {
	front := frontmatterOf(t, renderSkill("gofi-x", []byte("corpo sem título\n")))

	if !strings.Contains(front, "name: gofi-x") || !strings.Contains(front, "description:") {
		t.Errorf("both keys are required even without a heading:\n%s", front)
	}
}

// TestPruneLegacySkillFile covers the migration: a project installed by an
// older gofi has a flat `.claude/skills/<name>.md` that the engine ignores.
// Leaving it makes the folder list every skill twice.
func TestPruneLegacySkillFile(t *testing.T) {
	claudeDir := filepath.Join(t.TempDir(), ".claude")
	skills := filepath.Join(claudeDir, skillsDirName)
	if err := os.MkdirAll(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(skills, "gofi-eng.md")
	other := filepath.Join(skills, "minha-skill.md")
	for _, f := range []string{legacy, other} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneLegacySkillFile(claudeDir, "gofi-eng"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("legacy flat skill file should have been removed")
	}
	// Only the skill just installed is touched; a hand-written neighbour stays.
	if _, err := os.Stat(other); err != nil {
		t.Error("an unrelated file in skills/ must not be removed")
	}

	// Absent file is not an error — the common case on a fresh install.
	if err := pruneLegacySkillFile(claudeDir, "gofi-eng"); err != nil {
		t.Errorf("pruning an absent file should be a no-op, got %v", err)
	}
}

func TestSkillRelPath(t *testing.T) {
	got := skillRelPath("gofi-status")
	want := filepath.Join("skills", "gofi-status", "SKILL.md")
	if got != want {
		t.Errorf("skillRelPath = %q, want %q", got, want)
	}
}

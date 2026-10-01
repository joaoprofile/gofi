package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/scaffold"
)

func TestUpdateAgentsMovesAnEditedClaudeFile(t *testing.T) {
	root := configuredProject(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "ai", "AGENTS.md"), "# AGENTS — release\n")
	t.Setenv("GOFI_AGENTS_LOCAL_DIR", src)
	writeFile(t, filepath.Join(root, ".claude", "CLAUDE.md"), "# CLAUDE\n\nregra da casa\n")
	writeFile(t, filepath.Join(root, ".gemini", "settings.json"), `{"theme":"dark","context":{"fileName":"GEMINI.md"}}`)
	t.Chdir(root)

	if err := runAgentsUpdate(true, false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, scaffold.AgentsFile))
	if err != nil || !strings.Contains(string(b), "regra da casa") {
		t.Fatalf("the team's instructions did not become AGENTS.md: %q (%v)", b, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "CLAUDE.md")); !os.IsNotExist(err) {
		t.Error(".claude/CLAUDE.md is still hiding AGENTS.md")
	}
	g, _ := os.ReadFile(filepath.Join(root, ".gemini", "settings.json"))
	if !strings.Contains(string(g), `"AGENTS.md"`) || !strings.Contains(string(g), `"GEMINI.md"`) || !strings.Contains(string(g), `"dark"`) {
		t.Errorf("gemini settings = %s", g)
	}

	// Run again: nothing left to move, and the Gemini entry is not doubled.
	if err := runAgentsUpdate(true, false); err != nil {
		t.Fatal(err)
	}
	if g2, _ := os.ReadFile(filepath.Join(root, ".gemini", "settings.json")); string(g2) != string(g) {
		t.Errorf("second run rewrote the gemini settings:\n%s", g2)
	}
}

func TestGeminiLeftAloneWithoutGeminiDir(t *testing.T) {
	root := t.TempDir()
	if changed, err := addGeminiContext(root); err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gemini")); !os.IsNotExist(err) {
		t.Error("created .gemini/ in a project that does not use Gemini")
	}
}

func TestDoctorSeesWhatHidesAgentsFile(t *testing.T) {
	root := t.TempDir()
	if c := checkInstructions(root, host.ClaudeCode); !strings.Contains(c.Detail, "missing") {
		t.Errorf("no AGENTS.md: %+v", c)
	}
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# a\n")
	if c := checkInstructions(root, host.ClaudeCode); c.Detail != "AGENTS.md" {
		t.Errorf("clean project: %+v", c)
	}
	writeFile(t, filepath.Join(root, "CLAUDE.local.md"), "x")
	if c := checkInstructions(root, host.ClaudeCode); !strings.Contains(c.Detail, "CLAUDE.local.md") {
		t.Errorf("CLAUDE.local.md not reported: %+v", c)
	}
	writeFile(t, filepath.Join(root, ".claude", "CLAUDE.md"), "x")
	if c := checkInstructions(root, host.ClaudeCode); !strings.Contains(c.Hint, "gofi update agents") {
		t.Errorf("legacy file not reported: %+v", c)
	}
}

// A project that moves from Claude Code to Codex keeps its content in the
// folder Codex reads, without what only Claude Code used, and gets gofi's MCP
// server where Codex looks for it.
func TestUpdateAgentsMovesTheFolderWhenTheHostChanges(t *testing.T) {
	root := configuredProject(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "ai", "AGENTS.md"), "# AGENTS — release\n")
	t.Setenv("GOFI_AGENTS_LOCAL_DIR", src)
	writeFile(t, filepath.Join(root, ".claude", "skills", "gofi-eng", "SKILL.md"), "---\nname: gofi-eng\n---\n")
	writeFile(t, filepath.Join(root, ".claude", "knowledge", "eng", "nosso.md"), "# nosso\n")
	writeFile(t, filepath.Join(root, ".claude", "settings.json"), `{"hooks":{}}`)

	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Host = host.Codex.ID
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	if err := runAgentsUpdate(true, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".agents/skills/gofi-eng/SKILL.md", ".agents/knowledge/eng/nosso.md", ".codex/config.toml", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	for _, p := range []string{".claude", ".agents/settings.json"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); !os.IsNotExist(err) {
			t.Errorf("%s should not exist after the move", p)
		}
	}
}

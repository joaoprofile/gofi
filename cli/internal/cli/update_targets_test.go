package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// `gofi skills update` must land the skills and nothing else — it is the same
// install `gofi update` runs, minus the graph.
func TestRunSkillsUpdate_WritesOnlySkills(t *testing.T) {
	root := setupProject(t)

	claudeMD := filepath.Join(root, ".claude", "CLAUDE.md")
	writeFile(t, claudeMD, "ours")
	skill := filepath.Join(root, ".claude", "skills", "gofi-pd", "SKILL.md")
	writeFile(t, skill, "edited by us")

	if err := runSkillsUpdate(true, false); err != nil {
		t.Fatalf("skills update: %v", err)
	}

	if got := readFile(t, claudeMD); got != "ours" {
		t.Errorf("CLAUDE.md was rewritten: %q", got)
	}
	// Without --force an edited skill is the team's.
	if got := readFile(t, skill); got != "edited by us" {
		t.Errorf("an edited skill was overwritten: %q", got)
	}

	if err := runSkillsUpdate(true, true); err != nil {
		t.Fatalf("skills update --force: %v", err)
	}
	if got := readFile(t, skill); got == "edited by us" {
		t.Error("--force should have put the upstream skill back")
	}
	if got := readFile(t, claudeMD); got != "ours" {
		t.Errorf("--force reached outside skills/: %q", got)
	}
}

// `gofi sdk update` owns .claude/sdk/ and the checkout — and stops there.
func TestRunSDKUpdate_WritesOnlyTheSDKLayer(t *testing.T) {
	root := setupProject(t)

	// A doc the project tuned, and — upstream — one it never touched that moved.
	tuned := filepath.Join(root, ".claude", "sdk", "web", "knowledge", "design-tokens.md")
	writeFile(t, tuned, "our own tokens")
	repo := os.Getenv("GOFI_AGENTS_LOCAL_DIR")
	writeFile(t, filepath.Join(repo, "ai/sdk/go/sdk-docs/overview.md"), "sdk overview v2")
	skill := filepath.Join(root, ".claude", "skills", "gofi-pd", "SKILL.md")
	writeFile(t, skill, "edited by us")

	if err := runSDKUpdate(true, false); err != nil {
		t.Fatalf("sdk update: %v", err)
	}

	if got := readFile(t, tuned); got != "our own tokens" {
		t.Errorf("a tuned SDK doc was replaced: %q", got)
	}
	// The untouched neighbour still has to move, or the guard would have frozen
	// the whole tree instead of protecting one file.
	if got := readFile(t, filepath.Join(root, ".claude/sdk/go/sdk-docs/overview.md")); got != "sdk overview v2" {
		t.Errorf("an untouched SDK doc should have been refreshed, got %q", got)
	}
	if got := readFile(t, skill); got != "edited by us" {
		t.Errorf("the sdk update reached into skills/: %q", got)
	}
}

// The legacy dirs are the reason this command inherited the cleanup: they sit
// beside the current tree and the agents may read them as if they were live.
func TestRunSDKUpdate_RemovesTheLegacyLayout(t *testing.T) {
	root := setupProject(t)
	legacy := filepath.Join(root, ".claude", "boilerplates")
	writeFile(t, filepath.Join(legacy, "model.md"), "pre-v2.4 boilerplate")

	if err := runSDKUpdate(true, false); err != nil {
		t.Fatalf("sdk update: %v", err)
	}
	if _, err := os.Stat(legacy); err == nil {
		t.Error(".claude/boilerplates/ survived the sdk update")
	}
}

// `gofi update ds` exists apart from sdk because the two move on their own
// schedules: it must reach the surface docs and stop at the backend tree.
func TestRunDSUpdate_WritesOnlyTheSurfaceDocs(t *testing.T) {
	root := setupProject(t)
	declareWebSurface(t, root)

	tuned := filepath.Join(root, ".claude", "sdk", "web", "knowledge", "design-tokens.md")
	writeFile(t, tuned, "our own tokens")
	repo := os.Getenv("GOFI_AGENTS_LOCAL_DIR")
	writeFile(t, filepath.Join(repo, "ai/sdk/web/knowledge/patterns.md"), "patterns v2")
	writeFile(t, filepath.Join(repo, "ai/sdk/go/sdk-docs/overview.md"), "backend overview v2")

	if err := runDSUpdate(true, false); err != nil {
		t.Fatalf("ds update: %v", err)
	}

	if got := readFile(t, tuned); got != "our own tokens" {
		t.Errorf("a tuned design-system doc was replaced: %q", got)
	}
	if got := readFile(t, filepath.Join(root, ".claude/sdk/web/knowledge/patterns.md")); got != "patterns v2" {
		t.Errorf("an untouched design-system doc should have been refreshed, got %q", got)
	}
	// The backend SDK is the other target's business.
	if got := readFile(t, filepath.Join(root, ".claude/sdk/go/sdk-docs/overview.md")); got == "backend overview v2" {
		t.Error("ds update reached into the backend SDK docs")
	}
}

// And the reverse: sdk must not carry the design system with it.
func TestRunSDKUpdate_LeavesTheDesignSystemAlone(t *testing.T) {
	root := setupProject(t)

	repo := os.Getenv("GOFI_AGENTS_LOCAL_DIR")
	writeFile(t, filepath.Join(repo, "ai/sdk/web/knowledge/patterns.md"), "patterns v2")

	if err := runSDKUpdate(true, false); err != nil {
		t.Fatalf("sdk update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/sdk/web/knowledge/patterns.md")); err == nil {
		t.Error("sdk update installed a design-system doc; that belongs to gofi update ds")
	}
}

// declareWebSurface adds a web front end with a design system to the project's
// config, which is what makes the ds target have anything to do.
func declareWebSurface(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.Frontend = &config.UISurface{Framework: "react", Path: "apps/web", DS: "shadcn"}
	if err := config.Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
}

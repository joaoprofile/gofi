package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func TestRunAgentList_NoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runAgentList(); err == nil {
		t.Error("expected error without .gofi.yaml")
	}
}

func TestRunAgentList(t *testing.T) {
	setupProject(t)
	if err := runAgentList(); err != nil {
		t.Errorf("list: %v", err)
	}
}

func TestRunAgentAdd_Unknown(t *testing.T) {
	setupProject(t)
	if err := runAgentAdd("gofi-foo"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestRunAgentAdd_AlreadyInstalled(t *testing.T) {
	setupProject(t)
	if err := runAgentAdd("gofi-pd"); err != nil {
		t.Fatalf("add (already-installed should be no-op): %v", err)
	}
}

func TestRunAgentAdd_NewAgent(t *testing.T) {
	root := setupProject(t)

	// Pre-condition: remove gofi-qa to test re-adding.
	if err := runAgentRemove("gofi-qa", true); err != nil {
		t.Fatalf("remove pre-step: %v", err)
	}

	if err := runAgentAdd("gofi-qa"); err != nil {
		t.Fatalf("add: %v", err)
	}
	cfg, err := config.Load(config.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cfg.Agents, "gofi-qa") {
		t.Errorf("expected gofi-qa in cfg.Agents, got %v", cfg.Agents)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/gofi-qa.md")); err != nil {
		t.Errorf("expected gofi-qa.md installed: %v", err)
	}
}

func TestRunAgentRemove(t *testing.T) {
	root := setupProject(t)
	if err := runAgentRemove("gofi-eng", true); err != nil {
		t.Fatalf("remove: %v", err)
	}
	cfg, _ := config.Load(config.FileName)
	if slices.Contains(cfg.Agents, "gofi-eng") {
		t.Errorf("expected gofi-eng removed, got %v", cfg.Agents)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/gofi-eng.md")); !os.IsNotExist(err) {
		t.Errorf("expected skill removed")
	}
}

func TestRunAgentRemove_NotInstalled(t *testing.T) {
	setupProject(t)
	if err := runAgentRemove("gofi-eng", true); err != nil {
		t.Fatalf("first remove: %v", err)
	}
	// Second remove should be a no-op, not an error.
	if err := runAgentRemove("gofi-eng", true); err != nil {
		t.Errorf("second remove (should be no-op): %v", err)
	}
}

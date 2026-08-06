package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func oldProjectConfig() *GofiConfig {
	return &GofiConfig{
		Version: CurrentVersion,
		Project: Project{Name: "svc", Root: "/r"},
		Backend: &Backend{Language: LanguageGo, Path: "src"},
		AI:      AI{Host: "claude-vscode", Model: ModelOpus5},
	}
}

func TestBackfillSeedsWhatAnOldCLINeverWrote(t *testing.T) {
	cfg := oldProjectConfig()
	seeded := Backfill(cfg)

	for _, blk := range []string{"test", "hsec", "sonar", "ops", "graph", "ai.models"} {
		if !slices.Contains(seeded, blk) {
			t.Errorf("%s should have been seeded, got %v", blk, seeded)
		}
	}
	if cfg.Test.Default == "" {
		t.Error("the test block should carry the language defaults")
	}
	if cfg.Sonar.ProjectKey == "" {
		t.Error("the sonar block should be keyed by the project name")
	}
	if cfg.Ops == nil {
		t.Error("the ops block should exist")
	}
	// Seeding it must not change what the project already had: the defaults it
	// gets written with are the ones absence already meant.
	if cfg.Graph == nil || !cfg.Graph.On() || !cfg.Graph.HooksOn() || cfg.Graph.UseDeep() {
		t.Errorf("the graph block should be seeded with the defaults absence meant, got %+v", cfg.Graph)
	}
	if len(cfg.AI.Models) != 1 || cfg.AI.Models[0] != ModelOpus5 {
		t.Errorf("ai.models should be seeded with the active model, got %v", cfg.AI.Models)
	}
}

// The whole point of a backfill is that it fills gaps. A team that turned a
// block off must find it off after an update.
func TestBackfillNeverOverwritesAChoice(t *testing.T) {
	cfg := oldProjectConfig()
	cfg.Hsec = HsecConfig{Enabled: false, SeverityThreshold: "LOW", OutputFormat: "text"}
	cfg.Test = TestSection{Default: "meu-alvo", Tasks: map[string]TestTask{"meu-alvo": {Run: "make test"}}}

	seeded := Backfill(cfg)

	if slices.Contains(seeded, "hsec") || slices.Contains(seeded, "test") {
		t.Errorf("blocks already written must be left alone, got %v", seeded)
	}
	if cfg.Hsec.Enabled || cfg.Hsec.SeverityThreshold != "LOW" {
		t.Errorf("hsec was overwritten: %+v", cfg.Hsec)
	}
	if cfg.Test.Default != "meu-alvo" {
		t.Errorf("test was overwritten: %+v", cfg.Test)
	}
}

func TestBackfillOnACurrentConfigChangesNothing(t *testing.T) {
	cfg := oldProjectConfig()
	Backfill(cfg)
	if seeded := Backfill(cfg); len(seeded) != 0 {
		t.Errorf("a second pass should be a no-op, got %v", seeded)
	}
}

// Load migrates in memory, so the parsed struct always reports the current
// version — only the raw file says whether it still has to be rewritten.
func TestFileVersionReadsTheFileNotTheMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte("version: 1\nproject:\n  name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FileVersion(path); got != 1 {
		t.Errorf("FileVersion = %d, want 1", got)
	}
	if got := FileVersion(filepath.Join(dir, "nao-existe.yaml")); got != 0 {
		t.Errorf("a missing file should read as 0, got %d", got)
	}
}

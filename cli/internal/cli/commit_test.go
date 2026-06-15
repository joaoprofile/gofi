package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/gitops"
)

func TestRunCommit_CreatesCommit(t *testing.T) {
	dir := t.TempDir()
	if err := gitops.Init(dir); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := config.Save(filepath.Join(dir, config.FileName), validCommitTestConfig(dir)); err != nil {
		t.Fatalf("save .gofi.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	chdir(t, dir)

	if err := runCommit("chore: first"); err != nil {
		t.Fatalf("runCommit: %v", err)
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("commit obj: %v", err)
	}
	if commit.Message != "chore: first" {
		t.Errorf("expected message %q, got %q", "chore: first", commit.Message)
	}
}

func TestRunCommit_NothingToCommit(t *testing.T) {
	dir := t.TempDir()
	if err := gitops.Init(dir); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := config.Save(filepath.Join(dir, config.FileName), validCommitTestConfig(dir)); err != nil {
		t.Fatalf("save .gofi.yaml: %v", err)
	}
	if err := gitops.AddAndCommit(dir, "chore: initial"); err != nil {
		t.Fatalf("seed commit: %v", err)
	}

	chdir(t, dir)

	if err := runCommit("chore: noop"); err != nil {
		t.Fatalf("runCommit on clean tree should not error, got: %v", err)
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("commit obj: %v", err)
	}
	if commit.Message != "chore: initial" {
		t.Errorf("expected HEAD to remain at seed commit, got message %q", commit.Message)
	}
}

func TestRunCommit_EmptyMessage(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := runCommit("   "); err == nil {
		t.Fatal("expected error for empty message")
	}
}

func validCommitTestConfig(root string) *config.GofiConfig {
	return &config.GofiConfig{
		Version: config.CurrentVersion,
		Project: config.Project{Name: "demo", Root: root},
		Backend: &config.Backend{Language: config.LanguageGo, Path: "src"},
		AI:      config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus47},
		Agents:  []string{config.AgentPD},
		Sources: config.Sources{Agents: "github.com/joaoprofile/gofi-agents@v0.1.0"},
		Test: config.TestSection{
			Default: "unit",
			Tasks:   map[string]config.TestTask{"unit": {Desc: "unit tests", Run: "go test ./..."}},
		},
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

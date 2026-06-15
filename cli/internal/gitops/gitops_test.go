package gitops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("expected .git/: %v", err)
	}
}

func TestAddAndCommit(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := AddAndCommit(dir, "first"); err != nil {
		t.Fatalf("AddAndCommit: %v", err)
	}

	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	head, err := repo.Head()
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("commit obj: %v", err)
	}
	if commit.Message != "first" {
		t.Errorf("expected message 'first', got %q", commit.Message)
	}
}

func TestAddRemote(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := AddRemote(dir, "origin", "git@github.com:acme/foo.git"); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	repo, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	rem, err := repo.Remote("origin")
	if err != nil {
		t.Fatalf("remote: %v", err)
	}
	if rem.Config().URLs[0] != "git@github.com:acme/foo.git" {
		t.Errorf("unexpected URL: %s", rem.Config().URLs[0])
	}
}

func TestAddRemote_DuplicateFails(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := AddRemote(dir, "origin", "url1"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := AddRemote(dir, "origin", "url2"); err == nil {
		t.Fatal("expected duplicate remote to fail")
	}
}

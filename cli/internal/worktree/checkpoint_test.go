package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A run in the project's tree starts only from a clean tree — the change's
// own folder and the derived files aside.
func TestACheckpointSeesWhatTheRunWouldMix(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	c, ok := OpenCheckpoint(ctx, root)
	if !ok || c.Branch != "main" {
		t.Fatalf("checkpoint = %+v, %v", c, ok)
	}
	c.Keep = []string{"specs/pedidos/changes/c"}
	c.Derived = func(p string) bool { return strings.HasPrefix(p, "index/") }
	write(t, filepath.Join(root, "specs/pedidos/changes/c/proposal.md"), "c\n")
	write(t, filepath.Join(root, "index/code.json"), "{}\n")
	if d, err := c.Dirty(ctx); err != nil || len(d) != 0 {
		t.Fatalf("dirty = %v, %v", d, err)
	}
	write(t, filepath.Join(root, "app.txt"), "mine\n")
	write(t, filepath.Join(root, "notes.txt"), "mine\n")
	if d, _ := c.Dirty(ctx); strings.Join(d, " ") != "app.txt notes.txt" {
		t.Errorf("dirty = %v", d)
	}

	if _, ok := OpenCheckpoint(ctx, t.TempDir()); ok {
		t.Error("a folder with no repository has a checkpoint")
	}
}

// Accepted, the change is one commit on the current branch: the run's work
// and its own change archived — another change approved beside it stays
// out, the person's.
func TestAnAcceptedChangeIsOneCommit(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	c, _ := OpenCheckpoint(ctx, root)
	c.Keep = []string{"specs/pedidos/changes"}
	write(t, filepath.Join(root, "app.txt"), "v2\n")
	write(t, filepath.Join(root, "new.txt"), "n\n")
	_ = os.RemoveAll(filepath.Join(root, "specs/pedidos/changes/a"))
	write(t, filepath.Join(root, "specs/pedidos/changes/archive/2026-11-02-a/proposal.md"), "a\n")
	write(t, filepath.Join(root, "specs/pedidos/changes/d/proposal.md"), "d\n")
	if err := c.Commit(ctx, "feat(x): a", "specs/pedidos/changes/a", "specs/pedidos/changes/archive/2026-11-02-a", "specs/pedidos/changes/never-committed"); err != nil {
		t.Fatal(err)
	}
	if got := run(t, root, "log", "-1", "--format=%s"); got != "feat(x): a" {
		t.Errorf("head = %q", got)
	}
	if s := run(t, root, "status", "--porcelain"); s != "?? specs/pedidos/changes/d/" {
		t.Errorf("after the commit the tree holds %q, want only the other change", s)
	}
}

// Not accepted, the work goes to a branch of its own and the tree back to
// the head; the change's folder and gofi's own files stay.
func TestAWorkNotAcceptedIsKeptOnABranchAndTheTreeRestored(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	c, _ := OpenCheckpoint(ctx, root)
	c.Keep = []string{"specs/pedidos/changes/c"}
	write(t, filepath.Join(root, "specs/pedidos/changes/c/proposal.md"), "c\n")
	write(t, filepath.Join(root, "app.txt"), "half\n")
	write(t, filepath.Join(root, "pkg/new.go"), "package pkg\n")
	write(t, filepath.Join(root, ".gofi/runs/r-1/state.json"), "{}\n")
	branch, err := c.Shelve(ctx, "gofi/c", "wip(c): impasse")
	if err != nil || branch != "gofi/c" {
		t.Fatalf("shelve = %q, %v", branch, err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(b) != "v1\n" {
		t.Errorf("app.txt = %q, want the head's", b)
	}
	if _, err := os.Stat(filepath.Join(root, "pkg/new.go")); err == nil {
		t.Error("the run's new file is still in the tree")
	}
	for _, p := range []string{"specs/pedidos/changes/c/proposal.md", ".gofi/runs/r-1/state.json"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("%s was removed", p)
		}
	}
	if got := run(t, root, "show", "--name-only", "--format=", branch); got != "app.txt\npkg/new.go" {
		t.Errorf("the branch holds %q", got)
	}
	if got := run(t, root, "rev-parse", "--abbrev-ref", "HEAD"); got != "main" {
		t.Errorf("the project left main: %s", got)
	}

	write(t, filepath.Join(root, "app.txt"), "again\n")
	if branch, _ := c.Shelve(ctx, "gofi/c", "wip(c): impasse"); branch != "gofi/c-2" {
		t.Errorf("a second shelf = %q, want gofi/c-2", branch)
	}
	if branch, err := c.Shelve(ctx, "gofi/c", "x"); branch != "" || err != nil {
		t.Errorf("nothing to keep = %q, %v", branch, err)
	}
}

// A change run again in a worktree after an impasse gets a branch of its own.
func TestAWorktreeRunAgainGetsAFreeBranch(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	if _, err := Prepare(ctx, root, "r-1", "a"); err != nil {
		t.Fatal(err)
	}
	tr, err := Prepare(ctx, root, "r-2", "a")
	if err != nil || tr.Branch != "gofi/a-2" {
		t.Fatalf("second run = %+v, %v", tr, err)
	}
	again, err := Prepare(ctx, root, "r-2", "a")
	if err != nil || again.Branch != "gofi/a-2" {
		t.Errorf("reopened = %+v, %v", again, err)
	}
}

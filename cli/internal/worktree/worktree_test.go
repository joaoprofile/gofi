package worktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, body string) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// project is a repository on main with a committed change, its SDK checkout
// ignored as gofi init ignores it.
func project(t *testing.T) string {
	root := t.TempDir()
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	run(t, root, "init", "--quiet", "-b", "main")
	write(t, filepath.Join(root, ".gitignore"), ".gofi/*\n")
	write(t, filepath.Join(root, ".gofi.yaml"), "project: {name: loja}\n")
	write(t, filepath.Join(root, "specs/changes/a/proposal.md"), "a\n")
	write(t, filepath.Join(root, "specs/changes/b/proposal.md"), "b\n")
	write(t, filepath.Join(root, "app.txt"), "v1\n")
	write(t, filepath.Join(root, ".gofi/gofi-sdk-go/go.work"), "go 1.26\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "--quiet", "-m", "init")
	return root
}

// A run works in its own tree on its own branch, the SDK linked in; accepted,
// it lands on main by fast-forward and the worktree goes.
func TestARunWorksApartAndLandsByFastForward(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	tr, err := Prepare(ctx, root, "r-1", "a")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Branch != "gofi/a" || tr.Base != "main" {
		t.Fatalf("tree = %+v", tr)
	}
	if _, err := os.Stat(filepath.Join(tr.Dir, ".gofi/gofi-sdk-go/go.work")); err != nil {
		t.Error("the SDK checkout is not linked into the worktree")
	}
	write(t, filepath.Join(tr.Dir, "app.txt"), "v2\n")
	if err := tr.Commit(ctx, "feat(x): a"); err != nil {
		t.Fatal(err)
	}
	if s := run(t, tr.Dir, "show", "--name-only", "--format=", "HEAD"); s != "app.txt" {
		t.Errorf("the commit holds %q: the linked SDK must stay out of it", s)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(b) != "v1\n" {
		t.Fatal("the run touched the project's own tree")
	}
	if err := tr.Land(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(b) != "v2\n" {
		t.Error("main did not get the change")
	}
	if n := run(t, root, "rev-list", "--merges", "--count", "main"); n != "0" {
		t.Error("landing made a merge commit")
	}
	if err := tr.Remove(ctx); err != nil {
		t.Fatal(err)
	}
}

// Two runs at once each land, the second rebased on the first.
func TestTwoRunsLandOneAfterTheOther(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	a, _ := Prepare(ctx, root, "r-1", "a")
	b, _ := Prepare(ctx, root, "r-2", "b")
	write(t, filepath.Join(a.Dir, "a.txt"), "a\n")
	write(t, filepath.Join(b.Dir, "b.txt"), "b\n")
	_ = a.Commit(ctx, "a")
	_ = b.Commit(ctx, "b")
	if err := a.Land(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Land(ctx); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("%s did not land", f)
		}
	}
}

// Landing never touches the person's work: a tree with changes waits, and a
// conflict leaves the branch as it was.
func TestLandingWaitsForACleanTreeAndRefusesAConflict(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	tr, _ := Prepare(ctx, root, "r-1", "a")
	write(t, filepath.Join(tr.Dir, "app.txt"), "run\n")
	_ = tr.Commit(ctx, "a")

	write(t, filepath.Join(root, "app.txt"), "editing\n")
	if err := tr.Land(ctx); !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty tree: %v", err)
	}
	run(t, root, "commit", "--quiet", "-am", "the person's edit")
	before := run(t, tr.Dir, "rev-parse", "HEAD")
	if err := tr.Land(ctx); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	if after := run(t, tr.Dir, "rev-parse", "HEAD"); after != before {
		t.Error("the conflicting branch was changed")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "app.txt")); string(b) != "editing\n" {
		t.Error("the person's work was touched")
	}
}

// What is only on disk is not in a worktree: the change must be committed.
func TestAChangeNotCommittedIsRefused(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	write(t, filepath.Join(root, "specs/changes/c/proposal.md"), "c\n")
	if _, err := Prepare(ctx, root, "r-1", "c"); !errors.Is(err, ErrNotCommitted) {
		t.Errorf("uncommitted change: %v", err)
	}
	write(t, filepath.Join(root, "specs/changes/a/proposal.md"), "a, edited\n")
	if _, err := Prepare(ctx, root, "r-2", "a"); !errors.Is(err, ErrNotCommitted) {
		t.Errorf("edited after commit: %v", err)
	}
}

// A change that depends on another waits until the other's archive is on the
// base branch.
func TestLandedReadsTheArchiveOnTheBase(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	if Landed(ctx, root, "main", "a") {
		t.Fatal("nothing archived yet")
	}
	write(t, filepath.Join(root, "specs/changes/archive/2026-11-02-a/proposal.md"), "a\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "--quiet", "-m", "archive a")
	if !Landed(ctx, root, "main", "a") || Landed(ctx, root, "main", "b") {
		t.Error("Landed misread the archive")
	}
}

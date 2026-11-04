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

// A project may version what gofi init ignores — the SDK checkout, the
// installed manifest. The worktree has those from the commit: linking over
// them failed the run with "file exists", and copying over them carried the
// person's uncommitted edits into the run's commit.
func TestAVersionedGofiFileComesFromTheCommit(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	write(t, filepath.Join(root, ".gitignore"), "")
	write(t, filepath.Join(root, ".gofi/installed.yaml"), "version: 1\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "--quiet", "-m", "version the gofi files")
	write(t, filepath.Join(root, ".gofi/installed.yaml"), "version: 2\n")

	tr, err := Prepare(ctx, root, "r-1", "a")
	if err != nil {
		t.Fatal(err)
	}
	sdk := filepath.Join(tr.Dir, ".gofi/gofi-sdk-go")
	if fi, err := os.Lstat(sdk); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("the versioned SDK must be the commit's checkout, not a link: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(tr.Dir, ".gofi/installed.yaml")); string(b) != "version: 1\n" {
		t.Errorf("installed.yaml = %q: the worktree must hold the committed one", b)
	}
	if s := run(t, tr.Dir, "status", "--porcelain"); s != "" {
		t.Errorf("a fresh worktree must be clean, got:\n%s", s)
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

// Two runs both rewrite a derived file — the graph: the second lands
// rebased on the first, the file written again from both changes, never a
// conflict for the person. A conflict in anything else still is one.
func TestADerivedFileIsRegeneratedNotAConflict(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	write(t, filepath.Join(root, "graph.txt"), "app.txt\n")
	write(t, filepath.Join(root, "specs/changes/c/proposal.md"), "c\n")
	write(t, filepath.Join(root, "specs/changes/d/proposal.md"), "d\n")
	run(t, root, "add", "-A")
	run(t, root, "commit", "--quiet", "-m", "graph")
	// regenerate lists the .txt files of the tree, as an index lists the code.
	regenerate := func(_ context.Context, dir string) error {
		names, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
		var b strings.Builder
		for _, n := range names {
			if base := filepath.Base(n); base != "graph.txt" {
				b.WriteString(base + "\n")
			}
		}
		return os.WriteFile(filepath.Join(dir, "graph.txt"), []byte(b.String()), 0o644)
	}
	derived := func(p string) bool { return p == "graph.txt" }
	tree := func(run, change, file string) *Tree {
		tr, err := Prepare(ctx, root, run, change)
		if err != nil {
			t.Fatal(err)
		}
		tr.Derived, tr.Regenerate = derived, regenerate
		write(t, filepath.Join(tr.Dir, file), "x\n")
		_ = regenerate(ctx, tr.Dir)
		_ = tr.Commit(ctx, change)
		return tr
	}
	a, b := tree("r-1", "a", "a.txt"), tree("r-2", "b", "b.txt")
	if err := a.Land(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Land(ctx); err != nil {
		t.Fatalf("b did not land over a derived file: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "graph.txt"))
	if string(got) != "a.txt\napp.txt\nb.txt\n" {
		t.Errorf("graph.txt on main = %q: not regenerated from both changes", got)
	}

	// The same file conflicting outside the derived set is still a conflict.
	c := tree("r-3", "c", "c.txt")
	d, _ := Prepare(ctx, root, "r-4", "d")
	d.Derived, d.Regenerate = derived, regenerate
	write(t, filepath.Join(c.Dir, "app.txt"), "c\n")
	_ = c.Commit(ctx, "c2")
	write(t, filepath.Join(d.Dir, "app.txt"), "d\n")
	_ = d.Commit(ctx, "d")
	if err := c.Land(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Land(ctx); !errors.Is(err, ErrConflict) {
		t.Errorf("a real conflict: %v", err)
	}
}

// A derived file only one side has — an index the base never wrote — is
// taken from the base too: removed, and written again if it should exist.
func TestADerivedFileTheBaseLacksIsNotAConflict(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	regenerate := func(context.Context, string) error { return nil }
	derived := func(p string) bool { return filepath.Base(p) == "INDEX.md" }
	a, _ := Prepare(ctx, root, "r-1", "a")
	b, _ := Prepare(ctx, root, "r-2", "b")
	for _, tr := range []*Tree{a, b} {
		tr.Derived, tr.Regenerate = derived, regenerate
	}
	// a writes the index and lands; b writes and then deletes it.
	write(t, filepath.Join(a.Dir, "docs/INDEX.md"), "a\n")
	_ = a.Commit(ctx, "a")
	if err := a.Land(ctx); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(b.Dir, "docs/INDEX.md"), "b\n")
	write(t, filepath.Join(b.Dir, "b.txt"), "b\n")
	_ = b.Commit(ctx, "b")
	run(t, root, "rm", "--quiet", "docs/INDEX.md")
	run(t, root, "commit", "--quiet", "-m", "the base drops the index")
	if err := b.Land(ctx); err != nil {
		t.Fatalf("a derived file the base lacks: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.txt")); err != nil {
		t.Error("b did not land")
	}
}

func TestMain(m *testing.M) {
	trees, _ := os.MkdirTemp("", "gofi-worktrees-")
	os.Setenv(EnvDir, trees)
	code := m.Run()
	os.RemoveAll(trees)
	os.Exit(code)
}

// A run's worktree is never under its project: an agent in it would read the
// project's instructions from the folders above and see the person's tree.
func TestAWorktreeLivesOutsideItsProject(t *testing.T) {
	root := t.TempDir()
	dir := Location(root, "r-1")
	if rel, err := filepath.Rel(root, dir); err == nil && !strings.HasPrefix(rel, "..") {
		t.Errorf("worktree %s is under the project %s", dir, root)
	}
	if Location(root, "r-1") != dir || Location(t.TempDir(), "r-1") == dir {
		t.Error("the location is not one per project and run")
	}
}

// A worktree that could not be furnished is undone: no checkout, no branch
// left behind for every attempt to pile up as gofi/<change>-2, -3.
func TestAWorktreeThatCannotBeFurnishedLeavesNothing(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	env := filepath.Join(root, ".env")
	write(t, env, "SECRET=1\n")
	if err := os.Chmod(env, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(env, 0o644) })
	if _, err := os.ReadFile(env); err == nil {
		t.Skip("running with permissions that read any file")
	}

	if _, err := Prepare(ctx, root, "r-1", "a"); err == nil {
		t.Fatal("an unreadable .env must fail the worktree")
	}
	if _, err := os.Stat(Location(root, "r-1")); !os.IsNotExist(err) {
		t.Errorf("the checkout must be removed: %v", err)
	}
	if b := run(t, root, "branch", "--list", "gofi/*"); b != "" {
		t.Errorf("the branch must be removed, got %q", b)
	}
	if trees, _ := Trees(ctx, root); len(trees) != 0 {
		t.Errorf("git must know no worktree of the run, got %d", len(trees))
	}
}

// A run that ends without landing keeps its work on its branch and gives
// its checkout back; one that wrote nothing leaves no branch either.
func TestARetiredRunKeepsItsWorkOnItsBranchOnly(t *testing.T) {
	ctx := context.Background()
	root := project(t)
	worked, err := Prepare(ctx, root, "r-1", "a")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(worked.Dir, "app.txt"), "v2\n")
	idle, err := Prepare(ctx, root, "r-2", "b")
	if err != nil {
		t.Fatal(err)
	}
	trees, err := Trees(ctx, root)
	if err != nil || len(trees) != 2 {
		t.Fatalf("Trees = %d, %v; want the 2 runs", len(trees), err)
	}

	if kept, err := worked.Retire(ctx, "wip(a): impasse"); err != nil || !kept {
		t.Fatalf("Retire(worked) = %v, %v; want the branch kept", kept, err)
	}
	if got := run(t, root, "show", worked.Branch+":app.txt"); got != "v2" {
		t.Errorf("the branch must hold the run's edit, got %q", got)
	}
	if _, err := os.Stat(worked.Dir); !os.IsNotExist(err) {
		t.Errorf("the checkout must go: %v", err)
	}
	if kept, err := idle.Retire(ctx, "wip(b): impasse"); err != nil || kept {
		t.Fatalf("Retire(idle) = %v, %v; want the empty branch removed", kept, err)
	}
	if b := run(t, root, "branch", "--list", idle.Branch); b != "" {
		t.Errorf("an empty branch must be removed, got %q", b)
	}
	if trees, _ := Trees(ctx, root); len(trees) != 0 {
		t.Errorf("no worktree must be left, got %d", len(trees))
	}
}

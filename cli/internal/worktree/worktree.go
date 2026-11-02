// Package worktree gives each run of the loop a git worktree of its own, so
// two changes are implemented at the same time without touching each other
// (0002, D14/D22), and lands an accepted one on the base branch: rebased on
// it, in the order the changes depend on each other, by fast-forward only —
// never a merge commit, never a push.
package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dir is where the worktrees live, under the project root: in .gofi/, which
// git ignores, so a worktree never shows up in the project's own status.
const Dir = ".gofi/worktrees"

// shared are the ignored folders a worktree links to the project's instead of
// copying: read by the build and the agents, written by no run.
var shared = []string{".gofi/gofi-sdk-go", ".gofi/gofi-sdk-ts", ".gofi/cache"}

// copied are the ignored files and folders a worktree starts from a copy of:
// each run writes its own. The code graph is versioned, so the worktree has
// it from the commit; only the parts of the index git ignores are copied.
var copied = []string{".gofi/index/docs", ".gofi/index/manifest.json", ".env"}

// Tree is one run's worktree.
type Tree struct {
	Root   string // the project root, where the base branch is checked out
	Dir    string // the worktree
	Branch string // gofi/<change>
	Base   string // the branch it lands on
}

// ErrNotCommitted is a change the worktree would not see: a worktree starts
// from the commit, not from the files on disk.
var ErrNotCommitted = errors.New("not committed")

// Prepare creates the worktree of a run on a new branch gofi/<change> from the
// base branch's head, or reopens it when the run is resumed. The change, and
// the project's gofi files, must be committed: what is only on disk is not in
// the worktree.
func Prepare(ctx context.Context, root, run, change string) (*Tree, error) {
	base, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("the project has no branch checked out: %w", err)
	}
	t := &Tree{Root: root, Dir: filepath.Join(root, filepath.FromSlash(Dir), run), Branch: "gofi/" + change, Base: base}
	if _, err := os.Stat(t.Dir); err == nil {
		return t, nil
	}
	if err := Ready(ctx, root, change); err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "worktree", "add", "-b", t.Branch, t.Dir, "HEAD"); err != nil {
		return nil, fmt.Errorf("git worktree add %s: %w", t.Branch, err)
	}
	for _, p := range shared {
		src := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(t.Dir, filepath.FromSlash(p))
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.Symlink(src, dst); err != nil {
			return t, err
		}
	}
	for _, p := range copied {
		if err := copyTree(filepath.Join(root, filepath.FromSlash(p)), filepath.Join(t.Dir, filepath.FromSlash(p))); err != nil {
			return t, err
		}
	}
	return t, nil
}

// Ready reports whether a change can run in a worktree: it, and the
// project's configuration, are committed as they are on disk.
func Ready(ctx context.Context, root, change string) error {
	for _, p := range []string{".gofi.yaml", "specs/changes/" + change} {
		if out, _ := git(ctx, root, "ls-files", "--", p); out == "" {
			return fmt.Errorf("%s: %w — commit it before a run in a worktree", p, ErrNotCommitted)
		}
		if _, err := git(ctx, root, "diff", "--quiet", "HEAD", "--", p); err != nil {
			return fmt.Errorf("%s has changes after the last commit: %w", p, ErrNotCommitted)
		}
	}
	return nil
}

// Commit records what the run changed on its branch. Nothing to commit is not
// an error: a change can be accepted with no file touched.
func (t *Tree) Commit(ctx context.Context, message string) error {
	if _, err := git(ctx, t.Dir, "add", "-A"); err != nil {
		return err
	}
	if _, err := git(ctx, t.Dir, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := git(ctx, t.Dir, "commit", "--quiet", "-m", message)
	return err
}

// Why a branch cannot land yet. Each is a wait, not a failure: the caller
// tries again until it can, or gives up.
var (
	ErrDirty = errors.New("the base branch's tree has uncommitted changes")
	ErrMoved = errors.New("the project is no longer on the base branch")
	// ErrRaced is the base moving between the rebase and the fast-forward:
	// another run landed first, and this one rebases again.
	ErrRaced    = errors.New("the base branch moved while landing")
	ErrConflict = errors.New("the change conflicts with the base branch")
)

// Land rebases the branch on the base branch and fast-forwards the base to
// it. It refuses while the project's tree has uncommitted changes to tracked
// files or is on another branch — the person's work is never touched — and a
// rebase that conflicts is undone, the branch left as it was.
func (t *Tree) Land(ctx context.Context) error {
	if head, _ := git(ctx, t.Root, "symbolic-ref", "--quiet", "--short", "HEAD"); head != t.Base {
		return ErrMoved
	}
	if out, _ := git(ctx, t.Root, "status", "--porcelain", "--untracked-files=no"); out != "" {
		return ErrDirty
	}
	if _, err := git(ctx, t.Dir, "rebase", "--quiet", t.Base); err != nil {
		_, _ = git(ctx, t.Dir, "rebase", "--abort")
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	if _, err := git(ctx, t.Root, "merge", "--quiet", "--ff-only", t.Branch); err != nil {
		return fmt.Errorf("%w: %v", ErrRaced, err)
	}
	return nil
}

// Landed reports whether the base branch holds a change already: its
// archive, specs/changes/archive/<date>-<change>/, is on the branch. It is
// what a change that depends on another waits for.
func Landed(ctx context.Context, root, base, change string) bool {
	out, err := git(ctx, root, "ls-tree", "--name-only", base+":specs/changes/archive/")
	if err != nil {
		return false
	}
	for _, name := range strings.Split(out, "\n") {
		if strings.HasSuffix(name, "-"+change) {
			return true
		}
	}
	return false
}

// Remove deletes a landed run's worktree and its branch, which the base now
// holds.
func (t *Tree) Remove(ctx context.Context) error {
	if _, err := git(ctx, t.Root, "worktree", "remove", "--force", t.Dir); err != nil {
		return err
	}
	_, err := git(ctx, t.Root, "branch", "--quiet", "-d", t.Branch)
	return err
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%s", msg)
		}
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// copyTree copies a file or a folder; one that does not exist is nothing to
// copy.
func copyTree(src, dst string) error {
	fi, err := os.Stat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return copyFile(src, dst, fi.Mode())
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(p, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode.Perm())
}

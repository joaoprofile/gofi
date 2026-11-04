// Package worktree gives each run of the loop a git worktree of its own, so
// two changes are implemented at the same time without touching each other
// (0002, D14/D22), and lands an accepted one on the base branch: rebased on
// it, in the order the changes depend on each other, by fast-forward only —
// never a merge commit, never a push.
package worktree

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/contract"
)

// EnvDir names the folder worktrees are made in, over the user's cache.
const EnvDir = "GOFI_WORKTREES"

// Location is where a run's worktree lives: outside the project, in the
// user's cache — gofi/worktrees/<project>-<hash>/<run>. Never under the
// project root: an agent working in a folder inside the project reads the
// project's own instructions from the folders above it and, seeing the
// person's tree, writes there.
func Location(root, run string) string {
	base := os.Getenv(EnvDir)
	if base == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			cache = os.TempDir()
		}
		base = filepath.Join(cache, "gofi", "worktrees")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	sum := sha1.Sum([]byte(abs))
	return filepath.Join(base, filepath.Base(abs)+"-"+hex.EncodeToString(sum[:])[:10], run)
}

// shared are the ignored folders a worktree links to the project's instead of
// copying: read by the build and the agents, written by no run.
var shared = []string{".gofi/gofi-sdk-go", ".gofi/gofi-sdk-ts", ".gofi/cache"}

// copied are the ignored files and folders a worktree starts from a copy of:
// each run writes its own. The code graph is versioned, so the worktree has
// it from the commit; only the parts of the index git ignores are copied.
var copied = []string{".gofi/index/docs", ".gofi/index/manifest.json", ".gofi/installed.yaml", ".gofi/installed-files.yaml", ".env"}

// Tree is one run's worktree.
type Tree struct {
	Root   string // the project root, where the base branch is checked out
	Dir    string // the worktree
	Branch string // gofi/<change>
	Base   string // the branch it lands on
	// Derived says a path is generated from the rest of the tree — an index,
	// a graph. Two runs side by side both rewrite such a file; a rebase that
	// conflicts only on derived files takes the base's and lets Regenerate
	// write them again from the code they both changed.
	Derived    func(path string) bool
	Regenerate func(ctx context.Context, dir string) error
}

// ErrNotCommitted is a change the worktree would not see: a worktree starts
// from the commit, not from the files on disk.
var ErrNotCommitted = errors.New("not committed")

// Prepare creates the worktree of a run on a new branch gofi/<change> — or
// gofi/<change>-<n> when that one is taken — from the
// base branch's head, or reopens it when the run is resumed. The change, and
// the project's gofi files, must be committed: what is only on disk is not in
// the worktree.
func Prepare(ctx context.Context, root, run, change string) (*Tree, error) {
	base, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("the project has no branch checked out: %w", err)
	}
	t := &Tree{Root: root, Dir: Location(root, run), Branch: "gofi/" + change, Base: base}
	if _, err := os.Stat(t.Dir); err == nil {
		if b, err := git(ctx, t.Dir, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
			t.Branch = b
		}
		return t, nil
	}
	// A change run again after an impasse: the first run's branch keeps its
	// work, and this one gets a branch of its own.
	t.Branch = FreeBranch(ctx, root, t.Branch)
	if err := Ready(ctx, root, change); err != nil {
		return nil, err
	}
	if _, err := git(ctx, root, "worktree", "add", "-b", t.Branch, t.Dir, "HEAD"); err != nil {
		return nil, fmt.Errorf("git worktree add %s: %w", t.Branch, err)
	}
	return t, t.furnish()
}

// Scratch is a worktree of the project's head on a branch of its own, for
// work that never lands — a benchmark's project: it starts from the head as
// it is committed, with the project's ignored files linked or copied as a
// run's worktree has them.
func Scratch(ctx context.Context, root, name string) (*Tree, error) {
	base, err := git(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("the project has no branch checked out: %w", err)
	}
	t := &Tree{Root: root, Dir: Location(root, name), Branch: FreeBranch(ctx, root, "gofi/"+name), Base: base}
	if _, err := git(ctx, root, "worktree", "add", "-b", t.Branch, t.Dir, "HEAD"); err != nil {
		return nil, fmt.Errorf("git worktree add %s: %w", t.Branch, err)
	}
	return t, t.furnish()
}

// furnish gives a new worktree the project's ignored files: the shared ones
// linked, the rest copied.
func (t *Tree) furnish() error {
	root := t.Root
	for _, p := range shared {
		src := filepath.Join(root, filepath.FromSlash(p))
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(t.Dir, filepath.FromSlash(p))
		_ = os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	for _, p := range copied {
		if err := copyTree(filepath.Join(root, filepath.FromSlash(p)), filepath.Join(t.Dir, filepath.FromSlash(p))); err != nil {
			return err
		}
	}
	return nil
}

// Discard removes a scratch worktree and its branch, landed or not.
func (t *Tree) Discard(ctx context.Context) error {
	if _, err := git(ctx, t.Root, "worktree", "remove", "--force", t.Dir); err != nil {
		return err
	}
	_ = os.Remove(filepath.Dir(t.Dir))
	_, err := git(ctx, t.Root, "branch", "--quiet", "-D", t.Branch)
	return err
}

// Ready reports whether a change can run in a worktree: it, and the
// project's configuration, are committed as they are on disk.
func Ready(ctx context.Context, root, change string) error {
	dir, ok := contract.FindChange(root, change)
	if !ok {
		dir = "specs/changes/" + change
	}
	for _, p := range []string{".gofi.yaml", dir} {
		if out, _ := git(ctx, root, "ls-files", "--", p); out == "" {
			return fmt.Errorf("%s: %w — commit it before a run in a worktree", p, ErrNotCommitted)
		}
		if _, err := git(ctx, root, "diff", "--quiet", "HEAD", "--", p); err != nil {
			return fmt.Errorf("%s has changes after the last commit: %w", p, ErrNotCommitted)
		}
	}
	return nil
}

// Commit records what the run changed on its branch, the derived files
// written again first so the commit holds the index of its own code. Nothing
// to commit is not an error: a change can be accepted with no file touched.
func (t *Tree) Commit(ctx context.Context, message string) error {
	if t.Regenerate != nil {
		if err := t.Regenerate(ctx, t.Dir); err != nil {
			return fmt.Errorf("regenerate the derived files: %w", err)
		}
	}
	if _, err := git(ctx, t.Dir, "add", "-A"); err != nil {
		return err
	}
	if _, err := git(ctx, t.Dir, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := git(ctx, t.Dir, "commit", "--quiet", "-m", message)
	return err
}

// Ahead reports whether the branch already has commits the base does not.
func (t *Tree) Ahead(ctx context.Context) bool {
	n, err := git(ctx, t.Dir, "rev-list", "--count", t.Base+".."+t.Branch)
	return err == nil && n != "0"
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
	if err := t.rebase(ctx); err != nil {
		_, _ = git(ctx, t.Dir, "rebase", "--abort")
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	if _, err := git(ctx, t.Root, "merge", "--quiet", "--ff-only", t.Branch); err != nil {
		return fmt.Errorf("%w: %v", ErrRaced, err)
	}
	return nil
}

// rebase replays the branch on the base. Each commit that stops on
// conflicts in derived files only is taken with the base's version of them,
// and once the branch is replayed they are generated again and committed.
// A conflict in anything else is the person's: an error, the rebase left for
// Land to abort.
func (t *Tree) rebase(ctx context.Context) error {
	_, err := git(ctx, t.Dir, "rebase", "--quiet", t.Base)
	regenerate := false
	for err != nil {
		out, _ := git(ctx, t.Dir, "diff", "--name-only", "--diff-filter=U")
		conflicted := strings.Fields(out)
		if len(conflicted) == 0 || t.Derived == nil || t.Regenerate == nil {
			return err
		}
		for _, p := range conflicted {
			if !t.Derived(p) {
				return fmt.Errorf("%s: %w", p, err)
			}
		}
		// In a rebase, "ours" is the base the commit is replayed on: its
		// version is taken, and a derived file the base does not have is
		// removed. Regenerate writes what should be there at the end.
		for _, p := range conflicted {
			if _, e := git(ctx, t.Dir, "checkout", "--ours", "--", p); e == nil {
				_, e = git(ctx, t.Dir, "add", "--", p)
				if e != nil {
					return e
				}
				continue
			}
			if _, e := git(ctx, t.Dir, "rm", "--quiet", "--force", "--", p); e != nil {
				return e
			}
		}
		regenerate = true
		_, err = gitEnv(ctx, t.Dir, []string{"GIT_EDITOR=true"}, "rebase", "--continue")
	}
	if !regenerate {
		return nil
	}
	return t.Commit(ctx, "chore: regenerate the indexes after rebasing on "+t.Base)
}

// Landed reports whether the base branch holds a change already: its
// archive — changes/archive/<date>-<change>/, in the context's folder or in
// specs/ — is on the branch. It is what a change that depends on another
// waits for.
func Landed(ctx context.Context, root, base, change string) bool {
	out, err := git(ctx, root, "ls-tree", "-r", "-d", "--name-only", base, "--", "specs")
	if err != nil {
		return false
	}
	for _, name := range strings.Split(out, "\n") {
		dir, leaf := path.Split(name)
		if strings.HasSuffix(dir, contract.ChangesDir+"/"+contract.ArchiveDir+"/") && strings.HasSuffix(leaf, "-"+change) {
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
	// The project's folder of worktrees goes with its last one.
	_ = os.Remove(filepath.Dir(t.Dir))
	_, err := git(ctx, t.Root, "branch", "--quiet", "-d", t.Branch)
	return err
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	return gitEnv(ctx, dir, nil, args...)
}

// gitEnv runs git with the repository's hooks off. The project's hooks keep
// the graph current around a person's commits, checkouts and merges; inside
// the loop's own commit, rebase and fast-forward they would rewrite derived
// files mid-operation — a rebase aborted for "local changes", a base left
// dirty after landing. Here gofi regenerates the derived files itself.
func gitEnv(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	args = append([]string{"-c", "core.hooksPath=" + os.DevNull}, args...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
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

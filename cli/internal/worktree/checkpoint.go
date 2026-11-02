package worktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Checkpoint is a run that works in the project's own tree, one change after
// another (0002, D14): it starts from a clean tree, an accepted change
// becomes one commit on the current branch, and a run that is not accepted
// leaves no commit there — its work is kept on a branch of its own and the
// tree goes back to where the run started.
type Checkpoint struct {
	Dir    string // the project root
	Branch string // the branch checked out, where an accepted change is committed
	// Keep are paths, relative to Dir, the run may start with changed and
	// leaves as they are: the open changes, approved and maybe not yet
	// committed — the person's contract, not the run's work.
	Keep       []string
	Derived    func(path string) bool
	Regenerate func(ctx context.Context, dir string) error
}

// OpenCheckpoint is the checkpoint of a project's tree; ok is false when the
// project is not in a git repository with a branch and a commit — there is
// nothing to go back to.
func OpenCheckpoint(ctx context.Context, dir string) (*Checkpoint, bool) {
	branch, err := git(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, false
	}
	if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return nil, false
	}
	return &Checkpoint{Dir: dir, Branch: branch}, true
}

// Dirty lists what is changed in the project's tree — tracked or untracked,
// not ignored — besides the paths kept and the derived files: what a run in
// the tree would mix with its own work.
func (c *Checkpoint) Dirty(ctx context.Context) ([]string, error) {
	tracked, err := git(ctx, c.Dir, "diff", "--name-only", "--relative", "HEAD", "--", ".")
	if err != nil {
		return nil, err
	}
	untracked, err := git(ctx, c.Dir, "ls-files", "--others", "--exclude-standard", "--", ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range append(strings.Fields(tracked), strings.Fields(untracked)...) {
		if c.kept(p) || c.Derived != nil && c.Derived(p) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (c *Checkpoint) kept(p string) bool {
	for _, k := range c.Keep {
		if p == k || strings.HasPrefix(p, strings.TrimSuffix(k, "/")+"/") {
			return true
		}
	}
	return false
}

// Commit records an accepted change on the current branch, the derived
// files written again first: everything the run changed in the project's
// folder but the paths kept, and of those only own — the change's folder
// and its archive. Nothing to commit is not an error.
func (c *Checkpoint) Commit(ctx context.Context, message string, own ...string) error {
	if c.Regenerate != nil {
		if err := c.Regenerate(ctx, c.Dir); err != nil {
			return fmt.Errorf("regenerate the derived files: %w", err)
		}
	}
	if _, err := git(ctx, c.Dir, append([]string{"add", "-A", "--", "."}, c.excludes(false)...)...); err != nil {
		return err
	}
	for _, p := range own {
		// A path neither on disk nor in the index — a change folder archived
		// before it was ever committed — has nothing to add.
		_, onDisk := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(p)))
		if tracked, _ := git(ctx, c.Dir, "ls-files", "--", p); onDisk != nil && tracked == "" {
			continue
		}
		if _, err := git(ctx, c.Dir, "add", "-A", "--", p); err != nil {
			return err
		}
	}
	if _, err := git(ctx, c.Dir, "diff", "--cached", "--quiet"); err == nil {
		return nil
	}
	_, err := git(ctx, c.Dir, "commit", "--quiet", "-m", message)
	return err
}

// Shelve keeps what a run that was not accepted changed on a new branch,
// named after base and free, as one commit on the current head — through
// an index of its own, the person's untouched — and puts the tree back as
// the head has it: the paths kept, and gofi's own folder, stay as they are.
// It returns the branch, empty when the run changed nothing.
func (c *Checkpoint) Shelve(ctx context.Context, base, message string) (string, error) {
	spec := append([]string{"--", "."}, c.excludes(true)...)
	index := c.indexFile(ctx)
	defer os.Remove(index)
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := gitEnv(ctx, c.Dir, env, "read-tree", "HEAD"); err != nil {
		return "", err
	}
	if _, err := gitEnv(ctx, c.Dir, env, append([]string{"add", "-A"}, spec...)...); err != nil {
		return "", err
	}
	tree, err := gitEnv(ctx, c.Dir, env, "write-tree")
	if err != nil {
		return "", err
	}
	if head, _ := git(ctx, c.Dir, "rev-parse", "HEAD^{tree}"); head == tree {
		return "", nil
	}
	commit, err := git(ctx, c.Dir, "commit-tree", tree, "-p", "HEAD", "-m", message)
	if err != nil {
		return "", err
	}
	branch := FreeBranch(ctx, c.Dir, base)
	if _, err := git(ctx, c.Dir, "branch", branch, commit); err != nil {
		return "", err
	}
	// Kept on the branch, the work leaves the tree: tracked files as the
	// head has them — the derived ones under .gofi too —, and exactly the
	// files the branch added removed. Nothing else is touched.
	tracked := append([]string{"--", "."}, c.excludes(false)...)
	if _, err := git(ctx, c.Dir, append([]string{"restore", "--source=HEAD", "--staged", "--worktree"}, tracked...)...); err != nil {
		return branch, err
	}
	added, err := git(ctx, c.Dir, "diff", "--name-only", "--relative", "--diff-filter=A", "HEAD", commit)
	if err != nil {
		return branch, err
	}
	for _, p := range strings.Split(added, "\n") {
		if p == "" {
			continue
		}
		abs := filepath.Join(c.Dir, filepath.FromSlash(p))
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return branch, err
		}
		// The folders the run made, now empty, go with their files.
		for dir := filepath.Dir(abs); dir != c.Dir && strings.HasPrefix(dir, c.Dir); dir = filepath.Dir(dir) {
			if os.Remove(dir) != nil {
				break
			}
		}
	}
	return branch, nil
}

// excludes are the pathspecs Shelve leaves alone: the paths kept and, with
// gofi, gofi's folder — the run's own state, the proposal — which is never
// the work.
func (c *Checkpoint) excludes(gofi bool) []string {
	var out []string
	if gofi {
		out = append(out, ":(exclude).gofi")
	}
	for _, k := range c.Keep {
		out = append(out, ":(exclude)"+k)
	}
	return out
}

// indexFile is a scratch index, in the repository's git folder.
func (c *Checkpoint) indexFile(ctx context.Context) string {
	p, err := git(ctx, c.Dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return ".gofi-shelve-index"
	}
	return p + "/gofi-shelve-index"
}

// FreeBranch is base when no branch has that name, else base-2, base-3…: a
// change run again after an impasse gets a branch of its own, the first
// one's work still where it was kept.
func FreeBranch(ctx context.Context, dir, base string) string {
	name := base
	for n := 2; ; n++ {
		if _, err := git(ctx, dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
}

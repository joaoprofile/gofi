// Package gitops wraps the small subset of git operations the gofi CLI needs
// (init, add, commit, remote) using go-git so the user does not need a `git`
// binary on PATH.
package gitops

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Init runs the equivalent of `git init` at path. Idempotent: when the
// directory already hosts a git repository it returns nil so callers can
// safely run init "in place" inside an existing repo (the documented
// `gofi init` flow when the workspace is the current folder).
func Init(path string) error {
	if _, err := git.PlainInit(path, false); err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			return nil
		}
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

// AddAndCommit stages every change in the worktree and commits with the given
// message. Author/committer is read from the global git config; when missing,
// "gofi" / "gofi@local" is used as a sensible fallback.
func AddAndCommit(path, message string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}
	if err := wt.AddGlob("."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	name, email := authorIdentity()
	_, err = wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{Name: name, Email: email, When: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// HasChanges returns true when the worktree at path has any pending change
// (staged, unstaged or untracked) — i.e. `git status` would not be clean.
func HasChanges(path string) (bool, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return false, fmt.Errorf("open repo: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return !st.IsClean(), nil
}

// AddRemote registers a remote named `name` pointing at url.
func AddRemote(path, name, url string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})
	if err != nil {
		if errors.Is(err, git.ErrRemoteExists) {
			return fmt.Errorf("remote %q already exists", name)
		}
		return fmt.Errorf("create remote: %w", err)
	}
	return nil
}

// RemoveRemote deletes the remote named `name`. Returns an error if the
// remote does not exist.
func RemoveRemote(path, name string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("open repo: %w", err)
	}
	if err := repo.DeleteRemote(name); err != nil {
		return fmt.Errorf("delete remote: %w", err)
	}
	return nil
}

// GetRemote returns the URL of the named remote, or "" if it does not exist.
func GetRemote(path, name string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("open repo: %w", err)
	}
	rem, err := repo.Remote(name)
	if err != nil {
		if errors.Is(err, git.ErrRemoteNotFound) {
			return "", nil
		}
		return "", err
	}
	urls := rem.Config().URLs
	if len(urls) == 0 {
		return "", nil
	}
	return urls[0], nil
}

// authorIdentity reads user.name and user.email from the global git config,
// falling back to "gofi" / "gofi@local" when missing.
func authorIdentity() (string, string) {
	name, email := "gofi", "gofi@local"
	cfg, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		return name, email
	}
	if cfg.User.Name != "" {
		name = cfg.User.Name
	}
	if cfg.User.Email != "" {
		email = cfg.User.Email
	}
	return name, email
}

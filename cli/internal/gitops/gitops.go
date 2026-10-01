// Package gitops wraps the small subset of git operations the gofi CLI needs
// (init, remote) using go-git so the user does not need a `git`
// binary on PATH.
package gitops

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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

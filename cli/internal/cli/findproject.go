package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// ErrNotInProject is returned when no .gofi.yaml is found walking up from cwd.
var ErrNotInProject = errors.New("not inside a gofi project (no .gofi.yaml found in this directory or any ancestor)")

// findProjectRoot walks up from cwd until it finds a directory containing
// .gofi.yaml, returning that directory. Returns ErrNotInProject otherwise.
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, config.FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInProject
		}
		dir = parent
	}
}

// declaredRoot is the workspace folder .gofi.yaml names, which is where the rest
// of gofi — update, doctor, agent — already reads and writes. found is where the
// file was located.
//
// The declared root is stored absolute, so it goes stale the moment the project
// is cloned or copied somewhere else. When it no longer holds a .gofi.yaml it is
// a path from another machine, and the directory the file was actually found in
// is the only one that can be trusted.
func declaredRoot(cfg *config.GofiConfig, found string) string {
	if cfg == nil || cfg.Project.Root == "" {
		return found
	}
	if _, err := os.Stat(filepath.Join(cfg.Project.Root, config.FileName)); err != nil {
		return found
	}
	return cfg.Project.Root
}

// loadProjectConfig finds the project root and loads .gofi.yaml from there.
// Returns the loaded config plus the project root path.
func loadProjectConfig() (*config.GofiConfig, string, error) {
	root, err := findProjectRoot()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		return nil, root, err
	}
	return cfg, root, nil
}

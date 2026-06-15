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

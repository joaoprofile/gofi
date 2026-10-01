package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// templatesDir is the name of the document templates' tree, in the source
// (ai/) and under the project's agents folder.
const templatesDir = "templates"

// walkTemplates invokes fn for every template the source ships, rel relative
// to the templates tree.
func walkTemplates(agentsFS fs.FS, srcRoot string, fn func(rel string, content []byte) error) error {
	dir := path.Join(srcRoot, "ai", templatesDir)
	if !dirExistsInFS(agentsFS, dir) {
		return nil
	}
	return fs.WalkDir(agentsFS, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) == GitkeepName {
			return err
		}
		content, err := fs.ReadFile(agentsFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		return fn(strings.TrimPrefix(p, dir+"/"), content)
	})
}

// InstallTemplatesContent refreshes the PRD and spec templates from the
// source. They are gofi's — the format the document index reads — so an
// update rewrites them, keeps a file the team edited (InstallUpdate) or puts
// upstream back with a backup (InstallReset).
func InstallTemplatesContent(agentsFS fs.FS, srcRoot, projectRoot string, mode InstallMode) (created []string, err error) {
	if err := os.MkdirAll(layout.Templates().Abs(projectRoot), 0o755); err != nil {
		return nil, err
	}
	p := newPreserver(projectRoot, mode)
	defer func() {
		if saveErr := p.save(); saveErr != nil && err == nil {
			err = fmt.Errorf("record installed files: %w", saveErr)
		}
	}()
	err = walkTemplates(agentsFS, srcRoot, func(rel string, content []byte) error {
		target := layout.Templates().Abs(projectRoot, rel)
		written, err := p.write(target, content)
		if written {
			created = append(created, target)
		}
		return err
	})
	return created, err
}

// PlanTemplatesUpdate lists what InstallTemplatesContent would create or
// change, writing nothing.
func PlanTemplatesUpdate(agentsFS fs.FS, srcRoot, projectRoot string) ([]Change, error) {
	var changes []Change
	p := newPreserver(projectRoot, InstallUpdate)
	add := planAdder(projectRoot, p, &changes)
	err := walkTemplates(agentsFS, srcRoot, func(rel string, content []byte) error {
		return add(filepath.Join(templatesDir, filepath.FromSlash(rel)), content)
	})
	return changes, err
}

package layout

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
)

// Where older releases kept the same things. The graphs of a Go project sat at
// the top of legacyGraphDir; any other language had a directory of its own
// under it, and so did a graph built on the side with --lang.
const (
	legacyGraphDir = ".gofi/graph"
	legacyDocsDir  = ".gofi/docs"
)

// legacy graph file names, read here only to recognise a graph directory.
const (
	legacyGraphFile = "gofi_graph.json"
	legacyIndexFile = "gofi_graph_index.json"
)

// Migration reports what Migrate changed, as project-relative paths.
type Migration struct {
	Moved   [][2]string // from, to
	Dropped []string    // derived data with no place in the new layout
}

// Empty reports whether the project was already on the current layout.
func (m Migration) Empty() bool { return len(m.Moved) == 0 && len(m.Dropped) == 0 }

// HasLegacy reports whether a project still has anything in the old layout.
func HasLegacy(root string) bool {
	return exists(filepath.Join(root, legacyGraphDir)) || exists(filepath.Join(root, legacyDocsDir))
}

// Migrate moves a project from the old layout to this one. It is idempotent and
// never overwrites: a target already in place wins, and what it replaces is
// derived data, so the old copy is removed instead of merged.
//
// language is the project's backend language, which decides where the old
// layout kept the project graph.
func Migrate(root, language string) (Migration, error) {
	var m Migration
	legacy := filepath.Join(root, legacyGraphDir)

	if err := m.move(root, legacyGraphDir+"/extractors", ExtractorsDir); err != nil {
		return m, err
	}
	if exists(legacy) {
		from := legacyGraphDir
		if language != "" && language != "go" && isGraphDir(filepath.Join(legacy, language)) {
			from = legacyGraphDir + "/" + language
		}
		if err := m.move(root, from, CodeDir); err != nil {
			return m, err
		}
		if err := m.dropStandalone(root); err != nil {
			return m, err
		}
		if err := m.drop(root, legacyGraphDir); err != nil {
			return m, err
		}
	}
	if err := m.move(root, legacyDocsDir, DocsDir); err != nil {
		return m, err
	}
	return m, m.drop(root, legacyDocsDir)
}

// move renames from to to when from exists and to does not.
func (m *Migration) move(root, from, to string) error {
	src, dst := filepath.Join(root, from), filepath.Join(root, to)
	if !exists(src) || exists(dst) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	m.Moved = append(m.Moved, [2]string{from, to})
	return nil
}

// drop removes what is left of an old directory.
func (m *Migration) drop(root, rel string) error {
	dir := filepath.Join(root, rel)
	if !exists(dir) {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	m.Dropped = append(m.Dropped, rel)
	return nil
}

// dropStandalone removes the graphs that came along with the move but that no
// index lists: the ones built on the side with --lang, each with an index of
// its own. Nothing reads them in the new layout, where such a build is one more
// scope of the project, and the next `gofi index code --lang` recreates it.
func (m *Migration) dropStandalone(root string) error {
	code := filepath.Join(root, CodeDir)
	listed := scopeDirs(filepath.Join(code, legacyIndexFile))
	entries, err := os.ReadDir(code)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || slices.Contains(listed, e.Name()) || !isGraphDir(filepath.Join(code, e.Name())) {
			continue
		}
		if err := m.drop(root, path.Join(CodeDir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// scopeDirs reads the scope directories an index lists. It decodes only the
// field it needs, so the migration does not depend on the graph packages.
func scopeDirs(indexPath string) []string {
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return nil
	}
	var ix struct {
		Scopes []struct {
			Dir string `json:"dir"`
		} `json:"scopes"`
	}
	if json.Unmarshal(b, &ix) != nil {
		return nil
	}
	var dirs []string
	for _, s := range ix.Scopes {
		dirs = append(dirs, s.Dir)
	}
	return dirs
}

func isGraphDir(dir string) bool {
	return exists(filepath.Join(dir, legacyGraphFile)) || exists(filepath.Join(dir, legacyIndexFile))
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return !errors.Is(err, fs.ErrNotExist)
}

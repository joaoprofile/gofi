package contract

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Where a change lives. A change of one context lives in that context's
// folder, beside the living spec it changes — specs/<context>/changes/<id>/
// —, so everything about a context is in one place. A change of several
// contexts has no one owner: it lives in specs/changes/<id>/. Each changes/
// folder keeps the changes applied in its archive/.
const ArchiveDir = "archive"

// ChangeHome is the folder a new change of these contexts is born in,
// relative to the root.
func ChangeHome(contexts []string) string {
	if len(contexts) == 1 {
		return layout.Specs().Path(contexts[0], ChangesDir)
	}
	return layout.Specs().Path(ChangesDir)
}

// NewChangeDir is the folder of a new change, relative to the root.
func NewChangeDir(id string, contexts []string) string {
	return ChangeHome(contexts) + "/" + id
}

// FindChange is the folder of an open change, relative to the root, in
// either home.
func FindChange(root, id string) (string, bool) {
	for _, home := range changeHomes(root) {
		rel := home + "/" + id
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && st.IsDir() {
			return rel, true
		}
	}
	return "", false
}

// ChangeHomes are every changes/ folder of the project: the shared one and
// each context's, relative to the root.
func ChangeHomes(root string) []string { return changeHomes(root) }

func changeHomes(root string) []string {
	homes := []string{layout.Specs().Path(ChangesDir)}
	entries, _ := os.ReadDir(layout.Specs().Abs(root))
	for _, e := range entries {
		if e.IsDir() && e.Name() != ChangesDir {
			home := layout.Specs().Path(e.Name(), ChangesDir)
			if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(home))); err == nil && st.IsDir() {
				homes = append(homes, home)
			}
		}
	}
	return homes
}

// archiveOf is where a change applied from a folder is kept: the archive/
// beside it, as <date>-<id>.
func archiveOf(changeDir, date, id string) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(changeDir)), ArchiveDir, date+"-"+id))
}

// changeOf is the change a document sits in — specs/changes/<id>/… or
// specs/<context>/changes/<id>/… —, or "".
func changeOf(rel string) string {
	prefix := layout.Specs().Path() + "/"
	rest, ok := strings.CutPrefix(rel, prefix)
	if !ok {
		return ""
	}
	segs := strings.Split(rest, "/")
	switch {
	case len(segs) >= 3 && segs[0] == ChangesDir && segs[1] != ArchiveDir:
		return segs[1]
	case len(segs) >= 4 && segs[0] != ChangesDir && segs[1] == ChangesDir && segs[2] != ArchiveDir:
		return segs[2]
	}
	return ""
}

// isArchive is a changes/archive/ folder: history, not contract.
func isArchive(dir string) bool {
	return filepath.Base(dir) == ArchiveDir && filepath.Base(filepath.Dir(dir)) == ChangesDir
}

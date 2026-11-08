package contract

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Where a change lives. Every change lives in a context's folder, beside the
// living spec it changes — specs/<context>/changes/<id>/ —, so everything
// about a context is in one place. One that touches several contexts lives in
// the first one's, its owner, with a deltas/<other>.md for each of the others.
// Each changes/ folder keeps the changes applied in its archive/.
//
// The first change of a context that has no spec yet — its genesis — lives in
// the context's folder itself: specs/<context>/proposal.md, deltas/, design.md,
// with no changes/<id>/ around it, since there is nothing yet to keep it
// apart from. A genesis may carry deltas of other contexts too — a new
// context that adds a requirement to one that exists —: it is still the new
// context's, in its folder, with a deltas/<other>.md beside its own. Its id
// is the context's name, so the folder still says which change a document is
// part of. Archived, the spec takes its place and the
// change goes to specs/<context>/changes/archive/<date>-<context>/, where the
// later changes of the context are kept too.
const ArchiveDir = "archive"

// ProposalFile is the proposal of a change, in the change's folder.
const ProposalFile = "proposal.md"

// NewChangeDir is the folder of a new change that is not a genesis, relative
// to the root: in the changes/ of its first context, its owner.
func NewChangeDir(id string, contexts []string) string {
	return layout.Specs().Path(contexts[0], ChangesDir, id)
}

// GenesisDir is the folder of a context's genesis change, relative to the
// root: the context's own.
func GenesisDir(ctx string) string { return layout.Specs().Path(ctx) }

// IsGenesis reports whether a change folder is a genesis: a context's own
// folder rather than a changes/<id>/.
func IsGenesis(dir string) bool {
	rest, ok := strings.CutPrefix(dir, layout.Specs().Path()+"/")
	return ok && rest != "" && rest != ChangesDir && !strings.Contains(rest, "/")
}

// Genesis reports whether a new change of these contexts is a genesis: the
// first context — the one the change is born for — has no spec of any format
// and no genesis open in it already. The others, if any, are contexts the
// genesis also changes; the order matters, so contexts is as the person gave it.
func Genesis(root string, contexts []string) bool {
	if len(contexts) == 0 {
		return false
	}
	dir := filepath.Join(root, filepath.FromSlash(GenesisDir(contexts[0])))
	if _, err := os.Stat(filepath.Join(dir, ProposalFile)); err == nil {
		return false
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "INDEX.md" && isSpecFile(filepath.Join(dir, e.Name())) {
			return false
		}
	}
	return true
}

// isSpecFile is a living spec of any format: tipo: spec in its frontmatter.
func isSpecFile(p string) bool {
	if filepath.Ext(p) != ".md" {
		return false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	fm, _ := docs.Parse(string(b))
	return value(fm, "tipo") == KindSpec
}

// FindChange is the folder of an open change, relative to the root, in
// any context's changes/ — or, for a genesis, the context's own folder.
func FindChange(root, id string) (string, bool) {
	for _, home := range changeHomes(root) {
		rel := home + "/" + id
		if st, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil && st.IsDir() {
			return rel, true
		}
	}
	if id != ChangesDir && !strings.ContainsAny(id, "/\\") {
		rel := GenesisDir(id)
		if isChangeFile(filepath.Join(root, filepath.FromSlash(rel), ProposalFile)) {
			return rel, true
		}
	}
	return "", false
}

// isChangeFile is a proposal: tipo: mudanca in its frontmatter.
func isChangeFile(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	fm, _ := docs.Parse(string(b))
	return value(fm, "tipo") == KindChange
}

// ChangeHomes are every changes/ folder of the project, one per context that
// has any, relative to the root.
func ChangeHomes(root string) []string { return changeHomes(root) }

func changeHomes(root string) []string {
	var homes []string
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
// beside it, as <date>-<id> — for a genesis, the context's changes/archive/.
func archiveOf(changeDir, date, id string) string {
	if IsGenesis(changeDir) {
		return changeDir + "/" + ChangesDir + "/" + ArchiveDir + "/" + date + "-" + id
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(changeDir)), ArchiveDir, date+"-"+id))
}

// changeOf is the change a document sits in — specs/<context>/changes/<id>/…
// —, or "". A genesis is read from the
// document's kind as well (genesisOf): its folder is also the living spec's.
func changeOf(rel string) string {
	prefix := layout.Specs().Path() + "/"
	rest, ok := strings.CutPrefix(rel, prefix)
	if !ok {
		return ""
	}
	segs := strings.Split(rest, "/")
	if len(segs) >= 4 && segs[0] != ChangesDir && segs[1] == ChangesDir && segs[2] != ArchiveDir {
		return segs[2]
	}
	return ""
}

// genesisOf is the genesis a document of a context's own folder is part of:
// the proposal and the design beside the spec, a delta in its deltas/ — of
// the format that has changes. Anything else there, the spec above all, is
// part of none.
func genesisOf(rel, kind string, format int) string {
	rest, ok := strings.CutPrefix(rel, layout.Specs().Path()+"/")
	if !ok || format < FormatSplit {
		return ""
	}
	segs := strings.Split(rest, "/")
	if segs[0] == ChangesDir {
		return ""
	}
	switch {
	case len(segs) == 2 && kind == KindChange && segs[1] == ProposalFile,
		len(segs) == 2 && kind == KindDesign && segs[1] == "design.md",
		len(segs) == 3 && kind == KindDelta && segs[1] == "deltas":
		return segs[0]
	}
	return ""
}

// isArchive is a changes/archive/ folder: history, not contract.
func isArchive(dir string) bool {
	return filepath.Base(dir) == ArchiveDir && filepath.Base(filepath.Dir(dir)) == ChangesDir
}

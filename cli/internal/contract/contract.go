// Package contract reads the documents a person approves as the contract the
// loop implements (0002, D1–D2, D7; 0003): whether each is ready to be
// approved, which belong to a context or a change, and the order the contexts
// are built in. A document that does not pass here is one the loop would have
// to guess at, so the check runs before any model reads it.
package contract

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The kinds of document the contract is made of (0003, S3). The spec is the
// living truth of a context; a change is a proposal, the deltas it applies to
// the specs and the design of how. Tasks are derived, never approved, and are
// not part of it.
const (
	KindSpec   = "spec"
	KindChange = "mudanca"
	KindDelta  = "delta"
	KindDesign = "design"
)

var contractKinds = map[string]bool{KindSpec: true, KindChange: true, KindDelta: true, KindDesign: true}

// FormatSplit is the corpus format of 0003: living specs and changes. A
// document of an earlier format is checked by the rules it was written under,
// until gofi-migrate converts it.
const FormatSplit = 3

// ChangesDir is the folder changes live in — specs/<context>/changes/ for a
// change of one context, specs/changes/ for one of several (place.go): one
// folder per change, and archive/ for the ones applied.
const ChangesDir = "changes"

// Doc is one document of the contract.
type Doc struct {
	Path string // relative to the project root, slash-separated
	Kind string
	// Format is the corpus format the document declares (formato).
	Format int
	// Context is the context a spec, delta or design belongs to; Contexts is
	// every context the document is about — a proposal names all of its own.
	Context  string
	Contexts []string
	// Change is the change the document is part of, read from its folder —
	// specs/changes/<id>/ — never from the frontmatter, so the two cannot
	// disagree. Empty for a living spec.
	Change string
	Front  docs.Frontmatter
	Body   []string
	// Offset is the file line the body starts at, so a finding can point at
	// the line the person sees in the editor.
	Offset int
}

// Docs lists every document of the contract under specs/, sorted by path. A
// file is one by its frontmatter tipo, whatever its name; an archived change
// is history, not contract, and is left out.
func Docs(root string) ([]Doc, error) {
	dir := layout.Specs().Abs(root)
	var out []Doc
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && p == dir {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			if isArchive(p) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".md" || d.Name() == "INDEX.md" {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, body := docs.Parse(string(raw))
		kind := value(fm, "tipo")
		if !contractKinds[kind] {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		doc := Doc{
			Path: filepath.ToSlash(rel), Kind: kind, Format: formatOf(fm),
			Context: value(fm, "contexto"), Front: fm, Body: body, Offset: bodyLine(string(raw), body),
		}
		doc.Change = changeOf(doc.Path)
		doc.Contexts = fm.List("contextos")
		if doc.Context != "" && !contains(doc.Contexts, doc.Context) {
			doc.Contexts = append([]string{doc.Context}, doc.Contexts...)
		}
		if doc.Context == "" && len(doc.Contexts) > 0 {
			doc.Context = doc.Contexts[0]
		}
		out = append(out, doc)
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// Select narrows the corpus to what the person named: a context takes its
// living spec and every open change that touches it, a change id takes that
// change, a path takes that file. An empty selection is everything. A name
// that matches nothing is returned in missing.
func Select(all []Doc, names []string) (picked []Doc, missing []string) {
	if len(names) == 0 {
		return all, nil
	}
	seen := map[string]bool{}
	for _, n := range names {
		n = path.Clean(filepath.ToSlash(strings.TrimPrefix(n, "./")))
		found := false
		for _, d := range all {
			if d.Path == n || d.Change == n || contains(d.Contexts, n) {
				found = true
				if !seen[d.Path] {
					seen[d.Path] = true
					picked = append(picked, d)
				}
			}
		}
		if !found {
			missing = append(missing, n)
		}
	}
	sort.Slice(picked, func(i, j int) bool { return picked[i].Path < picked[j].Path })
	return picked, missing
}

// Living is the living spec of a context: the format-3 spec outside any
// change.
func Living(all []Doc, ctx string) (Doc, bool) {
	for _, d := range all {
		if d.Kind == KindSpec && d.Change == "" && d.Context == ctx && d.Format >= FormatSplit {
			return d, true
		}
	}
	return Doc{}, false
}

// FormatTwo reports whether a context still has specs of the format before
// the split (sdd-*.md) and no living spec: it is migrated (gofi spec migrate)
// before any change builds on it, or the change would start a spec of its own
// beside the old ones.
func FormatTwo(all []Doc, ctx string) bool {
	if _, ok := Living(all, ctx); ok {
		return false
	}
	for _, d := range all {
		if d.Kind == KindSpec && d.Change == "" && d.Context == ctx && d.Format < FormatSplit {
			return true
		}
	}
	return false
}

// bodyLine is the 1-based file line the body starts at, counted the way
// docs.Parse splits the file.
func bodyLine(raw string, body []string) int {
	total := strings.Count(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") + 1
	return total - len(body) + 1
}

// value reads a scalar field without the comment the template writes after
// some of them (status: aprovado  # ou proposto).
func value(fm docs.Frontmatter, key string) string {
	v := fm.Get(key)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"'`)
}

// formatOf is the corpus format a document declares. One that declares none,
// or something that is not a number, is read as the format before the split:
// the rules of an older document are the safe ones to apply to it.
func formatOf(fm docs.Frontmatter) int {
	n, err := strconv.Atoi(value(fm, "formato"))
	if err != nil {
		return FormatSplit - 1
	}
	return n
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

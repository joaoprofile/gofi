// Package contract reads the specs a person approves as the contract the loop
// implements (0002, D1–D2, D7): whether a spec is ready to be approved, and
// which specs belong to a context. A spec that does not pass here is a spec
// the loop would have to guess at, so the check runs before any model reads it.
package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Spec is one spec file of the corpus.
type Spec struct {
	Path    string // relative to the project root, slash-separated
	Context string
	Front   docs.Frontmatter
	Body    []string
	// Offset is the file line the body starts at, so a finding can point at
	// the line the person sees in the editor.
	Offset int
}

// Specs lists every spec in the corpus, sorted by path. A file whose
// frontmatter does not say tipo: spec is not one, whatever its folder.
func Specs(root string) ([]Spec, error) {
	dir := layout.Specs().Abs(root)
	var out []Spec
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == dir {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" || d.Name() == "INDEX.md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fm, body := docs.Parse(string(raw))
		if fm.Get("tipo") != "spec" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, Spec{
			Path: filepath.ToSlash(rel), Context: fm.Get("contexto"),
			Front: fm, Body: body, Offset: bodyLine(string(raw), body),
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// Select narrows the corpus to what the person named: a context name takes
// all of its specs, a path takes that file. An empty selection is everything.
// A name that matches nothing is returned in missing.
func Select(all []Spec, names []string) (picked []Spec, missing []string) {
	if len(names) == 0 {
		return all, nil
	}
	seen := map[string]bool{}
	for _, n := range names {
		n = filepath.ToSlash(strings.TrimPrefix(n, "./"))
		found := false
		for _, s := range all {
			if s.Context == n || s.Path == n {
				found = true
				if !seen[s.Path] {
					seen[s.Path] = true
					picked = append(picked, s)
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

package docs

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The controlled vocabulary of a project, one file per axis.
//
// Derived and hand-kept are separate files on purpose: 'docs migrate lexicon'
// rewrites the derived ones, and merging them into a single file would clobber
// the terms a person wrote.
const (
	LexEntities     = "entidades"    // derived from the schema
	LexOperations   = "operacoes"    // derived from shared keywords, then curated
	LexMarketplaces = "marketplaces" // hand-kept, with aliases
	LexSynonyms     = "sinonimos"    // hand-kept, grows whenever a search fails
)

// Lexicon is one axis: a canonical term and the spellings that mean it.
type Lexicon struct {
	// Terms in the order written, canonical form only.
	Terms []string
	// Alias maps every spelling — including each canonical term — to its
	// canonical form.
	Alias map[string]string
}

// Has reports whether a term is in the lexicon, by canonical form.
func (l Lexicon) Has(term string) bool {
	if l.Alias == nil {
		return false
	}
	_, ok := l.Alias[term]
	return ok
}

// ReadLexicon loads one axis from .claude/lexicon/<name>.md.
//
// Two shapes are accepted, because the axes differ in nature: a bullet list for
// a plain set of terms, and a two-column table when a term has other spellings.
// Anything else on the page — headings, prose, HTML comments — is ignored, so
// the file stays a document a person can read and annotate.
func ReadLexicon(root, name string) Lexicon {
	lex := Lexicon{Alias: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(root, LexiconDir, name+".md"))
	if err != nil {
		return lex
	}
	seen := map[string]bool{}
	add := func(canon string, aliases []string) {
		canon = strings.TrimSpace(strings.Trim(canon, "`"))
		if canon == "" || strings.EqualFold(canon, "termo") || strings.HasPrefix(canon, "-") {
			return
		}
		if !seen[canon] {
			seen[canon] = true
			lex.Terms = append(lex.Terms, canon)
		}
		lex.Alias[canon] = canon
		for _, a := range aliases {
			if a = strings.TrimSpace(strings.Trim(a, "`")); a != "" {
				lex.Alias[a] = canon
			}
		}
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "<!--"):
			continue
		case strings.HasPrefix(line, "|"):
			cells := splitRow(line)
			if len(cells) == 0 || isRuler(cells) {
				continue
			}
			var aliases []string
			if len(cells) > 1 {
				aliases = strings.Split(cells[1], ",")
			}
			add(cells[0], aliases)
		case strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			add(line[2:], nil)
		}
	}
	return lex
}

func splitRow(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

// isRuler skips the |---|---| separator of a markdown table.
func isRuler(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

// WriteLexicon rewrites a derived axis, keeping whatever preamble the file
// already had so a note left by a person survives regeneration.
func WriteLexicon(root, name, title, note string, terms []string) error {
	sort.Strings(terms)
	var b strings.Builder
	b.WriteString("# " + title + "\n\n")
	if note != "" {
		b.WriteString("> " + note + "\n\n")
	}
	for _, t := range terms {
		b.WriteString("- " + t + "\n")
	}
	dir := filepath.Join(root, LexiconDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".md"), []byte(b.String()), 0o644)
}

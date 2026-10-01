package retrieval

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// baseLexicon is the Portuguese ↔ English bridge every project starts with.
//
//go:embed lexicon_base.md
var baseLexicon string

// maxPhrase is the longest phrase, in words, a synonym row may bridge.
const maxPhrase = 3

// synonymWeight is how much a term reached through a synonym counts against
// one the question used: enough to find the document, not enough to outrank
// one that says what was asked.
const synonymWeight = 0.7

// loadSynonyms builds the bridge from the base lexicon and the project's own
// (.claude/lexicon/sinonimos.md). Phrases are keyed by the stems of their
// words, the form a question is matched in, so "máquina de estados" is found
// in "como modelar a máquina de estados" and "tentativas" reaches "retry".
//
// A row links its term to each spelling and back, never two spellings to each
// other: two ways of writing one thing rarely share a question, and linking
// them widens the search with words nobody asked about.
func loadSynonyms(root string) map[string][][]string {
	out := map[string][][]string{}
	link := func(from, to []string) {
		if len(from) == 0 || len(from) > maxPhrase || len(to) == 0 {
			return
		}
		k := strings.Join(from, " ")
		if k == strings.Join(to, " ") {
			return
		}
		out[k] = append(out[k], to)
	}
	read := func(text string) {
		for _, line := range strings.Split(text, "\n") {
			cells := tableCells(line)
			if len(cells) < 2 || strings.EqualFold(cells[0], "termo") || strings.HasPrefix(cells[0], "-") {
				continue
			}
			term := tokens(strings.Trim(cells[0], "`"))
			for _, alias := range strings.Split(cells[1], ",") {
				a := tokens(strings.Trim(strings.TrimSpace(alias), "`"))
				link(term, a)
				link(a, term)
			}
		}
	}
	read(baseLexicon)
	if b, err := os.ReadFile(filepath.Join(root, docs.LexiconDir(), docs.LexSynonyms+".md")); err == nil {
		read(string(b))
	}
	return out
}

// tableCells splits a markdown table row; anything that is not a row, or is
// the ruler under the header, has no cells.
func tableCells(line string) []string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") || strings.Trim(line, "|-: ") == "" {
		return nil
	}
	parts := strings.Split(strings.Trim(line, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// Names reports whether a word of a request names a term: the same stem, or a
// synonym of it in the base or the project lexicon ("pedidos" names order
// where the project's lexicon says so).
func (e *Engine) Names(word, term string) bool {
	w, t := tokens(word), tokens(term)
	if len(w) == 0 || len(t) == 0 {
		return false
	}
	if strings.Join(w, " ") == strings.Join(t, " ") {
		return true
	}
	target := strings.Join(t, " ")
	for _, syn := range e.synonyms[strings.Join(w, " ")] {
		if strings.Join(syn, " ") == target {
			return true
		}
	}
	return false
}

package docs

import (
	"strings"
	"unicode"

	"github.com/gofi-labs/gofi/cli/internal/layout"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// LexiconDir holds the project's controlled vocabulary. It is project data, not
// tool data: the terms belong to the domain, so they stay in the repository.
func LexiconDir() string { return layout.Lexicon().Dir }

var stopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a o e de da do das dos que qual quais como onde
quando quem por para com sem no na nos nas um uma os as em ao aos se ou nao the of
to is are and for esta este essa esse isso sao ser tem mais menos sobre entre pelo
pela`) {
		stopWords[w] = true
	}
}

// normalize lowercases and strips accents, so "precificação" and
// "precificacao" are the same token.
func normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// stem removes plurals only.
//
// Deliberately minimal: aggressive stemming merges distinct senses, while none
// at all means the question "deletes" misses the heading "DELETE".
func stem(t string) string {
	if len(t) <= 4 {
		return t
	}
	switch {
	case strings.HasSuffix(t, "oes"):
		return t[:len(t)-3] + "ao"
	case strings.HasSuffix(t, "ns"):
		return t[:len(t)-2] + "m"
	case strings.HasSuffix(t, "es"):
		return t[:len(t)-2]
	case strings.HasSuffix(t, "s"):
		return t[:len(t)-1]
	}
	return t
}

// The text rules below are shared with the retrieval core, which ranks
// documents and code with one model: the same question must turn into the same
// terms whichever of the two it is matched against.

// Normalize lowercases and strips accents.
func Normalize(s string) string { return normalize(s) }

// Stem folds plurals.
func Stem(t string) string { return stem(t) }

// IsStopWord reports a word too common to rank by.
func IsStopWord(w string) bool { return stopWords[w] }

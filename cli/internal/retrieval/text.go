package retrieval

import (
	"strings"
	"unicode"

	"github.com/gofi-labs/gofi/cli/internal/docs"
)

// tokens turns text into the terms it is ranked by: identifiers split at their
// case and separators ("BillOrder", "bill_order" and "bill order" are the same
// two terms), accents and case folded, words stemmed, stop words and
// one- and two-letter fragments dropped. Repeats are kept: they are the term
// frequency.
func tokens(text string) []string {
	var out []string
	for _, f := range words(text) {
		out = append(out, stem(f))
	}
	return out
}

// surfaces are the words as written, plurals folded, for the ones stemming
// changed. They let the exact form break a tie the stems cannot: "pedido
// cancelado não pode ser faturado" and "pedido faturado não pode ser
// cancelado" share every stem, and only the forms say which one was asked.
// The "=" prefix keeps them apart from the stems in the same index.
func surfaces(text string) []string {
	var out []string
	for _, f := range words(text) {
		if s := docs.Stem(f); s != stem(f) {
			out = append(out, "="+s)
		}
	}
	return out
}

// words splits text into the normalized words worth ranking by.
func words(text string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(docs.Normalize(splitCase(text)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(f) > 2 && !docs.IsStopWord(f) {
			out = append(out, f)
		}
	}
	return out
}

// splitCase puts a space where an identifier changes case — BillOrder →
// Bill Order, HTTPServer → HTTP Server — so a symbol is matched by its words.
func splitCase(s string) string {
	r := []rune(s)
	var b strings.Builder
	for i, c := range r {
		if i > 0 && unicode.IsUpper(c) {
			prev := r[i-1]
			nextLower := i+1 < len(r) && unicode.IsLower(r[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteRune(' ')
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}

// suffixes are stripped longest first, one per word, and only while a stem of
// minStem letters remains. They are Portuguese and English at once: the corpus
// mixes both, and the two often share a root once the ending is gone —
// cancelar, cancelado and cancelled all become "cancel", validação and
// validation both "valid".
var suffixes = []string{
	// nouns made from verbs
	"amentos", "imentos", "amento", "imento", "acoes", "icoes", "acao", "icao",
	"ations", "ation", "ments", "ment",
	// agents and qualities
	"adores", "edores", "idores", "ador", "edor", "idor", "ators", "ator", "avel", "ivel", "able", "ible", "mente",
	// verb forms
	"ando", "endo", "indo", "ados", "adas", "idos", "idas", "ado", "ada", "ido", "ida",
	"ing", "ate", "ed", "ar", "er", "ir",
}

const minStem = 3

// stem folds a word to the root the corpus and the question share. It is
// light on purpose — plurals, then one ending — because a stemmer that keeps
// cutting merges words that only look alike, and a search that ranks by the
// wrong sense is worse than one that misses an inflection.
func stem(t string) string {
	t = docs.Stem(t)
	for _, suf := range suffixes {
		if len(t)-len(suf) >= minStem && strings.HasSuffix(t, suf) {
			t = t[:len(t)-len(suf)]
			break
		}
	}
	// cancell → cancel, logg → log: a doubled final consonant is the English
	// spelling of the same root.
	if n := len(t); n > minStem && t[n-1] == t[n-2] && !strings.ContainsRune("aeiou", rune(t[n-1])) {
		t = t[:n-1]
	}
	return t
}

// pairs are the adjacent stems of a text, stop words already out: "pedido
// cancelado" is a pair where "cancelado … pedido" is not. Matching them is how
// word order enters a ranking that otherwise sees a bag of words — two rules
// with the same words in a different order are different rules. The "~"
// prefix keeps them apart from single terms in the same index.
func pairs(text string) []string {
	ts := tokens(text)
	out := make([]string, 0, len(ts))
	for i := 1; i < len(ts); i++ {
		out = append(out, "~"+ts[i-1]+" "+ts[i])
	}
	return out
}

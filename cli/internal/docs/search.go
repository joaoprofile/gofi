package docs

import (
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// LexiconDir holds the project's controlled vocabulary. It is project data, not
// tool data: the terms belong to the domain, so they stay in the repository.
const LexiconDir = ".claude/lexicon"

var stopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`a o e de da do das dos que qual quais como onde
quando quem por para com sem no na nos nas um uma os as em ao aos se ou nao the of
to is are and for esta este essa esse isso sao ser tem mais menos sobre entre pelo
pela`) {
		stopWords[w] = true
	}
}

// authority orders corpora by how normative they are, lowest first.
func authority(path string) int {
	switch {
	case strings.HasPrefix(path, "specs/"):
		return 0
	case strings.HasPrefix(path, "prd/"):
		return 1
	default:
		return 2 // context memory: a summary of the two above
	}
}

// Result is one search hit: the document, and the section that matched best.
type Result struct {
	facets  int // curated-facet hits; breaks ties before authority
	Doc     Doc
	Section *Section
	Score   int
}

// Searcher answers questions against a built index.
type Searcher struct {
	Root     string
	Index    *Index
	synonyms map[string][]string
}

// NewSearcher loads the synonym bridge for a project.
func NewSearcher(root string, idx *Index) *Searcher {
	return &Searcher{Root: root, Index: idx, synonyms: loadSynonyms(root)}
}

// Search ranks documents for a free-text question.
//
// Section headings are weighted above facets on purpose: they are written in
// natural language, which is the language the question arrives in, while facets
// are labels. A hit on "Retry with exponential backoff" says more than a hit on
// the keyword list.
func (s *Searcher) Search(query string, limit int) []Result {
	terms := s.expand(query)
	if len(terms) == 0 {
		return nil
	}
	var out []Result
	for _, d := range s.Index.Docs {
		blob := tokenSet(d.Context + " " + d.Title + " " + d.Facets + " " + d.Path)
		var best *Section
		bestScore := 0
		for i := range d.Sections {
			n := overlap(terms, tokenSet(d.Sections[i].Heading))
			if n > bestScore {
				bestScore, best = n, &d.Sections[i]
			}
		}
		facets := overlap(terms, blob)
		score := facets + 2*bestScore
		if score > 0 {
			out = append(out, Result{Doc: d, Section: best, Score: score, facets: facets})
		}
	}
	// Ties are common — a two-term question matches many documents equally — so
	// the tiebreak decides real results. Left to the index order it is decided
	// by nothing at all: whichever corpus happened to be walked first wins.
	// Rank the authoritative document first instead: the spec states the rule,
	// the PRD states the intent, context memory only summarizes both.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if a, b := authority(out[i].Doc.Path), authority(out[j].Doc.Path); a != b {
			return a < b
		}
		// Among documents of the same authority, the same score can still have
		// been reached two ways: one carried the question in its curated
		// facets, the other had a heading that happened to share a word. Facets
		// are written to describe the document; a heading word can be
		// incidental. This is what decides between sibling specs of one
		// context, where authority says nothing.
		if out[i].facets != out[j].facets {
			return out[i].facets > out[j].facets
		}
		return out[i].Doc.Path < out[j].Doc.Path
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// expand turns a question into the term set to match, following the synonym
// bridge one step.
func (s *Searcher) expand(query string) map[string]bool {
	terms := tokenSet(query)
	for t := range tokenSet(query) {
		for _, syn := range s.synonyms[t] {
			terms[stem(syn)] = true
		}
	}
	return terms
}

func overlap(a, b map[string]bool) int {
	n := 0
	for k := range a {
		if b[k] {
			n++
		}
	}
	return n
}

// loadSynonyms reads the bridge between the language of the question and the
// language things are named in.
//
// A corpus written in one language that names things in another breaks lexical
// search: the question says "tentativas", the heading says "retry". Without the
// bridge the right document is invisible even when it is the only match. The
// bridge is walked from either side.
func loadSynonyms(root string) map[string][]string {
	lex := ReadLexicon(root, LexSynonyms)
	pairs := map[string]map[string]bool{}
	link := func(a, b string) {
		if a == b {
			return
		}
		if pairs[a] == nil {
			pairs[a] = map[string]bool{}
		}
		pairs[a][b] = true
	}
	for alias, canon := range lex.Alias {
		a, c := normalize(alias), normalize(canon)
		// Only term <-> spelling, never spelling <-> spelling: two ways of
		// writing the same thing rarely share a question, and linking them
		// widens the expansion with terms nobody asked about.
		link(a, c)
		link(c, a)
	}
	out := make(map[string][]string, len(pairs))
	for k, set := range pairs {
		for v := range set {
			out[k] = append(out[k], v)
		}
		sort.Strings(out[k])
	}
	return out
}

// tokenSet splits text into stemmed, accent-free tokens.
func tokenSet(text string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(normalize(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(f) > 2 && !stopWords[f] {
			out[stem(f)] = true
		}
	}
	return out
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

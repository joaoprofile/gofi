package intake

import (
	_ "embed"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

//go:embed lexicon.yaml
var lexiconYAML []byte

// Lexicon is the vocabulary the intake reads a request by.
type Lexicon struct {
	Intents map[string][]string `yaml:"intents"`
	// WeakIntents decide only when no word of Intents is in the request.
	WeakIntents map[string][]string `yaml:"weak_intents"`
	Artifacts   map[string][]string `yaml:"artifacts"`
	Stopwords   []string            `yaml:"stopwords"`
	Vague       []string            `yaml:"vague"`
}

// baseLexicon is the lexicon gofi ships.
func baseLexicon() Lexicon {
	var l Lexicon
	if err := yaml.Unmarshal(lexiconYAML, &l); err != nil {
		panic("intake: embedded lexicon: " + err.Error())
	}
	return l
}

// normalize lowercases and strips accents, so "Sincronização" and
// "sincronizacao" are one word.
func normalize(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return out
}

// words splits normalized text into words, keeping '/' and '-' inside one.
func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '/' || r == '-' || r == '_')
	})
}

// match is a lexicon entry found in a request.
type match struct {
	ID   string // the intent or artifact
	Word string // what was written
	Pos  int    // word position in the request
}

// find returns the earliest entry of a vocabulary in the request's words.
// Phrases match as consecutive words.
func find(vocab map[string][]string, ws []string) (match, bool) {
	best := match{Pos: -1}
	ids := make([]string, 0, len(vocab))
	for id := range vocab {
		ids = append(ids, id)
	}
	sort.Strings(ids) // ties resolve the same way every run
	for _, id := range ids {
		for _, entry := range vocab[id] {
			phrase := words(normalize(entry))
			if len(phrase) == 0 {
				continue
			}
			for i := 0; i+len(phrase) <= len(ws); i++ {
				if equalWords(ws[i:i+len(phrase)], phrase) {
					if best.Pos < 0 || i < best.Pos {
						best = match{ID: id, Word: entry, Pos: i}
					}
					break
				}
			}
		}
	}
	return best, best.Pos >= 0
}

func equalWords(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

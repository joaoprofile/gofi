package intake

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/retrieval"
)

// Context is the context a request is about, and what it already has.
type Context struct {
	Name string `json:"name"`
	// New marks a context the request would create; nothing of it exists.
	New bool `json:"new"`
	// Has lists the artifacts that already exist: prd, spec, code.
	Has []string `json:"has"`
	// Neighbors are the contexts coupled to this one by shared tables or
	// citations — the reach of a change.
	Neighbors []string `json:"neighbors,omitempty"`
	// Why says how the context was chosen.
	Why string `json:"why"`
}

func (c *Context) has(artifact string) bool {
	return c != nil && slices.Contains(c.Has, artifact)
}

// dominance is how far ahead the best context must be of the second for the
// search alone to pick it. Below that, the intake asks. Calibrated by
// testdata/golden-intake.md.
const dominance = 1.5

// candidate is an existing context the search relates to a request.
type candidate struct {
	name  string
	score float64
}

// relatedContexts ranks the existing contexts the request's words reach,
// summing the scores of the hits that belong to each.
func relatedContexts(engine *retrieval.Engine, text string, known map[string]bool) []candidate {
	if engine == nil {
		return nil
	}
	score := map[string]float64{}
	for _, h := range engine.Find(retrieval.Query{Text: text, Areas: projectAreas, Limit: 12}) {
		if h.Context != "" && known[h.Context] {
			score[h.Context] += h.Score
		}
	}
	out := make([]candidate, 0, len(score))
	for n, s := range score {
		out = append(out, candidate{n, s})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].name < out[j].name
	})
	return out
}

// named returns a known context a word of the request names — by stem, or
// through the base and project lexicons ("pedidos" names order when the
// project's lexicon says so) — and the word's position. When headOnly is set,
// only the head of the subject counts: in "crie um PRD de sincronização de
// pedidos" the subject is the synchronisation, and order is only related.
//
// A word that is none of the names can still name a context through the
// keywords of its documents, when only one context declares it: contexts are
// named in English and asked for in the person's language, and "catálogo"
// names catalog when catalog's PRD lists catalogo — no lexicon needed.
func named(engine *retrieval.Engine, ws []string, skip map[int]bool, stop map[string]bool, known map[string]bool, keywords map[string][]string, headOnly bool) (string, int) {
	names := make([]string, 0, len(known))
	for n := range known {
		names = append(names, n)
	}
	sort.Strings(names)
	for i, w := range ws {
		if skip[i] || stop[w] || len(w) < 3 {
			continue
		}
		for _, n := range names {
			if w == n || strings.TrimSuffix(w, "s") == n || (engine != nil && engine.Names(w, n)) {
				return n, i
			}
		}
		for _, k := range []string{w, strings.TrimSuffix(w, "s")} {
			if owners := keywords[k]; len(owners) == 1 {
				return owners[0], i
			}
		}
		if headOnly {
			return "", -1
		}
	}
	return "", -1
}

// contextKeywords maps each keyword of a context's own documents — PRD, spec,
// memory — to the contexts that declare it, normalized.
func contextKeywords(root string, ex *docs.Explorer, known map[string]bool) map[string][]string {
	out := map[string][]string{}
	if ex == nil {
		return out
	}
	names := make([]string, 0, len(known))
	for n := range known {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		tree, ok := ex.Tree(n)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		for _, refs := range [][]docs.DocRef{tree.PRDs, tree.Specs, tree.Memory} {
			for _, r := range refs {
				for _, k := range frontmatterList(filepath.Join(root, filepath.FromSlash(r.Path)), "keywords") {
					k = normalize(k)
					if k != "" && !seen[k] {
						seen[k] = true
						out[k] = append(out[k], n)
					}
				}
			}
		}
	}
	return out
}

// frontmatterList reads a one-line list field of a document's frontmatter:
// `field: [a, b]`. Empty when there is none.
func frontmatterList(path, field string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil
	}
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			break
		}
		v, ok := strings.CutPrefix(l, field+":")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "[]")
		var out []string
		for _, item := range strings.Split(v, ",") {
			if item = strings.Trim(strings.TrimSpace(item), `"'`); item != "" {
				out = append(out, item)
			}
		}
		return out
	}
	return nil
}

// stateOf fills what a known context already has, from the document graph.
func stateOf(ex *docs.Explorer, name string) Context {
	c := Context{Name: name}
	if ex == nil {
		return c
	}
	tree, ok := ex.Tree(name)
	if !ok {
		return c
	}
	if len(tree.PRDs) > 0 {
		c.Has = append(c.Has, "prd")
	}
	if len(tree.Specs) > 0 {
		c.Has = append(c.Has, "spec")
	}
	implemented := len(tree.Packages) > 0
	for _, m := range tree.Memory {
		if strings.HasPrefix(normalize(m.Status), "implement") {
			implemented = true
		}
	}
	if implemented {
		c.Has = append(c.Has, "code")
	}
	for _, n := range tree.Neighbors {
		c.Neighbors = append(c.Neighbors, n.Name)
	}
	return c
}

// subject names a new context from the request: its words, less the verbs,
// the artifact, the stopwords and anything already a known context.
func subject(ws []string, lex Lexicon, skip map[int]bool) string {
	stop := map[string]bool{}
	for _, w := range lex.Stopwords {
		stop[normalize(w)] = true
	}
	var parts []string
	for i, w := range ws {
		if skip[i] || stop[w] || len(w) < 3 {
			continue
		}
		parts = append(parts, w)
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, "-")
}

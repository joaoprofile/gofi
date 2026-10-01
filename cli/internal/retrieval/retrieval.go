// Package retrieval is the one place gofi answers "where is it?" for an agent
// or a person: documents — specs, PRDs, memory and the reference libraries —
// and code, searched together and ranked by one model.
//
// Every surface goes through it: `gofi find`, the MCP server, the chat. None
// of them ranks anything itself, so an answer does not depend on which door
// the question came in by.
//
// The model is BM25F over small units — a section of a document, a symbol of
// the code — with a field weight for where a term matched, a light Portuguese
// and English stemmer, and a synonym bridge (a base lexicon shipped with gofi
// plus the project's own) for the vocabulary gap between the two languages. Deterministic, offline and free: the
// same index and question always give the same answer, and none of it spends a
// token.
package retrieval

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/graph/model"
	"github.com/joaoprofile/gofi/cli/internal/graph/workspace"
)

// AreaCode is the area of every code symbol. Documents keep the areas of
// docs.Areas.
const AreaCode = "code"

// Areas lists every area a search can be limited to, documents first in order
// of authority, then code.
var Areas = append(append([]string{}, docs.Areas...), AreaCode)

// Kind tells a document section from a code symbol.
type Kind string

const (
	KindSection  Kind = "section"
	KindDocument Kind = "document" // a document with no sections, or the text before the first
	KindSymbol   Kind = "symbol"
)

// Hit is one answer: where to look, and exactly which lines.
type Hit struct {
	Kind    Kind    `json:"kind"`
	Area    string  `json:"area"`
	Path    string  `json:"path"`
	Title   string  `json:"title"`             // section heading, or symbol name
	Detail  string  `json:"detail,omitempty"`  // symbol kind and signature
	Context string  `json:"context,omitempty"` // gofi context, when the unit declares one
	Start   int     `json:"start"`             // first line, 1-based
	End     int     `json:"end"`               // last line, inclusive
	Score   float64 `json:"score"`
	// Overrides is set on a team's learning shown beside the gofi rule it
	// corrects: that rule, as path#section.
	Overrides string `json:"overrides,omitempty"`
	// OverriddenBy is set on a gofi rule the team corrected: the learning to
	// read first, because it wins where the two differ.
	OverriddenBy string `json:"overridden_by,omitempty"`
}

// Query is a search.
type Query struct {
	Text  string
	Areas []string // none means every area
	Limit int
}

type unit struct {
	hit   Hit
	terms unitTerms
	// doc indexes the document-level terms of the file a section belongs to,
	// or -1 for a code symbol.
	doc int
	// group collapses the units of one document into its best one, so a result
	// list names documents, not five sections of the same file.
	group string
}

// Engine searches one project.
type Engine struct {
	root     string
	language string
	docGraph *docs.Graph
	ws       *workspace.Workspace
	units    []unit
	ranker   *ranker
	// docTerms are one per document: what the whole file is about — its title,
	// its file name, its facets and every heading. A section's score adds a
	// share of its document's, so a note whose subject is the question outranks
	// a long section elsewhere that merely repeats its words.
	docTerms  []unitTerms
	docRanker *ranker
	synonyms  map[string][][]string
	// corrections maps a gofi file to the team's learning that corrects it.
	corrections map[string][]correction
	// wholeDoc is each document as a single answer: what a correction the
	// question did not reach on its own is shown as.
	wholeDoc map[string]Hit
	// Missing records a source that was not there to search — no document
	// index built, no code graph — so a caller can say so instead of
	// presenting an empty answer as a real one.
	Missing []string
}

// Open reads a project's document index and code graph. language is the
// backend language from .gofi.yaml, which names the graph to read.
func Open(root, language string) (*Engine, error) {
	e := &Engine{root: root, language: language, synonyms: loadSynonyms(root)}
	if err := e.loadDocs(); err != nil {
		return nil, err
	}
	e.loadCode(language)
	terms := make([]unitTerms, len(e.units))
	for i := range e.units {
		terms[i] = e.units[i].terms
	}
	e.ranker = newRanker(terms)
	e.docRanker = newRanker(e.docTerms)
	return e, nil
}

// correction is one team learning over one gofi rule.
type correction struct {
	by      string // the learning's path
	section string // the heading it corrects; empty for the whole file
	target  string // as declared, path#section
}

func (e *Engine) loadDocs() error {
	e.corrections, e.wholeDoc = map[string][]correction{}, map[string]Hit{}
	idx, g, _ := docs.Load(e.root)
	e.docGraph = g
	if idx == nil {
		e.Missing = append(e.Missing, "documents: run `gofi index docs`")
		return nil
	}
	for _, d := range idx.Docs {
		raw, err := os.ReadFile(filepath.Join(e.root, filepath.FromSlash(d.Path)))
		if err != nil {
			continue // indexed, then removed: the next build drops it
		}
		lines := strings.Split(string(raw), "\n")
		area := docs.AreaOf(d.Path)
		for _, t := range d.Overrides {
			file, sec, _ := strings.Cut(t, "#")
			e.corrections[file] = append(e.corrections[file], correction{by: d.Path, section: strings.TrimSpace(sec), target: t})
		}
		head := strings.Join([]string{d.Title, d.Context, d.Facets}, " ")
		pathText := strings.ReplaceAll(d.Path, "/", " ")

		headings := make([]string, 0, len(d.Sections))
		for _, s := range d.Sections {
			headings = append(headings, s.Heading)
		}
		// The text before the first section. In a document with none it is the
		// whole note, and a unit of its own. In one with several it is the
		// preamble: what the file as a whole is about, so it describes the
		// document rather than competing with the sections that answer.
		first := len(lines) + 1
		if len(d.Sections) > 0 {
			first = d.Sections[0].Start
		}
		bodyStart := bodyStartOf(lines)
		e.wholeDoc[d.Path] = Hit{Kind: KindDocument, Area: area, Path: d.Path, Title: d.Title, Context: d.Context,
			Start: bodyStart, End: max(len(lines), bodyStart)}
		intro := sliceLines(lines, bodyStart, first-1)
		docIdx := len(e.docTerms)
		e.docTerms = append(e.docTerms, newUnitTerms([nFields]string{
			fTitle: d.Title + " " + strings.TrimSuffix(path.Base(d.Path), ".md"),
			fHead:  d.Context + " " + d.Facets,
			fBody:  strings.Join(headings, " ") + "\n" + preamble(intro, len(d.Sections)),
			fPath:  pathText,
		}))

		// The title line alone is no content: it would compete with the
		// document's own sections and win on the title weight, pointing the
		// reader at the top of the file instead of the rule they asked for.
		if len(d.Sections) < 2 && (withoutTitle(intro) != "" || len(d.Sections) == 0) {
			// In a document with sections the title says what the whole file
			// is about, and it reaches every section through the document's
			// score. Handing it to the preamble as its own heading made the
			// top of the file outrank the section the question was about.
			introTitle := d.Title
			if len(d.Sections) > 0 {
				introTitle = ""
			}
			e.units = append(e.units, unit{
				group: d.Path, doc: docIdx,
				hit: Hit{Kind: KindDocument, Area: area, Path: d.Path, Title: d.Title, Context: d.Context,
					Start: bodyStart, End: max(first-1, bodyStart)},
				terms: newUnitTerms([nFields]string{fTitle: introTitle, fHead: head, fBody: intro, fPath: pathText}),
			})
		}
		for _, s := range d.Sections {
			e.units = append(e.units, unit{
				group: d.Path, doc: docIdx,
				hit: Hit{Kind: KindSection, Area: area, Path: d.Path, Title: s.Heading, Context: d.Context,
					Start: s.Start, End: s.End},
				terms: newUnitTerms([nFields]string{fTitle: s.Heading, fHead: head, fBody: sliceLines(lines, s.Start+1, s.End), fPath: pathText}),
			})
		}
	}
	return nil
}

// preamble is the intro text that belongs to the document rather than to a
// unit of its own: all of it once the document has sections to answer from.
func preamble(intro string, sections int) string {
	if sections < 2 {
		return ""
	}
	return withoutTitle(intro)
}

// bodyStartOf is the first line after the frontmatter, 1-based.
func bodyStartOf(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 1
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 2
		}
	}
	return 1
}

// sliceLines joins lines from..to, 1-based and inclusive, clamped to the file.
func sliceLines(lines []string, from, to int) string {
	from, to = max(from, 1), min(to, len(lines))
	if from > to {
		return ""
	}
	return strings.Join(lines[from-1:to], "\n")
}

// codeKinds are the symbols worth answering with: something a reader would
// open. Packages carry no location, fields are reached through their type.
var codeKinds = map[model.Kind]bool{
	model.KindStruct: true, model.KindInterface: true, model.KindType: true,
	model.KindFunc: true, model.KindMethod: true, model.KindClass: true,
	model.KindTrait: true, model.KindEnum: true, model.KindConst: true,
	model.KindVar: true, model.KindComponent: true, model.KindHook: true,
	model.KindService: true,
}

func (e *Engine) loadCode(language string) {
	ws := workspace.Load(e.root, language)
	e.ws = ws
	found := false
	for _, s := range ws.Index.Scopes {
		g, err := ws.Graph(s.Name)
		if err != nil {
			continue
		}
		found = true
		for _, n := range g.Nodes {
			if n.External || n.File == "" || !codeKinds[n.Kind] {
				continue
			}
			file := path.Join(filepath.ToSlash(s.Root), n.File)
			short := model.ShortID(n.ID, g.Module)
			detail := symbolDetail(n)
			ctx := n.Context
			if ctx == "" && s.Name != workspace.ScopeProject {
				ctx = s.Name // the SDK or a surface: say which tree it is in
			}
			e.units = append(e.units, unit{
				group: "sym:" + n.ID, doc: -1,
				hit: Hit{Kind: KindSymbol, Area: AreaCode, Path: file, Title: short, Detail: detail, Context: ctx,
					Start: n.Line, End: n.Line + max(n.Lines-1, 0)},
				terms: newUnitTerms([nFields]string{
					fTitle: n.Name,
					fHead:  short + " " + n.Owner + " " + n.Context,
					fBody:  n.Doc + " " + n.Sig,
					fPath:  strings.ReplaceAll(file, "/", " "),
				}),
			})
		}
	}
	if !found {
		e.Missing = append(e.Missing, "code: run `gofi index code`")
	}
}

// namedSymbols are the words of a question written the way code names things:
// with a dot or a capital letter (netx.Response, FindFromCriteria). A question
// in prose has none.
func namedSymbols(text string) []string {
	var out []string
	for _, w := range strings.Fields(text) {
		w = strings.Trim(w, "`'\"()[]{},;:?!")
		if len(w) > 2 && (strings.Contains(w, ".") || strings.ToLower(w) != w) {
			out = append(out, w)
		}
	}
	return out
}

// names reports whether a heading is about one of the symbols a question
// names. Only the last segment counts: sqln.NewTransaction is the section
// transaction.NewTransaction re-exported, and FindFromCriteria is
// sqln.FindFromCriteria.
func names(symbols []string, title string) bool {
	for _, s := range symbols {
		if lastSegment(s) == lastSegment(title) {
			return true
		}
	}
	return false
}

func lastSegment(s string) string {
	return s[strings.LastIndex(s, ".")+1:]
}

// Find ranks every unit in the allowed areas for the question and returns the
// best, at most one per document.
func (e *Engine) Find(q Query) []Hit {
	terms := ordered(e.expand(q.Text))
	if len(terms) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, a := range q.Areas {
		allowed[a] = true
	}
	symbols := namedSymbols(q.Text)
	best := map[string]Hit{}
	docScore := map[int]float64{}
	for _, u := range e.units {
		if len(allowed) > 0 && !allowed[u.hit.Area] {
			continue
		}
		s := e.ranker.score(u.terms, terms)
		if s <= 0 {
			continue
		}
		if u.doc >= 0 {
			ds, ok := docScore[u.doc]
			if !ok {
				ds = e.docRanker.score(e.docTerms[u.doc], terms)
				docScore[u.doc] = ds
			}
			s += docShare * ds
		}
		switch {
		case u.hit.Area == docs.AreaSkills:
			s *= skillsWeight
		case u.hit.Area == docs.AreaSDK && strings.Contains(u.hit.Path, "/api/") && !names(symbols, u.hit.Title):
			s *= referenceWeight
		}
		if cur, ok := best[u.group]; !ok || s > cur.Score {
			h := u.hit
			h.Score = s
			best[u.group] = h
		}
	}
	out := make([]Hit, 0, len(best))
	for _, h := range best {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if a, b := authority(out[i].Area), authority(out[j].Area); a != b {
			return a < b
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Start < out[j].Start
	})
	out = e.withCorrections(out)
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out
}

// withCorrections puts the team's learning right above each gofi rule it
// corrects, whether or not the question reached it: the learning wins where
// the two differ, and a reader who stops at the rule would follow the version
// the team already fixed. The area filter does not hold it back, for the same
// reason. A learning that already ranks above its rule stays where it is.
func (e *Engine) withCorrections(hits []Hit) []Hit {
	if len(e.corrections) == 0 {
		return hits
	}
	pos := map[string]int{}
	for i, h := range hits {
		pos[h.Path] = i
	}
	before := map[int][]Hit{}
	moved := map[string]bool{}
	for j := range hits {
		for _, c := range e.corrections[hits[j].Path] {
			if c.section != "" && hits[j].Kind == KindSection && hits[j].Title != c.section {
				continue
			}
			if hits[j].OverriddenBy == "" {
				hits[j].OverriddenBy = c.by
			}
			if i, ok := pos[c.by]; ok && i < j {
				hits[i].Overrides = c.target
				continue
			}
			if moved[c.by] {
				continue
			}
			fix, ok := e.wholeDoc[c.by]
			if i, found := pos[c.by]; found {
				fix, ok = hits[i], true
			}
			if !ok {
				continue
			}
			fix.Score = max(fix.Score, hits[j].Score)
			fix.Overrides = c.target
			before[j] = append(before[j], fix)
			moved[c.by] = true
		}
	}
	out := make([]Hit, 0, len(hits)+len(moved))
	for j, h := range hits {
		out = append(out, before[j]...)
		if !moved[h.Path] {
			out = append(out, h)
		}
	}
	return out
}

// expand turns the question into weighted terms: the words asked count fully,
// their synonyms from the base and project lexicons a little less — a synonym is a
// bridge, not the question.
func (e *Engine) expand(text string) map[string]float64 {
	out := map[string]float64{}
	words := tokens(text)
	for _, t := range words {
		out[t] = 1
	}
	for _, t := range surfaces(text) {
		out[t] = surfaceWeight
	}
	for _, t := range pairs(text) {
		out[t] = pairWeight
	}
	// Every phrase of the question, longest first, so "máquina de estados"
	// bridges as a whole before its words are tried one by one.
	for n := min(maxPhrase, len(words)); n >= 1; n-- {
		for i := 0; i+n <= len(words); i++ {
			for _, syn := range e.synonyms[strings.Join(words[i:i+n], " ")] {
				for _, s := range syn {
					if _, asked := out[s]; !asked {
						out[s] = synonymWeight
					}
				}
			}
		}
	}
	return out
}

// authority orders areas for ties: the order of Areas.
func authority(area string) int {
	if i := slices.Index(Areas, area); i >= 0 {
		return i
	}
	return len(Areas)
}

// ValidArea reports whether a is a name --in accepts.
func ValidArea(a string) bool { return slices.Contains(Areas, a) }

// String is a one-line description of a hit, for logs and tests.
func (h Hit) String() string { return fmt.Sprintf("%s:%d-%d %s", h.Path, h.Start, h.End, h.Title) }

// withoutTitle is text with its "# " heading lines removed, trimmed.
func withoutTitle(text string) string {
	var keep []string
	for _, l := range strings.Split(text, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), "# ") {
			keep = append(keep, l)
		}
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

// symbolDetail reads like a declaration: the graph stores a function's
// signature without its name ("func(o *Order) error"), so the name goes back in.
func symbolDetail(n *model.Node) string {
	switch {
	case n.Sig == "":
		return string(n.Kind)
	case strings.HasPrefix(n.Sig, "func("):
		return "func " + n.Name + strings.TrimPrefix(n.Sig, "func")
	case strings.HasPrefix(n.Sig, string(n.Kind)):
		return n.Sig
	}
	return string(n.Kind) + " " + n.Sig
}

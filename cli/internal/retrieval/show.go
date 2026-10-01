package retrieval

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/docs"
	"github.com/gofi-labs/gofi/cli/internal/graph/model"
	"github.com/gofi-labs/gofi/cli/internal/graph/query"
)

// Show answers "what is it?" for any reference: the kind comes from the
// reference itself, never from a flag. A prefix pins it when a name could be
// two things:
//
//	sym:Server.Start               a code symbol
//	doc:specs/order/sdd-order.md   a document — or any path ending in .md
//	specs/order/sdd-order.md#RN-01 one section of it: only its lines
//	ctx:order                      everything filed under a context
//	table:order_order              which documents own a table, which code touches it
//
// Without a prefix the reference is tried as each kind. One match is shown;
// several are listed as candidates, never guessed between.

// ViewKind names what a View shows.
type ViewKind string

const (
	ViewSymbol     ViewKind = "symbol"
	ViewDocument   ViewKind = "document"
	ViewSection    ViewKind = "section"
	ViewContext    ViewKind = "context"
	ViewEntity     ViewKind = "entity"
	ViewPath       ViewKind = "path"
	ViewCandidates ViewKind = "candidates"
)

// View is the answer to Show or Path: structured Data for tools, Text for
// people. Both come from the same lookup, so they never disagree.
type View struct {
	Kind ViewKind `json:"kind"`
	Ref  string   `json:"ref"`
	Data any      `json:"data"`
	Text string   `json:"-"`
}

// ErrNotFound is returned when a reference names nothing in the indexes.
var ErrNotFound = errors.New("not found")

// Candidate is one thing an ambiguous reference could mean.
type Candidate struct {
	Ref  string `json:"ref"` // pass it back to Show to pick it
	Kind string `json:"kind"`
	Note string `json:"note,omitempty"`
}

// Show resolves a reference and describes what it names.
func (e *Engine) Show(ref string, limit int) (*View, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("%w: empty reference", ErrNotFound)
	}
	if limit <= 0 {
		limit = 12
	}
	name := ref
	if kind, rest, pinned := strings.Cut(ref, ":"); pinned {
		name = rest
		switch kind {
		case "sym":
			return e.showSymbol(name, limit)
		case "doc":
			return e.showDocument(name, limit)
		case "ctx":
			return e.showContext(name)
		case "table", "ent":
			return e.showEntity(name, limit)
		}
		// Not a known prefix: a Go ID (func:pkg.F) or a C++ name — try as is.
		name = ref
	}

	if strings.Contains(name, ".md") {
		return e.showDocument(name, limit)
	}
	var cands []Candidate
	if e.explorer() != nil {
		if _, ok := e.explorer().Tree(name); ok {
			cands = append(cands, Candidate{Ref: "ctx:" + name, Kind: string(ViewContext)})
		}
		if _, _, ok := e.explorer().Entity(name); ok {
			cands = append(cands, Candidate{Ref: "table:" + name, Kind: string(ViewEntity)})
		}
	}
	syms := e.resolveSymbols(name)
	for _, s := range syms {
		cands = append(cands, Candidate{Ref: "sym:" + s.id, Kind: string(ViewSymbol), Note: s.location()})
	}
	switch {
	case len(cands) == 0:
		return nil, fmt.Errorf("%w: %q — try `gofi find %s`", ErrNotFound, ref, ref)
	case len(cands) == 1:
		return e.Show(cands[0].Ref, limit)
	}
	// Several symbols of one name with nothing else competing: the graph's own
	// ranking (internal before external, then central) picks the one meant far
	// more often than not, and the rest are named below it.
	if len(syms) == len(cands) && syms[0].exact && (len(syms) == 1 || !syms[1].exact) {
		v, err := e.Show(cands[0].Ref, limit)
		if err == nil {
			v.Text = fmt.Sprintf("%d matches — showing the first; the others: %s\n\n", len(cands), refsOf(cands[1:], 6)) + v.Text
		}
		return v, err
	}
	return candidatesView(ref, cands), nil
}

func refsOf(cs []Candidate, max int) string {
	var out []string
	for i, c := range cs {
		if i == max {
			out = append(out, fmt.Sprintf("+%d", len(cs)-max))
			break
		}
		out = append(out, c.Ref)
	}
	return strings.Join(out, ", ")
}

func candidatesView(ref string, cands []Candidate) *View {
	var b strings.Builder
	fmt.Fprintf(&b, "%q names %d things — show one of them:\n", ref, len(cands))
	for _, c := range cands {
		fmt.Fprintf(&b, "  gofi show %s", c.Ref)
		if c.Note != "" {
			fmt.Fprintf(&b, "   (%s)", c.Note)
		}
		b.WriteString("\n")
	}
	return &View{Kind: ViewCandidates, Ref: ref, Data: cands, Text: b.String()}
}

func (e *Engine) explorer() *docs.Explorer {
	if e.docGraph == nil {
		return nil
	}
	return &docs.Explorer{Graph: e.docGraph}
}

// --- documents ------------------------------------------------------------

// SectionData is one section, with its lines: what an agent reads instead of
// the whole file.
type SectionData struct {
	Path    string `json:"path"`
	Heading string `json:"heading"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Text    string `json:"text"`
	// CorrectedBy lists the team's learning that corrects this section — read
	// it first.
	CorrectedBy []string `json:"corrected_by,omitempty"`
}

// SectionRef is a section's place in its file.
type SectionRef struct {
	Heading string `json:"heading"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

// DocumentData is a document's outline and links: enough to decide which
// section to read.
type DocumentData struct {
	Path     string       `json:"path"`
	Title    string       `json:"title"`
	Area     string       `json:"area"`
	Context  string       `json:"context,omitempty"`
	Status   string       `json:"status,omitempty"`
	Sections []SectionRef `json:"sections"`
	LinksIn  []docs.Link  `json:"links_in,omitempty"`
	LinksOut []docs.Link  `json:"links_out,omitempty"`
	Code     []string     `json:"code,omitempty"`
	// Overrides is, on a team's learning, the gofi rules it corrects.
	Overrides []string `json:"overrides,omitempty"`
	// CorrectedBy is, on a gofi file, the team's learning that corrects it,
	// as path — or path#section when it corrects one section only.
	CorrectedBy []string `json:"corrected_by,omitempty"`
}

func (e *Engine) findDoc(target string) (docs.Doc, bool) {
	idx, _, _ := docs.Load(e.root)
	if idx == nil {
		return docs.Doc{}, false
	}
	resolved := target
	if ex := e.explorer(); ex != nil {
		if id := ex.Resolve(target); id != "" {
			resolved = id
		}
	}
	for _, d := range idx.Docs {
		if d.Path == resolved || d.Path == target {
			return d, true
		}
	}
	return docs.Doc{}, false
}

func (e *Engine) showDocument(ref string, limit int) (*View, error) {
	target, frag, _ := strings.Cut(ref, "#")
	d, ok := e.findDoc(target)
	if !ok {
		return nil, fmt.Errorf("%w: document %q is not indexed — `gofi index docs`, or `gofi find` it", ErrNotFound, target)
	}
	if frag != "" {
		return e.showSection(d, frag)
	}
	data := DocumentData{Path: d.Path, Title: d.Title, Area: docs.AreaOf(d.Path), Context: d.Context, Status: d.Status, Sections: []SectionRef{}}
	for _, s := range d.Sections {
		data.Sections = append(data.Sections, SectionRef{Heading: s.Heading, Start: s.Start, End: s.End})
	}
	data.Overrides = d.Overrides
	for _, c := range e.corrections[d.Path] {
		ref := c.by
		if c.section != "" {
			ref += " (§" + c.section + ")"
		}
		data.CorrectedBy = append(data.CorrectedBy, ref)
	}
	if ex := e.explorer(); ex != nil {
		if n := ex.Explain(d.Path); n != nil {
			data.LinksIn, data.LinksOut, data.Code = n.Incoming, n.Outgoing, n.Code
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", d.Path, d.Title)
	meta := []string{data.Area}
	if d.Context != "" {
		meta = append(meta, "context "+d.Context)
	}
	if d.Status != "" {
		meta = append(meta, d.Status)
	}
	fmt.Fprintf(&b, "  %s\n", strings.Join(meta, " · "))
	writeList(&b, "corrected by the team — read first, it wins where they differ", data.CorrectedBy)
	writeList(&b, "corrects", data.Overrides)
	if len(d.Sections) > 0 {
		b.WriteString("\n  sections (show one with #heading):\n")
		for _, s := range d.Sections {
			fmt.Fprintf(&b, "    L%-5s %s\n", fmt.Sprintf("%d-%d", s.Start, s.End), s.Heading)
		}
	}
	writeLinks(&b, "pointed at by", data.LinksIn, limit)
	writeLinks(&b, "points to", data.LinksOut, limit)
	if len(data.Code) > 0 {
		shown := data.Code
		if len(shown) > limit {
			shown = shown[:limit]
		}
		fmt.Fprintf(&b, "\n  code (%d): %s\n", len(data.Code), strings.Join(shown, ", "))
	}
	return &View{Kind: ViewDocument, Ref: d.Path, Data: data, Text: b.String()}, nil
}

func writeList(b *strings.Builder, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n  %s:\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "    %s\n", it)
	}
}

func writeLinks(b *strings.Builder, title string, links []docs.Link, limit int) {
	if len(links) == 0 {
		return
	}
	fmt.Fprintf(b, "\n  %s (%d):\n", title, len(links))
	for i, l := range links {
		if i == limit {
			fmt.Fprintf(b, "    … +%d\n", len(links)-limit)
			break
		}
		fmt.Fprintf(b, "    [%s] %s\n", l.Kind, l.Node)
	}
}

// showSection prints one section's lines, numbered as in the file so they can
// be cited. The fragment is a heading — exact, then prefix, then contained,
// ignoring case and accents — or a line number (L10 or 10).
func (e *Engine) showSection(d docs.Doc, frag string) (*View, error) {
	sec, ok := matchSection(d.Sections, frag)
	if !ok {
		var names []string
		for _, s := range d.Sections {
			names = append(names, s.Heading)
		}
		return nil, fmt.Errorf("%w: no section %q in %s — sections: %s", ErrNotFound, frag, d.Path, strings.Join(names, " | "))
	}
	raw, err := os.ReadFile(filepath.Join(e.root, filepath.FromSlash(d.Path)))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(raw), "\n")
	end := min(sec.End, len(lines))
	var corrected []string
	for _, c := range e.corrections[d.Path] {
		if c.section == "" || c.section == sec.Heading {
			corrected = append(corrected, c.by)
		}
	}
	var text, b strings.Builder
	fmt.Fprintf(&b, "%s  §%s  L%d-%d\n", d.Path, sec.Heading, sec.Start, end)
	for _, by := range corrected {
		fmt.Fprintf(&b, "  ↳ corrected by the team in %s — read it first, it wins where they differ\n", by)
	}
	b.WriteString("\n")
	for i := sec.Start; i <= end; i++ {
		text.WriteString(lines[i-1] + "\n")
		fmt.Fprintf(&b, "%5d│ %s\n", i, lines[i-1])
	}
	return &View{Kind: ViewSection, Ref: d.Path + "#" + sec.Heading,
		Data: SectionData{Path: d.Path, Heading: sec.Heading, Start: sec.Start, End: end, Text: text.String(), CorrectedBy: corrected},
		Text: b.String()}, nil
}

func matchSection(secs []docs.Section, frag string) (docs.Section, bool) {
	f := strings.TrimSpace(frag)
	if n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(f), "L")); err == nil {
		for _, s := range secs {
			if n >= s.Start && n <= s.End {
				return s, true
			}
		}
		return docs.Section{}, false
	}
	want := docs.Normalize(f)
	for _, test := range []func(h string) bool{
		func(h string) bool { return h == want },
		func(h string) bool { return strings.HasPrefix(h, want) },
		func(h string) bool { return strings.Contains(h, want) },
	} {
		for _, s := range secs {
			if test(docs.Normalize(s.Heading)) {
				return s, true
			}
		}
	}
	return docs.Section{}, false
}

// --- contexts and entities ------------------------------------------------

func (e *Engine) showContext(name string) (*View, error) {
	ex := e.explorer()
	if ex == nil {
		return nil, fmt.Errorf("%w: no document index — `gofi index docs`", ErrNotFound)
	}
	tree, ok := ex.Tree(name)
	if !ok {
		return nil, fmt.Errorf("%w: context %q — contexts: %s", ErrNotFound, name, strings.Join(ex.Contexts(), ", "))
	}
	return &View{Kind: ViewContext, Ref: "ctx:" + name, Data: tree, Text: RenderContext(tree)}, nil
}

// EntityData is a table: the documents that describe it and the code that
// touches it — the one symbol that exists in both worlds.
type EntityData struct {
	Name      string   `json:"name"`
	Documents []string `json:"documents"`
	Code      []string `json:"code"`
}

func (e *Engine) showEntity(name string, limit int) (*View, error) {
	ex := e.explorer()
	if ex == nil {
		return nil, fmt.Errorf("%w: no document index — `gofi index docs`", ErrNotFound)
	}
	documents, code, ok := ex.Entity(name)
	if !ok {
		near := ""
		if like := ex.EntitiesLike(name); len(like) > 0 {
			near = " — near: " + strings.Join(like, ", ")
		}
		return nil, fmt.Errorf("%w: table %q%s", ErrNotFound, name, near)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n  documented in %d:\n", name, len(documents))
	for _, d := range documents {
		fmt.Fprintf(&b, "    %s\n", d)
	}
	fmt.Fprintf(&b, "\n  touched by code in %d:\n", len(code))
	for i, c := range code {
		if i == limit {
			fmt.Fprintf(&b, "    … +%d\n", len(code)-limit)
			break
		}
		fmt.Fprintf(&b, "    %s\n", c)
	}
	return &View{Kind: ViewEntity, Ref: "table:" + name,
		Data: EntityData{Name: name, Documents: nonNil(documents), Code: nonNil(code)}, Text: b.String()}, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// --- code -----------------------------------------------------------------

// symbolRef is a resolved code symbol and the scope that holds it.
type symbolRef struct {
	id, scope, root string
	g               *model.Graph
	n               *model.Node
	exact           bool
}

func (s symbolRef) location() string {
	if s.n.File == "" {
		return string(s.n.Kind)
	}
	return fmt.Sprintf("%s %s:%d", s.n.Kind, path.Join(s.root, s.n.File), s.n.Line)
}

// resolveSymbols finds symbols by ID, short name or suffix across every
// scope, project first. Partial matches are left to `gofi find`: showing is
// for a name, searching is for words.
func (e *Engine) resolveSymbols(term string) []symbolRef {
	if e.ws == nil {
		return nil
	}
	var out []symbolRef
	target := strings.ToLower(term)
	for _, s := range e.ws.Index.Scopes {
		g, err := e.ws.Graph(s.Name)
		if err != nil {
			continue
		}
		for _, n := range query.Resolve(g, term) {
			short := strings.ToLower(model.ShortID(n.ID, g.Module))
			exact := n.ID == term || short == target || strings.ToLower(n.Name) == target
			suffix := strings.HasSuffix(short, "."+target) || strings.HasSuffix(short, "/"+target)
			if !exact && !suffix {
				continue
			}
			out = append(out, symbolRef{id: n.ID, scope: s.Name, root: filepath.ToSlash(s.Root), g: g, n: n, exact: exact})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].exact != out[j].exact {
			return out[i].exact
		}
		return out[i].n.External != out[j].n.External && !out[i].n.External
	})
	return out
}

// SymbolData is a symbol and its immediate neighbourhood in the graph.
type SymbolData struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`
	Scope     string     `json:"scope"`
	File      string     `json:"file,omitempty"`
	Start     int        `json:"start,omitempty"`
	End       int        `json:"end,omitempty"`
	Signature string     `json:"signature,omitempty"`
	Doc       string     `json:"doc,omitempty"`
	Context   string     `json:"context,omitempty"`
	Out       []Relation `json:"out"`
	In        []Relation `json:"in"`
}

// Relation is one edge seen from the symbol.
type Relation struct {
	Rel    string `json:"rel"`
	Symbol string `json:"symbol"`
	At     string `json:"at,omitempty"` // where in the code the relation is made
}

func (e *Engine) showSymbol(term string, limit int) (*View, error) {
	refs := e.resolveSymbols(term)
	if len(refs) == 0 {
		return nil, fmt.Errorf("%w: symbol %q — try `gofi find --in code %s`", ErrNotFound, term, term)
	}
	r := refs[0]
	n, g := r.n, r.g
	data := SymbolData{ID: n.ID, Name: model.ShortID(n.ID, g.Module), Kind: string(n.Kind), Scope: r.scope,
		Signature: symbolDetail(n), Doc: n.Doc, Context: n.Context, Out: []Relation{}, In: []Relation{}}
	if n.File != "" {
		data.File, data.Start, data.End = path.Join(r.root, n.File), n.Line, n.Line+max(n.Lines-1, 0)
	}
	rel := func(es []*model.Edge, outgoing bool) []Relation {
		var out []Relation
		for _, ed := range es {
			other := ed.To
			if !outgoing {
				other = ed.From
			}
			at := ""
			if ed.File != "" {
				at = fmt.Sprintf("%s:%d", path.Join(r.root, ed.File), ed.Line)
			}
			out = append(out, Relation{Rel: string(ed.Rel), Symbol: model.ShortID(other, g.Module), At: at})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Rel != out[j].Rel {
				return out[i].Rel < out[j].Rel
			}
			return out[i].Symbol < out[j].Symbol
		})
		return out
	}
	data.Out = append(data.Out, rel(g.Out(n.ID), true)...)
	data.In = append(data.In, rel(g.In(n.ID), false)...)
	return &View{Kind: ViewSymbol, Ref: "sym:" + n.ID, Data: data, Text: query.Explain(g, n.ID, limit)}, nil
}

// Path answers "how does A reach B" in the code graph: every hop between two
// symbols.
func (e *Engine) Path(from, to string) (*View, error) {
	refs := e.resolveSymbols(from)
	if len(refs) == 0 {
		return nil, fmt.Errorf("%w: symbol %q", ErrNotFound, from)
	}
	g := refs[0].g
	text := query.Path(g, refs[0].id, to)
	return &View{Kind: ViewPath, Ref: from + " → " + to, Data: map[string]string{"from": from, "to": to, "text": text}, Text: text}, nil
}

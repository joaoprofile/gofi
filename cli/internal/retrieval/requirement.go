package retrieval

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// ViewRequirement is the answer to Show for a requirement.
const ViewRequirement ViewKind = "requirement"

// RequirementData is a requirement of a context's living spec, traced
// (0002, D10): the symbols that implement it — gofi:implements in the code —
// and, scenario by scenario, the tests that test it — gofi:tests.
type RequirementData struct {
	Context    string         `json:"context"`
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Doc        string         `json:"doc"`
	Line       int            `json:"line"`
	Implements []TracePlace   `json:"implements"`
	Scenarios  []ScenarioData `json:"scenarios"`
}

// ScenarioData is one scenario, with the tests that test it.
type ScenarioData struct {
	ID    string       `json:"id"`
	Title string       `json:"title"`
	Line  int          `json:"line"`
	Tests []TracePlace `json:"tests"`
}

// TracePlace is where a directive is, and the symbol holding it.
type TracePlace struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Symbol string `json:"symbol,omitempty"`
}

var reDirectiveLine = regexp.MustCompile(`gofi:(implements|tests)\b`)

// showRequirement answers req:<context>/REQ-NN — or REQ-NN.n, its scenario's
// requirement — from the living spec and the code's directives.
func (e *Engine) showRequirement(ref string) (*View, error) {
	ctx, id, ok := strings.Cut(ref, "/")
	if !ok || ctx == "" || id == "" {
		return nil, fmt.Errorf("%w: %q — req:<contexto>/REQ-NN", ErrNotFound, ref)
	}
	if parent := docs.Parent(id); parent != "" {
		id = parent
	}
	rel := filepath.ToSlash(filepath.Join("specs", ctx, "spec.md"))
	data, err := os.ReadFile(filepath.Join(e.root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("%w: %s has no living spec (%s) — a requirement lives there once its change is archived", ErrNotFound, ctx, rel)
	}
	var req *docs.Requirement
	for _, r := range docs.ParseRequirements(strings.Split(string(data), "\n"), 1) {
		if r.ID == id {
			r := r
			req = &r
		}
	}
	if req == nil {
		return nil, fmt.Errorf("%w: %s is not a requirement of %s", ErrNotFound, id, rel)
	}
	d := RequirementData{Context: ctx, ID: id, Title: req.Title, Doc: rel, Line: req.Line, Implements: []TracePlace{}, Scenarios: []ScenarioData{}}
	marks := e.directives(ctx, id)
	d.Implements = append(d.Implements, marks[id+" implements"]...)
	for _, s := range req.Scenarios {
		// A scenario is implemented through its requirement; a mark on the
		// scenario's own ID counts for the requirement too.
		d.Implements = append(d.Implements, marks[s.ID+" implements"]...)
		d.Scenarios = append(d.Scenarios, ScenarioData{ID: s.ID, Title: s.Title, Line: s.Line, Tests: append([]TracePlace{}, marks[s.ID+" tests"]...)})
	}
	return &View{Kind: ViewRequirement, Ref: "req:" + ctx + "/" + id, Data: d, Text: renderRequirement(d)}, nil
}

// directives are the code's gofi:implements and gofi:tests lines naming a
// requirement or one of its scenarios, in a context's code, by "ID kind".
func (e *Engine) directives(ctx, id string) map[string][]TracePlace {
	out := map[string][]TracePlace{}
	res, err := e.Text(TextQuery{Pattern: `gofi:(implements|tests)\b.*\b` + regexp.QuoteMeta(id) + `\b`, Regex: true, Areas: []string{AreaCode}, Limit: 200})
	if err != nil {
		return out
	}
	reID := regexp.MustCompile(`(?:\b([a-z0-9][a-z0-9_-]*)/)?\b(` + regexp.QuoteMeta(id) + `(?:\.\d+)?)\b`)
	for _, m := range res.Matches {
		kind := reDirectiveLine.FindStringSubmatch(m.Text)
		if kind == nil {
			continue
		}
		// The directive is the context's when it names the context, or sits
		// in the context's code.
		inCtx := strings.Contains("/"+m.Path+"/", "/"+ctx+"/") || m.In != nil && m.In.Context == ctx
		for _, found := range reID.FindAllStringSubmatch(m.Text[strings.Index(m.Text, "gofi:"):], -1) {
			if found[1] != "" && found[1] != ctx || found[1] == "" && !inCtx {
				continue
			}
			p := TracePlace{Path: m.Path, Line: m.Line}
			if m.In != nil {
				p.Symbol = m.In.Title
			}
			key := found[2] + " " + kind[1]
			out[key] = append(out[key], p)
		}
	}
	return out
}

func renderRequirement(d RequirementData) string {
	var b strings.Builder
	place := func(ps []TracePlace) string {
		var s []string
		for _, p := range ps {
			at := fmt.Sprintf("%s:%d", p.Path, p.Line)
			if p.Symbol != "" {
				at += " (" + p.Symbol + ")"
			}
			s = append(s, at)
		}
		return strings.Join(s, ", ")
	}
	fmt.Fprintf(&b, "%s/%s — %s\n  spec: %s:%d\n", d.Context, d.ID, d.Title, d.Doc, d.Line)
	if len(d.Implements) == 0 {
		b.WriteString("  implementa: — nada no código o marca (gofi:implements " + d.ID + ")\n")
	} else {
		b.WriteString("  implementa: " + place(d.Implements) + "\n")
	}
	for _, s := range d.Scenarios {
		fmt.Fprintf(&b, "\n  %s — %s\n", s.ID, s.Title)
		if len(s.Tests) == 0 {
			b.WriteString("    testa: — nenhum teste (gofi:tests " + s.ID + ")\n")
		} else {
			b.WriteString("    testa: " + place(s.Tests) + "\n")
		}
	}
	return b.String()
}

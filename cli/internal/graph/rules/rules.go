// Package rules holds a change to the rules of its code's architecture
// (0002, D11): which layer may not reach which, what a layer may not import,
// that no new cycle crosses packages. The rules come from the language's
// pack and the project; each is checked on what the change did — its new
// edges, its new cycles — so the code it started from is not its to answer
// for. A violation is a finding with a symbol, found without a token.
package rules

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/graph/diff"
	"github.com/joaoprofile/gofi/cli/internal/graph/model"
)

// Rule is one rule, as written.
type Rule struct {
	Text string
	kind string // layer | import | cycle
	from string // the layer an edge leaves, or imports from
	to   string // the layer it may not reach, or the package
}

// Parse reads rules, one a line: "deny: <layer> -> <layer>", "deny: import
// \"<package>\" from <layer>", "deny: new-cycle". layers are the layers a
// rule may name; a rule naming another is an error, never a rule that holds
// nothing.
func Parse(lines []string, layers []string) ([]Rule, error) {
	var out []Rule
	var problems []string
	for _, line := range lines {
		r, err := parse(line, layers)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		out = append(out, r)
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("graph.rules:\n  %s", strings.Join(problems, "\n  "))
	}
	return out, nil
}

func parse(line string, layers []string) (Rule, error) {
	text := strings.TrimSpace(line)
	body, ok := strings.CutPrefix(text, "deny:")
	if !ok {
		return Rule{}, fmt.Errorf("%q: a rule starts with deny:", line)
	}
	body = strings.TrimSpace(body)
	layer := func(name string) error {
		if !slices.Contains(layers, name) {
			return fmt.Errorf("%q: layer %q — %s", line, name, strings.Join(layers, ", "))
		}
		return nil
	}
	switch {
	case body == "new-cycle":
		return Rule{Text: text, kind: "cycle"}, nil
	case body == "new-cross-context-edge":
		return Rule{Text: text, kind: "context"}, nil
	case strings.HasPrefix(body, "import "):
		rest := strings.TrimSpace(strings.TrimPrefix(body, "import "))
		pkg, from, ok := strings.Cut(rest, " from ")
		pkg = strings.Trim(strings.TrimSpace(pkg), `"`)
		from = strings.TrimSpace(from)
		if !ok || pkg == "" {
			return Rule{}, fmt.Errorf(`%q: deny: import "<package>" from <layer>`, line)
		}
		if err := layer(from); err != nil {
			return Rule{}, err
		}
		return Rule{Text: text, kind: "import", from: from, to: pkg}, nil
	}
	from, to, ok := strings.Cut(body, "->")
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	if !ok || from == "" || to == "" {
		return Rule{}, fmt.Errorf("%q: deny: <layer> -> <layer>, deny: import \"<package>\" from <layer>, deny: new-cycle or deny: new-cross-context-edge", line)
	}
	if err := layer(from); err != nil {
		return Rule{}, err
	}
	if err := layer(to); err != nil {
		return Rule{}, err
	}
	return Rule{Text: text, kind: "layer", from: from, to: to}, nil
}

// Violation is a rule a change broke, at the symbol that broke it.
type Violation struct {
	Rule    string
	Symbol  string
	File    string
	Line    int
	Message string

	pair    string // the rule and the two packages, for a layer rule
	imports bool   // found on an import, not at a symbol
}

// verbs say what an edge does, in the findings' language.
var verbs = map[model.Rel]string{model.RelCalls: "chama", model.RelUses: "usa", model.RelImplements: "implementa",
	model.RelEmbeds: "embute", model.RelExtends: "estende", model.RelImports: "importa"}

// reach are the relations by which one layer reaches another.
var reach = map[model.Rel]bool{model.RelCalls: true, model.RelUses: true, model.RelImplements: true,
	model.RelEmbeds: true, model.RelExtends: true, model.RelImports: true}

// Scope is what a check knows of the tree and of the change: the layer and
// the context of a file — empty for one in none —, and the contexts the
// change declares, which its code may reach one another across.
type Scope struct {
	LayerOf   func(file string) string
	ContextOf func(file string) string
	Declared  []string
}

// Check holds the change d made to the graph g to the rules.
func Check(rules []Rule, d *diff.Delta, g *model.Graph, sc Scope) []Violation {
	layerOf := sc.LayerOf
	if layerOf == nil {
		layerOf = func(string) string { return "" }
	}
	units := unitLayers(g, layerOf)
	nodeLayer := func(id string) string {
		n := g.Get(id)
		if n == nil {
			return ""
		}
		if n.File != "" {
			return layerOf(n.File)
		}
		return units[n.Unit]
	}
	short := func(id string) string { return model.ShortID(id, g.Module) }
	unitOf := func(id string) string {
		if n := g.Get(id); n != nil {
			return n.Unit
		}
		return ""
	}
	var out []Violation
	// A package that imports another a layer may not reach is reported at
	// the symbol that reaches it; the import alone, only when no symbol of
	// it does — a call fast mode could not resolve.
	bySymbol := map[string]bool{}
	for _, r := range rules {
		switch r.kind {
		case "layer", "import":
			for _, e := range d.Edges {
				if !reach[e.Rel] {
					continue
				}
				from := nodeLayer(e.From)
				if e.File != "" {
					from = layerOf(e.File)
				}
				if from != r.from {
					continue
				}
				if r.kind == "import" {
					if e.Rel != model.RelImports || e.To != model.PkgID(r.to) {
						continue
					}
					out = append(out, Violation{Rule: r.Text, Symbol: e.From, File: e.File, Line: e.Line,
						Message: fmt.Sprintf("%s importa %s, que a camada %s não importa (%s)", short(e.From), r.to, r.from, r.Text)})
					continue
				}
				if nodeLayer(e.To) != r.to {
					continue
				}
				pair := r.Text + "\x00" + unitOf(e.From) + "\x00" + unitOf(e.To)
				if e.Rel != model.RelImports {
					bySymbol[pair] = true
				}
				out = append(out, Violation{Rule: r.Text, Symbol: e.From, File: e.File, Line: e.Line, pair: pair, imports: e.Rel == model.RelImports,
					Message: fmt.Sprintf("%s (%s) %s %s (%s): a camada %s não alcança a %s (%s)", short(e.From), r.from, verbs[e.Rel], short(e.To), r.to, r.from, r.to, r.Text)})
			}
		case "context":
			ctxs := unitLayers(g, contextOrNone(sc.ContextOf))
			ctxOf := func(id string) string {
				n := g.Get(id)
				if n == nil || sc.ContextOf == nil {
					return ""
				}
				if n.File != "" {
					return sc.ContextOf(n.File)
				}
				return ctxs[n.Unit]
			}
			for _, e := range d.Edges {
				if !reach[e.Rel] {
					continue
				}
				from, to := ctxOf(e.From), ctxOf(e.To)
				if from == "" || to == "" || from == to || slices.Contains(sc.Declared, from) && slices.Contains(sc.Declared, to) {
					continue
				}
				pair := r.Text + "\x00" + unitOf(e.From) + "\x00" + unitOf(e.To)
				if e.Rel != model.RelImports {
					bySymbol[pair] = true
				}
				out = append(out, Violation{Rule: r.Text, Symbol: e.From, File: e.File, Line: e.Line, pair: pair, imports: e.Rel == model.RelImports,
					Message: fmt.Sprintf("%s (contexto %s) %s %s (contexto %s): a mudança não declara %s — um contexto só alcança outro que a mudança declara, em contextos ou no depende_de (%s)", short(e.From), from, verbs[e.Rel], short(e.To), to, to, r.Text)})
			}
		case "cycle":
			for _, c := range d.Cycles {
				if !crossesUnits(g, c) {
					continue // recursion inside one package is the package's
				}
				names := make([]string, len(c))
				for i, id := range c {
					names[i] = short(id)
				}
				v := Violation{Rule: r.Text, Symbol: c[0], Message: fmt.Sprintf("ciclo novo de chamadas entre pacotes: %s (%s)", strings.Join(names, " → "), r.Text)}
				if n := g.Get(c[0]); n != nil {
					v.File, v.Line = n.File, n.Line
				}
				out = append(out, v)
			}
		}
	}
	out = slices.DeleteFunc(out, func(v Violation) bool { return v.imports && bySymbol[v.pair] })
	slices.SortStableFunc(out, func(a, b Violation) int {
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		return a.Line - b.Line
	})
	return out
}

// unitLayers is the layer of each package, from its files: a package with
// no file in a layer has none.
func unitLayers(g *model.Graph, layerOf func(string) string) map[string]string {
	out := map[string]string{}
	paths := make([]string, 0, len(g.Files))
	for p := range g.Files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		f := g.Files[p]
		if _, ok := out[f.Unit]; !ok {
			if l := layerOf(p); l != "" {
				out[f.Unit] = l
			}
		}
	}
	return out
}

func contextOrNone(f func(string) string) func(string) string {
	if f == nil {
		return func(string) string { return "" }
	}
	return f
}

func crossesUnits(g *model.Graph, cycle []string) bool {
	unit := ""
	for _, id := range cycle {
		n := g.Get(id)
		if n == nil {
			continue
		}
		if unit == "" {
			unit = n.Unit
		} else if n.Unit != unit {
			return true
		}
	}
	return false
}

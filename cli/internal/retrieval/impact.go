package retrieval

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/graph/model"
	"github.com/joaoprofile/gofi/cli/internal/graph/query"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// ViewImpact is the answer to Impact.
const ViewImpact ViewKind = "impact"

// ImpactData is what a change to some symbols can break: each dependent,
// and the contexts, packages and tests they fall in (0002, D11).
type ImpactData struct {
	Targets    []string    `json:"targets"`
	Dependents []Dependent `json:"dependents"`
	Contexts   []string    `json:"contexts"`
	Units      []string    `json:"units"`
	Tests      []string    `json:"tests"`
}

// Dependent is one symbol that depends on a target.
type Dependent struct {
	Symbol  string `json:"symbol"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Depth   int    `json:"depth"`
	Rel     string `json:"rel"`
	Via     string `json:"via"`
	Context string `json:"context,omitempty"`
	Unit    string `json:"unit,omitempty"`
	Test    bool   `json:"test,omitempty"`
}

// Impact answers "what can this change break?": every symbol that depends,
// edge by edge, on the targets — symbols, or files whose symbols are — over
// the edges of conf minConf or more. Only the project's own code: a change
// is made there.
func (e *Engine) Impact(refs []string, minConf float64) (*View, error) {
	if e.ws == nil {
		return nil, fmt.Errorf("%w: no code graph — `gofi index code`", ErrNotFound)
	}
	lp, _ := pack.Load(e.language)
	byScope := map[string][]string{}
	var scopes []string
	data := ImpactData{Targets: []string{}, Dependents: []Dependent{}, Contexts: []string{}, Units: []string{}, Tests: []string{}}
	for _, ref := range refs {
		scope, ids := e.targets(ref)
		if len(ids) == 0 {
			return nil, fmt.Errorf("%w: %q is neither a symbol nor a file of the code graph", ErrNotFound, ref)
		}
		if _, ok := byScope[scope]; !ok {
			scopes = append(scopes, scope)
		}
		byScope[scope] = append(byScope[scope], ids...)
	}
	contexts, units, tests := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, scope := range scopes {
		g, _ := e.ws.Graph(scope)
		info, _ := e.ws.Index.Scope(scope)
		root := filepath.ToSlash(info.Root)
		for _, id := range byScope[scope] {
			data.Targets = append(data.Targets, model.ShortID(id, g.Module))
		}
		for _, d := range query.Impact(g, byScope[scope], minConf, 0) {
			n := g.Get(d.ID)
			row := Dependent{Symbol: model.ShortID(d.ID, g.Module), Line: d.Line, Depth: d.Depth, Rel: d.Rel,
				Via: model.ShortID(d.Via, g.Module), Unit: model.ShortID(d.Unit, g.Module), Context: n.Context}
			if d.File != "" {
				row.File = path.Join(root, d.File)
				if row.Context == "" {
					row.Context = lp.ContextOf(d.File)
				}
				row.Test = lp.IsTest(d.File)
			}
			data.Dependents = append(data.Dependents, row)
			if row.Context != "" {
				contexts[row.Context] = true
			}
			if row.Unit != "" {
				units[row.Unit] = true
			}
			if row.Test {
				tests[row.File] = true
			}
		}
	}
	data.Contexts, data.Units, data.Tests = sortedKeys(contexts), sortedKeys(units), sortedKeys(tests)
	return &View{Kind: ViewImpact, Ref: strings.Join(refs, " "), Data: data, Text: renderImpact(data)}, nil
}

// targets are the symbols a reference names, with their scope: the symbol
// itself, or every symbol of a file — given from the project root or from
// its tree's.
func (e *Engine) targets(ref string) (string, []string) {
	if !e.isFile(ref) {
		if refs := e.resolveSymbols(ref); len(refs) > 0 {
			return refs[0].scope, []string{refs[0].id}
		}
		return "", nil
	}
	file := strings.TrimPrefix(filepath.ToSlash(ref), "./")
	for _, s := range e.ws.Index.Scopes {
		g, err := e.ws.Graph(s.Name)
		if err != nil {
			continue
		}
		rel := file
		if root := filepath.ToSlash(s.Root); root != "" && root != "." {
			if !strings.HasPrefix(file, root+"/") {
				continue
			}
			rel = strings.TrimPrefix(file, root+"/")
		}
		if ids := query.InFile(g, rel); len(ids) > 0 {
			return s.Name, ids
		}
	}
	return "", nil
}

// isFile tells a path from a symbol, whose ID may hold a slash too: a file
// is one on disk, from the project root.
func (e *Engine) isFile(ref string) bool {
	st, err := os.Stat(filepath.Join(e.root, filepath.FromSlash(ref)))
	return err == nil && !st.IsDir()
}

// verbs say what a dependent does to what it depends on.
var verbs = map[string]string{"calls": "chama", "uses": "usa", "implements": "implementa", "embeds": "embute", "extends": "estende"}

func renderImpact(d ImpactData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "impacto de %s\n", strings.Join(d.Targets, ", "))
	if len(d.Dependents) == 0 {
		b.WriteString("  nenhum símbolo do projeto depende disto no grafo\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  %d dependentes · %d contextos · %d pacotes · %d testes\n", len(d.Dependents), len(d.Contexts), len(d.Units), len(d.Tests))
	groups := map[string][]Dependent{}
	var order []string
	for _, r := range d.Dependents {
		k := r.Context
		if k == "" {
			k = "(sem contexto)"
		}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}
	slices.Sort(order)
	for _, k := range order {
		fmt.Fprintf(&b, "\n  %s (%d)\n", k, len(groups[k]))
		for _, r := range groups[k] {
			at := ""
			if r.File != "" {
				at = fmt.Sprintf("  %s:%d", r.File, r.Line)
			}
			test := ""
			if r.Test {
				test = "  [teste]"
			}
			fmt.Fprintf(&b, "    %d· %s %s %s%s%s\n", r.Depth, r.Symbol, verbs[r.Rel], r.Via, at, test)
		}
	}
	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

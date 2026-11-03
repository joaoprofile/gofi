package query

import (
	"cmp"
	"slices"

	"github.com/joaoprofile/gofi/cli/internal/graph/model"
)

// ImpactRels are the relations a change travels back along: what calls,
// uses, implements, embeds or extends a symbol depends on it (0002, D11).
// contains and imports are not dependence on a symbol: a package holds what
// changed, it does not call it.
var ImpactRels = []model.Rel{model.RelCalls, model.RelUses, model.RelImplements, model.RelEmbeds, model.RelExtends}

// Impacted is one symbol that depends on a target, and how near it is.
type Impacted struct {
	ID string `json:"id"`
	// Depth is how many edges back from the nearest target: 1, a direct
	// dependent.
	Depth int    `json:"depth"`
	Rel   string `json:"rel"`
	// Via is the symbol it depends on, one edge nearer the targets.
	Via  string `json:"via"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	Unit string `json:"unit,omitempty"`
}

// Impact is the reverse transitive closure of the targets over ImpactRels,
// edges of conf below minConf left out: every symbol a change to them can
// break, nearest first. The targets themselves are not in it; external
// symbols never are — gofi cannot change them. depth bounds the hops, 0 for
// none.
func Impact(g *model.Graph, targets []string, minConf float64, depth int) []Impacted {
	follow := map[model.Rel]bool{}
	for _, r := range ImpactRels {
		follow[r] = true
	}
	seen := map[string]bool{}
	frontier := []string{}
	for _, t := range targets {
		if g.Get(t) != nil && !seen[t] {
			seen[t] = true
			frontier = append(frontier, t)
		}
	}
	slices.Sort(frontier)
	var out []Impacted
	for d := 1; len(frontier) > 0 && (depth == 0 || d <= depth); d++ {
		var next []string
		for _, id := range frontier {
			edges := append([]*model.Edge{}, g.In(id)...)
			slices.SortFunc(edges, func(a, b *model.Edge) int { return cmp.Compare(a.From, b.From) })
			for _, e := range edges {
				if !follow[e.Rel] || e.Conf < minConf || seen[e.From] {
					continue
				}
				n := g.Get(e.From)
				if n == nil || n.External {
					continue
				}
				seen[e.From] = true
				next = append(next, e.From)
				out = append(out, Impacted{ID: n.ID, Depth: d, Rel: string(e.Rel), Via: id, File: n.File, Line: n.Line, Unit: n.Unit})
			}
		}
		slices.Sort(next)
		frontier = next
	}
	slices.SortStableFunc(out, func(a, b Impacted) int {
		return cmp.Or(cmp.Compare(a.Depth, b.Depth), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// InFile are the symbols a file declares — what a change to the file
// changes; packages, which span files, are left out.
func InFile(g *model.Graph, file string) []string {
	var out []string
	for _, n := range g.Nodes {
		if n.File == file && n.Kind != model.KindPackage && !n.External {
			out = append(out, n.ID)
		}
	}
	slices.Sort(out)
	return out
}

// At is the innermost symbol whose lines hold a line of a file: a method
// over the type that owns it; empty when none does.
func At(g *model.Graph, file string, line int) string {
	var best *model.Node
	for _, n := range g.Nodes {
		if n.File != file || n.Kind == model.KindPackage || line < n.Line || line > n.Line+max(n.Lines-1, 0) {
			continue
		}
		if best == nil || n.Lines < best.Lines || n.Lines == best.Lines && n.ID < best.ID {
			best = n
		}
	}
	if best == nil {
		return ""
	}
	return best.ID
}

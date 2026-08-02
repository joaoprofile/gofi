// Package query answers questions about the graph returning only the relevant
// neighbourhood. It is what replaces opening file after file.
package query

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Resolve finds nodes from a free-form term. It accepts a full ID, the short
// form (package.Symbol) or just the name.
func Resolve(g *model.Graph, term string) []*model.Node {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil
	}
	if n := g.Get(term); n != nil {
		return []*model.Node{n}
	}
	target := strings.ToLower(term)

	var exact, suffix, partial []*model.Node
	for _, n := range g.Nodes {
		shortID := strings.ToLower(model.ShortID(n.ID, g.Module))
		name := strings.ToLower(n.Name)
		switch {
		case shortID == target || name == target:
			exact = append(exact, n)
		case strings.HasSuffix(shortID, "."+target) || strings.HasSuffix(shortID, "/"+target):
			suffix = append(suffix, n)
		case strings.Contains(shortID, target):
			partial = append(partial, n)
		}
	}
	sortNodes := func(ns []*model.Node) {
		slices.SortFunc(ns, func(a, b *model.Node) int {
			// Internal nodes come first; then the most central; ID breaks ties.
			if a.External != b.External {
				if !a.External {
					return -1
				}
				return 1
			}
			return cmp.Or(
				cmp.Compare(b.Score, a.Score),
				cmp.Compare(a.ID, b.ID),
			)
		})
	}
	sortNodes(exact)
	sortNodes(suffix)
	sortNodes(partial)
	return append(append(exact, suffix...), partial...)
}

// Explain describes a node and all of its edges, grouped by relation.
func Explain(g *model.Graph, term string, limit int) string {
	candidates := Resolve(g, term)
	if len(candidates) == 0 {
		return fmt.Sprintf("nada encontrado para %q\n", term)
	}
	var b strings.Builder
	if len(candidates) > 1 {
		fmt.Fprintf(&b, "%d candidatos para %q, mostrando o mais central:\n", len(candidates), term)
		for i, c := range candidates[1:] {
			if i >= 6 {
				fmt.Fprintf(&b, "  ... +%d\n", len(candidates)-7)
				break
			}
			fmt.Fprintf(&b, "  %s\n", model.ShortID(c.ID, g.Module))
		}
		b.WriteString("\n")
	}
	b.WriteString(describe(g, candidates[0], limit))
	return b.String()
}

// Query finds symbols by term and shows the neighbourhood of each one.
func Query(g *model.Graph, term string, limit, maxNodes int) string {
	candidates := Resolve(g, term)
	if len(candidates) == 0 {
		return fmt.Sprintf("nada encontrado para %q\n", term)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d resultado(s) para %q\n\n", len(candidates), term)
	for i, n := range candidates {
		if i >= maxNodes {
			fmt.Fprintf(&b, "... +%d resultados. Refine o termo ou use explain.\n", len(candidates)-maxNodes)
			break
		}
		b.WriteString(describe(g, n, limit))
		b.WriteString("\n")
	}
	return b.String()
}

func describe(g *model.Graph, n *model.Node, limit int) string {
	var b strings.Builder
	short := func(id string) string { return model.ShortID(id, g.Module) }

	fmt.Fprintf(&b, "%s\n", short(n.ID))
	loc := "sem origem"
	if n.File != "" {
		loc = fmt.Sprintf("%s:%d", n.File, n.Line)
	}
	fmt.Fprintf(&b, "  %s · %s · entrada %d / saida %d · comunidade C%d\n",
		n.Kind, loc, n.InDeg, n.OutDeg, n.Community)
	if n.Context != "" {
		fmt.Fprintf(&b, "  contexto: %s (specs/%s/, .claude/memory/contexts/%s.md)\n", n.Context, n.Context, n.Context)
	}
	if n.Sig != "" {
		fmt.Fprintf(&b, "  sig: %s\n", n.Sig)
	}
	if n.Doc != "" {
		fmt.Fprintf(&b, "  doc: %s\n", n.Doc)
	}

	lines := func(title string, es []*model.Edge, outgoing bool) {
		if len(es) == 0 {
			return
		}
		byRel := map[model.Rel][]*model.Edge{}
		for _, e := range es {
			byRel[e.Rel] = append(byRel[e.Rel], e)
		}
		for _, rel := range model.AllRels {
			group := byRel[rel]
			if len(group) == 0 {
				continue
			}
			slices.SortFunc(group, func(x, y *model.Edge) int {
				return cmp.Or(
					cmp.Compare(x.To, y.To),
					cmp.Compare(x.Line, y.Line),
				)
			})
			for i, e := range group {
				if limit > 0 && i >= limit {
					fmt.Fprintf(&b, "  %s %-10s ... +%d\n", title, rel, len(group)-limit)
					break
				}
				other := e.To
				if !outgoing {
					other = e.From
				}
				origin := ""
				if e.File != "" {
					origin = fmt.Sprintf("  %s:%d", e.File, e.Line)
				}
				fmt.Fprintf(&b, "  %s %-10s %-44s%s  conf %.2f\n",
					title, rel, short(other), origin, e.Conf)
			}
		}
	}
	lines("->", g.Out(n.ID), true)
	lines("<-", g.In(n.ID), false)
	return b.String()
}

// Path finds the shortest path between two nodes and explains each hop.
func Path(g *model.Graph, from, to string) string {
	fromNodes := Resolve(g, from)
	toNodes := Resolve(g, to)
	if len(fromNodes) == 0 {
		return fmt.Sprintf("nada encontrado para %q\n", from)
	}
	if len(toNodes) == 0 {
		return fmt.Sprintf("nada encontrado para %q\n", to)
	}
	src, dst := fromNodes[0], toNodes[0]

	prev := map[string]*model.Edge{}
	seen := map[string]bool{src.ID: true}
	queue := []string{src.ID}
	found := false
	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]
		edges := append([]*model.Edge{}, g.Out(current)...)
		slices.SortFunc(edges, func(a, b *model.Edge) int { return cmp.Compare(a.To, b.To) })
		for _, e := range edges {
			if seen[e.To] {
				continue
			}
			seen[e.To] = true
			prev[e.To] = e
			if e.To == dst.ID {
				found = true
				break
			}
			queue = append(queue, e.To)
		}
	}
	var out strings.Builder
	short := func(id string) string { return model.ShortID(id, g.Module) }
	if !found {
		fmt.Fprintf(&out, "nao existe caminho de %s ate %s seguindo as arestas do grafo\n",
			short(src.ID), short(dst.ID))
		return out.String()
	}
	var path []*model.Edge
	for id := dst.ID; id != src.ID; {
		e := prev[id]
		path = append([]*model.Edge{e}, path...)
		id = e.From
	}
	fmt.Fprintf(&out, "%s\n", short(src.ID))
	for _, e := range path {
		originTxt := ""
		if e.File != "" {
			originTxt = fmt.Sprintf("  (%s:%d)", e.File, e.Line)
		}
		fmt.Fprintf(&out, "  --%s--> %s%s  conf %.2f\n", e.Rel, short(e.To), originTxt, e.Conf)
	}
	fmt.Fprintf(&out, "\n%d salto(s)\n", len(path))
	return out.String()
}

// Stats returns the numeric summary of the graph.
func Stats(g *model.Graph) string {
	s := g.Stats
	var b strings.Builder
	fmt.Fprintf(&b, "modulo      %s\n", g.Module)
	fmt.Fprintf(&b, "modo        %s\n", g.Mode)
	fmt.Fprintf(&b, "arquivos    %d (%d linhas)\n", s.Files, s.LOC)
	fmt.Fprintf(&b, "pacotes     %d\n", s.Packages)
	fmt.Fprintf(&b, "tipos       %d\n", s.Types)
	fmt.Fprintf(&b, "funcoes     %d\n", s.Funcs)
	fmt.Fprintf(&b, "metodos     %d\n", s.Methods)
	fmt.Fprintf(&b, "nos         %d\n", s.Nodes)
	fmt.Fprintf(&b, "arestas     %d (%d chamadas)\n", s.Edges, s.CallEdges)
	fmt.Fprintf(&b, "comunidades %d\n", s.Communities)
	fmt.Fprintf(&b, "ciclos      %d\n", len(g.Cycles))
	return b.String()
}

// Package analyze extracts the architectural reading of the graph: which nodes
// concentrate coupling, which groups of symbols travel together, where multiple
// recursion exists and which edges cross boundaries almost nobody crosses.
package analyze

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// countsForDegree reports whether a relation counts towards degree. Structural
// relations are ignored: "contains" only mirrors the directory tree and would
// make every package look like a giant hub.
func countsForDegree(r model.Rel) bool {
	return r != model.RelContains
}

// countsForCommunity reports whether a relation defines communities. Import is
// left out because it groups by folder, and what matters is the actual coupling
// between symbols.
func countsForCommunity(r model.Rel) bool {
	switch r {
	case model.RelCalls, model.RelUses, model.RelEmbeds, model.RelImplements:
		return true
	}
	return false
}

// Run fills in degrees, score, communities, cycles and unexpected edges.
func Run(g *model.Graph) {
	g.Index()
	degrees(g)
	detectCommunities(g)
	g.Cycles = callCycles(g)
	g.Surprises = unexpectedEdges(g)
	g.Stats.Communities = len(g.Communities)
	g.Stats.Nodes = len(g.Nodes)
	g.Stats.Edges = len(g.Edges)
}

// degrees computes fan-in, fan-out and the hub score of each node.
func degrees(g *model.Graph) {
	for _, n := range g.Nodes {
		n.InDeg, n.OutDeg, n.Score = 0, 0, 0
	}
	for _, e := range g.Edges {
		if !countsForDegree(e.Rel) {
			continue
		}
		if from := g.Get(e.From); from != nil {
			from.OutDeg++
		}
		if to := g.Get(e.To); to != nil {
			to.InDeg++
		}
	}
	for _, n := range g.Nodes {
		// Fan-in weighs more: a node that is called a lot is more central to
		// understanding the system than a node that calls a lot of things.
		n.Score = float64(2*n.InDeg + n.OutDeg)
	}
}

// Hubs returns the most central nodes, optionally filtering by kind.
func Hubs(g *model.Graph, limit int, includePackages bool) []*model.Node {
	var out []*model.Node
	for _, n := range g.Nodes {
		if n.External {
			continue
		}
		if !includePackages && n.Kind == model.KindPackage {
			continue
		}
		if includePackages && n.Kind != model.KindPackage {
			continue
		}
		if n.Score <= 0 {
			continue
		}
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b *model.Node) int {
		return cmp.Or(
			cmp.Compare(b.Score, a.Score),
			cmp.Compare(a.ID, b.ID),
		)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ---------- communities (deterministic Louvain) ----------

func detectCommunities(g *model.Graph) {
	// Only internal symbols take part: packages and external dependencies are
	// left out.
	idx := map[string]int{}
	var ids []string
	for _, n := range g.Nodes {
		if n.External || n.Kind == model.KindPackage {
			continue
		}
		ids = append(ids, n.ID)
	}
	slices.Sort(ids)
	for i, id := range ids {
		idx[id] = i
	}
	n := len(ids)
	if n == 0 {
		return
	}

	adj := make([]map[int]float64, n)
	for i := range adj {
		adj[i] = map[int]float64{}
	}
	for _, e := range g.Edges {
		if !countsForCommunity(e.Rel) {
			continue
		}
		a, ok1 := idx[e.From]
		b, ok2 := idx[e.To]
		if !ok1 || !ok2 || a == b {
			continue
		}
		// Low confidence edges weigh less in the grouping.
		w := e.Conf
		if w <= 0 {
			w = 1
		}
		adj[a][b] += w
		adj[b][a] += w
	}

	comm := louvain(adj)

	// Renumber by size, so that community 0 is always the largest.
	members := map[int][]string{}
	for i, id := range ids {
		members[comm[i]] = append(members[comm[i]], id)
	}
	keys := slices.SortedFunc(maps.Keys(members), func(x, y int) int {
		a, b := members[x], members[y]
		return cmp.Or(
			cmp.Compare(len(b), len(a)),
			cmp.Compare(a[0], b[0]),
		)
	})

	g.Communities = nil
	for newID, c := range keys {
		ms := members[c]
		slices.Sort(ms)
		pkgCount := map[string]int{}
		for _, id := range ms {
			if nd := g.Get(id); nd != nil {
				nd.Community = newID
				pkgCount[nd.Unit]++
			}
		}
		// A lone node is not a community: it simply has no coupling with anyone.
		// Counting those cases would inflate the number without saying anything.
		if len(ms) < 2 {
			continue
		}
		pkgs := slices.SortedFunc(maps.Keys(pkgCount), func(a, b string) int {
			return cmp.Or(
				cmp.Compare(pkgCount[b], pkgCount[a]),
				cmp.Compare(a, b),
			)
		})
		if len(pkgs) > 6 {
			pkgs = pkgs[:6]
		}
		label := "misc"
		if len(pkgs) > 0 {
			label = pkgs[0]
			if i := strings.LastIndex(label, "/"); i >= 0 {
				label = label[i+1:]
			}
		}
		// The most central symbols act as the showcase of the community.
		top := slices.Clone(ms)
		slices.SortFunc(top, func(x, y string) int {
			a, b := g.Get(x), g.Get(y)
			if a == nil || b == nil {
				return cmp.Compare(x, y)
			}
			return cmp.Or(
				cmp.Compare(b.Score, a.Score),
				cmp.Compare(a.ID, b.ID),
			)
		})
		if len(top) > 5 {
			top = top[:5]
		}
		g.Communities = append(g.Communities, model.Community{
			ID: newID, Label: label, Size: len(ms), Packages: pkgs, Top: top,
		})
	}
}

// louvain groups by modularity. The visit order is always ascending and ties are
// broken by the lowest index, so the result is reproducible.
func louvain(adj []map[int]float64) []int {
	n := len(adj)
	comm := make([]int, n)
	for i := range comm {
		comm[i] = i
	}
	current := adj
	mapping := make([]int, n) // original node -> node of the current level
	for i := range mapping {
		mapping[i] = i
	}

	for level := 0; level < 10; level++ {
		local, improved := localMoves(current)
		if !improved {
			break
		}
		// Propagate the level result back to the original nodes.
		for i := range mapping {
			mapping[i] = local[mapping[i]]
		}
		next := aggregate(current, local)
		if len(next) == len(current) {
			break
		}
		current = next
	}
	copy(comm, mapping)
	return comm
}

func localMoves(adj []map[int]float64) ([]int, bool) {
	n := len(adj)
	degree := make([]float64, n)
	total := 0.0
	for i := range adj {
		for _, w := range adj[i] {
			degree[i] += w
		}
		total += degree[i]
	}
	comm := make([]int, n)
	ktot := make([]float64, n)
	for i := range comm {
		comm[i] = i
		ktot[i] = degree[i]
	}
	if total == 0 {
		return comm, false
	}

	improvedAny := false
	for iter := 0; iter < 20; iter++ {
		moved := false
		for i := 0; i < n; i++ {
			ci := comm[i]
			ktot[ci] -= degree[i]
			weight := map[int]float64{}
			for j, w := range adj[i] {
				if j != i {
					weight[comm[j]] += w
				}
			}
			best, maxGain := ci, weight[ci]-ktot[ci]*degree[i]/total
			candidates := slices.Sorted(maps.Keys(weight))
			for _, c := range candidates {
				gain := weight[c] - ktot[c]*degree[i]/total
				if gain > maxGain+1e-12 {
					best, maxGain = c, gain
				}
			}
			ktot[best] += degree[i]
			if best != ci {
				comm[i] = best
				moved, improvedAny = true, true
			}
		}
		if !moved {
			break
		}
	}
	// Renumber compactly and stably.
	renum := map[int]int{}
	for i := range comm {
		if _, ok := renum[comm[i]]; !ok {
			renum[comm[i]] = len(renum)
		}
		comm[i] = renum[comm[i]]
	}
	return comm, improvedAny
}

func aggregate(adj []map[int]float64, comm []int) []map[int]float64 {
	k := 0
	for _, c := range comm {
		if c+1 > k {
			k = c + 1
		}
	}
	next := make([]map[int]float64, k)
	for i := range next {
		next[i] = map[int]float64{}
	}
	for i := range adj {
		for j, w := range adj[i] {
			a, b := comm[i], comm[j]
			if a == b {
				continue
			}
			next[a][b] += w
		}
	}
	return next
}

// ---------- cycles in the call graph ----------

// callCycles finds strongly connected components with more than one node in the
// call graph. In Go imports never form a cycle (the language forbids it), but
// mutual recursion between functions does exist and is usually exactly the point
// that makes a module hard to understand or refactor.
func callCycles(g *model.Graph) [][]string {
	adj := map[string][]string{}
	for _, e := range g.Edges {
		if e.Rel != model.RelCalls || e.Conf < 0.9 {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	order := slices.Sorted(maps.Keys(adj))
	for k := range adj {
		slices.Sort(adj[k])
	}

	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	counter := 0
	var out [][]string

	var strongConnect func(v string)
	strongConnect = func(v string) {
		index[v], low[v] = counter, counter
		counter++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, seen := index[w]; !seen {
				strongConnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] {
				if index[w] < low[v] {
					low[v] = index[w]
				}
			}
		}
		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			if len(comp) > 1 {
				slices.Sort(comp)
				out = append(out, comp)
			}
		}
	}
	for _, v := range order {
		if _, seen := index[v]; !seen {
			strongConnect(v)
		}
	}
	slices.SortFunc(out, func(a, b []string) int {
		return cmp.Or(
			cmp.Compare(len(b), len(a)),
			cmp.Compare(a[0], b[0]),
		)
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

// ---------- unexpected edges ----------

// unexpectedEdges highlights edges that cross communities through a rarely
// travelled path. These are the points where the architecture "leaks": it is
// almost always worth looking at each one of them.
func unexpectedEdges(g *model.Graph) []model.Edge {
	if len(g.Communities) < 2 {
		return nil
	}
	type pair struct{ a, b int }
	freq := map[pair]int{}
	var crossing []model.Edge
	for _, e := range g.Edges {
		if !countsForCommunity(e.Rel) {
			continue
		}
		from, to := g.Get(e.From), g.Get(e.To)
		if from == nil || to == nil || from.External || to.External {
			continue
		}
		if from.Kind == model.KindPackage || to.Kind == model.KindPackage {
			continue
		}
		if from.Community == to.Community {
			continue
		}
		p := pair{from.Community, to.Community}
		if p.a > p.b {
			p.a, p.b = p.b, p.a
		}
		freq[p]++
		crossing = append(crossing, *e)
	}
	var out []model.Edge
	for _, e := range crossing {
		from, to := g.Get(e.From), g.Get(e.To)
		p := pair{from.Community, to.Community}
		if p.a > p.b {
			p.a, p.b = p.b, p.a
		}
		// A single bridge between two communities: the most surprising case.
		if freq[p] <= 2 {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b model.Edge) int {
		return cmp.Or(
			cmp.Compare(a.From, b.From),
			cmp.Compare(a.To, b.To),
		)
	})
	if len(out) > 25 {
		out = out[:25]
	}
	return out
}

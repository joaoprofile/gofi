package docs

import (
	"sort"
	"strings"
)

// DocRef is a document as it appears in a tree listing.
type DocRef struct {
	Path     string   `json:"path"`
	Version  string   `json:"versao,omitempty"`
	Status   string   `json:"status,omitempty"`
	Entities []string `json:"entidades,omitempty"`
	Cites    []string `json:"cita,omitempty"` // documents in other contexts
}

// EntityRef is a table the context touches, and who else touches it.
type EntityRef struct {
	Name      string   `json:"nome"`
	CodeFiles int      `json:"arquivos_codigo"`
	Shared    []string `json:"tambem_em,omitempty"`
}

// NeighborContext is another context this one is coupled to, and why.
//
// Derived at query time, never stored: nobody wrote "pricing is near
// promotion", it falls out of them describing the same tables. Materializing it
// would make the graph assert something the corpus never says.
type NeighborContext struct {
	Name string `json:"nome"`
	// Entities both describe, rarest first. A table nearly every context
	// touches — the tenant column, the central product row — says almost
	// nothing about coupling; one that only two contexts touch says a lot.
	Entities []string `json:"tabelas_em_comum,omitempty"`
	// Common counts the shared tables left out of Entities for being
	// ubiquitous. Reported rather than hidden, so a low score is explainable.
	Common int `json:"tabelas_comuns"`
	Cites  int `json:"citacoes"`
	// Weight ranks neighbours: each shared table contributes the inverse of how
	// many contexts touch it.
	Weight float64 `json:"peso"`
}

// UnlinkedMention is a pair of documents that describe the same tables and
// never reference each other — the corpus equivalent of Obsidian's unlinked
// mentions. Often the first sign of a spec that should have been split, or of
// two teams writing the same rule twice.
type UnlinkedMention struct {
	A      string   `json:"a"`
	B      string   `json:"b"`
	Shared []string `json:"tabelas_em_comum"`
}

// Package is a code folder that declares it belongs to this context.
type Package struct {
	Dir   string `json:"dir"`
	Files int    `json:"arquivos"`
}

// Gap is a break in the chain a context is supposed to form: a PRD with no
// spec, a spec no code claims, a context nothing implements.
//
// Reported, never inferred as failure: a PRD still in discovery legitimately
// has no spec yet. The point is to make the hole visible, not to call it a bug.
type Gap struct {
	What string `json:"o_que"`
	Why  string `json:"porque"`
}

// ContextTree is everything reachable from one context.
type ContextTree struct {
	Name      string            `json:"contexto"`
	Packages  []Package         `json:"pacotes,omitempty"`
	Gaps      []Gap             `json:"lacunas,omitempty"`
	Specs     []DocRef          `json:"specs,omitempty"`
	PRDs      []DocRef          `json:"prd,omitempty"`
	Memory    []DocRef          `json:"memoria,omitempty"`
	Entities  []EntityRef       `json:"tabelas,omitempty"`
	Neighbors []NeighborContext `json:"vizinhos,omitempty"`
	Unlinked  []UnlinkedMention `json:"mencoes_sem_link,omitempty"`
}

// Tree assembles the neighbourhood of a context.
func (e *Explorer) Tree(name string) (*ContextTree, bool) {
	paths, ok := e.Context(name)
	if !ok {
		return nil, false
	}
	out, in := e.edges()

	ctxOf := map[string]string{}
	for id, n := range e.Graph.Nodes {
		if n.Kind == NodeDoc {
			ctxOf[id] = n.Context
		}
	}

	t := &ContextTree{Name: name}
	mine := map[string]bool{}
	for _, p := range paths {
		mine[p] = true
	}

	entities := map[string]bool{}
	for _, p := range paths {
		n := e.Graph.Nodes[p]
		ref := DocRef{Path: p, Version: n.Version, Status: n.Status}
		for _, l := range out[p] {
			switch l.Kind {
			case EdgeTouches:
				ent := l.Node[len(PrefixEntity):]
				ref.Entities = append(ref.Entities, ent)
				entities[ent] = true
			case EdgeCites, EdgeDeclares, EdgeWikilink:
				// Only links that leave the context are interesting here; the
				// ones inside it are already visible in the listing.
				if c, isDoc := ctxOf[l.Node]; isDoc && c != name {
					ref.Cites = append(ref.Cites, l.Node)
				}
			}
		}
		sort.Strings(ref.Entities)
		sort.Strings(ref.Cites)
		switch corpusOfPath(p) {
		case "specs":
			t.Specs = append(t.Specs, ref)
		case "prd":
			t.PRDs = append(t.PRDs, ref)
		default:
			t.Memory = append(t.Memory, ref)
		}
	}

	// Entities, and which other contexts describe them.
	names := make([]string, 0, len(entities))
	for ent := range entities {
		names = append(names, ent)
	}
	sort.Strings(names)
	neighborEnts := map[string]map[string]bool{}
	for _, ent := range names {
		id := PrefixEntity + ent
		ref := EntityRef{Name: ent}
		others := map[string]bool{}
		for _, l := range in[id] {
			if l.Kind != EdgeTouches {
				continue
			}
			if c := ctxOf[l.Node]; c != "" && c != name {
				others[c] = true
				if neighborEnts[c] == nil {
					neighborEnts[c] = map[string]bool{}
				}
				neighborEnts[c][ent] = true
			}
		}
		for _, l := range out[id] {
			if l.Kind == EdgeImplements {
				ref.CodeFiles++
			}
		}
		ref.Shared = keys(others)
		t.Entities = append(t.Entities, ref)
	}

	// Citation traffic with each neighbour, both directions.
	cites := map[string]int{}
	for _, p := range paths {
		for _, l := range out[p] {
			if isReference(l.Kind) {
				if c := ctxOf[l.Node]; c != "" && c != name {
					cites[c]++
				}
			}
		}
		for _, l := range in[p] {
			if isReference(l.Kind) {
				if c := ctxOf[l.Node]; c != "" && c != name {
					cites[c]++
				}
			}
		}
	}
	spread := e.entitySpread(ctxOf, in)
	totalCtx := 0
	for _, n := range e.Graph.Nodes {
		if n.Kind == NodeContext {
			totalCtx++
		}
	}
	// A table touched by more than a third of the contexts is infrastructure,
	// not a shared subject.
	ubiquitous := max(2, totalCtx/3)

	seen := map[string]bool{}
	for c := range neighborEnts {
		seen[c] = true
	}
	for c := range cites {
		seen[c] = true
	}
	for _, c := range keys(seen) {
		n := NeighborContext{Name: c, Cites: cites[c]}
		for _, ent := range keys(neighborEnts[c]) {
			if spread[ent] >= ubiquitous {
				n.Common++
			} else {
				n.Entities = append(n.Entities, ent)
			}
			n.Weight += 1 / float64(max(1, spread[ent]))
		}
		sort.Slice(n.Entities, func(i, j int) bool {
			if spread[n.Entities[i]] != spread[n.Entities[j]] {
				return spread[n.Entities[i]] < spread[n.Entities[j]]
			}
			return n.Entities[i] < n.Entities[j]
		})
		t.Neighbors = append(t.Neighbors, n)
	}
	sort.Slice(t.Neighbors, func(i, j int) bool {
		a, b := t.Neighbors[i], t.Neighbors[j]
		if a.Weight != b.Weight {
			return a.Weight > b.Weight
		}
		if a.Cites != b.Cites {
			return a.Cites > b.Cites
		}
		return a.Name < b.Name
	})

	t.Packages = e.packagesOf(name, in)
	t.Gaps = e.gapsOf(t, out, in, ctxOf)
	t.Unlinked = e.unlinked(paths, out, in)
	return t, true
}

// entitySpread counts how many distinct contexts describe each table.
func (e *Explorer) entitySpread(ctxOf map[string]string, in map[string][]Link) map[string]int {
	spread := map[string]int{}
	for id, n := range e.Graph.Nodes {
		if n.Kind != NodeEntity {
			continue
		}
		ctxs := map[string]bool{}
		for _, l := range in[id] {
			if l.Kind == EdgeTouches {
				if c := ctxOf[l.Node]; c != "" {
					ctxs[c] = true
				}
			}
		}
		spread[id[len(PrefixEntity):]] = len(ctxs)
	}
	return spread
}

// packagesOf lists the code folders that declare this context, collapsed from
// files to folders: a package is the unit someone opens, not a file.
func (e *Explorer) packagesOf(name string, in map[string][]Link) []Package {
	count := map[string]int{}
	for _, l := range in[PrefixCtx+name] {
		if l.Kind != EdgeBelongs || e.Graph.Nodes[l.Node].Kind != NodeCode {
			continue
		}
		file := l.Node[len(PrefixCode):]
		dir := file
		if i := strings.LastIndex(file, "/"); i >= 0 {
			dir = file[:i]
		}
		count[dir]++
	}
	out := make([]Package, 0, len(count))
	for dir, n := range count {
		out = append(out, Package{Dir: dir, Files: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Files != out[j].Files {
			return out[i].Files > out[j].Files
		}
		return out[i].Dir < out[j].Dir
	})
	return out
}

// gapsOf reports where the chain PRD → spec → code breaks.
//
// This is the question someone opens a context with: is it thought through,
// specified and built, or does it stop somewhere. The links are read from the
// graph, so a one-way declaration in the frontmatter is not a hole — the
// backlink exists whether or not the other document knows about it.
func (e *Explorer) gapsOf(t *ContextTree, out, in map[string][]Link, ctxOf map[string]string) []Gap {
	var gaps []Gap
	linked := func(path string) bool {
		for _, l := range append(out[path], in[path]...) {
			if isReference(l.Kind) && e.Graph.Nodes[l.Node].Kind == NodeDoc {
				return true
			}
		}
		return false
	}
	for _, d := range t.PRDs {
		if !linked(d.Path) {
			gaps = append(gaps, Gap{
				What: shortPath(d.Path),
				Why:  "PRD sem spec e sem backlink — a corrente para aqui",
			})
		}
	}
	for _, d := range t.Specs {
		if !linked(d.Path) {
			gaps = append(gaps, Gap{
				What: shortPath(d.Path),
				Why:  "spec que nenhum PRD e nenhum documento referencia",
			})
		}
	}
	if len(t.Packages) == 0 && len(t.Specs) > 0 {
		gaps = append(gaps, Gap{
			What: t.Name,
			Why:  "nenhum pacote marcado `//gofi:context " + t.Name + "` — o elo com o código só existe por inferência de tabela",
		})
	}
	return gaps
}

func shortPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// unlinked finds pairs inside the context that share tables but never point at
// each other.
func (e *Explorer) unlinked(paths []string, out, in map[string][]Link) []UnlinkedMention {
	ents := map[string]map[string]bool{}
	for _, p := range paths {
		ents[p] = map[string]bool{}
		for _, l := range out[p] {
			if l.Kind == EdgeTouches {
				ents[p][l.Node[len(PrefixEntity):]] = true
			}
		}
	}
	linked := func(a, b string) bool {
		for _, l := range out[a] {
			if l.Node == b && isReference(l.Kind) {
				return true
			}
		}
		for _, l := range in[a] {
			if l.Node == b && isReference(l.Kind) {
				return true
			}
		}
		return false
	}
	var res []UnlinkedMention
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			a, b := paths[i], paths[j]
			if linked(a, b) {
				continue
			}
			var shared []string
			for ent := range ents[a] {
				if ents[b][ent] {
					shared = append(shared, ent)
				}
			}
			// One shared table is usually just the tenant column; two or more
			// means they are describing the same thing.
			if len(shared) >= 2 {
				sort.Strings(shared)
				res = append(res, UnlinkedMention{A: a, B: b, Shared: shared})
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return len(res[i].Shared) > len(res[j].Shared) })
	return res
}

func isReference(kind string) bool { return referenceKinds[kind] }

func corpusOfPath(p string) string {
	switch {
	case len(p) > 6 && p[:6] == "specs/":
		return "specs"
	case len(p) > 4 && p[:4] == "prd/":
		return "prd"
	default:
		return "memory"
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

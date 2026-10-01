package docs

import (
	"sort"

	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// Neighbors is the local view of one node: what points at it, what it points
// to, and the code it reaches through its entities.
type Neighbors struct {
	ID       string
	Kind     string
	Incoming []Link
	Outgoing []Link
	Code     []string
}

// Link is one edge seen from a node.
type Link struct {
	Node string `json:"node"`
	Kind string `json:"kind"`
}

// Explorer answers graph questions.
type Explorer struct{ Graph *Graph }

func (e *Explorer) edges() (out, in map[string][]Link) {
	out, in = map[string][]Link{}, map[string][]Link{}
	for _, ed := range e.Graph.Edges {
		out[ed[0]] = append(out[ed[0]], Link{ed[1], ed[2]})
		in[ed[1]] = append(in[ed[1]], Link{ed[0], ed[2]})
	}
	return
}

// Resolve accepts a node ID, a bare context or entity name, or a file stem.
func (e *Explorer) Resolve(target string) string {
	if _, ok := e.Graph.Nodes[target]; ok {
		return target
	}
	for _, p := range []string{PrefixCtx, PrefixEntity} {
		if _, ok := e.Graph.Nodes[p+target]; ok {
			return p + target
		}
	}
	var match []string
	for id := range e.Graph.Nodes {
		if stemOf(id) == stemOf(target) || id == target {
			match = append(match, id)
		}
	}
	sort.Strings(match)
	if len(match) > 0 {
		return match[0]
	}
	return ""
}

// Explain returns the neighborhood of a node. Code links are separated because
// there are usually many and they answer a different question.
func (e *Explorer) Explain(id string) *Neighbors {
	node, ok := e.Graph.Nodes[id]
	if !ok {
		return nil
	}
	out, in := e.edges()
	n := &Neighbors{ID: id, Kind: node.Kind}
	for _, l := range in[id] {
		if !isCode(l.Node) {
			n.Incoming = append(n.Incoming, l)
		}
	}
	for _, l := range out[id] {
		if isCode(l.Node) {
			n.Code = append(n.Code, l.Node[len(PrefixCode):])
		} else {
			n.Outgoing = append(n.Outgoing, l)
		}
	}
	sortLinks(n.Incoming)
	sortLinks(n.Outgoing)
	sort.Strings(n.Code)
	return n
}

// Entity answers "who documents this table" and "who implements it" together —
// the one question the code graph and the document index could never answer on
// their own, because the table name is the only symbol they share.
func (e *Explorer) Entity(name string) (docs, code []string, ok bool) {
	id := PrefixEntity + name
	if _, ok := e.Graph.Nodes[id]; !ok {
		return nil, nil, false
	}
	out, in := e.edges()
	for _, l := range in[id] {
		if l.Kind == EdgeTouches {
			docs = append(docs, l.Node)
		}
	}
	for _, l := range out[id] {
		if l.Kind == EdgeImplements {
			code = append(code, l.Node[len(PrefixCode):])
		}
	}
	sort.Strings(docs)
	sort.Strings(code)
	return docs, code, true
}

// Context lists the documents of one context — the map of content for it.
func (e *Explorer) Context(name string) ([]string, bool) {
	id := PrefixCtx + name
	if _, ok := e.Graph.Nodes[id]; !ok {
		return nil, false
	}
	_, in := e.edges()
	var docs []string
	for _, l := range in[id] {
		// Code files also belong to a context now, via the gofi:context
		// directive. Only documents belong here.
		if l.Kind == EdgeBelongs && e.Graph.Nodes[l.Node].Kind == NodeDoc {
			docs = append(docs, l.Node)
		}
	}
	sort.Strings(docs)
	return docs, true
}

// Contexts lists every known context, for when the asked-for one is unknown.
func (e *Explorer) Contexts() []string {
	var out []string
	for id, n := range e.Graph.Nodes {
		if n.Kind == NodeContext {
			out = append(out, id[len(PrefixCtx):])
		}
	}
	sort.Strings(out)
	return out
}

// EntitiesLike helps when a name is close but not exact.
func (e *Explorer) EntitiesLike(part string) []string {
	var out []string
	for id, n := range e.Graph.Nodes {
		if n.Kind == NodeEntity && contains(id, part) {
			out = append(out, id[len(PrefixEntity):])
		}
	}
	sort.Strings(out)
	return out
}

// referenceKinds are the edges that mean "another document points here".
// Belonging to a context and touching an entity are not references.
var referenceKinds = map[string]bool{
	EdgeDeclares: true, EdgeCites: true, EdgeWikilink: true,
}

// Orphans lists documents nothing points to.
//
// Context memory and the reference libraries are excluded unless all is set:
// memory is an entry point by design, pointing out at specs and PRDs with
// nothing pointing back, and reference material is reached by searching, not
// by being cited. Counting either as orphaned buries the real finding under
// false alarms.
func (e *Explorer) Orphans(all bool) (orphans []string, total int) {
	_, in := e.edges()
	for id, n := range e.Graph.Nodes {
		if n.Kind != NodeDoc || (!all && (n.Corpus == layout.Contexts().Dir || isLibrary(id))) {
			continue
		}
		total++
		referenced := false
		for _, l := range in[id] {
			if referenceKinds[l.Kind] {
				referenced = true
				break
			}
		}
		if !referenced {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	return orphans, total
}

func isCode(id string) bool { return len(id) > 5 && id[:5] == PrefixCode }

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func sortLinks(l []Link) {
	sort.Slice(l, func(i, j int) bool {
		if l[i].Kind != l[j].Kind {
			return l[i].Kind < l[j].Kind
		}
		return l[i].Node < l[j].Node
	})
}

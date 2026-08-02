// Package model defines the code graph schema: nodes, typed edges and the
// canonical JSON serialization. Everything here is deterministic: the same
// codebase always produces exactly the same file.
package model

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SchemaVersion changes whenever the gofi_graph.json format breaks
// compatibility. It is the same string external extractors put in their header,
// because there is one schema, not one per producer.
const SchemaVersion = "gofi-graph/v1"

// Tool names the producer recorded in every graph.
const Tool = "gofi-graph"

// Kind is the type of entity a node represents.
type Kind string

const (
	KindPackage   Kind = "package"
	KindStruct    Kind = "struct"
	KindInterface Kind = "interface"
	KindType      Kind = "type"
	KindFunc      Kind = "func"
	KindMethod    Kind = "method"

	// Kinds below have no Go counterpart and are produced only by extractors
	// for other languages. They are listed here because the schema is shared:
	// one gofi_graph.json format, whatever produced it.
	KindClass Kind = "class"
	KindTrait Kind = "trait"
	KindEnum  Kind = "enum"
	KindField Kind = "field"
	KindConst Kind = "const"
	KindVar   Kind = "var"

	// A front end is not read usefully as functions and classes: what a reader
	// looks for is the component tree, the hooks that hold its state and the
	// services it talks to. These kinds keep that distinction instead of
	// flattening it into KindFunc.
	KindComponent Kind = "component"
	KindHook      Kind = "hook"
	KindService   Kind = "service"
)

// Rel is the type of relation an edge represents. Every edge is explicit:
// there is never a link by "similarity", only by a fact extracted from the code.
type Rel string

const (
	RelContains   Rel = "contains"   // package -> symbol declared in it
	RelImports    Rel = "imports"    // package -> imported package
	RelCalls      Rel = "calls"      // func/method -> called func/method
	RelImplements Rel = "implements" // type -> interface it satisfies
	RelEmbeds     Rel = "embeds"     // type -> embedded type
	RelUses       Rel = "uses"       // symbol -> type referenced in a field/signature
	RelExtends    Rel = "extends"    // subclass -> superclass (languages with inheritance)
)

// AllRels lists the relations in the order they appear in reports.
var AllRels = []Rel{RelContains, RelImports, RelCalls, RelImplements, RelExtends, RelEmbeds, RelUses}

// Vis is how far a symbol is visible outside the unit that declares it. Go has
// two levels and Java four, so the graph records the language's own word rather
// than a bool that would have to lie for one of them.
type Vis string

const (
	VisPublic    Vis = "public"
	VisProtected Vis = "protected"
	VisPackage   Vis = "package"
	VisPrivate   Vis = "private"
)

// Node is a code entity: a unit (package/namespace/module), a type, or a
// callable declared in one.
type Node struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	Name string `json:"name"`
	// Unit is the ID of the package, namespace or module this node belongs to.
	Unit string `json:"unit,omitempty"`
	// Owner is the type a method or field is declared on: a Go receiver, a Java
	// or C# class, a Rust impl block.
	Owner string `json:"owner,omitempty"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
	Lines int    `json:"lines,omitempty"`
	Vis   Vis    `json:"vis,omitempty"`
	// Context is the gofi context this symbol was tagged with by a
	// //gofi:context directive. Empty when the code carries no tag.
	Context  string `json:"context,omitempty"`
	External bool   `json:"external,omitempty"`
	Doc      string `json:"doc,omitempty"`
	Sig      string `json:"sig,omitempty"`

	// Filled in by the analysis phase.
	Community int     `json:"community"`
	InDeg     int     `json:"in_deg"`
	OutDeg    int     `json:"out_deg"`
	Score     float64 `json:"score"`
}

// Edge is a relation between two nodes, always carrying its origin in the code
// and a confidence level. Conf 1.0 = resolved by the type-checker; below that
// the edge came from a fast-mode heuristic.
type Edge struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	Rel  Rel     `json:"rel"`
	File string  `json:"file,omitempty"`
	Line int     `json:"line,omitempty"`
	Conf float64 `json:"conf"`
}

// Key identifies the edge uniquely (used for deduplication).
func (e Edge) Key() string {
	return e.From + "\x00" + string(e.Rel) + "\x00" + e.To
}

// IsPublic reports whether the symbol is visible outside its unit. Anything the
// extractor left unset counts as private: a symbol is exported on purpose.
func (n *Node) IsPublic() bool { return n.Vis == VisPublic }

// VisOf maps the two-level Go notion of visibility onto Vis.
func VisOf(exported bool) Vis {
	if exported {
		return VisPublic
	}
	return VisPrivate
}

// FileInfo holds the hash of each file to enable incremental updates.
type FileInfo struct {
	Path  string `json:"path"`
	Hash  string `json:"hash"`
	Unit  string `json:"unit"`
	Lines int    `json:"lines"`
}

// Community is a group of nodes densely connected to each other.
type Community struct {
	ID       int      `json:"id"`
	Label    string   `json:"label"`
	Size     int      `json:"size"`
	Packages []string `json:"packages"`
	Top      []string `json:"top"`
}

// Stats summarizes the graph in numbers.
type Stats struct {
	Packages    int `json:"packages"`
	Types       int `json:"types"`
	Funcs       int `json:"funcs"`
	Methods     int `json:"methods"`
	Files       int `json:"files"`
	LOC         int `json:"loc"`
	Nodes       int `json:"nodes"`
	Edges       int `json:"edges"`
	CallEdges   int `json:"call_edges"`
	Unresolved  int `json:"unresolved_calls"`
	Ambiguous   int `json:"ambiguous_calls"`
	Communities int `json:"communities"`
	// DurationMS describes the run, not the code, and differs on every machine.
	// It is reported on the command line and deliberately not persisted, so the
	// committed graph stays a pure function of its inputs.
	DurationMS int `json:"-"`
}

// Graph is the complete document, and the source of truth written to gofi_graph.json.
type Graph struct {
	Schema string `json:"schema"`
	Tool   string `json:"tool"`
	// Language is what the scanned tree is written in, and so which extractor
	// produced this graph.
	Language string `json:"language,omitempty"`
	// Framework is what the project declared it builds that tree with — react,
	// react-native. Empty when it declared none, which is the usual case for a
	// backend. A reader that only has the graph would otherwise have to guess it
	// from the imports.
	Framework string `json:"framework,omitempty"`
	Module    string `json:"module"`
	// Root is where the scan ran, as an absolute path on the machine that ran
	// it. Not persisted: every checkout would spell it differently. Positions
	// inside the graph are relative to it, which is what survives the trip.
	Root string `json:"-"`
	Mode string `json:"mode"`
	// No timestamp here on purpose. The graph is committed alongside the code
	// it describes, so a wall clock would change the file on every build and
	// make two developers scanning the same commit disagree — and git already
	// records when it was written, and by whom.
	Stats       Stats               `json:"stats"`
	Files       map[string]FileInfo `json:"files"`
	Communities []Community         `json:"communities"`
	Cycles      [][]string          `json:"cycles"`
	Surprises   []Edge              `json:"surprises"`
	Nodes       []*Node             `json:"nodes"`
	Edges       []*Edge             `json:"edges"`

	idx  map[string]*Node
	out  map[string][]*Edge
	in   map[string][]*Edge
	seen map[string]*Edge
}

// New creates an empty graph ready to receive nodes and edges.
func New(module, root, mode string) *Graph {
	return &Graph{
		Schema: SchemaVersion,
		Tool:   Tool,
		Module: module,
		Root:   root,
		Mode:   mode,
		Files:  map[string]FileInfo{},
		idx:    map[string]*Node{},
		seen:   map[string]*Edge{},
	}
}

// ---------- node identity ----------
//
// IDs are built so that the AST scanner and the type resolver always arrive at
// the same string for the same symbol.

func PkgID(pkg string) string        { return "pkg:" + pkg }
func TypeID(pkg, name string) string { return "type:" + pkg + "." + name }
func FuncID(pkg, name string) string { return "func:" + pkg + "." + name }
func MethodID(pkg, recv, name string) string {
	return "method:" + pkg + "." + strings.TrimPrefix(recv, "*") + "." + name
}

// ShortID returns a readable version of the ID, without the full module path.
func ShortID(id, module string) string {
	s := id
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if module != "" {
		s = strings.TrimPrefix(s, module+"/")
		s = strings.TrimPrefix(s, module+".")
	}
	return s
}

// ---------- construction ----------

// AddNode inserts a node if it does not exist yet and returns the effective node.
func (g *Graph) AddNode(n *Node) *Node {
	if g.idx == nil {
		g.idx = map[string]*Node{}
	}
	if old, ok := g.idx[n.ID]; ok {
		// An already known node is only enriched, never overwritten.
		if old.File == "" && n.File != "" {
			old.File, old.Line, old.Lines = n.File, n.Line, n.Lines
		}
		if old.Doc == "" {
			old.Doc = n.Doc
		}
		if old.Sig == "" {
			old.Sig = n.Sig
		}
		if old.Kind == KindType && n.Kind != KindType {
			old.Kind = n.Kind
		}
		return old
	}
	g.idx[n.ID] = n
	g.Nodes = append(g.Nodes, n)
	return n
}

// AddEdge inserts an edge, keeping only the highest-confidence one per relation.
func (g *Graph) AddEdge(e *Edge) {
	if e.From == e.To || e.From == "" || e.To == "" {
		return
	}
	if g.seen == nil {
		g.seen = map[string]*Edge{}
	}
	k := e.Key()
	// Constant-time dedup: the same relation between the same two nodes keeps
	// only the highest-confidence occurrence.
	if old, ok := g.seen[k]; ok {
		if e.Conf > old.Conf {
			old.Conf, old.File, old.Line = e.Conf, e.File, e.Line
		}
		return
	}
	g.seen[k] = e
	g.Edges = append(g.Edges, e)
}

// Has reports whether the node exists in the graph.
func (g *Graph) Has(id string) bool {
	_, ok := g.idx[id]
	return ok
}

// Get returns the node by ID (nil if it does not exist).
func (g *Graph) Get(id string) *Node { return g.idx[id] }

// ---------- query indexes ----------

// Index (re)builds the in-memory indexes. Called after loading from disk.
func (g *Graph) Index() {
	g.idx = make(map[string]*Node, len(g.Nodes))
	g.out = make(map[string][]*Edge, len(g.Nodes))
	g.in = make(map[string][]*Edge, len(g.Nodes))
	g.seen = make(map[string]*Edge, len(g.Edges))
	for _, n := range g.Nodes {
		g.idx[n.ID] = n
	}
	for _, e := range g.Edges {
		g.out[e.From] = append(g.out[e.From], e)
		g.in[e.To] = append(g.in[e.To], e)
		g.seen[e.Key()] = e
	}
}

// Out returns the edges leaving the node.
func (g *Graph) Out(id string) []*Edge { return g.out[id] }

// In returns the edges arriving at the node.
func (g *Graph) In(id string) []*Edge { return g.in[id] }

// ---------- persistence ----------

// Sort orders nodes and edges canonically, so that gofi_graph.json produces minimal
// git diffs between runs.
func (g *Graph) Sort() {
	slices.SortFunc(g.Nodes, func(a, b *Node) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(g.Edges, func(a, b *Edge) int {
		return cmp.Or(
			cmp.Compare(a.From, b.From),
			cmp.Compare(a.Rel, b.Rel),
			cmp.Compare(a.To, b.To),
			cmp.Compare(a.Line, b.Line),
		)
	})
}

// Save writes the graph to disk atomically. The write goes through an os.Root
// opened on the output directory, so a crafted node ID or output path can never
// make the temp file or the rename land outside it.
func (g *Graph) Save(path string) error {
	g.Sort()
	dir, name := filepath.Dir(path), filepath.Base(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(g, "", " ")
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	tmp := name + ".tmp"
	if err := root.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return root.Rename(tmp, name)
}

// Load reads a graph from disk and rebuilds the indexes.
func Load(path string) (*Graph, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	if g.Files == nil {
		g.Files = map[string]FileInfo{}
	}
	g.Index()
	return &g, nil
}

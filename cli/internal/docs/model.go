// Package docs builds and queries a retrieval index over the project's own
// documents — specs, PRDs and per-context memory.
//
// It is the document counterpart of internal/graph: graph maps the code, docs
// maps what was written about it. The two meet at the entity — a schema table
// name is the one symbol that exists in both worlds, so "which spec owns this
// table" and "which code implements it" become the same question.
//
// Both artifacts land in .gofi/docs/ and are derived: never edited by hand,
// rebuilt by 'gofi docs build'.
package docs

// Corpora are the document trees gofi indexes, in the layout gofi init creates.
var Corpora = []string{"specs", "prd", ".claude/memory/contexts"}

// Section is a heading and the line range it covers. The range is what makes
// the index worth having: an agent reads those lines, not the whole file.
type Section struct {
	Heading string `json:"h"`
	Start   int    `json:"ini"`
	End     int    `json:"fim"`
}

// Doc is one indexed document.
type Doc struct {
	Path     string    `json:"path"`
	Context  string    `json:"ctx"`
	Kind     string    `json:"tipo"`
	Status   string    `json:"status"`
	Title    string    `json:"titulo"`
	Facets   string    `json:"facetas"`
	Sections []Section `json:"secs"`
}

// Index is the searchable form of the corpus.
type Index struct {
	Schema string `json:"schema"`
	Docs   []Doc  `json:"docs"`
}

// Node kinds in the document graph.
const (
	NodeDoc      = "doc"
	NodeContext  = "contexto"
	NodeEntity   = "entidade"
	NodeCode     = "codigo"
	PrefixCtx    = "ctx:"
	PrefixEntity = "ent:"
	PrefixCode   = "code:"
)

// Edge kinds. Each is a link that already existed in the corpus and was only
// ever implicit: frontmatter cross-references, paths cited in prose, wikilinks,
// and shared entities.
const (
	EdgeDeclares   = "declara"    // frontmatter spec:/prd:
	EdgeCites      = "cita"       // a document path written in the body
	EdgeWikilink   = "wikilink"   // [[target]]
	EdgeTouches    = "toca"       // doc -> entity, from the entidades facet
	EdgeBelongs    = "pertence"   // doc -> context
	EdgeImplements = "implementa" // entity -> source file
)

// Node carries what a listing needs without opening the document.
type Node struct {
	Kind    string `json:"tipo"`
	Context string `json:"ctx,omitempty"`
	Corpus  string `json:"corpus,omitempty"`
	Status  string `json:"status,omitempty"`
	Version string `json:"versao,omitempty"`
}

// Edge is [from, to, kind]; the array form keeps the file small, and it is
// written once per rebuild and read on every query.
type Edge [3]string

// Graph is the materialized document graph.
type Graph struct {
	Schema string          `json:"schema"`
	Nodes  map[string]Node `json:"nos"`
	Edges  []Edge          `json:"arestas"`
}

const (
	IndexSchema = "gofi-docindex/v1"
	GraphSchema = "gofi-docgraph/v1"
	IndexFile   = "index.json"
	GraphFile   = "graph.json"
	OutDir      = ".gofi/docs"
)

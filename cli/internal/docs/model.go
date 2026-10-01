// Package docs builds and queries a retrieval index over the project's own
// documents — specs, PRDs, memory and the reference libraries.
//
// It is the document counterpart of internal/graph: graph maps the code, docs
// maps what was written about it. The two meet at the entity — a schema table
// name is the one symbol that exists in both worlds, so "which spec owns this
// table" and "which code implements it" become the same question.
//
// Besides the corpora it indexes the libraries — knowledge, SDK docs, the
// institutional base, project memory — so everything an agent is told to look
// up is reachable from one search.
//
// Both artifacts land in .gofi/index/docs/ and are derived: never edited by hand,
// rebuilt by 'gofi index docs'.
package docs

import (
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// Corpora are the document trees gofi indexes, in the layout gofi init creates.
// Their documents carry a frontmatter contract, which validate and migrate
// enforce.
func Corpora() []string { return []string{layout.Specs().Dir, layout.PRD().Dir, layout.Contexts().Dir} }

// Library is a reference tree searched alongside the corpora: the knowledge an
// agent is told to look up rather than load. It has no frontmatter contract —
// most of it is plain markdown, titled by its first heading — so a file is
// indexed as it is, and frontmatter keywords, when present, only sharpen it.
type Library struct {
	Dir  string // relative to the project root
	Area string // its name in `gofi find --in`
}

// Libraries are the reference trees, in the layout gofi init creates.
func Libraries() []Library {
	return []Library{
		{layout.Memory().Dir, AreaMemory}, // project.md; contexts/ is a corpus already
		{layout.Institutional().Dir, AreaInstitutional},
		{layout.Knowledge().Dir, AreaKnowledge},
		{layout.Expertise().Dir, AreaExpertise},
		{layout.SDK().Dir, AreaSDK},
		{layout.Skills().Dir, AreaSkills}, // a skill's reference files, not SKILL.md
	}
}

// Areas name where a document lives, from most to least authoritative: the
// spec states the rule, the PRD the intent, memory summarizes both, and the
// libraries hold what applies across contexts.
const (
	AreaSpecs         = "specs"
	AreaPRD           = "prd"
	AreaMemory        = "memory"
	AreaInstitutional = "institutional"
	AreaKnowledge     = "knowledge"
	AreaSDK           = "sdk"
	AreaSkills        = "skills"
	AreaExpertise     = "expertise"
)

// Areas lists every area in order of authority.
// The project's own knowledge comes before the expertise packs: where the two
// disagree, the team's learning is the one that holds here.
var Areas = []string{AreaSpecs, AreaPRD, AreaMemory, AreaInstitutional, AreaKnowledge, AreaExpertise, AreaSDK, AreaSkills}

// AreaOf names the area a project-relative path belongs to, or "" when it is
// in none.
func AreaOf(path string) string {
	switch {
	case strings.HasPrefix(path, "specs/"):
		return AreaSpecs
	case strings.HasPrefix(path, "prd/"):
		return AreaPRD
	}
	for _, l := range Libraries() {
		if strings.HasPrefix(path, l.Dir+"/") {
			return l.Area
		}
	}
	return ""
}

// isLibrary reports a path in a reference tree rather than a corpus.
func isLibrary(path string) bool {
	switch AreaOf(path) {
	case AreaSpecs, AreaPRD, "":
		return false
	case AreaMemory:
		return !strings.HasPrefix(path, layout.Contexts().Dir+"/")
	}
	return true
}

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
	// Overrides names the gofi rules a team's learning corrects, as
	// "<path>#<section>" (or a whole file) relative to the project root. Only
	// knowledge/ may declare them: the team corrects gofi, never the reverse.
	Overrides []string `json:"sobrepoe,omitempty"`
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
	IndexSchema = "gofi-docindex/v2"
	GraphSchema = "gofi-docgraph/v1"
	IndexFile   = "index.json"
	GraphFile   = "graph.json"
	OutDir      = layout.DocsDir
)

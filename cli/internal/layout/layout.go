// Package layout is where gofi keeps what it derives from a project, and the
// record of when it did.
//
// Everything a query answers from lives under one directory, .gofi/index/, one
// subdirectory per target: code for the graphs, docs for the document index.
// One root means one thing to ignore, to clean and to look for, and a target
// added later gets a sibling instead of a new top-level name. The extractors
// are tools, not derived data, so they sit beside it rather than inside.
package layout

// Paths relative to the project root, in slash form.
const (
	IndexDir      = ".gofi/index"
	CodeDir       = IndexDir + "/code"
	DocsDir       = IndexDir + "/docs"
	ManifestFile  = IndexDir + "/manifest.json"
	ExtractorsDir = ".gofi/extractors"
)

// Targets the index is built for, in build order.
const (
	TargetCode = "code"
	TargetDocs = "docs"
)

// IgnoreRules keep .gofi/ out of git except for the code graph.
//
// The code graph is versioned on purpose: it is the map the agents read, so the
// team and CI should share the one the code actually produced instead of each
// rebuilding their own. The document index is not — it is rebuilt in a second
// from files that are already versioned, and a shared copy would conflict on
// every pair of edits to two specs. The manifest records one machine's builds.
//
// Excluding a directory and re-including a child of it needs the `.gofi/*`
// spelling — a plain `.gofi/` stops git from ever descending, and the negation
// below would never be reached.
var IgnoreRules = []string{
	".gofi/*",
	"!" + IndexDir + "/",
	DocsDir + "/",
	ManifestFile,
}

// LegacyIgnoreRules are the lines older releases wrote, and the one some
// projects added by hand to version the document index. They are taken out when
// the rules are rewritten, so the two sets never compete over the same paths.
var LegacyIgnoreRules = []string{
	".gofi/",
	"!" + legacyDocsDir + "/",
	legacyDocsDir + "/",
	"!" + legacyGraphDir + "/",
	legacyGraphDir + "/extractors/",
}

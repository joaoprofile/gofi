// Package external ingests code graphs produced by extractors that gofi does
// not implement itself.
//
// gofi extracts Go natively. Every other language is handled by a separate
// program that gofi runs as a subprocess and talks to over a line-oriented
// protocol on stdout. That boundary is deliberate: an extractor for Java or
// Rust needs that language's own parser, which would mean either linking a C
// library (the CLI is built CGO_ENABLED=0) or embedding a WASM runtime (a
// dependency the CLI does not otherwise carry). A subprocess costs one process
// spawn and keeps the CLI a single static binary.
//
// # The contract
//
// An extractor is any executable that:
//
//   - accepts `--root <dir>` and `--mode fast|deep`, and may ignore --mode;
//   - writes one JSON object per line to stdout, described below;
//   - writes anything else — logs, progress, panics — to stderr;
//   - exits 0 on success.
//
// Lines are JSON objects discriminated by the "rec" field. Records may appear
// in any order after the header, and a reader must ignore both unknown "rec"
// values and unknown fields, so the protocol can grow without breaking older
// extractors or older gofi builds.
//
//	{"rec":"header","schema":"gofi-graph/v1","language":"java","module":"com.acme.app","tool":"gofi-graph-java 0.1.0","mode":"deep"}
//	{"rec":"node","id":"java:com.acme.api","kind":"package","name":"com.acme.api","file":"src/main/java/com/acme/api","lines":420}
//	{"rec":"node","id":"java:com.acme.api.Server","kind":"class","name":"Server","unit":"java:com.acme.api","file":"src/main/java/com/acme/api/Server.java","line":12,"vis":"public","sig":"class Server implements Handler","doc":"HTTP entry point."}
//	{"rec":"node","id":"java:com.acme.api.Server#start","kind":"method","name":"start","unit":"java:com.acme.api","owner":"Server","file":"src/main/java/com/acme/api/Server.java","line":31,"vis":"public"}
//	{"rec":"edge","from":"java:com.acme.api","to":"java:com.acme.api.Server","rel":"contains","file":"src/main/java/com/acme/api/Server.java","line":12,"conf":1}
//	{"rec":"edge","from":"java:com.acme.api.Server#start","to":"java:com.acme.store.Repo#find","rel":"calls","file":"src/main/java/com/acme/api/Server.java","line":34,"conf":1}
//	{"rec":"diag","severity":"warn","msg":"3 call sites skipped: dynamic dispatch"}
//	{"rec":"summary","files":128,"loc":9310,"unresolved":3,"ambiguous":0}
//
// # Rules an extractor must follow
//
// Node IDs are opaque strings and must be prefixed with the language, so that a
// polyglot repository — a Go backend next to a TypeScript frontend — cannot
// collide. IDs must be stable across runs: the same code must produce the same
// ID, or every rebuild becomes a diff.
//
// Every edge carries "conf", the confidence that the relation is real: 1 means
// resolved against a type system, below that means a heuristic. An extractor
// that cannot resolve a call must not guess — it should skip the edge and count
// it in "unresolved". A wrong edge is worse than a missing one, because the
// agent reading the graph trusts it.
//
// Positions ("file", "line") are relative to --root and should be present on
// every node and every edge that has one. They are what lets a reader jump from
// the graph to the code.
package external

import (
	"fmt"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Protocol is the wire format this package speaks. The major version is the
// compatibility boundary: gofi accepts any "gofi-graph/v1.x" extractor.
const Protocol = "gofi-graph/v1"

// Record discriminators.
const (
	recHeader  = "header"
	recNode    = "node"
	recEdge    = "edge"
	recDiag    = "diag"
	recSummary = "summary"
)

// line is one decoded record. The wire format is flat rather than nested so an
// extractor in any language can emit it with a single object per line; the cost
// is this one wide struct, which is never exposed outside the package.
type line struct {
	Rec string `json:"rec"`

	// header
	Schema   string `json:"schema"`
	Language string `json:"language"`
	Module   string `json:"module"`
	Tool     string `json:"tool"`
	Mode     string `json:"mode"`

	// node
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Unit    string `json:"unit"`
	Owner   string `json:"owner"`
	Vis     string `json:"vis"`
	Context string `json:"ctx"`
	Sig     string `json:"sig"`
	Doc     string `json:"doc"`
	Lines   int    `json:"lines"`

	// edge
	From string   `json:"from"`
	To   string   `json:"to"`
	Rel  string   `json:"rel"`
	Conf *float64 `json:"conf"`

	// node and edge
	File string `json:"file"`
	Line int    `json:"line"`

	// diag
	Severity string `json:"severity"`
	Msg      string `json:"msg"`

	// summary
	Files      int `json:"files"`
	LOC        int `json:"loc"`
	Unresolved int `json:"unresolved"`
	Ambiguous  int `json:"ambiguous"`
}

// Diagnostic is a message an extractor emitted about its own limits — a
// construct it could not resolve, a file it skipped. These are surfaced to the
// user rather than swallowed, because they explain gaps in the graph.
type Diagnostic struct {
	Severity string
	Message  string
}

func (d Diagnostic) String() string { return d.Severity + ": " + d.Message }

// wireKinds maps the protocol's language-neutral node kinds onto the model.
// The protocol vocabulary is deliberately wider than Go's: an extractor should
// describe what the language actually has (a Java class, a Rust trait) instead
// of forcing it into Go's shape.
var wireKinds = map[string]model.Kind{
	"package":   model.KindPackage,
	"module":    model.KindPackage,
	"namespace": model.KindPackage,
	"struct":    model.KindStruct,
	"class":     model.KindClass,
	"interface": model.KindInterface,
	"trait":     model.KindTrait,
	"protocol":  model.KindInterface,
	"enum":      model.KindEnum,
	"type":      model.KindType,
	"func":      model.KindFunc,
	"method":    model.KindMethod,
	"field":     model.KindField,
	"const":     model.KindConst,
	"var":       model.KindVar,
}

var wireRels = map[string]model.Rel{
	"contains":   model.RelContains,
	"imports":    model.RelImports,
	"calls":      model.RelCalls,
	"implements": model.RelImplements,
	"extends":    model.RelExtends,
	"embeds":     model.RelEmbeds,
	"uses":       model.RelUses,
}

// checkSchema accepts any minor revision of the major version we speak.
func checkSchema(got string) error {
	if got == "" {
		return fmt.Errorf("header sem campo schema (esperado %q)", Protocol)
	}
	major := func(s string) string {
		s = strings.TrimSpace(s)
		if i := strings.LastIndex(s, "."); i > strings.Index(s, "/") {
			return s[:i]
		}
		return s
	}
	if major(got) != major(Protocol) {
		return fmt.Errorf("schema %q incompativel (esta CLI fala %q)", got, Protocol)
	}
	return nil
}

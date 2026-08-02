package graph

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/joaoprofile/gofi-cli/internal/graph/extract"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/tsjs"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Extractor turns a source tree into a graph. Go is built in; every other
// language is an executable behind this same interface, so teaching gofi a
// language it can read natively is a registration rather than an edit to Build.
type Extractor interface {
	// Language is the code this extractor answers to, lowercase.
	Language() string
	// Extract produces the graph and nothing else. Analysis, report rendering
	// and writing to disk are the same whatever the language, and stay in Build.
	Extract(ctx context.Context, root string, opt BuildOptions) (*Extraction, error)
}

// Incremental is the optional half of Extractor: an extractor that can tell
// cheaply whether a previous graph is still current. Build uses it to honour
// --update; one that does not implement it always rescans.
type Incremental interface {
	// Unchanged reports the file count it considered and whether prev still
	// describes the tree.
	Unchanged(prev *model.Graph, root string, opt BuildOptions) (int, bool)
}

// Extraction is what an extractor hands back.
type Extraction struct {
	Graph       *model.Graph
	Diagnostics []external.Diagnostic
	DiagCount   int
}

// Registry resolves a language to its extractor.
type Registry struct {
	mu     sync.RWMutex
	native map[string]Extractor
}

// Extractors is the registry Build uses.
var Extractors = NewRegistry(
	goExtractor{},
	tsjsExtractor{lang: LangTypeScript},
	tsjsExtractor{lang: LangJavaScript},
)

// NewRegistry returns a registry holding the given native extractors.
func NewRegistry(exs ...Extractor) *Registry {
	r := &Registry{native: map[string]Extractor{}}
	for _, e := range exs {
		r.Register(e)
	}
	return r
}

// Register adds a native extractor, replacing any previous one for its language.
func (r *Registry) Register(e Extractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.native[e.Language()] = e
}

// For returns the extractor for a language. An unregistered language is not an
// error: it resolves to the external protocol, which is how a language gofi
// cannot read natively is meant to arrive.
func (r *Registry) For(lang string) Extractor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.native[lang]; ok {
		return e
	}
	return externalExtractor{lang: lang}
}

// Native lists the languages gofi reads without an external extractor.
func (r *Registry) Native() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.native))
}

// goExtractor reads Go with go/ast (fast) or go/types (deep).
type goExtractor struct{}

func (goExtractor) Language() string { return LangGo }

func (goExtractor) Extract(_ context.Context, root string, opt BuildOptions) (*Extraction, error) {
	var (
		g   *model.Graph
		err error
	)
	if opt.Deep {
		g, err = extract.Deep(scanOptions(root, opt))
	} else {
		g, err = extract.Fast(scanOptions(root, opt))
	}
	if err != nil {
		return nil, err
	}
	return &Extraction{Graph: g}, nil
}

// Unchanged hashes the files and compares. Hashing is milliseconds where
// parsing is not, which is what makes --update cheap enough for a git hook.
func (goExtractor) Unchanged(prev *model.Graph, root string, opt BuildOptions) (int, bool) {
	current, err := extract.Fingerprint(scanOptions(root, opt))
	if err != nil {
		return 0, false
	}
	return len(current), prev.Mode == opt.Mode() && sameFingerprints(prev.Files, current)
}

func scanOptions(root string, opt BuildOptions) extract.Options {
	return extract.Options{
		Root:       root,
		Deep:       opt.Deep,
		WithTests:  opt.WithTests,
		Exclude:    opt.Exclude,
		MaxFileKB:  opt.MaxFileKB,
		ShowErrors: opt.Log != nil,
	}
}

// tsjsExtractor reads a TypeScript or JavaScript tree syntactically.
//
// Both languages resolve to the same code: they share the syntax of everything
// that reaches the graph, and a tree usually holds both. They stay two
// registrations because the language a project declares is the one the graph
// must be stamped with.
type tsjsExtractor struct{ lang string }

func (e tsjsExtractor) Language() string { return e.lang }

func (e tsjsExtractor) Extract(_ context.Context, root string, opt BuildOptions) (*Extraction, error) {
	// Deep mode would mean running tsc, and the whole point of this extractor is
	// to need no toolchain. The flag is accepted and the scan stays syntactic.
	g, err := tsjs.Fast(e.scanOptions(root, opt))
	if err != nil {
		return nil, err
	}
	return &Extraction{Graph: g}, nil
}

func (e tsjsExtractor) Unchanged(prev *model.Graph, root string, opt BuildOptions) (int, bool) {
	current, err := tsjs.Fingerprint(e.scanOptions(root, opt))
	if err != nil {
		return 0, false
	}
	return len(current), sameFingerprints(prev.Files, current)
}

func (e tsjsExtractor) scanOptions(root string, opt BuildOptions) tsjs.Options {
	return tsjs.Options{
		Root:      root,
		Language:  e.lang,
		WithTests: opt.WithTests,
		Exclude:   opt.Exclude,
		MaxFileKB: opt.MaxFileKB,
	}
}

// externalExtractor speaks the line protocol to a third-party executable.
//
// It deliberately does not implement Incremental: fingerprinting here is a Go
// file hash, and only the extractor knows which files its language compiles.
type externalExtractor struct{ lang string }

func (e externalExtractor) Language() string { return e.lang }

func (e externalExtractor) Extract(ctx context.Context, root string, opt BuildOptions) (*Extraction, error) {
	spec, err := external.Find(opt.ExtractorsRoot(root), e.lang)
	if err != nil {
		return nil, err
	}
	opt.logger().Debug("running external extractor", "language", e.lang, "path", spec.Path, "mode", opt.Mode())

	run, err := external.Run(ctx, spec, external.RunOptions{
		Root:    root,
		Deep:    opt.Deep,
		Timeout: opt.Timeout,
		Limits:  external.DefaultLimits(),
	})
	if err != nil {
		return nil, err
	}
	// The native scan counts these as it walks; a decoded graph has nobody to
	// do it, so they are derived from the nodes the extractor sent.
	countKinds(run.Graph)
	return &Extraction{Graph: run.Graph, Diagnostics: run.Diagnostics, DiagCount: run.DiagCount}, nil
}

// countKinds fills the per-kind counters from the nodes themselves. Kinds with
// no Go counterpart are folded into the closest counter, so the summary line
// reads the same whatever language produced the graph.
func countKinds(g *model.Graph) {
	g.Stats.Packages, g.Stats.Types, g.Stats.Funcs, g.Stats.Methods = 0, 0, 0, 0
	for _, n := range g.Nodes {
		switch n.Kind {
		case model.KindPackage:
			g.Stats.Packages++
		case model.KindStruct, model.KindInterface, model.KindType,
			model.KindClass, model.KindTrait, model.KindEnum, model.KindService:
			g.Stats.Types++
		case model.KindFunc, model.KindComponent, model.KindHook:
			g.Stats.Funcs++
		case model.KindMethod:
			g.Stats.Methods++
		}
	}
}

func sameFingerprints(a, b map[string]model.FileInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if o, ok := b[k]; !ok || o.Hash != v.Hash {
			return false
		}
	}
	return true
}

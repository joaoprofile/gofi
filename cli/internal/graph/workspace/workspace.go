// Package workspace builds and reads the several graphs a gofi project has.
//
// A project is not one code base. There is the backend the team writes, the
// front-end and mobile trees beside it, and the vendored SDK under
// .gofi/gofi-sdk-<lang>/ that the backend is written against. Which folders
// those are is declared in .gofi.yaml, never assumed.
//
// Each is a scope with a graph of its own. One combined graph is not an option:
// they are different languages, so no single extractor could read them, and the
// SDK's internals would drown the project's own structure. Leaving any of them
// out is not either, because then a call into it dead-ends. So they are built
// separately, and a lookup that misses in one falls through to the next.
package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Scope names.
const (
	ScopeProject = "project"
	ScopeSDK     = "sdk"
)

// Surface is a source folder the project declares beyond its backend — the web
// and mobile trees — paired with the language an extractor reads it with. They
// are separate scopes rather than one scan of the root because they are separate
// languages: a single graph could not be extracted by a single extractor.
type Surface struct {
	Name      string // scope name, e.g. "frontend"
	Dir       string // relative to the project root
	Language  string
	Framework string // react, react-native; recorded in the graph
}

// SDKDirPrefix is where `gofi init` vendors the SDK, relative to the project
// root; the language is appended.
const SDKDirPrefix = ".gofi/gofi-sdk-"

// Scope is one independently buildable graph.
type Scope struct {
	Name      string // ScopeProject, ScopeSDK or a declared surface
	Language  string
	Framework string
	Root      string // absolute directory to scan
	Dir       string // absolute directory the graph is written to
	// Key changes when the scope's inputs change in a way gofi knows about
	// without scanning — the SDK commit, for instance. An unchanged key plus a
	// graph already on disk means the build can be skipped outright.
	Key string
}

// Options controls a workspace build.
type Options struct {
	Root     string // project root
	Language string // the project's language
	// SrcDir is the subdirectory holding the project's own code. Empty scans
	// the whole root, which then includes the vendored SDK; projects that keep
	// their code under src/ get a graph of just their code.
	SrcDir string
	// Surfaces are the other source folders the project declares, front-end and
	// mobile. Each becomes a scope of its own, and only if an extractor for its
	// language is installed — a project with no TypeScript extractor is a
	// project with no front-end graph, not a failing build.
	Surfaces []Surface
	// SDKKey is the SDK checkout's identity, normally the resolved commit. When
	// it matches what the index recorded, the SDK scope is left alone.
	SDKKey string
	// SkipSDK builds only the project scope.
	SkipSDK bool
	// SkipProject leaves out the project scope. A repository that is only a front
	// end has no backend tree to scan, and scanning its root as the project's
	// language would produce an empty graph next to the real one.
	SkipProject bool

	Deep      bool
	WithTests bool
	Exclude   []string
	MaxFileKB int
	NoHTML    bool
	Update    bool
	Timeout   time.Duration
	Log       *slog.Logger
}

func (o Options) logger() *slog.Logger {
	if o.Log != nil {
		return o.Log
	}
	return slog.New(slog.DiscardHandler)
}

// Scopes returns the scopes that exist for a project, project first. The SDK
// scope only appears when the checkout is actually there: a project without an
// SDK override is a perfectly normal project, not a broken one.
func Scopes(opt Options) []Scope {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		root = opt.Root
	}
	lang := (graph.BuildOptions{Language: opt.Language}).Lang()
	base := graph.Dir(root, lang)

	src := root
	if opt.SrcDir != "" {
		src = filepath.Join(root, opt.SrcDir)
	}
	var scopes []Scope
	if !opt.SkipProject {
		scopes = append(scopes, Scope{Name: ScopeProject, Language: lang, Root: src, Dir: base})
	}

	for _, s := range opt.Surfaces {
		dir := filepath.Join(root, s.Dir)
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		if !canExtract(root, s.Language) {
			continue
		}
		scopes = append(scopes, Scope{
			Name: s.Name, Language: s.Language, Framework: s.Framework, Root: dir,
			Dir: filepath.Join(base, s.Name),
		})
	}

	if opt.SkipSDK {
		return scopes
	}
	sdk := filepath.Join(root, SDKDirPrefix+lang)
	if fi, err := os.Stat(sdk); err == nil && fi.IsDir() {
		scopes = append(scopes, Scope{
			Name: ScopeSDK, Language: lang, Root: sdk,
			Dir: filepath.Join(base, ScopeSDK), Key: opt.SDKKey,
		})
	}
	return scopes
}

// canExtract reports whether this project can read a language at all. The
// registry is asked rather than a list of names: a language gofi learns to read
// natively must stop requiring an installed extractor on the same day, not on
// the day someone remembers to edit this function.
func canExtract(projectRoot, lang string) bool {
	if lang == "" || slices.Contains(graph.Extractors.Native(), lang) {
		return true
	}
	_, err := external.Find(projectRoot, lang)
	return err == nil
}

// ScopeResult is what building one scope produced.
type ScopeResult struct {
	Scope   Scope
	Result  *graph.Result
	Skipped bool // the key matched the index, so nothing ran
	Err     error
}

// Result is the outcome of a workspace build.
type Result struct {
	Scopes []ScopeResult
	Index  *Index
}

// Built returns the scopes that produced a graph, skipped or not.
func (r *Result) Built() []ScopeResult {
	out := make([]ScopeResult, 0, len(r.Scopes))
	for _, s := range r.Scopes {
		if s.Err == nil {
			out = append(out, s)
		}
	}
	return out
}

// Build builds every scope of the project, in parallel, and writes the index.
//
// Scopes do not share state, so one failing does not stop the others: the
// returned error joins whatever went wrong, and Result still carries the scopes
// that succeeded. A caller running best-effort — `gofi init` — can write what it
// got and report the rest.
func Build(ctx context.Context, opt Options) (*Result, error) {
	scopes := Scopes(opt)
	prev, _ := LoadIndex(opt.Root, opt.Language)
	log := opt.logger()

	results := make([]ScopeResult, len(scopes))
	var wg sync.WaitGroup
	for i, sc := range scopes {
		wg.Go(func() {
			results[i] = buildScope(ctx, sc, opt, prev, log)
		})
	}
	wg.Wait()

	res := &Result{Scopes: results, Index: newIndex(opt, results, prev)}
	// An index with no scopes describes nothing. Writing it would leave a file
	// git tracks behind a build that failed outright, and would delete a usable
	// index the next time a scan cannot run.
	if len(res.Index.Scopes) > 0 {
		if err := res.Index.Save(opt.Root, opt.Language); err != nil {
			log.Debug("could not write the graph index", "err", err)
		}
	}

	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("escopo %s: %w", r.Scope.Name, r.Err))
		}
	}
	return res, errors.Join(errs...)
}

func buildScope(ctx context.Context, sc Scope, opt Options, prev *Index, log *slog.Logger) ScopeResult {
	// A key is a promise from the caller that nothing changed. Trust it only
	// when a graph is actually on disk to go with it.
	if sc.Key != "" && prev != nil {
		if e, ok := prev.Scope(sc.Name); ok && e.Key == sc.Key {
			if g, err := model.Load(filepath.Join(sc.Dir, graph.GraphFile)); err == nil {
				log.Debug("scope is current", "scope", sc.Name, "key", sc.Key)
				return ScopeResult{Scope: sc, Skipped: true, Result: &graph.Result{
					Graph: g, Dir: sc.Dir, Unchanged: true, Files: g.Stats.Files,
				}}
			}
		}
	}

	r, err := graph.Build(ctx, graph.BuildOptions{
		Root: sc.Root,
		// A scope scans its own folder, but the extractors are installed once,
		// at the project root.
		ProjectRoot: opt.Root,
		Out:         sc.Dir,
		Language:    sc.Language,
		Framework:   sc.Framework,
		// Every scope here is listed by name in one index, so a reader reaches
		// it through that index's language and not through its own.
		IndexLang: (graph.BuildOptions{Language: opt.Language}).Lang(),
		Deep:        opt.Deep,
		WithTests:   opt.WithTests,
		Exclude:     opt.Exclude,
		MaxFileKB:   opt.MaxFileKB,
		NoHTML:      opt.NoHTML,
		Update:      opt.Update,
		Timeout:     opt.Timeout,
		Log:         opt.Log,
	})
	return ScopeResult{Scope: sc, Result: r, Err: err}
}

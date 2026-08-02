// Package graph builds and locates the code graph of a project. The heavy
// lifting lives in the subpackages — extract reads the source, analyze derives
// structure, report renders it — and this file is the orchestration the CLI and
// `gofi init` both call.
package graph

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/graph/analyze"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
	"github.com/joaoprofile/gofi-cli/internal/graph/report"
)

// Languages gofi reads with an extractor compiled into the binary. Everything
// else goes through an external extractor.
const (
	LangGo         = "go"
	LangTypeScript = "typescript"
	LangJavaScript = "javascript"
)

// OutDir is where the graph is written, relative to the project root. It sits
// under .gofi/ with the other derived artifacts (horusec, sonar).
const OutDir = ".gofi/graph"

// File names inside OutDir. Everything gofi generates carries the gofi_ prefix,
// so an artifact is never mistaken for something the project itself wrote, and
// a single `gofi_graph*` glob matches the whole set.
const (
	GraphFile  = "gofi_graph.json"
	ReportFile = "gofi_graph_report.md"
	HTMLFile   = "gofi_graph.html"
)

// BuildOptions controls one graph build.
type BuildOptions struct {
	Root string // project root to scan
	Out  string // output directory; relative paths resolve against Root
	// ProjectRoot is where installed extractors are looked up. Empty means Root,
	// which is right whenever the scan covers the whole project. A scope that
	// scans one folder of it — the front-end tree — has to point back at the
	// project, because that is where `gofi graph install` put the binary.
	ProjectRoot string
	Language    string // "" or "go" for the native extractor, otherwise an external one
	// Framework the project declared for this tree — react, react-native. It is
	// recorded in the graph, not acted on: the extractor is chosen by language.
	Framework string
	Deep      bool          // use go/types: exact calls and interface implementations
	WithTests bool          // include the language's test files
	Exclude   []string      // directory glob patterns to skip
	MaxFileKB int           // skip files bigger than this (0 = no limit)
	NoHTML    bool          // skip the HTML visualization
	Update    bool          // rebuild only if some source file changed
	Timeout   time.Duration // deadline for an external extractor; 0 means none
	Log       *slog.Logger
}

// Result reports what a build produced.
type Result struct {
	Graph       *model.Graph
	Dir         string   // absolute output directory
	Outputs     []string // file names written, in write order
	Unchanged   bool     // Update was set and nothing had changed, so nothing was written
	Files       int      // source files considered
	Diagnostics []external.Diagnostic
	DiagCount   int // diagnostics emitted, including those past the kept limit
}

// Lang is the language this build targets, normalized.
func (o BuildOptions) Lang() string {
	l := strings.ToLower(strings.TrimSpace(o.Language))
	if l == "" {
		return LangGo
	}
	return l
}

// ExtractorsRoot is the project whose installed extractors this build may use.
func (o BuildOptions) ExtractorsRoot(scanRoot string) string {
	if o.ProjectRoot == "" {
		return scanRoot
	}
	return o.ProjectRoot
}

// Mode is the scan mode name recorded in the graph.
func (o BuildOptions) Mode() string {
	if o.Deep {
		return "deep"
	}
	return "fast"
}

func (o BuildOptions) logger() *slog.Logger {
	if o.Log != nil {
		return o.Log
	}
	return slog.New(slog.DiscardHandler)
}

// Build scans the project and writes gofi_graph.json, gofi_graph_report.md and
// (unless disabled) gofi_graph.html into the output directory.
//
// For a language other than Go the scan is delegated to an external extractor
// and the output lands in a subdirectory named after the language, so a
// polyglot repository keeps one graph per language instead of overwriting.
func Build(ctx context.Context, opt BuildOptions) (*Result, error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return nil, err
	}
	lang := opt.Lang()
	out := outDir(root, opt.Out, lang)
	log := opt.logger()
	ex := Extractors.For(lang)

	// --update is what makes it cheap to hang this off a git hook, but only an
	// extractor that can compare cheaply gets to answer.
	if inc, ok := ex.(Incremental); ok && opt.Update {
		if prev, err := model.Load(filepath.Join(out, GraphFile)); err == nil {
			if files, unchanged := inc.Unchanged(prev, root, opt); unchanged {
				log.Debug("graph is current", "files", files)
				return &Result{Graph: prev, Dir: out, Unchanged: true, Files: files}, nil
			}
		}
	}

	start := time.Now()
	log.Debug("scanning", "root", root, "language", lang, "mode", opt.Mode())

	ext, err := ex.Extract(ctx, root, opt)
	if err != nil {
		return nil, err
	}
	g := ext.Graph
	g.Root = root
	// The language comes from the extractor, which knows it; the framework only
	// from the project, which declared it.
	g.Framework = opt.Framework
	analyze.Run(g)
	g.Stats.DurationMS = int(time.Since(start).Milliseconds())

	res := &Result{
		Graph:       g,
		Dir:         out,
		Files:       g.Stats.Files,
		Diagnostics: ext.Diagnostics,
		DiagCount:   ext.DiagCount,
	}
	if err := writeOutputs(g, out, opt.NoHTML, res); err != nil {
		return nil, err
	}
	log.Debug("graph written", "dir", out, "nodes", g.Stats.Nodes, "edges", g.Stats.Edges)
	return res, nil
}

// outDir resolves the directory a build writes to. An empty Out means the
// default layout: Go sits at the top so the common case has no extra level, and
// every other language gets a directory of its own, so a polyglot repository
// keeps one graph per language instead of overwriting. An Out given explicitly
// is used as it is — the caller has already decided where the graph goes.
func outDir(root, out, lang string) string {
	if out == "" {
		out = OutDir
		if lang != LangGo {
			out = filepath.Join(out, lang)
		}
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}
	return out
}

func writeOutputs(g *model.Graph, dir string, noHTML bool, res *Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := g.Save(filepath.Join(dir, GraphFile)); err != nil {
		return err
	}
	res.Outputs = append(res.Outputs, GraphFile)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	if err := root.WriteFile(ReportFile, []byte(report.Markdown(g)), 0o644); err != nil {
		return err
	}
	res.Outputs = append(res.Outputs, ReportFile)

	if noHTML {
		return nil
	}
	html, err := report.HTML(g)
	if err != nil {
		return err
	}
	if err := root.WriteFile(HTMLFile, []byte(html), 0o644); err != nil {
		return err
	}
	res.Outputs = append(res.Outputs, HTMLFile)
	return nil
}

// Dir returns the graph output directory for a project root.
func Dir(projectRoot, language string) string {
	return outDir(projectRoot, "", (BuildOptions{Language: language}).Lang())
}

// Load reads the graph written under a project root.
func Load(projectRoot, language string) (*model.Graph, error) {
	path := filepath.Join(Dir(projectRoot, language), GraphFile)
	g, err := model.Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("grafo ausente em %s — rode `%s`", filepath.Dir(path), buildHint(language))
		}
		return nil, err
	}
	return g, nil
}

// Open hands the HTML visualization to the system's default browser.
func Open(projectRoot, language string) error {
	path := filepath.Join(Dir(projectRoot, language), HTMLFile)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("visualizacao ausente em %s — rode `%s`", path, buildHint(language))
	}
	return openInBrowser(path)
}

func buildHint(language string) string {
	if lang := (BuildOptions{Language: language}).Lang(); lang != LangGo {
		return "gofi graph build --lang " + lang
	}
	return "gofi graph build"
}

func openInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("nao foi possivel abrir o navegador (%w) — abra manualmente: %s", err, path)
	}
	// Detach: the browser outlives the CLI process.
	return cmd.Process.Release()
}

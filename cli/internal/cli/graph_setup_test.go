package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
	"github.com/joaoprofile/gofi-cli/internal/graph/workspace"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func goProject(t *testing.T) (*config.GofiConfig, string) {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) { writeFile(t, filepath.Join(root, rel), body) }
	write("src/go.mod", "module example.com/app\n\ngo 1.26\n")
	write("src/app/app.go", "package app\n\n// Run starts the application.\nfunc Run() error { return nil }\n")

	return &config.GofiConfig{
		Project: config.Project{Name: "app", Root: root},
		Backend: &config.Backend{Language: config.LanguageGo, Path: "src"},
	}, root
}

func TestGraphOptionsFollowTheConfig(t *testing.T) {
	cfg, root := goProject(t)
	cfg.Graph = &config.Graph{Deep: true, Exclude: []string{"vendor"}}

	opt := graphOptions(cfg, root)
	if opt.SrcDir != "src" || opt.Language != config.LanguageGo {
		t.Errorf("options = %+v", opt)
	}
	if !opt.Deep || len(opt.Exclude) != 1 {
		t.Errorf("deep/exclude not carried over: %+v", opt)
	}
	// The hooks call this on every commit, so it has to be the cheap path.
	if !opt.Update {
		t.Error("Update is off, so every hook run would rescan the project")
	}
}

// The front-end and mobile folders are declared, not conventional: `gofi init`
// writes whatever the developer chose, and the graph has to read it back rather
// than assume "frontend".
func TestGraphOptionsCarryTheDeclaredSurfaces(t *testing.T) {
	cfg, root := goProject(t)
	cfg.Frontend = &config.UISurface{Framework: config.FrameworkReact, Path: "web"}
	cfg.Mobile = &config.UISurface{Framework: config.FrameworkReactNative, Path: "app"}

	got := graphOptions(cfg, root).Surfaces
	want := []workspace.Surface{
		{Name: "frontend", Dir: "web", Language: "typescript", Framework: config.FrameworkReact},
		{Name: "mobile", Dir: "app", Language: "typescript", Framework: config.FrameworkReactNative},
	}
	if !slices.Equal(got, want) {
		t.Errorf("surfaces = %+v, want %+v", got, want)
	}

	cfg.Frontend, cfg.Mobile = nil, nil
	if s := graphOptions(cfg, root).Surfaces; len(s) != 0 {
		t.Errorf("a backend-only project declared surfaces: %+v", s)
	}
}

// A project that declares nothing gofi can scan has no graph and nothing to
// report about one.
func TestGraphSkippedWithNothingToScan(t *testing.T) {
	cfg, root := goProject(t)
	cfg.Backend = nil

	if graphEnabled(cfg) {
		t.Fatal("a project with no declared area claims a graph")
	}
	if note := buildGraphQuietly(t.Context(), cfg, root); note != "" {
		t.Errorf("note = %q, want silence", note)
	}
	if note := installGraphHooksQuietly(cfg, root); note != "" {
		t.Errorf("hooks note = %q, want silence", note)
	}
}

// An Angular or React repository with no backend is a code base like any other:
// gofi reads TypeScript itself, so the front end is the graph.
func TestGraphForAFrontOnlyProject(t *testing.T) {
	cfg, root := goProject(t)
	cfg.Backend = nil
	cfg.Frontend = &config.UISurface{Framework: config.FrameworkAngular, Path: "web"}
	writeFile(t, filepath.Join(root, "web", "package.json"), `{"name":"admin"}`)
	writeFile(t, filepath.Join(root, "web", "src", "app.component.ts"),
		"import { Component } from '@angular/core';\n\n@Component({ selector: 'app-root' })\nexport class AppComponent {}\n")

	if !graphEnabled(cfg) {
		t.Fatal("a front-only project was denied a graph")
	}
	// The project scope would scan the root for Go and write an empty graph
	// beside the real one.
	if !graphOptions(cfg, root).SkipProject {
		t.Error("the project scope was planned with no backend to scan")
	}
	if note := buildGraphQuietly(t.Context(), cfg, root); note == "" {
		t.Fatal("no note reported")
	}

	g, err := model.Load(filepath.Join(graph.Dir(root, ""), "frontend", graph.GraphFile))
	if err != nil {
		t.Fatalf("front-end graph: %v", err)
	}
	if g.Framework != config.FrameworkAngular {
		t.Errorf("framework = %q, want angular", g.Framework)
	}
	if !slices.ContainsFunc(g.Nodes, func(n *model.Node) bool { return n.Kind == model.KindComponent }) {
		t.Error("the decorated class was not recognized as a component")
	}
}

// The staleness check has to watch every folder that feeds the graph: a front
// end edited all week must not report a graph that is current.
func TestGraphIsStaleAfterTheFrontEndMoves(t *testing.T) {
	cfg, root := goProject(t)
	cfg.Frontend = &config.UISurface{Framework: config.FrameworkReact, Path: "web"}
	writeFile(t, filepath.Join(root, "web", "package.json"), `{"name":"web"}`)
	writeFile(t, filepath.Join(root, "web", "src", "App.tsx"), "export const App = () => null\n")

	if note := buildGraphQuietly(t.Context(), cfg, root); note == "" {
		t.Fatal("no graph built")
	}
	if stale, err := graphIsStale(cfg, root); err != nil || stale {
		t.Fatalf("fresh graph reported stale=%v (%v)", stale, err)
	}

	touched := filepath.Join(root, "web", "src", "Next.tsx")
	writeFile(t, touched, "export const Next = () => null\n")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(touched, future, future); err != nil {
		t.Fatal(err)
	}
	if stale, err := graphIsStale(cfg, root); err != nil || !stale {
		t.Errorf("a new front-end file did not make the graph stale (%v)", err)
	}
}

func TestBuildGraphQuietlyWritesTheGraph(t *testing.T) {
	cfg, root := goProject(t)

	if note := buildGraphQuietly(t.Context(), cfg, root); note == "" {
		t.Fatal("no note reported")
	}
	if _, err := os.Stat(filepath.Join(graph.Dir(root, config.LanguageGo), graph.GraphFile)); err != nil {
		t.Fatalf("graph not written: %v", err)
	}
}

// `gofi init` must survive a project the extractor cannot read. The note is the
// whole error report.
func TestBuildGraphQuietlyNeverFails(t *testing.T) {
	cfg, root := goProject(t)
	if err := os.Remove(filepath.Join(root, "src", "go.mod")); err != nil {
		t.Fatal(err)
	}

	if note := buildGraphQuietly(t.Context(), cfg, root); note == "" {
		t.Error("a failed scan reported nothing")
	}
}

func TestGraphIsStaleAfterTheCodeMoves(t *testing.T) {
	cfg, root := goProject(t)
	if note := buildGraphQuietly(t.Context(), cfg, root); note == "" {
		t.Fatal("no graph built")
	}
	if stale, err := graphIsStale(cfg, root); err != nil || stale {
		t.Fatalf("fresh graph reported stale=%v (%v)", stale, err)
	}

	newer := filepath.Join(root, "src", "app", "more.go")
	if err := os.WriteFile(newer, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newer, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if stale, err := graphIsStale(cfg, root); err != nil || !stale {
		t.Errorf("new source did not make the graph stale (%v)", err)
	}
}

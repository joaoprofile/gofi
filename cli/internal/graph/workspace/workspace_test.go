package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
)

// project lays out a temporary project with its own code under src/ and a
// vendored SDK, the shape `gofi init` produces.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write(t, filepath.Join(root, "src", "go.mod"), "module example.com/app\n\ngo 1.26\n")
	write(t, filepath.Join(root, "src", "app", "app.go"), `package app

// Run starts the application.
func Run() error { return nil }
`)

	sdk := filepath.Join(root, SDKDirPrefix+graph.LangGo)
	write(t, filepath.Join(sdk, "go.mod"), "module example.com/sdk\n\ngo 1.26\n")
	write(t, filepath.Join(sdk, "errs", "errs.go"), `package errs

// New builds an error.
func New(msg string) error { return nil }
`)
	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func options(root string) Options {
	return Options{Root: root, SrcDir: "src", NoHTML: true}
}

// A project without a vendored SDK is a normal project, not a broken one.
func TestScopesFollowTheCheckout(t *testing.T) {
	root := project(t)

	if got := len(Scopes(options(root))); got != 2 {
		t.Fatalf("scopes = %d, want project and sdk", got)
	}
	opt := options(root)
	opt.SkipSDK = true
	if got := Scopes(opt); len(got) != 1 || got[0].Name != ScopeProject {
		t.Errorf("SkipSDK gave %v", got)
	}

	bare := t.TempDir()
	if got := Scopes(Options{Root: bare}); len(got) != 1 || got[0].Name != ScopeProject {
		t.Errorf("a project with no SDK gave %v", got)
	}
}

// The front-end tree is declared in .gofi.yaml and written in another language,
// so it is a scope of its own — folding it into the backend graph would need one
// extractor to read both.
func TestScopesIncludeTheDeclaredSurfaces(t *testing.T) {
	root := project(t)
	write(t, filepath.Join(root, "frontend", "app.tsx"), "export const App = () => null\n")
	opt := options(root)
	opt.Surfaces = []Surface{
		{Name: "frontend", Dir: "frontend", Language: graph.LangTypeScript},
		{Name: "mobile", Dir: "mobile", Language: graph.LangTypeScript},
	}
	// Find falls back to PATH, and a real extractor there would decide the test.
	t.Setenv("PATH", t.TempDir())

	// TypeScript is read by an extractor compiled into the binary, so the scope
	// exists with nothing installed.
	scopes := Scopes(opt)
	front, ok := findScope(scopes, "frontend")
	if !ok {
		t.Fatalf("no front-end scope for a natively readable language: %+v", scopes)
	}
	if want := filepath.Join(root, "frontend"); front.Root != want {
		t.Errorf("front-end scope scans %s, want the declared %s", front.Root, want)
	}
	if want := filepath.Join(graph.Dir(root, ""), "frontend"); front.Dir != want {
		t.Errorf("front-end graph goes to %s, want %s", front.Dir, want)
	}
	// mobile: is declared but the folder was never created.
	if _, ok := findScope(scopes, "mobile"); ok {
		t.Error("a scope was planned for a folder the project does not have")
	}
}

// A surface in a language gofi cannot read is dropped rather than built: a
// project with no extractor for it is a project with no graph for it, not a
// failing build.
func TestScopesSkipASurfaceWithNoExtractor(t *testing.T) {
	root := project(t)
	write(t, filepath.Join(root, "mobile", "Main.kt"), "fun main() {}\n")
	opt := options(root)
	opt.Surfaces = []Surface{{Name: "mobile", Dir: "mobile", Language: "kotlin"}}
	t.Setenv("PATH", t.TempDir())

	if _, ok := findScope(Scopes(opt), "mobile"); ok {
		t.Error("a scope was planned with no extractor able to build it")
	}

	fakeExtractor(t, root, "kotlin")
	if _, ok := findScope(Scopes(opt), "mobile"); !ok {
		t.Error("no scope once the extractor was installed")
	}
}

// A repository that is only a front end still gets a graph. Scanning its root
// for the project's language would produce an empty graph beside the real one.
func TestScopesForAFrontOnlyProject(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "web", "src", "app.tsx"), "export const App = () => null\n")

	scopes := Scopes(Options{
		Root:        root,
		SkipProject: true,
		SkipSDK:     true,
		Surfaces:    []Surface{{Name: "frontend", Dir: "web", Language: graph.LangTypeScript}},
	})
	if len(scopes) != 1 || scopes[0].Name != "frontend" {
		t.Fatalf("front-only project planned %+v", scopes)
	}
}

func findScope(scopes []Scope, name string) (Scope, bool) {
	for _, s := range scopes {
		if s.Name == name {
			return s, true
		}
	}
	return Scope{}, false
}

// fakeExtractor installs an executable that speaks the line protocol, so a
// scope in a language gofi does not read natively can be built in a test.
func fakeExtractor(t *testing.T, root, language string) {
	t.Helper()
	dir := filepath.Join(root, external.ExtractorsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `#!/bin/sh
echo '{"rec":"header","schema":"gofi-graph/v1","language":"` + language + `","module":"web","tool":"fake 0.1.0","mode":"fast"}'
echo '{"rec":"node","id":"ext:src","kind":"package","name":"src","file":"src","lines":1}'
echo '{"rec":"node","id":"ext:src.App","kind":"func","name":"App","unit":"ext:src","file":"src/App.kt","line":1,"vis":"public","doc":"App root."}'
echo '{"rec":"edge","from":"ext:src","to":"ext:src.App","rel":"contains","file":"src/App.kt","line":1,"conf":1}'
echo '{"rec":"summary","files":1,"loc":1,"unresolved":0,"ambiguous":0}'
`
	if err := os.WriteFile(filepath.Join(dir, external.BinaryName(language)), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// An extractor is installed once, at the project root. A surface scope scans a
// folder below it, and looking for the binary relative to what it scans would
// find nothing — the front-end graph would be missing on every project that has
// the extractor installed correctly.
func TestBuildFindsTheExtractorAtTheProjectRoot(t *testing.T) {
	root := project(t)
	write(t, filepath.Join(root, "mobile", "src", "Main.kt"), "fun main() {}\n")
	t.Setenv("PATH", t.TempDir())
	fakeExtractor(t, root, "kotlin")

	opt := options(root)
	opt.Surfaces = []Surface{{Name: "mobile", Dir: "mobile", Language: "kotlin"}}
	if _, err := Build(t.Context(), opt); err != nil {
		t.Fatal(err)
	}

	n, scope, ok := Load(root, "").Find("ext:src.App")
	if !ok {
		t.Fatal("the surface symbol is in no scope")
	}
	if scope != "mobile" {
		t.Errorf("App resolved to scope %q", scope)
	}
	if n.Doc == "" {
		t.Error("the extractor's documentation did not survive the round trip")
	}
}

// Each graph says what its tree is written in and what it is built with. A
// reader that opened one scope's gofi_graph.json has no other way to tell a
// React front end from the Go backend beside it.
func TestBuildRecordsTheDeclaredStack(t *testing.T) {
	root := project(t)
	write(t, filepath.Join(root, "frontend", "src", "App.tsx"), "export const App = () => null\n")
	t.Setenv("PATH", t.TempDir())

	opt := options(root)
	opt.Surfaces = []Surface{
		{Name: "frontend", Dir: "frontend", Language: graph.LangTypeScript, Framework: "react"},
	}
	res, err := Build(t.Context(), opt)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range res.Built() {
		g := s.Result.Graph
		switch s.Scope.Name {
		case "frontend":
			if g.Language != "typescript" || g.Framework != "react" {
				t.Errorf("front-end graph = %q/%q, want typescript/react", g.Language, g.Framework)
			}
		default:
			if g.Language != graph.LangGo || g.Framework != "" {
				t.Errorf("%s graph = %q/%q, want go and no framework", s.Scope.Name, g.Language, g.Framework)
			}
		}
	}

	if e := mustScope(t, res.Index, "frontend"); e.Framework != "react" {
		t.Errorf("index framework = %q, want react", e.Framework)
	}
	if e := mustScope(t, res.Index, ScopeProject); e.Framework != "" {
		t.Errorf("the backend scope claims framework %q", e.Framework)
	}
}

// The SDK must land in its own directory: one graph overwriting the other is
// exactly what scopes exist to prevent.
func TestScopesDoNotShareADirectory(t *testing.T) {
	scopes := Scopes(options(project(t)))
	if scopes[0].Dir == scopes[1].Dir {
		t.Fatalf("both scopes write to %s", scopes[0].Dir)
	}
	if want := filepath.Join(scopes[0].Dir, ScopeSDK); scopes[1].Dir != want {
		t.Errorf("sdk dir = %s, want %s", scopes[1].Dir, want)
	}
}

func TestBuildWritesEveryScopeAndAnIndex(t *testing.T) {
	root := project(t)

	res, err := Build(t.Context(), options(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Built()); got != 2 {
		t.Fatalf("built %d scopes, want 2", got)
	}
	for _, s := range res.Scopes {
		if _, err := os.Stat(filepath.Join(s.Scope.Dir, graph.GraphFile)); err != nil {
			t.Errorf("scope %s: %v", s.Scope.Name, err)
		}
	}
	if _, err := os.Stat(IndexPath(root, "")); err != nil {
		t.Fatalf("index: %v", err)
	}

	ix, err := LoadIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Scopes) != 2 || ix.Scopes[0].Name != ScopeProject {
		t.Fatalf("index scopes = %+v", ix.Scopes)
	}
	// Paths are recorded relative so the file survives the project moving.
	for _, s := range ix.Scopes {
		if filepath.IsAbs(s.Dir) || filepath.IsAbs(s.Root) {
			t.Errorf("scope %s recorded an absolute path: %+v", s.Name, s)
		}
	}
	if sdk, _ := ix.Scope(ScopeSDK); sdk.Nodes == 0 {
		t.Error("sdk scope recorded no nodes")
	}
}

// A key is the caller promising the scope's inputs did not change. Honouring it
// is what keeps `gofi update` from rescanning the SDK on every run.
func TestBuildSkipsAScopeWhoseKeyIsUnchanged(t *testing.T) {
	root := project(t)
	opt := options(root)
	opt.SDKKey = "abc123"

	if _, err := Build(t.Context(), opt); err != nil {
		t.Fatal(err)
	}
	res, err := Build(t.Context(), opt)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range res.Scopes {
		if s.Scope.Name == ScopeSDK && !s.Skipped {
			t.Error("the sdk scope was rebuilt despite an unchanged key")
		}
		if s.Scope.Name == ScopeProject && s.Skipped {
			t.Error("the project scope was skipped, but it carries no key")
		}
	}

	opt.SDKKey = "def456"
	res, err = Build(t.Context(), opt)
	if err != nil {
		t.Fatal(err)
	}
	if sdk := res.Scopes[1]; sdk.Skipped {
		t.Error("a new key did not force a rebuild")
	}
}

// The graphs are committed; the vendored SDK they were built from is not. So a
// fresh clone has an SDK graph and no SDK sources, and a build there must leave
// that scope alone instead of erasing it from the index — which would both hide
// a graph that is right there and dirty a tracked file on every clone.
func TestBuildKeepsAScopeWhoseSourcesAreGone(t *testing.T) {
	root := project(t)
	if _, err := Build(t.Context(), options(root)); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(IndexPath(root, ""))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(root, SDKDirPrefix+graph.LangGo)); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(t.Context(), options(root)); err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(IndexPath(root, ""))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("the index changed in a clone without the SDK checkout:\n%s\nwant\n%s", after, before)
	}
	if _, _, ok := Load(root, "").Find("func:example.com/sdk/errs.New"); !ok {
		t.Error("the committed SDK graph stopped resolving once its sources were gone")
	}
}

// A build that scanned nothing has nothing to say. Left to write anyway it
// drops an empty index into a directory git tracks.
func TestBuildThatGraphsNothingWritesNoIndex(t *testing.T) {
	root := t.TempDir()
	opt := options(root)
	opt.SrcDir = "nowhere"

	if _, err := Build(t.Context(), opt); err == nil {
		t.Fatal("scanning a directory that does not exist succeeded")
	}
	if _, err := os.Stat(IndexPath(root, "")); !os.IsNotExist(err) {
		t.Errorf("a failed build wrote an index: %v", err)
	}
}

// Carrying a scope forward is for sources that vanished, not graphs that did.
func TestBuildDropsAScopeWhoseGraphIsGone(t *testing.T) {
	root := project(t)
	if _, err := Build(t.Context(), options(root)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, SDKDirPrefix+graph.LangGo)); err != nil {
		t.Fatal(err)
	}
	sdkDir, _ := LoadIndex(root, "")
	dir := filepath.Join(graph.Dir(root, ""), filepath.FromSlash(mustScope(t, sdkDir, ScopeSDK).Dir))
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := Build(t.Context(), options(root)); err != nil {
		t.Fatal(err)
	}
	ix, err := LoadIndex(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Scope(ScopeSDK); ok {
		t.Error("the index still advertises an sdk graph that is not on disk")
	}
}

func mustScope(t *testing.T, ix *Index, name string) ScopeInfo {
	t.Helper()
	s, ok := ix.Scope(name)
	if !ok {
		t.Fatalf("scope %s missing from the index", name)
	}
	return s
}

func TestWorkspaceResolvesAcrossScopes(t *testing.T) {
	root := project(t)
	if _, err := Build(t.Context(), options(root)); err != nil {
		t.Fatal(err)
	}

	w := Load(root, "")
	if _, _, ok := w.Find("func:example.com/app/app.Run"); !ok {
		t.Error("Run was not found in the project scope")
	}
	n, scope, ok := w.Find("func:example.com/sdk/errs.New")
	if !ok {
		t.Fatal("New was not found in any scope")
	}
	if scope != ScopeSDK {
		t.Errorf("New resolved to scope %q", scope)
	}
	if n.Doc == "" {
		t.Error("the resolved node lost its documentation")
	}
	if _, _, ok := w.Find("example.com/app/app.Missing"); ok {
		t.Error("an unknown symbol resolved")
	}
}

// A graph written by `gofi graph build` has no index, and reading it must not
// depend on knowing that.
func TestLoadWithoutAnIndex(t *testing.T) {
	root := project(t)
	opt := options(root)
	opt.SkipSDK = true
	if _, err := Build(t.Context(), opt); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(IndexPath(root, "")); err != nil {
		t.Fatal(err)
	}

	w := Load(root, "")
	if _, _, ok := w.Find("func:example.com/app/app.Run"); !ok {
		t.Error("Run was not found without an index")
	}
	if _, err := w.Graph(ScopeSDK); err == nil {
		t.Error("a scope that was never built resolved")
	}
}

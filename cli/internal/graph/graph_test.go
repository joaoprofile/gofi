package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
)

func TestLang(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", LangGo},
		{"go", LangGo},
		{" Go ", LangGo},
		{"Java", "java"},
		{"RUST", "rust"},
	}
	for _, tt := range tests {
		if got := (BuildOptions{Language: tt.in}).Lang(); got != tt.want {
			t.Errorf("Lang(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// Go sits at the top of .gofi/graph/ so the common case has no extra level;
// every other language gets its own directory and cannot overwrite it.
func TestDirPerLanguage(t *testing.T) {
	root := filepath.FromSlash("/tmp/proj")
	if got, want := Dir(root, ""), filepath.Join(root, OutDir); got != want {
		t.Errorf("Dir(go) = %q, want %q", got, want)
	}
	if got, want := Dir(root, "Java"), filepath.Join(root, OutDir, "java"); got != want {
		t.Errorf("Dir(java) = %q, want %q", got, want)
	}
}

func TestLoadMissingSaysHowToBuild(t *testing.T) {
	root := t.TempDir()
	_, err := Load(root, "")
	if err == nil || !strings.Contains(err.Error(), "gofi graph build") {
		t.Fatalf("err = %v", err)
	}
	// The hint has to carry the language, or following it rebuilds the wrong graph.
	_, err = Load(root, "java")
	if err == nil || !strings.Contains(err.Error(), "gofi graph build --lang java") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildWithoutExtractor(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Build(t.Context(), BuildOptions{Root: t.TempDir(), Language: "cobol"})
	if err == nil || !strings.Contains(err.Error(), "gofi graph install cobol") {
		t.Fatalf("err = %v, want the install hint", err)
	}
}

// buildFakeExtractor compiles testdata/fakeextractor and installs it as the
// extractor for a made-up language. It also isolates PATH, so the test can only
// find the extractor it just installed.
func buildFakeExtractor(t *testing.T, projectRoot string) {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go toolchain to build the fake extractor")
	}
	dir := filepath.Join(projectRoot, external.ExtractorsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, external.BinaryName("fake"))
	cmd := exec.Command(goBin, "build", "-o", dst, filepath.Join("testdata", "fakeextractor", "main.go"))
	// The fake extractor imports nothing outside the standard library, so a
	// workspace can only get in the way here.
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the fake extractor: %v\n%s", err, out)
	}
	t.Setenv("PATH", t.TempDir())
}

func TestBuildExternal(t *testing.T) {
	root := t.TempDir()
	buildFakeExtractor(t, root)

	res, err := Build(t.Context(), BuildOptions{Root: root, Language: "fake", Deep: true})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if want := filepath.Join(root, OutDir, "fake"); res.Dir != want {
		t.Errorf("dir = %q, want %q", res.Dir, want)
	}
	for _, name := range []string{GraphFile, ReportFile, HTMLFile} {
		if _, err := os.Stat(filepath.Join(res.Dir, name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	if res.Graph.Module != "com.acme.app" || res.Graph.Mode != "deep" {
		t.Errorf("header not applied: module=%q mode=%q", res.Graph.Module, res.Graph.Mode)
	}
	if res.Graph.Stats.Nodes != 3 || res.Graph.Stats.Edges != 3 {
		t.Errorf("analysis did not run: %+v", res.Graph.Stats)
	}
	// A decoded graph has nobody counting kinds as it is built, so the summary
	// line would read "0 packages" if this were not derived.
	s := res.Graph.Stats
	if s.Packages != 1 || s.Types != 1 || s.Methods != 1 || s.Funcs != 0 {
		t.Errorf("kind counters = pkg:%d types:%d funcs:%d methods:%d", s.Packages, s.Types, s.Funcs, s.Methods)
	}
	if res.Graph.Stats.Files != 2 || res.Graph.Stats.Unresolved != 1 {
		t.Errorf("summary not applied: %+v", res.Graph.Stats)
	}
	// --root is what the extractor scans, so getting it wrong is silent breakage.
	if len(res.Diagnostics) != 1 || !strings.Contains(res.Diagnostics[0].Message, root) {
		t.Errorf("diagnostics = %+v", res.Diagnostics)
	}

	// The report is the first thing an agent reads, and every command it
	// suggests has to point back at this graph rather than the Go one.
	md, err := os.ReadFile(filepath.Join(res.Dir, ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "gofi graph explain <no> --lang fake") {
		t.Error("gofi_graph_report.md tells the reader to query the Go graph")
	}

	// The graph must be readable back through the same language-aware path.
	g, err := Load(root, "fake")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if g.Get("fake:com.acme.api.Server") == nil {
		t.Error("node missing after a round trip through disk")
	}
}

// An explicit Out is a decision already made. Appending the language to it
// would nest a scope inside itself the moment a workspace passes the directory
// it just computed.
func TestExplicitOutIsUsedAsGiven(t *testing.T) {
	root := t.TempDir()
	buildFakeExtractor(t, root)

	out := filepath.Join(root, OutDir, "fake", "sdk")
	res, err := Build(t.Context(), BuildOptions{Root: root, Language: "fake", Out: out})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Dir != out {
		t.Errorf("dir = %q, want %q", res.Dir, out)
	}
}

// A polyglot repository keeps one graph per language, and building one must not
// disturb another.
func TestBuildExternalKeepsGoGraph(t *testing.T) {
	root := t.TempDir()
	buildFakeExtractor(t, root)

	goDir := filepath.Join(root, OutDir)
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(goDir, GraphFile)
	if err := os.WriteFile(marker, []byte("go graph"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Build(t.Context(), BuildOptions{Root: root, Language: "fake"}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	b, err := os.ReadFile(marker)
	if err != nil || string(b) != "go graph" {
		t.Errorf("the Go graph was overwritten: %q (%v)", b, err)
	}
}

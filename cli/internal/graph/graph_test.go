package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/graph/extract/external"
	"github.com/gofi-labs/gofi/cli/internal/graph/model"
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

// Every language writes to the one code index; a language is a scope in it,
// never a directory of its own beside it.
func TestDirIsTheCodeIndex(t *testing.T) {
	root := filepath.FromSlash("/tmp/proj")
	if got, want := Dir(root), filepath.Join(root, filepath.FromSlash(OutDir)); got != want {
		t.Errorf("Dir = %q, want %q", got, want)
	}
}

func TestOpenMissingSaysHowToBuild(t *testing.T) {
	err := Open(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "gofi index code") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildWithoutExtractor(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Build(t.Context(), BuildOptions{Root: t.TempDir(), Language: "cobol"})
	if err == nil || !strings.Contains(err.Error(), "gofi index install cobol") {
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

	if want := Dir(root); res.Dir != want {
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

	// The report is the first thing an agent reads, and it has to send the
	// reader to the query commands rather than to the files.
	md, err := os.ReadFile(filepath.Join(res.Dir, ReportFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "gofi show <simbolo>") {
		t.Error("gofi_graph_report.md does not name the query commands")
	}

	// The graph must be readable back from the code index.
	g, err := model.Load(filepath.Join(Dir(root), GraphFile))
	if err != nil {
		t.Fatalf("load: %v", err)
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

	out := filepath.Join(Dir(root), "sdk")
	res, err := Build(t.Context(), BuildOptions{Root: root, Language: "fake", Out: out})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Dir != out {
		t.Errorf("dir = %q, want %q", res.Dir, out)
	}
}

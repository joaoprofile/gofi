package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/graph"
	"github.com/joaoprofile/gofi/cli/internal/graph/workspace"
)

// configuredProject writes a valid .gofi.yaml next to the sources goProject
// lays out, so the command has to read the layout instead of assuming it.
func configuredProject(t *testing.T) string {
	t.Helper()
	cfg, root := goProject(t)
	cfg.Version = config.CurrentVersion
	cfg.AI = config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus5}
	cfg.Sources = config.Sources{Agents: config.DefaultAgentsRef}
	cfg.Test = config.DefaultTestSection(config.LanguageGo, cfg.Backend.Path)

	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}
	return root
}

func runGraphBuildIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	t.Chdir(dir)

	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"code", "--no-html"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index code: %v\n%s", err, out.String())
	}
	return out.String()
}

// A bare `gofi index code` runs from the repository root. Scanning that root directly finds no go.mod, because every project
// gofi init produces keeps its code under the folder backend.path names.
func TestGraphBuildFollowsTheConfiguredLayout(t *testing.T) {
	root := configuredProject(t)

	out := runGraphBuildIn(t, root)

	if _, err := os.Stat(workspace.IndexPath(root)); err != nil {
		t.Fatalf("no workspace index written: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(graph.Dir(root), graph.GraphFile)); err != nil {
		t.Fatalf("project scope not graphed: %v\n%s", err, out)
	}
}

// .gofi.yaml names the workspace folder, and the rest of gofi already reads and
// writes there. A .gofi.yaml copied into another tree describes the project it
// names, not the directory it happens to sit in.
func TestGraphBuildFollowsTheDeclaredRoot(t *testing.T) {
	real := configuredProject(t)
	elsewhere := t.TempDir()
	body, err := os.ReadFile(filepath.Join(real, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(elsewhere, config.FileName), body, 0o644); err != nil {
		t.Fatal(err)
	}

	runGraphBuildIn(t, elsewhere)

	if _, err := os.Stat(workspace.IndexPath(real)); err != nil {
		t.Errorf("the declared project was not graphed: %v", err)
	}
	if _, err := os.Stat(graph.Dir(elsewhere)); !os.IsNotExist(err) {
		t.Errorf("a graph was written to the folder the copy sat in: %v", err)
	}
}

// A declared root that is no longer a gofi project is a path from another
// machine, and the only trustworthy root is where the file was found.
func TestGraphBuildIgnoresAStaleDeclaredRoot(t *testing.T) {
	root := configuredProject(t)
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Project.Root = filepath.Join(t.TempDir(), "gone")
	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatal(err)
	}

	runGraphBuildIn(t, root)

	if _, err := os.Stat(workspace.IndexPath(root)); err != nil {
		t.Errorf("a stale declared root stopped the project being graphed: %v", err)
	}
}

// The declared source folder is the one thing the output has to name: a graph
// built from the wrong directory is indistinguishable from a correct one. The
// language goes with it, because a project has scopes in more than one.
func TestGraphBuildNamesTheFolderItScanned(t *testing.T) {
	root := configuredProject(t)

	out := runGraphBuildIn(t, root)

	if !strings.Contains(out, "project (src, go)") {
		t.Errorf("the output does not say which folder was scanned:\n%s", out)
	}
}

// A .gofi.yaml that cannot be read is not a project without one. Scanning the
// root instead would graph a tree the project never declared and hand it to the
// agents as the truth.
func TestGraphBuildRefusesABrokenConfig(t *testing.T) {
	root := configuredProject(t)
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte("backend: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"code", "--no-html"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("a broken .gofi.yaml was ignored and something got graphed:\n%s", out.String())
	}
	if _, err := os.Stat(graph.Dir(root)); !os.IsNotExist(err) {
		t.Errorf("a graph was written despite the unreadable layout: %v", err)
	}
}

// The command is run from wherever the developer is, which is rarely the
// repository root.
func TestGraphBuildWorksFromASubdirectory(t *testing.T) {
	root := configuredProject(t)

	runGraphBuildIn(t, filepath.Join(root, "src", "app"))

	if _, err := os.Stat(workspace.IndexPath(root)); err != nil {
		t.Fatalf("no workspace index written: %v", err)
	}
}

// Indexing is incremental by default: the second run must not rewrite a graph
// nothing invalidated.
func TestGraphBuildUpdateSkipsAnUnchangedProject(t *testing.T) {
	root := configuredProject(t)
	runGraphBuildIn(t, root)

	path := filepath.Join(graph.Dir(root), graph.GraphFile)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	runGraphBuildIn(t, root)

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("the incremental build rescanned a project nothing changed")
	}
}

// Another language reads the backend tree as one more scope of the project's
// own index: find, show and path reach it with no flag, and the scopes built
// before are neither rebuilt nor dropped.
func TestIndexLangAddsAScope(t *testing.T) {
	root := configuredProject(t)
	runGraphBuildIn(t, root)
	project := filepath.Join(graph.Dir(root), graph.GraphFile)
	before, err := os.ReadFile(project)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(root, "src", "web", "App.ts"), "export function renderApp() {}\n")
	runGraphBuildIn(t, root, "--lang", "typescript")

	ix, err := workspace.LoadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{workspace.ScopeProject, "typescript"} {
		if _, ok := ix.Scope(name); !ok {
			t.Errorf("scope %s missing from the index: %+v", name, ix.Scopes)
		}
	}
	if ix.Language != config.LanguageGo {
		t.Errorf("index language = %q, want the project's", ix.Language)
	}
	if after, _ := os.ReadFile(project); !bytes.Equal(before, after) {
		t.Error("the project graph was rewritten by a build of another language")
	}
	g, err := workspace.Load(root, config.LanguageGo).Graph("typescript")
	if err != nil || g.Language != "typescript" {
		t.Fatalf("typescript scope not readable: %v", err)
	}
}

// A project indexed by an older release is moved to the current layout on the
// next build, and the side graphs no index lists are dropped, not carried.
func TestIndexMovesTheOldLayout(t *testing.T) {
	root := configuredProject(t)
	runGraphBuildIn(t, root)
	legacy := filepath.Join(root, ".gofi", "graph")
	if err := os.Rename(graph.Dir(root), legacy); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(legacy, "java", workspace.IndexFile), `{"scopes":[{"dir":"."}]}`)
	writeFile(t, filepath.Join(legacy, "extractors", "gofi-graph-java"), "bin")
	writeFile(t, filepath.Join(root, ".gofi", "docs", "index.json"), "{}")

	out := runGraphBuildIn(t, root)

	for _, gone := range []string{legacy, filepath.Join(root, ".gofi", "docs")} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s left behind\n%s", gone, out)
		}
	}
	for _, kept := range []string{
		workspace.IndexPath(root),
		filepath.Join(root, ".gofi", "extractors", "gofi-graph-java"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s not moved: %v\n%s", kept, err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(graph.Dir(root), "java")); !os.IsNotExist(err) {
		t.Error("an unlisted side graph was carried into the new layout")
	}
}

func TestIndexStatusFollowsTheSources(t *testing.T) {
	root := configuredProject(t)
	runGraphBuildIn(t, root)
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatal(err)
	}

	st, err := indexStatus(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if st.Code.State != workspace.Fresh || st.Code.Built == nil {
		t.Fatalf("code after a build: %s, recorded %v", st.Code.State, st.Code.Built)
	}
	if st.Docs.State != workspace.Missing {
		t.Errorf("docs never built reported %s", st.Docs.State)
	}

	writeFile(t, filepath.Join(root, "src", "app", "more.go"), "package app\n\nfunc More() {}\n")
	if st, _ = indexStatus(cfg, root); st.Code.State != workspace.Stale || st.Current() {
		t.Errorf("an edit left the code %s", st.Code.State)
	}
}

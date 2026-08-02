package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/workspace"
)

// configuredProject writes a valid .gofi.yaml next to the sources goProject
// lays out, so the command has to read the layout instead of assuming it.
func configuredProject(t *testing.T) string {
	t.Helper()
	cfg, root := goProject(t)
	cfg.Version = config.CurrentVersion
	cfg.AI = config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus5}
	cfg.Agents = []string{config.AgentEng}
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
	cmd := newGraphCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"build", "--no-html"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("graph build: %v\n%s", err, out.String())
	}
	return out.String()
}

// The git hook runs a bare `gofi graph build --update` from the repository
// root. Scanning that root directly finds no go.mod, because every project
// gofi init produces keeps its code under the folder backend.path names.
func TestGraphBuildFollowsTheConfiguredLayout(t *testing.T) {
	root := configuredProject(t)

	out := runGraphBuildIn(t, root)

	if _, err := os.Stat(workspace.IndexPath(root, config.LanguageGo)); err != nil {
		t.Fatalf("no workspace index written: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(graph.Dir(root, config.LanguageGo), graph.GraphFile)); err != nil {
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

	if _, err := os.Stat(workspace.IndexPath(real, config.LanguageGo)); err != nil {
		t.Errorf("the declared project was not graphed: %v", err)
	}
	if _, err := os.Stat(graph.Dir(elsewhere, config.LanguageGo)); !os.IsNotExist(err) {
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

	if _, err := os.Stat(workspace.IndexPath(root, config.LanguageGo)); err != nil {
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
	cmd := newGraphCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"build", "--no-html"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("a broken .gofi.yaml was ignored and something got graphed:\n%s", out.String())
	}
	if _, err := os.Stat(graph.Dir(root, config.LanguageGo)); !os.IsNotExist(err) {
		t.Errorf("a graph was written despite the unreadable layout: %v", err)
	}
}

// An explicit path is the developer overriding the declared layout, and has to
// keep working — it is how a subdirectory or a sibling repository gets graphed.
func TestGraphBuildPathArgumentOverridesTheLayout(t *testing.T) {
	root := configuredProject(t)

	runGraphBuildIn(t, root, filepath.Join(root, "src"))

	if _, err := os.Stat(workspace.IndexPath(root, config.LanguageGo)); !os.IsNotExist(err) {
		t.Errorf("an explicit path went through the workspace build: %v", err)
	}
}

// The hook is also run from wherever the commit happened, which is rarely the
// repository root.
func TestGraphBuildWorksFromASubdirectory(t *testing.T) {
	root := configuredProject(t)

	runGraphBuildIn(t, filepath.Join(root, "src", "app"))

	if _, err := os.Stat(workspace.IndexPath(root, config.LanguageGo)); err != nil {
		t.Fatalf("no workspace index written: %v", err)
	}
}

// --update is the hook's whole reason to be cheap: the second run must not
// rewrite a graph nothing invalidated.
func TestGraphBuildUpdateSkipsAnUnchangedProject(t *testing.T) {
	root := configuredProject(t)
	runGraphBuildIn(t, root)

	path := filepath.Join(graph.Dir(root, config.LanguageGo), graph.GraphFile)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	runGraphBuildIn(t, root, "--update")

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("--update rescanned a project nothing changed")
	}
}

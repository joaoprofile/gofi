package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

func webProject(t *testing.T) (string, *config.GofiConfig) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "frontend/package.json"), `{"scripts":{"tsc":"tsc --noEmit","lint":"eslint .","test":"jest"}}`)
	writeFile(t, filepath.Join(root, "frontend/yarn.lock"), "")
	cfg := &config.GofiConfig{
		Backend:  &config.Backend{Language: "go", Path: "backend"},
		Frontend: &config.UISurface{Framework: "react", Path: "frontend", Testing: "jest"},
	}
	return root, cfg
}

// A change of .tsx files only was sensed by go build ./... of the whole
// backend, for minutes, and its own code never checked. A web change is
// sensed by the web surface's own scripts, in its folder.
func TestAWebChangeIsSensedByTheWebSurfaceNotTheBackend(t *testing.T) {
	root, cfg := webProject(t)
	cmds, err := sensorCommandsFor(cfg, root, []string{"web"})
	if err != nil {
		t.Fatal(err)
	}
	for stage, c := range cmds {
		if strings.Contains(c.Run, "go -C") {
			t.Errorf("%s runs the backend's %q for a web change", stage, c.Run)
		}
	}
	want := map[string]string{
		"build":  "cd frontend && yarn run tsc",
		"static": "cd frontend && yarn run lint",
		"test":   "cd frontend && CI=true yarn run test",
	}
	for stage, run := range want {
		if cmds[stage].Run != run {
			t.Errorf("%s = %q, want %q", stage, cmds[stage].Run, run)
		}
	}
}

func TestAChangeOfBothSurfacesBuildsBoth(t *testing.T) {
	root, cfg := webProject(t)
	cmds, err := sensorCommandsFor(cfg, root, []string{"backend", "web"})
	if err != nil {
		t.Fatal(err)
	}
	if b := cmds["build"].Run; !strings.Contains(b, "go -C backend build") || !strings.Contains(b, "yarn run tsc") {
		t.Errorf("build of both = %q", b)
	}
	if tst := cmds["test"].Run; !strings.Contains(tst, "go -C backend test") {
		t.Errorf("test of both = %q, want the backend's", tst)
	}
	be, _ := sensorCommandsFor(cfg, root, []string{"backend"})
	if !strings.Contains(be["build"].Run, "go -C backend build") || strings.Contains(be["build"].Run, "yarn") {
		t.Errorf("a backend change = %q", be["build"].Run)
	}
}

// A worktree had no node_modules: every check of the web surface failed on
// a missing tool, not on the change. The surface's dependencies are linked.
func TestTheUIDependenciesAreLinkedIntoTheWorktree(t *testing.T) {
	root, cfg := webProject(t)
	if err := os.MkdirAll(filepath.Join(root, "frontend/node_modules/jest"), 0o755); err != nil {
		t.Fatal(err)
	}
	deps := uiDependencies(cfg, root)
	if len(deps) != 1 || deps[0] != "frontend/node_modules" {
		t.Fatalf("dependencies = %v", deps)
	}
}

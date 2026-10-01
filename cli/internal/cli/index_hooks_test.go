package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// A real commit through the installed hook: the graph rides in the same
// commit as the code, and a machine without gofi still commits, with a line
// saying the graph was left behind.
func TestPreCommitPutsTheGraphInTheCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := configuredProject(t)
	bin := buildGofi(t, t.TempDir())
	withGofi := []string{"PATH=" + filepath.Dir(bin) + string(os.PathListSeparator) + os.Getenv("PATH"), "GOFI_NO_SETUP=1", "NO_COLOR=1"}

	runGit(t, root, nil, "init", "-q")
	if _, err := installIndexHooks(root); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, nil, "add", "-A")
	runGit(t, root, withGofi, "commit", "-qm", "first")
	if files := runGit(t, root, nil, "show", "--name-only", "--format=", "HEAD"); !strings.Contains(files, ".gofi/index/code/gofi_graph.json") {
		t.Fatalf("the first commit has no graph:\n%s", files)
	}

	writeFile(t, filepath.Join(root, "src", "app", "bill.go"), "package app\n\n// Bill charges an order.\nfunc Bill() error { return nil }\n")
	runGit(t, root, nil, "add", "src/app/bill.go")
	runGit(t, root, withGofi, "commit", "-qm", "bill")
	files := runGit(t, root, nil, "show", "--name-only", "--format=", "HEAD")
	if !strings.Contains(files, "src/app/bill.go") || !strings.Contains(files, ".gofi/index/code/gofi_graph.json") {
		t.Fatalf("the code and its graph are not in the same commit:\n%s", files)
	}
	if graph := runGit(t, root, nil, "show", "HEAD:.gofi/index/code/gofi_graph.json"); !strings.Contains(graph, "Bill") {
		t.Error("the committed graph does not know the new function")
	}

	writeFile(t, filepath.Join(root, "src", "app", "more.go"), "package app\n")
	runGit(t, root, nil, "add", "src/app/more.go")
	noGofi := []string{"PATH=" + filepath.Dir(mustLookPath(t, "git"))}
	out := runGit(t, root, noGofi, "commit", "-m", "without gofi")
	if !strings.Contains(out, "gofi: not on PATH") {
		t.Errorf("a commit without gofi said nothing:\n%s", out)
	}
}

func mustLookPath(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skip(name + " not on PATH")
	}
	return p
}

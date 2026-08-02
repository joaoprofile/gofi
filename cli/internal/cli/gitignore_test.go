package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// checkIgnore asks git itself whether a path is ignored, because the ordering
// rules between an exclusion and the negation that re-includes a child of it
// are subtle enough that reading the file proves nothing.
func checkIgnore(t *testing.T, root, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "check-ignore", "-q", "--", path)
	err := cmd.Run()
	if err == nil {
		return true
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s: %v", path, err)
	return false
}

func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git nao esta no PATH")
	}
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return root
}

func TestGraphIsVersionedAndTheRestOfGofiIsNot(t *testing.T) {
	root := gitRepo(t)
	if err := ensureGofiIgnored(root); err != nil {
		t.Fatal(err)
	}

	for path, wantIgnored := range map[string]bool{
		".gofi/gofi-sdk-go/go.mod":            true,
		".gofi/installed.json":                true,
		".gofi/graph/gofi_graph.json":         false,
		".gofi/graph/gofi_graph_report.md":    false,
		".gofi/graph/sdk/gofi_graph.json":     false,
		".gofi/graph/extractors/gofi-graph-x": true,
	} {
		if got := checkIgnore(t, root, path); got != wantIgnored {
			t.Errorf("%s: ignored = %v, want %v", path, got, wantIgnored)
		}
	}
}

// Projects scaffolded before the graph existed carry a blanket `.gofi/`, which
// git will not descend into — the negation would never be reached.
func TestGofiIgnoreUpgradesTheLegacyRule(t *testing.T) {
	root := gitRepo(t)
	legacy := "node_modules/\n.gofi/\n.env\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureGofiIgnored(root); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"node_modules/", ".env"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("dropped an unrelated rule %q:\n%s", want, body)
		}
	}
	if checkIgnore(t, root, ".gofi/graph/gofi_graph.json") {
		t.Errorf("graph still ignored after the upgrade:\n%s", body)
	}
}

func TestGofiIgnoreIsIdempotent(t *testing.T) {
	root := gitRepo(t)
	if err := ensureGofiIgnored(root); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	if err := ensureGofiIgnored(root); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("rules stacked up:\n%s\n---\n%s", first, second)
	}
}

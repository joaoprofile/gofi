package githooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const body = "gofi graph build --update >/dev/null 2>&1 || true"

// every gives one body to all managed hooks, which is what most of these tests
// care about; per-hook bodies are exercised by TestInstallSkipsAHookWithNoBody.
func every(b string) map[string]string {
	m := make(map[string]string, len(Managed))
	for _, hook := range Managed {
		m[hook] = b
	}
	return m
}

func repo(t *testing.T) string {
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

func read(t *testing.T, root, hook string) string {
	t.Helper()
	dir, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, hook))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstallCreatesEveryManagedHook(t *testing.T) {
	root := repo(t)

	results, err := Install(root, every(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(Managed) {
		t.Fatalf("results = %d, want %d", len(results), len(Managed))
	}
	for _, r := range results {
		if r.Action != Created {
			t.Errorf("%s: action = %q", r.Hook, r.Action)
		}
		fi, err := os.Stat(r.Path)
		if err != nil {
			t.Fatalf("%s: %v", r.Hook, err)
		}
		// A hook git cannot execute is a hook that silently never runs.
		if fi.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s is not executable: %v", r.Hook, fi.Mode())
		}
	}
	if got := Installed(root); !slices.Equal(got, Managed) {
		t.Errorf("Installed() = %v, want %v", got, Managed)
	}
	// The graph is tracked, so it has to be regenerated while the commit is
	// still being assembled — after it, the rebuild would only dirty the tree.
	if !slices.Contains(Managed, "pre-commit") {
		t.Error("pre-commit is not managed, so the graph would lag a commit behind")
	}
	if slices.Contains(Managed, "post-commit") {
		t.Error("post-commit rebuilds into a tree that is already committed")
	}
}

// The bodies differ per hook — only pre-commit stages what it rebuilt — so a
// hook nobody gave a body to must be left alone rather than given an empty one.
func TestInstallSkipsAHookWithNoBody(t *testing.T) {
	root := repo(t)

	results, err := Install(root, map[string]string{"pre-commit": body})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Hook != "pre-commit" {
		t.Fatalf("results = %+v, want pre-commit only", results)
	}
	dir, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "post-merge")); !os.IsNotExist(err) {
		t.Errorf("post-merge was written without a body: %v", err)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	root := repo(t)
	if _, err := Install(root, every(body)); err != nil {
		t.Fatal(err)
	}
	before := read(t, root, "pre-commit")

	results, err := Install(root, every(body))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Action != Unchanged {
			t.Errorf("%s: action = %q, want unchanged", r.Hook, r.Action)
		}
	}
	if after := read(t, root, "pre-commit"); after != before {
		t.Errorf("hook rewritten:\n%s\n---\n%s", before, after)
	}
}

// The whole point of the markers: a repository that already uses husky or a
// hand-written hook keeps it.
func TestInstallPreservesAnExistingHook(t *testing.T) {
	root := repo(t)
	dir, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "#!/bin/sh\nnpx lefthook run pre-commit\n"
	path := filepath.Join(dir, "pre-commit")
	if err := os.WriteFile(path, []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(root, every(body)); err != nil {
		t.Fatal(err)
	}
	got := read(t, root, "pre-commit")
	for _, want := range []string{"npx lefthook run pre-commit", Begin, body, End} {
		if !strings.Contains(got, want) {
			t.Errorf("hook lost %q:\n%s", want, got)
		}
	}

	// Reinstalling with a new body must replace the block, not stack another.
	if _, err := Install(root, every("echo two")); err != nil {
		t.Fatal(err)
	}
	got = read(t, root, "pre-commit")
	if strings.Count(got, Begin) != 1 {
		t.Errorf("block was duplicated:\n%s", got)
	}
	if strings.Contains(got, body) {
		t.Errorf("old body survived:\n%s", got)
	}
}

func TestUninstallGivesTheHookBack(t *testing.T) {
	root := repo(t)
	dir, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "#!/bin/sh\nnpx lefthook run pre-commit\n"
	kept := filepath.Join(dir, "pre-commit")
	if err := os.WriteFile(kept, []byte(existing), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Install(root, every(body)); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(root); err != nil {
		t.Fatal(err)
	}

	if got := read(t, root, "pre-commit"); got != existing {
		t.Errorf("hook not restored:\n%q\nwant\n%q", got, existing)
	}
	// A hook gofi created has nothing left in it, so it goes away entirely.
	if _, err := os.Stat(filepath.Join(dir, "post-merge")); !os.IsNotExist(err) {
		t.Errorf("post-merge survived uninstall: %v", err)
	}
	if got := Installed(root); len(got) != 0 {
		t.Errorf("Installed() = %v after uninstall", got)
	}
}

func TestDirRejectsSomethingThatIsNotARepository(t *testing.T) {
	if _, err := Dir(t.TempDir()); err == nil {
		t.Error("a directory with no git repository resolved a hooks dir")
	}
}

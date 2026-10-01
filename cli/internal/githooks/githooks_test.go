package githooks

import (
	"os"
	"os/exec"
	"path/filepath"
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

	// A block as a release that installed hooks wrote it: appended to a hook
	// someone else owns, and alone in a hook gofi created.
	block := Begin + "\ngofi graph build --update --fast >/dev/null 2>&1 || true\n" + End + "\n"
	if err := os.WriteFile(kept, []byte(existing+block), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "post-merge"), []byte("#!/bin/sh\n"+block), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Installed(root); len(got) != 2 {
		t.Fatalf("Installed() = %v, want both hooks carrying the block", got)
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

func TestInstallKeepsTheTeamsHookAndIsIdempotent(t *testing.T) {
	root := repo(t)
	dir, err := Dir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	team := "#!/bin/sh\nnpx lefthook run pre-commit\n"
	if err := os.WriteFile(filepath.Join(dir, "pre-commit"), []byte(team), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Install(root, every("gofi index code"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != len(Managed) {
		t.Fatalf("results = %v", res)
	}
	got := read(t, root, "pre-commit")
	if !strings.HasPrefix(got, team) || !strings.Contains(got, Begin+"\ngofi index code\n"+End) {
		t.Errorf("pre-commit = %q", got)
	}
	again, _ := Install(root, every("gofi index code"))
	for _, r := range again {
		if r.Action != Unchanged {
			t.Errorf("second install touched %s: %s", r.Hook, r.Action)
		}
	}
}

// Only the old blocks go: the current ones are the project's hooks now.
func TestUninstallLegacyLeavesCurrentBlocks(t *testing.T) {
	root := repo(t)
	if _, err := Install(root, map[string]string{"pre-commit": body, "post-merge": "gofi index code"}); err != nil {
		t.Fatal(err)
	}
	if got := Legacy(root); len(got) != 1 || got[0] != "pre-commit" {
		t.Fatalf("legacy = %v", got)
	}
	res, err := UninstallLegacy(root)
	if err != nil || len(res) != 1 || res[0].Hook != "pre-commit" {
		t.Fatalf("removed %v (%v)", res, err)
	}
	if got := Installed(root); len(got) != 1 || got[0] != "post-merge" {
		t.Errorf("installed after cleanup = %v", got)
	}
}

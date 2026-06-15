package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGoMod(t *testing.T, root, rel, module string) {
	t.Helper()
	writeGoModWithVersion(t, root, rel, module, "1.25")
}

func writeGoModWithVersion(t *testing.T, root, rel, module, goVersion string) {
	t.Helper()
	full := filepath.Join(root, rel, "go.mod")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "module " + module + "\n\ngo " + goVersion + "\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureGoWorkSDK_AddsRootModuleWhenSinglePresent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi-sdk-go")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(body)
	if !strings.Contains(got, "./src") || !strings.Contains(got, "./.gofi/gofi-sdk-go") {
		t.Errorf("expected both paths, got:\n%s", got)
	}
	if !strings.Contains(got, "use (") {
		t.Errorf("expected grouped use directive, got:\n%s", got)
	}
}

func TestEnsureGoWorkSDK_AddsAllSubmodulesForMultiModuleSDK(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi")
	writeGoMod(t, root, ".gofi/gofi-sdk-go/sqln", "github.com/joaoprofile/gofi/sqln")
	writeGoMod(t, root, ".gofi/gofi-sdk-go/iam", "github.com/joaoprofile/gofi/iam")
	writeGoMod(t, root, ".gofi/gofi-sdk-go/netx", "github.com/joaoprofile/gofi/netx")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(body)

	for _, want := range []string{
		"./src",
		"./.gofi/gofi-sdk-go",
		"./.gofi/gofi-sdk-go/iam",
		"./.gofi/gofi-sdk-go/netx",
		"./.gofi/gofi-sdk-go/sqln",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in go.work, got:\n%s", want, got)
		}
	}
}

func TestEnsureGoWorkSDK_RemovesAllManagedPathsWhenSDKAbsent(t *testing.T) {
	root := t.TempDir()
	body := "go 1.25\n\nuse (\n\t./src\n\t./.gofi/gofi-sdk-go\n\t./.gofi/gofi-sdk-go/sqln\n\t./.gofi/gofi-sdk-go/iam\n)\n"
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// no .gofi/gofi-sdk-go dir on purpose

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	out, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(out)
	if strings.Contains(got, ".gofi/gofi-sdk-go") {
		t.Errorf("expected all managed paths removed, got:\n%s", got)
	}
	if !strings.Contains(got, "use ./src") {
		t.Errorf("expected single-line use ./src after collapse, got:\n%s", got)
	}
}

func TestEnsureGoWorkSDK_ResyncsWhenSubmoduleSetChanges(t *testing.T) {
	root := t.TempDir()
	// previous run added the root + sqln; new run adds iam and drops sqln
	body := "go 1.25\n\nuse (\n\t./src\n\t./.gofi/gofi-sdk-go\n\t./.gofi/gofi-sdk-go/sqln\n)\n"
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi")
	writeGoMod(t, root, ".gofi/gofi-sdk-go/iam", "github.com/joaoprofile/gofi/iam")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	out, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(out)
	if strings.Contains(got, "./.gofi/gofi-sdk-go/sqln") {
		t.Errorf("stale submodule sqln should have been removed, got:\n%s", got)
	}
	if !strings.Contains(got, "./.gofi/gofi-sdk-go/iam") {
		t.Errorf("new submodule iam should have been added, got:\n%s", got)
	}
}

func TestEnsureGoWorkSDK_Idempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoMod(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi")
	writeGoMod(t, root, ".gofi/gofi-sdk-go/sqln", "github.com/joaoprofile/gofi/sqln")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(root, "go.work"))
	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("second: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(root, "go.work"))
	if string(first) != string(second) {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestEnsureGoWorkSDK_BumpsGoDirectiveToMatchSDK(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi", "1.25.0")
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go/sqln", "github.com/joaoprofile/gofi/sqln", "1.25.0")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(body)
	if !strings.Contains(got, "go 1.25.0") {
		t.Errorf("expected go.work to bump go directive to 1.25.0, got:\n%s", got)
	}
}

func TestEnsureGoWorkSDK_DoesNotDowngradeGoDirective(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.25.5\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi", "1.25.0")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(body)
	if !strings.Contains(got, "go 1.25.5") {
		t.Errorf("expected go.work to keep 1.25.5 (>= sdk 1.25.0), got:\n%s", got)
	}
	if strings.Contains(got, "go 1.25.0") {
		t.Errorf("expected go.work NOT to mention 1.25.0, got:\n%s", got)
	}
}

func TestEnsureGoWorkSDK_PicksMaxAcrossSubmodules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.24\n\nuse ./src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go", "github.com/joaoprofile/gofi", "1.24.5")
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go/sqln", "github.com/joaoprofile/gofi/sqln", "1.25.3")
	writeGoModWithVersion(t, root, ".gofi/gofi-sdk-go/iam", "github.com/joaoprofile/gofi/iam", "1.24.0")

	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "go.work"))
	got := string(body)
	if !strings.Contains(got, "go 1.25.3") {
		t.Errorf("expected go.work to bump to max submodule version 1.25.3, got:\n%s", got)
	}
}

func TestCmpGoVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.25", "1.25.0", -1}, // toolchain treats 1.25.0 as stricter than 1.25
		{"1.25.0", "1.25", 1},
		{"1.25", "1.25.1", -1},
		{"1.25.5", "1.25.0", 1},
		{"1.26", "1.25.99", 1},
		{"1.25.0", "1.25.0", 0},
		{"", "1.25", -1},
		{"1.25", "", 1},
	}
	for _, c := range cases {
		if got := cmpGoVersions(c.a, c.b); got != c.want {
			t.Errorf("cmpGoVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestEnsureGoWorkSDK_NoOpWhenFileMissing(t *testing.T) {
	root := t.TempDir()
	if err := EnsureGoWorkSDK(root, "go"); err != nil {
		t.Errorf("no-op on missing go.work should succeed, got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work")); !os.IsNotExist(err) {
		t.Errorf("EnsureGoWorkSDK should not create go.work, got err=%v", err)
	}
}

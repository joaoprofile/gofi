package external

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fakeExtractor(t *testing.T, dir, language string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, BinaryName(language))
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindInProject(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	want := fakeExtractor(t, filepath.Join(root, ExtractorsDir), "java")

	spec, err := Find(root, "java")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if spec.Path != want || spec.Language != "java" {
		t.Errorf("got %+v, want path %q", spec, want)
	}
}

func TestFindOnPath(t *testing.T) {
	dir := t.TempDir()
	want := fakeExtractor(t, dir, "rust")
	t.Setenv("PATH", dir)

	spec, err := Find(t.TempDir(), "rust")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if spec.Path != want {
		t.Errorf("path = %q, want %q", spec.Path, want)
	}
}

// A version pinned inside the project must never be shadowed by whatever
// happens to be installed globally on the machine.
func TestFindPrefersProjectOverPath(t *testing.T) {
	root := t.TempDir()
	global := t.TempDir()
	fakeExtractor(t, global, "java")
	t.Setenv("PATH", global)
	want := fakeExtractor(t, filepath.Join(root, ExtractorsDir), "java")

	spec, err := Find(root, "java")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if spec.Path != want {
		t.Errorf("path = %q, want the project copy %q", spec.Path, want)
	}
}

func TestFindMissingIsActionable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Find(t.TempDir(), "cobol")
	if err == nil {
		t.Fatal("expected an error")
	}
	// The message is the only thing standing between the user and a dead end,
	// so it has to name the command that fixes it.
	if !strings.Contains(err.Error(), "gofi graph install cobol") {
		t.Errorf("error %q does not say how to fix it", err)
	}
}

func TestInstalled(t *testing.T) {
	root := t.TempDir()
	global := t.TempDir()
	fakeExtractor(t, filepath.Join(root, ExtractorsDir), "java")
	fakeExtractor(t, filepath.Join(root, ExtractorsDir), "rust")
	// "java" in both places must be reported once.
	fakeExtractor(t, global, "java")
	fakeExtractor(t, global, "python")
	// Files that do not follow the naming convention are not extractors.
	fakeExtractor(t, global, "")
	if err := os.WriteFile(filepath.Join(global, "gofi"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ExtractorsDir, BinaryName("adir")), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", global)

	got := Installed(root)
	want := []string{"java", "python", "rust"}
	if !slices.Equal(got, want) {
		t.Errorf("Installed = %v, want %v", got, want)
	}
}

func TestInstalledOnEmptyProject(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := Installed(t.TempDir()); len(got) != 0 {
		t.Errorf("Installed = %v, want none", got)
	}
}

func TestLanguageOf(t *testing.T) {
	tests := []struct{ file, want string }{
		{"gofi-graph-java", "java"},
		{"gofi-graph-java.exe", "java"},
		{"gofi-graph-c-sharp", "c-sharp"},
		{"gofi", ""},
		{"gofi-graph-", ""},
		{"graph-java", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := languageOf(tt.file); got != tt.want {
			t.Errorf("languageOf(%q) = %q, want %q", tt.file, got, tt.want)
		}
	}
}

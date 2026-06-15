package sources

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectCache_GofiCacheDirOverride(t *testing.T) {
	t.Setenv("GOFI_CACHE_DIR", "/custom/cache")
	c, err := ProjectCache("/some/proj")
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != "/custom/cache" {
		t.Errorf("expected /custom/cache, got %s", c.Root)
	}
}

func TestProjectCache_DefaultsToProject(t *testing.T) {
	t.Setenv("GOFI_CACHE_DIR", "")
	c, err := ProjectCache("/some/proj")
	if err != nil {
		t.Fatal(err)
	}
	if c.Root != filepath.Join("/some/proj", ".gofi", "cache") {
		t.Errorf("unexpected root: %s", c.Root)
	}
}

func TestProjectCache_RequiresProjectRootWhenNoEnv(t *testing.T) {
	t.Setenv("GOFI_CACHE_DIR", "")
	if _, err := ProjectCache(""); err == nil {
		t.Error("expected error when no project root and no env")
	}
}

func TestSourcePath(t *testing.T) {
	c := &Cache{Root: "/tmp/gofi-cache"}
	r := Ref{Host: "github.com", Owner: "x", Repo: "y", Ref: "abc123"}
	want := filepath.Join("/tmp/gofi-cache", "sources", "github.com", "x", "y", "abc123")
	if got := c.SourcePath(r); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestHasSource(t *testing.T) {
	tmp := t.TempDir()
	c := &Cache{Root: tmp}
	r := Ref{Host: "github.com", Owner: "x", Repo: "y", Ref: "abc"}
	if c.HasSource(r) {
		t.Fatal("expected false on empty cache")
	}
	if err := os.MkdirAll(c.SourcePath(r), 0o755); err != nil {
		t.Fatal(err)
	}
	if !c.HasSource(r) {
		t.Fatal("expected true after creating source path")
	}
}

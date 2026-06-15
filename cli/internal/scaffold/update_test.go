package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstallAgentsContent_UpdateMode_PreservesUserContent(t *testing.T) {
	projectRoot := t.TempDir()

	// Seed a fully-installed .claude/ via the embedded snapshot.
	installAgentsFromFixture(t, projectRoot, sampleData())

	// Simulate user-installed training topic and edited memory.
	userTopic := filepath.Join(projectRoot, ".claude/knowledge/pd/dominio-fiscal.md")
	if err := os.WriteFile(userTopic, []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}
	userMemory := filepath.Join(projectRoot, ".claude/memory/project.md")
	if err := os.WriteFile(userMemory, []byte("USER EDITED MEMORY"), 0o644); err != nil {
		t.Fatal(err)
	}

	// New gofi-agents tarball: updated CLAUDE.md + new agent body. Memory
	// template must NOT be re-rendered, knowledge seed must NOT overwrite.
	updatedFS := fstest.MapFS{
		"ai/claude/CLAUDE.md":          {Data: []byte("# CLAUDE — updated")},
		"ai/skills/gofi-pd.md":         {Data: []byte("new pd skill")},
		"ai/templates/sdd-template.md": {Data: []byte("new spec template")},
		"ai/memory/project.md.tmpl":    {Data: []byte("# Memory — {{.ProjectName}}")},
		"ai/knowledge/shared/seed.md":  {Data: []byte("seed should not appear post-update")},
	}

	if _, err := InstallAgentsContent(updatedFS, ".", projectRoot, sampleData(), InstallUpdate); err != nil {
		t.Fatalf("update: %v", err)
	}

	mustContain(t, filepath.Join(projectRoot, ".claude/CLAUDE.md"), "updated")
	mustContain(t, filepath.Join(projectRoot, ".claude/skills/gofi-pd.md"), "new pd skill")
	mustContain(t, filepath.Join(projectRoot, ".claude/templates/sdd-template.md"), "new spec template")

	got, err := os.ReadFile(userTopic)
	if err != nil || string(got) != "user content" {
		t.Errorf("user training topic was modified: err=%v content=%q", err, got)
	}
	gotMem, err := os.ReadFile(userMemory)
	if err != nil || string(gotMem) != "USER EDITED MEMORY" {
		t.Errorf("user memory was modified: err=%v content=%q", err, gotMem)
	}
}

func TestInstallSDKContent_OverwritesPriorInstall(t *testing.T) {
	projectRoot := t.TempDir()

	first := fstest.MapFS{
		"sdk/go/boilerplates/m.md":    {Data: []byte("v1")},
		"sdk/go/sdk-docs/overview.md": {Data: []byte("v1 docs")},
		"sdk/go/knowledge/k.md":       {Data: []byte("v1 knowledge")},
	}
	if _, err := InstallSDKContent(first, "sdk/go", projectRoot, "go"); err != nil {
		t.Fatalf("install v1: %v", err)
	}
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/boilerplates/m.md"), "v1")
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/sdk-docs/overview.md"), "v1 docs")
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/knowledge/k.md"), "v1 knowledge")

	second := fstest.MapFS{
		"sdk/go/boilerplates/m.md":    {Data: []byte("v2")},
		"sdk/go/sdk-docs/overview.md": {Data: []byte("v2 docs")},
		"sdk/go/knowledge/k.md":       {Data: []byte("v2 knowledge")},
	}
	if _, err := InstallSDKContent(second, "sdk/go", projectRoot, "go"); err != nil {
		t.Fatalf("install v2: %v", err)
	}
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/boilerplates/m.md"), "v2")
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/sdk-docs/overview.md"), "v2 docs")
	mustContain(t, filepath.Join(projectRoot, ".claude/sdk/go/knowledge/k.md"), "v2 knowledge")
}

func TestInstallSDKContent_NoLayoutReturnsSentinel(t *testing.T) {
	projectRoot := t.TempDir()

	// Source dir exists but has none of boilerplates/sdk-docs/knowledge.
	src := fstest.MapFS{
		"base/foo.go": {Data: []byte("package base")},
		"netx/bar.go": {Data: []byte("package netx")},
		"README.md":   {Data: []byte("# sdk repo")},
	}
	_, err := InstallSDKContent(src, ".", projectRoot, "go")
	if err == nil {
		t.Fatal("expected ErrNoSDKLayout, got nil")
	}
	if !errors.Is(err, ErrNoSDKLayout) {
		t.Fatalf("expected ErrNoSDKLayout, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(projectRoot, ".claude/sdk/go")); statErr == nil {
		t.Errorf(".claude/sdk/go should not exist after a failed install")
	}
}

func TestCleanLegacySDKLayout_RemovesPreV24Dirs(t *testing.T) {
	projectRoot := t.TempDir()
	mk := func(rel string) {
		full := filepath.Join(projectRoot, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("legacy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(".claude/boilerplates/m.md")
	mk(".claude/gofi-sdk-go/overview.md")
	mk(".claude/gofi-sdk-rust/overview.md")
	mk(".claude/sdk-knowledge/go/k.md")
	mk(".claude/CLAUDE.md") // must survive

	removed := CleanLegacySDKLayout(projectRoot)
	if len(removed) != 4 {
		t.Errorf("expected 4 legacy dirs removed, got %d: %v", len(removed), removed)
	}
	for _, gone := range []string{
		".claude/boilerplates",
		".claude/gofi-sdk-go",
		".claude/gofi-sdk-rust",
		".claude/sdk-knowledge",
	} {
		if _, err := os.Stat(filepath.Join(projectRoot, gone)); err == nil {
			t.Errorf("%s should have been removed", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".claude/CLAUDE.md")); err != nil {
		t.Errorf("CLAUDE.md should not be touched: %v", err)
	}
}

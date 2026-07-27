package extensions

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/vsix"
)

// repoRoot walks up from the test's directory to the repo root, identified by
// the extension source folder sitting next to the cli module.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if info, err := os.Stat(filepath.Join(dir, vsix.GofiAIDir, "package.json")); err == nil && !info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find %s/ above %s", vsix.GofiAIDir, dir)
		}
		dir = parent
	}
}

// TestEmbeddedVSIXMatchesSources is the guard that makes the committed
// artefact trustworthy: it rebuilds the package from gofi-ai/ and compares
// bytes. Editing the extension without running `go generate` fails here rather
// than silently shipping the previous build to users.
func TestEmbeddedVSIXMatchesSources(t *testing.T) {
	srcDir := filepath.Join(repoRoot(t), vsix.GofiAIDir)

	fresh, freshManifest, err := vsix.Build(os.DirFS(srcDir))
	if err != nil {
		t.Fatalf("build from sources: %v", err)
	}
	embedded, embeddedManifest, err := Embedded()
	if err != nil {
		t.Fatalf("read embedded: %v", err)
	}

	if embeddedManifest.Version != freshManifest.Version {
		t.Fatalf("embedded vsix is version %s, sources are %s — run `go generate ./internal/extensions`",
			embeddedManifest.Version, freshManifest.Version)
	}
	if !bytes.Equal(fresh, embedded) {
		t.Errorf("embedded vsix differs from gofi-ai/ — run `go generate ./internal/extensions`")
	}
}

// TestVSIXLayout pins the parts `code --install-extension` actually reads.
func TestVSIXLayout(t *testing.T) {
	data, manifest, err := Embedded()
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a readable zip: %v", err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}

	for _, required := range []string{
		"extension.vsixmanifest",
		"[Content_Types].xml",
		"extension/package.json",
		"extension/extension.js",
		"extension/media/main.js",
		"extension/media/main.css",
		"extension/images/icon.png",
	} {
		if !names[required] {
			t.Errorf("vsix is missing %s", required)
		}
	}

	// Dev-only trees must not reach users: node_modules would balloon the
	// binary, and the test suite is for this repo. Both exclusions are
	// load-bearing, and the .vscodeignore next to the extension does not drive
	// this packager — only vsix.go does — so they are pinned here.
	for name := range names {
		for _, unwanted := range []string{"extension/node_modules/", "extension/test/"} {
			if strings.HasPrefix(name, unwanted) {
				t.Errorf("vsix contains %s — %s must not be packaged", name, unwanted)
			}
		}
	}

	if manifest.ID() != "gofi.gofi-ai" {
		t.Errorf("extension id = %q, want gofi.gofi-ai", manifest.ID())
	}
}

// TestBuildIsReproducible backs the byte comparison in the staleness test:
// if packaging were not deterministic, that test would fail at random.
func TestBuildIsReproducible(t *testing.T) {
	srcDir := filepath.Join(repoRoot(t), vsix.GofiAIDir)

	first, _, err := vsix.Build(os.DirFS(srcDir))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := vsix.Build(os.DirFS(srcDir))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two builds of the same sources produced different bytes")
	}
}

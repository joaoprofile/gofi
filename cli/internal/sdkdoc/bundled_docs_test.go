package sdkdoc

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

// bundledGoDocs is the Go SDK harness content shipped in this repo.
const bundledGoDocs = "ai/sdk/go"

// repoRoot is the gofi monorepo root, three levels above this package.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, bundledGoDocs)); err != nil {
		t.Skipf("%s not found from %s", bundledGoDocs, root)
	}
	return root
}

// The bundled docs describe the release DefaultSDKGoRef pins: every curated
// doc says so in its frontmatter and the generated reference in its header.
// Bumping config.SDKGoVersion without the docs, or the docs without it, fails.
func TestBundledGoDocsDescribePinnedRelease(t *testing.T) {
	root := repoRoot(t)
	want := config.SDKGoVersion

	for _, dir := range []string{"knowledge", "boilerplates"} {
		files, _ := filepath.Glob(filepath.Join(root, bundledGoDocs, dir, "*.md"))
		if len(files) == 0 {
			t.Fatalf("no docs under %s/%s", bundledGoDocs, dir)
		}
		for _, f := range files {
			if got := frontmatterField(t, f, "sdk"); got != want {
				t.Errorf("%s: sdk: %q, want %q", rel(root, f), got, want)
			}
		}
	}

	header := "SDK gofi/" + want
	api, _ := filepath.Glob(filepath.Join(root, bundledGoDocs, "api", "*.md"))
	for _, f := range api {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), header) {
			t.Errorf("%s: generated from another release (want %q); regenerate it from %s", rel(root, f), header, config.DefaultSDKGoRef)
		}
	}
}

// The curated docs cite only identifiers the pinned SDK release exports. It
// needs that release's source: CI checks it out and sets GOFI_SDK_CHECKOUT
// (make sdk-docs-check does the same locally).
func TestBundledGoDocsCitePinnedSDK(t *testing.T) {
	checkout := os.Getenv("GOFI_SDK_CHECKOUT")
	if checkout == "" {
		t.Skipf("set GOFI_SDK_CHECKOUT to a checkout of %s", config.DefaultSDKGoRef)
	}
	root := repoRoot(t)
	exports, err := ReadExports(checkout)
	if err != nil {
		t.Fatalf("read %s: %v", checkout, err)
	}
	if len(exports["netx"]) == 0 {
		t.Fatalf("%s does not look like a gofi-sdk-go checkout", checkout)
	}
	for _, d := range exports.CheckDocs(root, bundledGoDocs+"/knowledge", bundledGoDocs+"/boilerplates") {
		t.Errorf("%s cites what %s does not export: %s", d.Path, config.DefaultSDKGoRef, strings.Join(d.Stale, ", "))
	}
}

// frontmatterField returns the value of key in the leading --- block.
func frontmatterField(t *testing.T, path, key string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return ""
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if v, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

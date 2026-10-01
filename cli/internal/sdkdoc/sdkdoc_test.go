package sdkdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeSDK(t *testing.T) string {
	src := t.TempDir()
	put(t, src, "sqln/go.mod", "module example.com/sdk/sqln\n\ngo 1.22\n")
	put(t, src, "sqln/criteria/criteria.go", `// Package criteria builds queries.
package criteria

// Criteria is a query under construction.
type Criteria struct{ table string }

// From starts a query on a table.
func From(table, alias string) *Criteria { return &Criteria{table: table} }

// Select picks the columns.
func (c *Criteria) Select(cols ...string) *Criteria { return c }

func hidden() {}
`)
	put(t, src, "sqln/internal/x/x.go", "package x\n\n// X is internal.\nfunc X() {}\n")
	put(t, src, "examples/search/go.mod", "module example.com/ex\n")
	put(t, src, "examples/search/main.go", "package main\n\nfunc main() {}\n")
	put(t, src, "examples/search/README.md", "# Busca paginada\n\nUm job que lista produtos com filtros.\n\n## Rodar\n")
	return src
}

// The reference is the code's own: signatures and doc comments, one section
// per exported symbol, internal and example packages left out.
func TestGenerateWritesOneSectionPerSymbol(t *testing.T) {
	src, out := fakeSDK(t), t.TempDir()
	put(t, out, "gone.md", "# stale\n")
	pkgs, err := Generate(src, out, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 || pkgs[0].ImportPath != "example.com/sdk/sqln/criteria" || pkgs[0].File != "sqln-criteria.md" {
		t.Fatalf("pkgs = %+v", pkgs)
	}
	b, _ := os.ReadFile(filepath.Join(out, "sqln-criteria.md"))
	body := string(b)
	for _, want := range []string{
		"# sqln/criteria", `import "example.com/sdk/sqln/criteria"`, "SDK v1.2.3",
		"### criteria.From", "func From(table, alias string) *Criteria", "From starts a query on a table.",
		"### Criteria.Select", "## criteria.Criteria",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reference lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "hidden") || strings.Contains(body, "table string") {
		t.Errorf("unexported detail leaked:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(out, "gone.md")); !os.IsNotExist(err) {
		t.Error("a reference for a package the SDK no longer has survived")
	}
	idx, _ := os.ReadFile(filepath.Join(out, IndexFile))
	if !strings.Contains(string(idx), "Package criteria builds queries.") && !strings.Contains(string(idx), "criteria builds queries") {
		t.Errorf("index = %s", idx)
	}
}

func TestExamplesPointAtTheCheckout(t *testing.T) {
	src, out := fakeSDK(t), t.TempDir()
	exs, err := WriteExamples(src, out, ".gofi/gofi-sdk-go", "v1.2.3")
	if err != nil || len(exs) != 1 {
		t.Fatalf("examples = %+v (%v)", exs, err)
	}
	b, _ := os.ReadFile(filepath.Join(out, ExamplesFile))
	for _, want := range []string{"## Busca paginada", "Um job que lista produtos com filtros.", "`.gofi/gofi-sdk-go/examples/search/main.go`"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("examples lack %q:\n%s", want, b)
		}
	}
}

func TestExamplesKeepAURLBase(t *testing.T) {
	if got := under("https://github.com/o/r/tree/main", "examples/a", "main.go"); got != "https://github.com/o/r/tree/main/examples/a/main.go" {
		t.Errorf("under = %q", got)
	}
}

// A document citing an API the SDK no longer has is caught; what the SDK has,
// and names from other packages, pass.
func TestStaleCitations(t *testing.T) {
	src := fakeSDK(t)
	exp, err := ReadExports(src)
	if err != nil {
		t.Fatal(err)
	}
	got := exp.Stale("Use criteria.From, then criteria.Where; errors come from fmt.Errorf; x.Criteria is fine? criteria.Criteria too.")
	if len(got) != 1 || got[0] != "criteria.Where" {
		t.Errorf("stale = %v", got)
	}
	if _, internal := exp["x"]; internal {
		t.Error("internal packages count as the SDK's API")
	}
}

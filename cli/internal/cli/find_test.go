package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

func runFind(t *testing.T, dir string, args ...string) string {
	t.Helper()
	t.Chdir(dir)
	var out bytes.Buffer
	cmd := newFindCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("find: %v\n%s", err, out.String())
	}
	return out.String()
}

// The document index is not versioned, so a fresh clone has none: the first
// query builds it, and an edited document is picked up by the next one. The
// markdown indexes the project versions are never rewritten by a read.
func TestFindBuildsTheDocumentIndex(t *testing.T) {
	root := configuredProject(t)
	spec := filepath.Join(root, "specs", "order", "sdd-order.md")
	writeFile(t, spec, "---\ncontexto: order\n---\n# Pedido\n\n## Faturamento\n\nO pedido e faturado apos a reserva.\n")
	markdownIndex := filepath.Join(root, "specs", docs.IndexMarkdown)

	if out := runFind(t, root, "faturamento"); !strings.Contains(out, "sdd-order.md") {
		t.Fatalf("the spec was not found on a project with no index:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(layout.DocsDir), docs.IndexFile)); err != nil {
		t.Fatalf("no document index written: %v", err)
	}
	if _, err := os.Stat(markdownIndex); !os.IsNotExist(err) {
		t.Error("a query wrote the versioned markdown index")
	}

	writeFile(t, spec, "---\ncontexto: order\n---\n# Pedido\n\n## Estorno\n\nO estorno devolve o credito ao cliente.\n")
	if out := runFind(t, root, "estorno credito"); !strings.Contains(out, "sdd-order.md") {
		t.Errorf("an edited spec was answered from the stale index:\n%s", out)
	}
}

func TestFindTextListsTheLines(t *testing.T) {
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, "src", "app", "errors.go"), "package app\n\nvar ErrGone = \"pedido removido\"\n")

	out := runFind(t, root, "--text", "pedido removido")
	for _, want := range []string{"src/app/errors.go", "3  var ErrGone", "1 matching lines in 1 files"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if out := runFind(t, root, "--regex", "-i", "ERRGONE\\s*="); !strings.Contains(out, "errors.go") {
		t.Errorf("regex found nothing:\n%s", out)
	}
}

// A pack whose contract does not load fails the check, like a document with a
// broken frontmatter.
func TestIndexCheckFailsOnABrokenPack(t *testing.T) {
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, ".claude", "expertise", "x", "PACK.md"), "---\npack: x\ntitle: X\nserves: [eng]\n---\n")
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check"})
	if err := cmd.Execute(); err == nil || !strings.Contains(out.String(), "no signal") {
		t.Errorf("err=%v output:\n%s", err, out.String())
	}
}

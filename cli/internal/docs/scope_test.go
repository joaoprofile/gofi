package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, root, rel, ctx, version, keywords string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntipo: spec\nformato: 2\ncontexto: " + ctx + "\nsubmodulo: n/a\nversao: \"" + version +
		"\"\nstatus: aprovado\nkeywords: [" + keywords + "]\n---\n# t\n\n## X\n\nt.\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStagedScope(t *testing.T) {
	s := StagedScope([]string{
		"specs/order/sdd-order.md",
		"prd/cms/INDEX.md",
		"specs/order/diagrams/flow.md",
		".claude/memory/contexts/order.md",
		".claude/knowledge/shared/clean-code.md",
		"backend/main.go",
	})
	if !s.Dirs["specs/order"] || len(s.Dirs) != 1 {
		t.Errorf("only folders with a staged document are in scope: %v", s.Dirs)
	}
	if !s.Documents || !s.Knowledge || !s.Anything {
		t.Errorf("unexpected flags: %+v", s)
	}
	if idle := StagedScope([]string{"backend/main.go"}); idle.Documents || idle.Knowledge || len(idle.Dirs) != 0 {
		t.Errorf("a code-only commit stages no document: %+v", idle)
	}
	if !StagedScope(nil).idle(true) || !StagedScope([]string{"prd/x/INDEX.md"}).idle(false) {
		t.Error("nothing staged (or only an INDEX.md) must be idle")
	}
}

// staleProject reproduces the situation behind the fix: a committed index that
// no longer matches its documents (context "billing" moved to 1.1 without a
// rebuild), and a commit that touches only context "order".
func staleProject(t *testing.T) (root, staleShard, staleRouter string) {
	t.Helper()
	root = t.TempDir()
	writeSpec(t, root, "specs/billing/sdd-billing.md", "billing", "1.0", "fatura")
	writeSpec(t, root, "specs/order/sdd-order.md", "order", "1.0", "venda")
	if _, _, _, err := (&Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, root, "specs/billing/sdd-billing.md", "billing", "1.1", "fatura, competencia")
	return root, read(t, root, "specs/billing/INDEX.md"), read(t, root, "specs/INDEX.md")
}

// Regression: every commit rebuilt every INDEX.md, rewriting contexts the
// commit did not touch and leaving them dirty in every developer's tree.
func TestScopedBuild_LeavesUntouchedContextsByteForByte(t *testing.T) {
	root, staleShard, staleRouter := staleProject(t)
	writeSpec(t, root, "specs/order/sdd-order.md", "order", "1.0", "venda, pedido")

	b := &Builder{Root: root, Scope: StagedScope([]string{"specs/order/sdd-order.md"})}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, root, "specs/billing/INDEX.md"); got != staleShard {
		t.Errorf("the billing shard was not staged and must not change:\n%s", got)
	}
	router := read(t, root, "specs/INDEX.md")
	billingRow := func(s string) string {
		for _, l := range strings.Split(s, "\n") {
			if strings.HasPrefix(l, "| billing |") {
				return l
			}
		}
		return ""
	}
	if billingRow(router) != billingRow(staleRouter) {
		t.Errorf("the billing router row must stay as it was:\nbefore %s\nafter  %s", billingRow(staleRouter), billingRow(router))
	}
	if !strings.Contains(read(t, root, "specs/order/INDEX.md"), "pedido") {
		t.Error("the staged order shard must be rebuilt")
	}
	for _, w := range b.Written {
		if strings.Contains(w, "billing") {
			t.Errorf("billing must not be written: %v", b.Written)
		}
	}
}

// Rule: with nothing to index, the scoped build touches nothing.
func TestScopedBuild_NothingStagedTouchesNothing(t *testing.T) {
	root, staleShard, staleRouter := staleProject(t)
	b := &Builder{Root: root, WithCode: false, Scope: StagedScope([]string{"backend/main.go"})}
	idx, _, _, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if idx != nil || len(b.Written) != 0 {
		t.Fatalf("idle scope must not build or write: idx=%v written=%v", idx != nil, b.Written)
	}
	if read(t, root, "specs/billing/INDEX.md") != staleShard || read(t, root, "specs/INDEX.md") != staleRouter {
		t.Fatal("no index may change when nothing is staged")
	}
}

func TestScopedBuild_UnchangedContentIsNotRewritten(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "specs/order/sdd-order.md", "order", "1.0", "venda")
	if _, _, _, err := (&Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	b := &Builder{Root: root, Scope: StagedScope([]string{"specs/order/sdd-order.md"})}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if len(b.Written) != 0 {
		t.Fatalf("an index whose content did not change must not be written: %v", b.Written)
	}
}

func TestScopedBuild_DeletedFolderDropsShardAndRow(t *testing.T) {
	root := t.TempDir()
	writeSpec(t, root, "specs/order/sdd-order.md", "order", "1.0", "venda")
	writeSpec(t, root, "specs/legacy/sdd-legacy.md", "legacy", "1.0", "velho")
	if _, _, _, err := (&Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "specs/legacy/sdd-legacy.md")); err != nil {
		t.Fatal(err)
	}
	b := &Builder{Root: root, Scope: StagedScope([]string{"specs/legacy/sdd-legacy.md"})}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "specs/legacy/INDEX.md")); !os.IsNotExist(err) {
		t.Errorf("the index of a folder left without documents must go: %v", err)
	}
	router := read(t, root, "specs/INDEX.md")
	if strings.Contains(router, "| legacy |") || !strings.Contains(router, "| order |") {
		t.Errorf("router must drop the removed context and keep the rest:\n%s", router)
	}
	if !strings.Contains(router, "_1 documentos em 1 contextos, 1 pastas._") {
		t.Errorf("footer must be recounted from the rows kept:\n%s", router)
	}
}

// Regression: an operation that is also a keyword was listed twice.
func TestRenderShard_SubjectHasNoDuplicates(t *testing.T) {
	content, _ := renderShard("specs/sync", "specs", []indexEntry{{
		rel: "specs/sync/a.md", dir: "specs/sync", ctx: "sync", sub: "n/a", version: "1.0", status: "draft",
		operations: []string{"buybox", "pricing"}, keywords: []string{"buybox", "gatilho"},
	}})
	if strings.Contains(content, "buybox, pricing, buybox") || !strings.Contains(content, "buybox, pricing, gatilho") {
		t.Errorf("subject must list each term once:\n%s", content)
	}
}

// Regression: a knowledge file with YAML frontmatter got "name: …" as its
// description.
func TestFirstProse_SkipsFrontmatter(t *testing.T) {
	got := firstProse("---\nname: document-versioning\ndescription: x\n---\n# Título\n\nA versão conta estados de produção.\n")
	if got != "A versão conta estados de produção." {
		t.Errorf("got %q", got)
	}
	if firstProse("Primeira frase.\n") != "Primeira frase." {
		t.Error("files without frontmatter keep working")
	}
}

// Regression: the merge re-sorted every router row, moving rows of untouched
// contexts when the committed router was not in byte order (e.g. cms before
// churn, as a hand edit left it).
func TestMergeRootIndex_KeepsExistingRowOrder(t *testing.T) {
	head := "# Índice\n\n> Derivado — não edite à mão.\n\n| Contexto | Docs | Do que trata | Onde |\n|---|--:|---|---|\n"
	current := head +
		"| cms | 1 | x | `specs/cms/INDEX.md` |\n" +
		"| churn | 1 | y | `specs/churn/INDEX.md` |\n" +
		"| order | 1 | old | `specs/order/INDEX.md` |\n" +
		"\n_3 documentos em 3 contextos, 3 pastas._\n"
	fresh := head +
		"| churn | 1 | Y2 | `specs/churn/INDEX.md` |\n" +
		"| cms | 1 | X2 | `specs/cms/INDEX.md` |\n" +
		"| dash | 2 | new | `specs/dash/INDEX.md` |\n" +
		"| order | 1 | NEW | `specs/order/INDEX.md` |\n" +
		"\n_5 documentos em 4 contextos, 4 pastas._\n"
	got, err := mergeRootIndex(current, fresh, map[string]bool{"order": true, "dash": true})
	if err != nil {
		t.Fatal(err)
	}
	want := head +
		"| cms | 1 | x | `specs/cms/INDEX.md` |\n" +
		"| churn | 1 | y | `specs/churn/INDEX.md` |\n" +
		"| dash | 2 | new | `specs/dash/INDEX.md` |\n" +
		"| order | 1 | NEW | `specs/order/INDEX.md` |\n" +
		"\n_5 documentos em 4 contextos, 4 pastas._\n"
	if got != want {
		t.Errorf("unaffected rows must keep content and position:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

package retrieval

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/docs"
	"github.com/gofi-labs/gofi/cli/internal/graph"
	"github.com/gofi-labs/gofi/cli/internal/graph/model"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is a project with a spec, a knowledge note and a code graph.
func fixture(t *testing.T, withCode bool) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "specs/order/sdd-order.md", `---
tipo: spec
contexto: order
entidades: [order_order]
keywords: [pedido]
---
# SDD — Pedido

## 5. Regras de Negócio

### RN-01 — Pedido cancelado não pode ser faturado

Faturar um pedido cancelado é recusado com erro de conflito.

### RN-02 — Pedido faturado não pode ser cancelado

Cancelar depois de faturar é recusado.
`)
	write(t, root, ".claude/knowledge/shared/ddd-principles.md", `# DDD

## Agregado e raiz

Texto sobre agregados.

## Ciclo de vida (state machine)

Modele as transições de estado da entidade como uma máquina de estados explícita.
`)
	write(t, root, ".claude/knowledge/shared/short-note.md", "# Nota curta\n\nUse idempotência por chave única.\n")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	if withCode {
		g := &model.Graph{Schema: model.SchemaVersion, Tool: model.Tool, Language: "go", Module: "example.com/acme"}
		g.AddNode(&model.Node{ID: "func:example.com/acme/domain/order/service.BillOrder", Kind: model.KindFunc, Name: "BillOrder",
			Unit: "pkg:example.com/acme/domain/order/service", File: "domain/order/service/order_service.go", Line: 12, Lines: 8,
			Doc: "BillOrder charges an order unless it was cancelled.", Sig: "func(o *Order) error", Context: "order"})
		g.AddNode(&model.Node{ID: "func:fmt.Println", Kind: model.KindFunc, Name: "Println", External: true})
		g.AddNode(&model.Node{ID: "func:example.com/acme/domain/order/handler.Bill", Kind: model.KindFunc, Name: "Bill",
			File: "domain/order/handler/order_handler.go", Line: 20, Lines: 5})
		g.AddNode(&model.Node{ID: "func:example.com/acme/domain/invoice/service.Bill", Kind: model.KindFunc, Name: "Bill",
			File: "domain/invoice/service/invoice_service.go", Line: 8, Lines: 5})
		g.AddEdge(&model.Edge{From: "func:example.com/acme/domain/order/handler.Bill", To: "func:example.com/acme/domain/order/service.BillOrder",
			Rel: model.RelCalls, File: "domain/order/handler/order_handler.go", Line: 22, Conf: 1})
		if err := g.Save(filepath.Join(graph.Dir(root), graph.GraphFile)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func open(t *testing.T, root string) *Engine {
	t.Helper()
	e, err := Open(root, "go")
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestFindsTheSectionWithItsFileLines(t *testing.T) {
	root := fixture(t, true)
	hits := open(t, root).Find(Query{Text: "faturar pedido cancelado", Limit: 3})
	if len(hits) == 0 || hits[0].Path != "specs/order/sdd-order.md" || !strings.HasPrefix(hits[0].Title, "RN-01") {
		t.Fatalf("got %v", hits)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "specs/order/sdd-order.md"))
	lines := strings.Split(string(raw), "\n")
	if got := lines[hits[0].Start-1]; !strings.Contains(got, "RN-01") {
		t.Errorf("line %d is %q, want the RN-01 heading", hits[0].Start, got)
	}
}

// The words of a question are often in the text, not in the heading: "máquina
// de estados" is only in the body of "Ciclo de vida (state machine)".
func TestMatchesTheSectionText(t *testing.T) {
	hits := open(t, fixture(t, false)).Find(Query{Text: "como modelar a máquina de estados", Limit: 1})
	if len(hits) == 0 || hits[0].Title != "Ciclo de vida (state machine)" {
		t.Fatalf("got %v", hits)
	}
}

func TestFindsCodeSymbolsByTheirWords(t *testing.T) {
	hits := open(t, fixture(t, true)).Find(Query{Text: "bill order", Areas: []string{AreaCode}})
	if len(hits) == 0 {
		t.Fatalf("got %v", hits)
	}
	h := hits[0]
	if h.Kind != KindSymbol || h.Path != "domain/order/service/order_service.go" || h.Start != 12 || h.End != 19 || h.Context != "order" {
		t.Errorf("symbol hit = %+v", h)
	}
	if strings.Contains(h.Path, "fmt") {
		t.Error("external symbols are not answers")
	}
	if h.Detail != "func BillOrder(o *Order) error" {
		t.Errorf("detail = %q, want the declaration with its name", h.Detail)
	}
}

func TestAreasFilterAndOneHitPerDocument(t *testing.T) {
	e := open(t, fixture(t, true))
	for _, h := range e.Find(Query{Text: "pedido faturado cancelado", Areas: []string{"knowledge"}}) {
		if h.Area != "knowledge" {
			t.Errorf("--in knowledge returned %v", h)
		}
	}
	seen := map[string]bool{}
	for _, h := range e.Find(Query{Text: "pedido faturado cancelado recusado"}) {
		if h.Kind != KindSymbol && seen[h.Path] {
			t.Errorf("%s returned twice", h.Path)
		}
		seen[h.Path] = true
	}
}

func TestShortNoteWithoutSectionsIsAnswer(t *testing.T) {
	hits := open(t, fixture(t, false)).Find(Query{Text: "idempotência chave única", Limit: 1})
	if len(hits) == 0 || hits[0].Path != ".claude/knowledge/shared/short-note.md" || hits[0].Kind != KindDocument {
		t.Fatalf("got %v", hits)
	}
}

func TestDeterministicAndHonestAboutMissingSources(t *testing.T) {
	root := fixture(t, false)
	a := open(t, root).Find(Query{Text: "pedido estado"})
	b := open(t, root).Find(Query{Text: "pedido estado"})
	if len(a) != len(b) {
		t.Fatal("two runs differ")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("two runs differ at %d: %v vs %v", i, a[i], b[i])
		}
	}
	if e := open(t, root); len(e.Missing) != 1 || !strings.Contains(e.Missing[0], "code") {
		t.Errorf("a project without a graph should say so: %v", e.Missing)
	}
}

func TestSplitCase(t *testing.T) {
	for in, want := range map[string]string{"BillOrder": "Bill Order", "HTTPServer": "HTTP Server", "parseV2Token": "parse V2 Token", "plain": "plain"} {
		if got := splitCase(in); got != want {
			t.Errorf("splitCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEvaluateCountsDocumentsAndSections(t *testing.T) {
	e := open(t, fixture(t, false))
	ev := e.Evaluate([]docs.Question{
		{ID: "1", Ask: "pedido cancelado recusado com erro de conflito", Document: "specs/order/sdd-order.md", Section: "RN-01 — Pedido cancelado não pode ser faturado"},
		{ID: "2", Ask: "máquina de estados", Document: ".claude/knowledge/shared/ddd-principles.md", Section: "Agregado e raiz"},
		{ID: "3", Ask: "kubernetes helm chart", Document: "ops/none.md"},
	})
	if ev.DocTop1 != 2 || ev.SectionTop3 != 1 || len(ev.Misses) != 1 {
		t.Errorf("evaluation = %+v", ev)
	}
}

func TestShowDocumentOutlineAndSection(t *testing.T) {
	e := open(t, fixture(t, true))
	v, err := e.Show("specs/order/sdd-order.md", 0)
	if err != nil || v.Kind != ViewDocument {
		t.Fatalf("got %v, %v", v, err)
	}
	if d := v.Data.(DocumentData); len(d.Sections) != 3 || d.Context != "order" {
		t.Errorf("outline = %+v", d)
	}

	for _, ref := range []string{"specs/order/sdd-order.md#RN-01", "sdd-order.md#rn-01", "specs/order/sdd-order.md#L13"} {
		v, err := e.Show(ref, 0)
		if err != nil || v.Kind != ViewSection {
			t.Fatalf("%s: got %v, %v", ref, v, err)
		}
		sec := v.Data.(SectionData)
		if !strings.HasPrefix(sec.Heading, "RN-01") || !strings.Contains(sec.Text, "erro de conflito") || strings.Contains(sec.Text, "RN-02") {
			t.Errorf("%s: section = %+v", ref, sec)
		}
		if !strings.Contains(v.Text, "   11│ ### RN-01") {
			t.Errorf("%s: text should number the lines as in the file:\n%s", ref, v.Text)
		}
	}
	if _, err := e.Show("specs/order/sdd-order.md#nothing-like-this", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown section should be ErrNotFound, got %v", err)
	}
}

func TestShowContextTableAndSymbol(t *testing.T) {
	e := open(t, fixture(t, true))
	if v, err := e.Show("ctx:order", 0); err != nil || v.Kind != ViewContext || !strings.Contains(v.Text, "sdd-order.md") {
		t.Errorf("ctx: %v %v", v, err)
	}
	if v, err := e.Show("table:order_order", 0); err != nil || v.Kind != ViewEntity || len(v.Data.(EntityData).Documents) != 1 {
		t.Errorf("table: %v %v", v, err)
	}
	v, err := e.Show("BillOrder", 0)
	if err != nil || v.Kind != ViewSymbol {
		t.Fatalf("symbol: %v %v", v, err)
	}
	sym := v.Data.(SymbolData)
	if sym.File != "domain/order/service/order_service.go" || sym.Start != 12 || len(sym.In) != 1 || sym.In[0].Rel != "calls" {
		t.Errorf("symbol = %+v", sym)
	}
}

// Two symbols named Bill and nothing to rank one over the other: the answer is
// the list to choose from, never a guess.
func TestShowListsCandidatesInsteadOfGuessing(t *testing.T) {
	e := open(t, fixture(t, true))
	v, err := e.Show("Bill", 0)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind == ViewSymbol {
		// the graph ranking may pick one; then the others must be named
		if !strings.Contains(v.Text, "sym:") {
			t.Errorf("picked one symbol without naming the other:\n%s", v.Text)
		}
		return
	}
	if v.Kind != ViewCandidates || len(v.Data.([]Candidate)) != 2 {
		t.Errorf("got %v", v)
	}
	if _, err := e.Show("NoSuchThing", 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown ref should be ErrNotFound, got %v", err)
	}
}

func TestPathFollowsCalls(t *testing.T) {
	e := open(t, fixture(t, true))
	v, err := e.Path("handler.Bill", "BillOrder")
	if err != nil || !strings.Contains(v.Text, "BillOrder") {
		t.Errorf("path: %v %v", v, err)
	}
}

// fixtureWithLexicon adds what the vocabulary tests need: a synonym bridge, and
// a spec and a PRD that tie on every term.
func fixtureWithLexicon(t *testing.T) string {
	t.Helper()
	root := fixture(t, false)
	write(t, root, ".claude/lexicon/sinonimos.md", "# Sinônimos\n\n| Termo | Equivale a |\n|---|---|\n| expurgo | purge, delete |\n")
	write(t, root, "specs/churn/sdd-churn.md", "---\ntipo: spec\ncontexto: churn\noperacoes: [purge]\n---\n# Churn\n\n## Ordem de DELETE\n\nTexto.\n")
	write(t, root, "specs/tie/sdd-tie.md", "---\ntipo: spec\ncontexto: tie\nkeywords: [rebate]\n---\n# Tie\n\n## Regra\n\nTexto.\n")
	write(t, root, "prd/tie/prd-tie.md", "---\ntipo: prd\ncontexto: tie\nkeywords: [rebate]\n---\n# Tie\n\n## Regra\n\nTexto.\n")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	return root
}

// "purge" is only in a facet; the question is in Portuguese. The lexicon is
// the bridge, walked from either side.
func TestSynonymsBridgeLanguages(t *testing.T) {
	hits := open(t, fixtureWithLexicon(t)).Find(Query{Text: "expurgo", Limit: 3})
	if len(hits) == 0 || hits[0].Path != "specs/churn/sdd-churn.md" {
		t.Fatalf("got %v", hits)
	}
}

// The heading says DELETE; the question says "deletes".
func TestPluralsMatchTheSingular(t *testing.T) {
	hits := open(t, fixtureWithLexicon(t)).Find(Query{Text: "ordem dos deletes", Limit: 3})
	if len(hits) == 0 || hits[0].Path != "specs/churn/sdd-churn.md" {
		t.Fatalf("got %v", hits)
	}
}

// Scores tie constantly. The spec states the rule, the PRD the intent: on a
// tie the spec comes first, not whichever the walker reached first.
func TestTiesGoToTheMostAuthoritativeArea(t *testing.T) {
	hits := open(t, fixtureWithLexicon(t)).Find(Query{Text: "rebate", Limit: 5})
	if len(hits) < 2 || hits[0].Score != hits[1].Score {
		t.Fatalf("fixture should tie the spec and the PRD: %v", hits)
	}
	if hits[0].Path != "specs/tie/sdd-tie.md" {
		t.Errorf("spec should outrank prd on a tie, got %s", hits[0].Path)
	}
	for _, pair := range [][2]string{{"specs", "prd"}, {"prd", "memory"}, {"memory", "knowledge"}, {"knowledge", "code"}} {
		if authority(pair[0]) >= authority(pair[1]) {
			t.Errorf("%s should outrank %s", pair[0], pair[1])
		}
	}
}

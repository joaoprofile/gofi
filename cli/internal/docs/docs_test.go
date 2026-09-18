package docs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusFixture writes a miniature project: two specs, one PRD and one context
// memory, wired the four ways a real corpus wires itself.
func corpusFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("specs/churn/sdd-churn.md", `---
tipo: spec
formato: 2
contexto: churn
versao: "2.0"
status: aprovado
prd: prd/churn/prd-churn.md
entidades: [core_company, core_account]
operacoes: [purge, cascata]
keywords: [expurgo, offboarding]
---
# SDD — Churn

## 5. Regras de Negócio

### RN-02 — Ordem de DELETE respeita FK

Texto. Ver specs/account/sdd-account.md para a raiz.
`)
	write("specs/account/sdd-account.md", `---
tipo: spec
formato: 2
contexto: account
versao: "1.0"
status: aprovado
entidades: [core_company]
keywords: [billing]
---
# SDD — Account

## 3. Modelo de Dados

Conteúdo.
`)
	write("prd/churn/prd-churn.md", `---
tipo: prd
formato: 2
contexto: churn
versao: "1.0"
status: aprovado
keywords: [expurgo]
---
# PRD — Churn

## 6. Personas

Conteúdo.
`)
	write(".claude/memory/contexts/churn.md", `---
formato: 2
contexto: churn
versao: "2.0"
status: implementado
spec: specs/churn/sdd-churn.md
---
# churn

## Estado atual

Ver [[account]] para a raiz.
`)
	// A deliberate tie: same context, same title words, same keyword, no section
	// that either can match. Only the corpus differs, which is what the
	// tiebreak has to decide.
	write("specs/tie/sdd-tie.md", `---
tipo: spec
formato: 2
contexto: tie
versao: "1.0"
status: aprovado
keywords: [rebate]
---
# Tie

## Conteudo

Texto.
`)
	write("prd/tie/prd-tie.md", `---
tipo: prd
formato: 2
contexto: tie
versao: "1.0"
status: aprovado
keywords: [rebate]
---
# Tie

## Conteudo

Texto.
`)
	write(".claude/lexicon/sinonimos.md", `# Sinônimos

> Ponte entre a língua da pergunta e a do identificador.

| Termo | Equivale a |
|---|---|
| expurgo | purge, delete |
| tentativa | retry |
`)
	return root
}

func build(t *testing.T, root string) (*Index, *Graph) {
	t.Helper()
	idx, g, _, err := (&Builder{Root: root}).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return idx, g
}

func TestFrontmatterToleratesDrift(t *testing.T) {
	fm, body := Parse("---\ntipo: spec\nkeywords: [a, b]\nprd: n/a\n---\n# Title\ntext\n")
	if fm.Get("tipo") != "spec" {
		t.Errorf("tipo = %q", fm.Get("tipo"))
	}
	if got := fm.List("keywords"); len(got) != 2 {
		t.Errorf("keywords = %v", got)
	}
	// n/a is how the corpus writes "none"; a caller should not have to know that.
	if fm.Get("prd") != "" {
		t.Errorf("n/a should read as empty, got %q", fm.Get("prd"))
	}
	if len(body) == 0 || body[0] != "# Title" {
		t.Errorf("body = %v", body)
	}
}

func TestParseWithoutFrontmatterKeepsWholeFileAsBody(t *testing.T) {
	fm, body := Parse("# Just a title\ntext\n")
	if len(fm) != 0 {
		t.Errorf("expected no frontmatter, got %v", fm)
	}
	if body[0] != "# Just a title" {
		t.Errorf("body lost its first line: %v", body)
	}
}

func TestSectionsCarryLineRanges(t *testing.T) {
	idx, _ := build(t, corpusFixture(t))
	var churn Doc
	for _, d := range idx.Docs {
		if d.Path == "specs/churn/sdd-churn.md" {
			churn = d
		}
	}
	if len(churn.Sections) != 2 {
		t.Fatalf("expected ## and ###, got %d: %v", len(churn.Sections), churn.Sections)
	}
	// The range is the point of the index: an agent reads those lines only.
	for _, s := range churn.Sections {
		if s.End <= s.Start {
			t.Errorf("section %q has empty range L%d-%d", s.Heading, s.Start, s.End)
		}
	}
}

func TestGraphMaterializesEveryLinkKind(t *testing.T) {
	_, g := build(t, corpusFixture(t))
	want := map[string]bool{
		EdgeDeclares: false, EdgeCites: false,
		EdgeWikilink: false, EdgeTouches: false, EdgeBelongs: false,
	}
	for _, e := range g.Edges {
		want[e[2]] = true
	}
	for kind, seen := range want {
		if !seen {
			t.Errorf("no %q edge was materialized", kind)
		}
	}
}

func TestWikilinkResolvesToContext(t *testing.T) {
	_, g := build(t, corpusFixture(t))
	found := false
	for _, e := range g.Edges {
		if e[2] == EdgeWikilink && e[1] == PrefixCtx+"account" {
			found = true
		}
	}
	if !found {
		t.Error("[[account]] should resolve to the account context node")
	}
}

func TestSearchBridgesLanguagesThroughSynonyms(t *testing.T) {
	root := corpusFixture(t)
	idx, _ := build(t, root)
	s := NewSearcher(root, idx)
	// "purge" only appears as an operacoes facet; the question uses the
	// Portuguese word. Without the bridge this finds nothing.
	got := s.Search("expurgo", 5)
	if len(got) == 0 || got[0].Doc.Path != "specs/churn/sdd-churn.md" {
		t.Fatalf("expected the churn spec first, got %v", got)
	}
}

func TestSearchStemsPlurals(t *testing.T) {
	root := corpusFixture(t)
	idx, _ := build(t, root)
	// The heading says DELETE; the question says "deletes".
	if got := NewSearcher(root, idx).Search("ordem dos deletes", 5); len(got) == 0 {
		t.Error("plural in the question should still match the singular heading")
	}
}

// TestTieBreakPrefersTheAuthoritativeDocument pins the fix for a real defect:
// scores tie constantly, and with a stable sort the winner was decided by
// whichever corpus the walker happened to reach first. Ordering the corpus
// differently silently changed results — recall moved 15 points on the same
// question set with no change to the ranking code.
func TestTieBreakPrefersTheAuthoritativeDocument(t *testing.T) {
	root := corpusFixture(t)
	idx, _ := build(t, root)
	got := NewSearcher(root, idx).Search("rebate", 5)
	if len(got) < 2 {
		t.Fatalf("both tie documents should match, got %v", got)
	}
	if got[0].Score != got[1].Score {
		t.Fatalf("fixture no longer produces a tie (%d vs %d); the test is not "+
			"exercising the tiebreak anymore", got[0].Score, got[1].Score)
	}
	if got[0].Doc.Path != "specs/tie/sdd-tie.md" {
		t.Errorf("spec should outrank prd on a tie, got %q first", got[0].Doc.Path)
	}
}

func TestOrphansLeaveContextMemoryOut(t *testing.T) {
	root := corpusFixture(t)
	_, g := build(t, root)
	e := &Explorer{Graph: g}
	for _, o := range mustOrphans(e, false) {
		if o == ".claude/memory/contexts/churn.md" {
			t.Error("context memory is an entry point, not an orphan")
		}
	}
	all := mustOrphans(e, true)
	found := false
	for _, o := range all {
		if o == ".claude/memory/contexts/churn.md" {
			found = true
		}
	}
	if !found {
		t.Error("--all should bring context memory back in")
	}
}

func mustOrphans(e *Explorer, all bool) []string {
	o, _ := e.Orphans(all)
	return o
}

func TestEntityJoinsDocumentsAndCode(t *testing.T) {
	root := corpusFixture(t)
	_, g := build(t, root)
	docs, _, ok := (&Explorer{Graph: g}).Entity("core_company")
	if !ok {
		t.Fatal("core_company should be a node: two specs declare it")
	}
	if len(docs) != 2 {
		t.Errorf("expected both specs to touch core_company, got %v", docs)
	}
}

func TestIndexSkipsGeneratedIndexFiles(t *testing.T) {
	root := corpusFixture(t)
	if err := os.WriteFile(filepath.Join(root, "specs", "INDEX.md"),
		[]byte("---\ntipo: spec\ncontexto: x\n---\n# Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, _ := build(t, root)
	for _, d := range idx.Docs {
		if filepath.Base(d.Path) == "INDEX.md" {
			t.Error("INDEX.md is generated from these documents; indexing it is circular")
		}
	}
}

func TestTreeRanksNeighboursByRarityNotCount(t *testing.T) {
	root := corpusFixture(t)
	// A table every context touches must not make them all look coupled: this
	// is the same failure as ranking index terms by frequency. churn and
	// account already share core_company; give churn one more partner that
	// shares only that, and it must rank below account, which also shares the
	// rarer core_account.
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []string{"a", "b", "c"} {
		write("specs/"+c+"/sdd-"+c+".md", `---
tipo: spec
formato: 2
contexto: `+c+`
versao: "1.0"
status: aprovado
entidades: [core_company]
---
# `+c+`

## X

t.
`)
	}
	_, g := build(t, root)
	tree, ok := (&Explorer{Graph: g}).Tree("churn")
	if !ok {
		t.Fatal("churn should have a tree")
	}
	if len(tree.Neighbors) == 0 {
		t.Fatal("expected neighbours")
	}
	if tree.Neighbors[0].Name != "account" {
		t.Errorf("account shares the rarer table and should rank first, got %q",
			tree.Neighbors[0].Name)
	}
}

func TestTreeFindsUnlinkedMentions(t *testing.T) {
	root := corpusFixture(t)
	// Two documents in one context describing the same two tables, neither
	// citing the other.
	for _, n := range []string{"one", "two"} {
		p := filepath.Join(root, "specs", "dup", "sdd-"+n+".md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `---
tipo: spec
formato: 2
contexto: dup
versao: "1.0"
status: aprovado
entidades: [core_company, core_account]
---
# ` + n + `

## X

t.
`
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, g := build(t, root)
	tree, _ := (&Explorer{Graph: g}).Tree("dup")
	if len(tree.Unlinked) != 1 {
		t.Fatalf("expected one unlinked pair, got %v", tree.Unlinked)
	}
	if len(tree.Unlinked[0].Shared) != 2 {
		t.Errorf("both shared tables should be reported, got %v", tree.Unlinked[0].Shared)
	}
}

func TestContextDirectiveTiesCodeToContextWithoutATable(t *testing.T) {
	root := corpusFixture(t)
	// A context whose spec owns no table: inference by table name can never
	// reach its code, which is exactly why the directive exists.
	mk := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("specs/router/sdd-router.md", `---
tipo: spec
formato: 2
contexto: router
versao: "1.0"
status: aprovado
---
# Router

## X

t.
`)
	mk("backend/router/route.go", "//gofi:context router\npackage router\n")
	mk("backend/router/pick.go", "//gofi:context router\npackage router\n")

	// The directive is only read when code is scanned.
	idx, g, _, err := (&Builder{Root: root, WithCode: true}).Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_ = idx
	tree, ok := (&Explorer{Graph: g}).Tree("router")
	if !ok {
		t.Fatal("router should have a tree")
	}
	if len(tree.Packages) != 1 || tree.Packages[0].Files != 2 {
		t.Fatalf("expected one package with two files, got %v", tree.Packages)
	}
	for _, gap := range tree.Gaps {
		if strings.Contains(gap.Why, "gofi:context") {
			t.Error("a context with tagged packages must not be reported as missing them")
		}
	}
}

func TestGapsReportWhereTheChainStops(t *testing.T) {
	root := corpusFixture(t)
	p := filepath.Join(root, "prd", "lonely", "prd-lonely.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`---
tipo: prd
formato: 2
contexto: lonely
versao: "1.0"
status: draft
---
# Lonely

## X

t.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, g, _, err := (&Builder{Root: root}).Build()
	if err != nil {
		t.Fatal(err)
	}
	tree, _ := (&Explorer{Graph: g}).Tree("lonely")
	if len(tree.Gaps) == 0 {
		t.Fatal("a PRD nothing links to is where the chain stops; it must be reported")
	}
	// The churn PRD IS linked from its spec, one way only, and the backlink is
	// computed — so it must NOT be reported as a gap.
	churn, _ := (&Explorer{Graph: g}).Tree("churn")
	for _, gap := range churn.Gaps {
		if strings.HasPrefix(gap.What, "prd-churn") {
			t.Error("a one-way declaration still links: the backlink is computed, not a hole")
		}
	}
}

// determinismFixture writes a corpus whose code index is wide enough for map
// iteration order to show: many entities, each implemented in many files.
//
// The width is the point. A handful of entities can produce the same order
// twice by luck, and a test that passes by luck is worse than none.
func determinismFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var entities []string
	for i := range 12 {
		entities = append(entities, fmt.Sprintf("tabela_%02d", i))
	}
	write("specs/wide/sdd-wide.md", "---\ntipo: spec\nformato: 2\ncontexto: wide\nversao: \"1.0\"\nstatus: aprovado\nentidades: ["+strings.Join(entities, ", ")+"]\nkeywords: [largo]\n---\n# SDD\n\n## 1. Escopo\n\nTexto.\n")

	// Each file mentions every entity, so each one lands in every bucket.
	for i := range 15 {
		body := fmt.Sprintf("package pkg%02d\n\n// %s\nfunc F%02d() {}\n", i, strings.Join(entities, " "), i)
		write(fmt.Sprintf("backend/domain/pkg%02d/repo.go", i), body)
	}
	return root
}

// TestBuildIsDeterministic pins the invariant the whole artifact rests on: the
// graph is committed, so two builds of an unchanged corpus must produce the
// same bytes. Without this, an unordered map walk turns every rebuild into a
// diff of the entire file — and that is exactly the bug this test was written
// for, where the implementa edges came out in a different order each run.
func TestBuildIsDeterministic(t *testing.T) {
	root := determinismFixture(t)

	var first []byte
	for i := range 8 {
		b := &Builder{Root: root, WithCode: true, Language: "go"}
		_, g, _, err := b.Build()
		if err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		raw, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = raw
			if len(g.Edges) == 0 {
				t.Fatal("fixture não gerou arestas — o teste não estaria provando nada")
			}
			continue
		}
		if !bytes.Equal(first, raw) {
			t.Fatalf("build %d difere do primeiro: o grafo é commitado e precisa ser função pura das entradas", i)
		}
	}
}

func TestFacetsBreakTiesBetweenSiblingSpecs(t *testing.T) {
	root := corpusFixture(t)
	// Two specs of one context, so authority cannot separate them. One carries
	// the whole question in its curated facets; the other only has a heading
	// that happens to share a word. Weighting headings above facets makes them
	// tie, and index order then decides — which is how the document that
	// answers the question ended up below one that merely mentions a word.
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("specs/back/sdd-back-templates.md", `---
tipo: spec
formato: 2
contexto: back
submodulo: templates
versao: "1.0"
status: aprovado
operacoes: [scraper-template, scraping]
keywords: [templates]
---
# Templates

## 3. Operações

Corpo.
`)
	// Alphabetically first, so index order would put it ahead.
	write("specs/back/sdd-back-audit.md", `---
tipo: spec
formato: 2
contexto: back
submodulo: audit
versao: "1.0"
status: aprovado
keywords: [audit]
---
# Audit

## Templates de auditoria

Corpo.
`)
	idx, _, _, err := (&Builder{Root: root}).Build()
	if err != nil {
		t.Fatal(err)
	}
	hits := NewSearcher(root, idx).Search("templates de scraping", 3)
	if len(hits) == 0 {
		t.Fatal("expected hits")
	}
	if got := hits[0].Doc.Path; got != "specs/back/sdd-back-templates.md" {
		t.Errorf("the document whose facets carry the question must rank first, got %q", got)
	}
}

func TestBuildSweepsTheIndexOfAnEmptiedFolder(t *testing.T) {
	root := corpusFixture(t)
	b := &Builder{Root: root}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	shard := filepath.Join(root, "specs", "churn", IndexMarkdown)
	if _, err := os.Stat(shard); err != nil {
		t.Fatalf("a folder with documents must have an index: %v", err)
	}

	// Merging the context is NOT what makes the index stale: the folder still
	// holds the document, and whoever opens the folder still needs to see what
	// is in it. The index describes the folder, and the router says which
	// context the folder serves.
	spec := filepath.Join(root, "specs", "churn", "sdd-churn.md")
	raw, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(spec, []byte(strings.Replace(string(raw),
		"contexto: churn", "contexto: account", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shard); err != nil {
		t.Error("the folder still holds a document, so its index must stay")
	}

	// Emptying the folder is what makes it stale. Left behind, the index would
	// keep listing documents that are no longer there.
	if err := os.Remove(spec); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shard); !os.IsNotExist(err) {
		t.Error("the index of a folder with no documents must be removed")
	}
}

func TestSweepLeavesAHandWrittenIndexAlone(t *testing.T) {
	root := corpusFixture(t)
	mine := filepath.Join(root, "specs", "INDEX-notes.md")
	if err := os.WriteFile(mine, []byte("# minhas notas\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "specs", "churn", IndexMarkdown)
	if err := os.WriteFile(stray, []byte("# escrito à mão, sem marca\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Empty the folder so the sweep considers its index stale.
	_ = os.Remove(filepath.Join(root, "specs", "churn", "sdd-churn.md"))
	if _, _, _, err := (&Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Error("a file without the generated marker belongs to somebody else and must survive")
	}
	if _, err := os.Stat(mine); err != nil {
		t.Error("a file that is not INDEX.md must never be touched")
	}
}

func TestFolderNamedAfterASubmoduleIsNotDrift(t *testing.T) {
	root := corpusFixture(t)
	write := func(rel, ctx, sub string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\ntipo: spec\nformato: 2\ncontexto: " + ctx +
			"\nsubmodulo: " + sub + "\nversao: \"1.0\"\nstatus: aprovado\n---\n# t\n\n## X\n\nt.\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A context that grew submodules spreads across folders. That is the layout
	// working, not drifting — reporting it every build teaches people to ignore
	// the report.
	write("specs/shop/sdd-shop.md", "shop", "n/a")
	write("specs/shop_cart/sdd-shop-cart.md", "shop", "cart")
	write("specs/shop_pay/sdd-shop-pay.md", "shop", "pay-checkout")
	// A folder whose name no submodule explains is a typo.
	write("specs/shops/sdd-shops.md", "shopping", "n/a")

	drift, err := WriteIndexes(root, "specs")
	if err != nil {
		t.Fatal(err)
	}
	var reported []string
	for _, d := range drift {
		if strings.Contains(string(d), "serve só o contexto") {
			reported = append(reported, string(d))
		}
	}
	if len(reported) != 1 || !strings.Contains(reported[0], "specs/shops") {
		t.Errorf("only the folder no submodule explains is drift, got %v", reported)
	}
	// Every folder that holds documents gets its own index — that is the point
	// of sharding by folder instead of by context.
	for _, dir := range []string{"shop", "shop_cart", "shop_pay"} {
		if _, err := os.Stat(filepath.Join(root, "specs", dir, IndexMarkdown)); err != nil {
			t.Errorf("folder %s holds documents and must have an index", dir)
		}
	}
}

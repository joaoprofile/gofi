package scan

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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

// A repository that is no gofi project yet: its code and contexts, what its
// documents read as, its acceptance scenarios, API contract and tables — and
// what of gofi it lacks.
func TestAScanReadsWhatARepositoryHolds(t *testing.T) {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/loja\n\ngo 1.22\n")
	write(t, root, "domain/pedidos/service.go", "package pedidos\n")
	write(t, root, "domain/pedidos/service_test.go", "package pedidos\n")
	write(t, root, "domain/estoque/repo.go", "package estoque\n")
	write(t, root, "CLAUDE.md", "# notas do time\n")
	write(t, root, "docs/produto/pedidos.md", "# Pedidos\n\n## Problema\nx\n\n## Personas\nComprador.\n\n## Escopo\nx\n")
	write(t, root, "docs/requisitos.md", "# Requisitos\n\n## Regras de negócio\nRN-01 o pedido deve ter itens.\n\n## Critérios de aceite\nDado que…\n")
	write(t, root, "docs/adr/0001-postgres.md", "# Usar Postgres\n\n## Status\nAceito\n\n## Contexto\nx\n\n## Decisão\nx\n")
	write(t, root, "docs/notas.md", "# Reunião\n\nO objetivo e o escopo da conversa.\n")
	write(t, root, "features/pedido.feature", "Funcionalidade: Criar pedido\n  Cenário: com itens\n  Cenário: vazio\n")
	write(t, root, "api/openapi.yaml", "openapi: 3.0.0\ninfo:\n  title: Loja\npaths:\n  /pedidos:\n    get:\n      x: 1\n    post:\n      x: 1\n")
	write(t, root, "migrations/001.sql", "CREATE TABLE IF NOT EXISTS pedidos (id int);\ncreate table estoque_item (id int);\n")
	write(t, root, "node_modules/x/README.md", "# ignorado\n")

	r, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if r.Gofi.Config || !r.Gofi.ClaudeRoot || r.Gofi.ClaudeDir {
		t.Errorf("gofi %+v", r.Gofi)
	}
	if r.Backend == nil || r.Backend.Language != "go" || r.Backend.Module != "example.com/loja" {
		t.Fatalf("backend %+v", r.Backend)
	}
	if r.Sources != 2 || r.Tests != 1 {
		t.Errorf("sources %d, tests %d", r.Sources, r.Tests)
	}
	var names []string
	for _, c := range r.Contexts {
		names = append(names, c.Name)
		if c.Name == "pedidos" && (c.Dir != "domain/pedidos/" || c.Sources != 1 || c.Tests != 1 || !slices.Contains(c.Tables, "pedidos") || len(c.Documents) == 0) {
			t.Errorf("pedidos %+v", c)
		}
		if c.Name == "estoque" && !slices.Contains(c.Tables, "estoque_item") {
			t.Errorf("estoque %+v", c)
		}
	}
	if !slices.Equal(names, []string{"estoque", "pedidos"}) && !slices.Equal(names, []string{"pedidos", "estoque"}) {
		t.Errorf("contexts %v", names)
	}
	kind := map[string]string{}
	for _, d := range r.Documents {
		kind[d.Path] = d.Kind
	}
	for path, want := range map[string]string{
		"docs/produto/pedidos.md": KindPRD, "docs/requisitos.md": KindSpec,
		"docs/adr/0001-postgres.md": KindDecision, "docs/notas.md": KindOther,
	} {
		if kind[path] != want {
			t.Errorf("%s reads as %q, want %q", path, kind[path], want)
		}
	}
	if _, ok := kind["node_modules/x/README.md"]; ok {
		t.Error("node_modules was read")
	}
	if len(r.Features) != 1 || r.Features[0].Scenarios != 2 || r.Features[0].Title != "Criar pedido" {
		t.Errorf("features %+v", r.Features)
	}
	if len(r.APIs) != 1 || r.APIs[0].Format != "openapi" || r.APIs[0].Operations != 2 || r.APIs[0].Title != "Loja" {
		t.Errorf("apis %+v", r.APIs)
	}
	if len(r.Tables) != 2 {
		t.Errorf("tables %+v", r.Tables)
	}
}

// What gofi already reads as its own is not read again as a foreign
// document, and a gofi project says what it has.
func TestAScanOfAGofiProjectCountsItsContract(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gofi.yaml", "project:\n    name: x\n")
	write(t, root, "AGENTS.md", "# x\n")
	write(t, root, "specs/pedidos/spec.md", "# Pedidos\n\n### Requisito: REQ-01 — x\n")
	write(t, root, "specs/INDEX.md", "# index\n")
	write(t, root, "prd/pedidos/prd-pedidos.md", "# PRD\n\n## Personas\nx\n")
	r, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Gofi.Config || !r.Gofi.Agents || r.Gofi.Specs != 1 || r.Gofi.Contract != 1 || r.Gofi.PRDs != 1 {
		t.Errorf("gofi %+v", r.Gofi)
	}
	if len(r.Documents) != 0 {
		t.Errorf("gofi's own documents read as foreign: %+v", r.Documents)
	}
}

func TestAContextIsNamedAsAWord(t *testing.T) {
	for text, want := range map[string]bool{"docs/pedidos.md": true, "pedido_item": true, "despedidas": false, "Pedidos do cliente": true} {
		if names(text, "pedidos") != want {
			t.Errorf("%q names pedidos: %v", text, !want)
		}
	}
}

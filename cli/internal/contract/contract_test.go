package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
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

const front = `---
tipo: spec
formato: 2
contexto: pedidos
versao: "1.0"
status: proposto
keywords: [pedido]
---
`

// readySpec passes: every required section, rules with text, a route
// parameter and a struct literal in braces that are not placeholders.
const readySpec = front + `# SDD — pedidos

## 1. Visão Geral

Pedidos do cliente.

## 4. Operações

### 4.1 Criar pedido

` + "`POST /pedidos/{id}`" + `

` + "```go\nreturn Pedido{Total: t} // TODO in code is not an open decision\n```" + `

## 5. Regras de Negócio

### RN-01 — Total positivo
> O total do pedido é maior que zero.

### RN-02 — Cancelamento antes do envio
> Pedido enviado não pode ser cancelado.
`

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, layout.Templates().Path("sdd-template.md"), "### RN-01 — {Nome da Regra}\n`Err{Contexto}Validation`\n")
	return root
}

func only(t *testing.T, root string) Doc {
	t.Helper()
	all, err := Docs(root)
	if err != nil || len(all) != 1 {
		t.Fatalf("specs = %+v, err %v", all, err)
	}
	return all[0]
}

func messages(fs []docs.Finding) string {
	var b strings.Builder
	for _, f := range fs {
		kind := "warn"
		if f.Error {
			kind = "error"
		}
		b.WriteString(kind + " " + f.Message + "\n")
	}
	return b.String()
}

func TestAReadySpecHasNoFindings(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", readySpec)
	if got := CheckAll(root, []Doc{only(t, root)}, []Doc{only(t, root)}); len(got) != 0 {
		t.Errorf("findings:\n%s", messages(got))
	}
}

func TestCheckFindsWhatBlocksTheApproval(t *testing.T) {
	root := project(t)
	spec := front + `# SDD — pedidos

## 1. Visão Geral

Para {Contexto}. Prazo TBD.

## 5. Regras de Negócio

### RN-01 — Total positivo
> O total é maior que zero.

### RN-01 — De novo

Texto.

### RN-03 — {Nome da Regra}
`
	write(t, root, "specs/pedidos/sdd-pedidos.md", spec)
	got := messages(Check(root, only(t, root), mustPlaceholders(t, root)))
	for _, want := range []string{
		"error L13: placeholder do template {Contexto} não preenchido",
		`error L13: marcador em aberto "TBD"`,
		"error L20: RN-01 repetida (já declarada na L17)",
		"error L24: placeholder do template {Nome da Regra} não preenchido",
		"error falta a seção 4 (Operações)",
		"error L24: RN-03 sem descrição",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRulesSectionWithoutRulesIsAWarning(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", front+"## 1. Visão\n\nx\n\n## 4. Operações\n\nx\n\n## 5. Regras de Negócio\n\nNenhuma.\n")
	got := Check(root, only(t, root), mustPlaceholders(t, root))
	if len(got) != 1 || got[0].Error || !strings.Contains(got[0].Message, "sem nenhuma regra") {
		t.Errorf("findings:\n%s", messages(got))
	}
}

func TestCorpusFindingsComeWithTheCheck(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", strings.Replace(readySpec, "keywords:", "palavras: [x]\nkeywords:", 1))
	got := messages(CheckAll(root, []Doc{only(t, root)}, []Doc{only(t, root)}))
	if !strings.Contains(got, `error campo desconhecido "palavras"`) {
		t.Errorf("findings:\n%s", got)
	}
}

func TestWithoutTheTemplateThePlaceholdersAreNotChecked(t *testing.T) {
	root := t.TempDir()
	write(t, root, "specs/pedidos/sdd-pedidos.md", readySpec)
	got := CheckAll(root, []Doc{only(t, root)}, []Doc{only(t, root)})
	if len(got) != 1 || got[0].Error || !strings.Contains(got[0].Message, "nenhum template") {
		t.Errorf("findings:\n%s", messages(got))
	}
}

func TestSpecsAndSelect(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", readySpec)
	write(t, root, "specs/pedidos/sdd-pedidos-envio.md", strings.Replace(readySpec, "# SDD — pedidos", "# SDD — pedidos/envio", 1))
	write(t, root, "specs/clientes/sdd-clientes.md", strings.Replace(readySpec, "contexto: pedidos", "contexto: clientes", 1))
	write(t, root, "specs/INDEX.md", "# índice gerado\n")
	write(t, root, "specs/pedidos/notas.md", "---\ntipo: nota\n---\n")

	all, err := Docs(root)
	if err != nil || len(all) != 3 {
		t.Fatalf("specs = %+v, err %v", all, err)
	}
	got, missing := Select(all, []string{"pedidos", "./specs/clientes/sdd-clientes.md", "estoque"})
	if len(got) != 3 || got[0].Path != "specs/clientes/sdd-clientes.md" {
		t.Errorf("selected = %+v", got)
	}
	if len(missing) != 1 || missing[0] != "estoque" {
		t.Errorf("missing = %v", missing)
	}
}

func mustPlaceholders(t *testing.T, root string) map[string]bool {
	t.Helper()
	p, ok := Placeholders(root)
	if !ok {
		t.Fatal("template not found")
	}
	return p
}

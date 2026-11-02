package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sddV2 = `---
tipo: spec
formato: 2
contexto: pedidos
versao: "1.2"
status: aprovado
prd: prd/pedidos/prd-pedidos.md
depende_de: [clientes]
entidades: [pedido_pedido]
keywords: [pedido]
---
# SDD — pedidos

## 0. Manifesto do Serviço

| Atributo | Valor |
|---|---|
| **Go module** | x |

## 1. Visão Geral

Pedidos do cliente, do carrinho ao envio.

## 3. Modelo de Dados

` + "```sql\nCREATE TABLE pedido_pedido (id BIGINT);\n```" + `

## 5. Regras de Negócio

### RN-01 — Total positivo
> O sistema DEVE recusar pedido com total zero.
> **Implementação:** service/pedidos_service.go
> **Erro:** ` + "`ErrPedidosValidation`" + `

### RN-04 — Pedido enviado não pode ser cancelado
> Cancelar um pedido enviado é recusado.

## 7. Erros do Serviço

| Var | Kind |
|---|---|
`

func TestMigrateTurnsAFormatTwoSpecIntoTheLivingSpec(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", sddV2)
	write(t, root, ".claude/memory/contexts/pedidos.md", "---\ncontexto: pedidos\nversao: \"1.0\"\nstatus: aprovado\nspec: specs/pedidos/sdd-pedidos.md\nkeywords: [pedido]\n---\n# pedidos\n")

	res, err := Migrate(root, corpus(t, root), "pedidos", "2026-11-02")
	if err != nil {
		t.Fatal(err)
	}
	if res.Spec != "specs/pedidos/spec.md" || res.Pending != 2 || len(res.Renamed) != 0 ||
		res.Archive != "specs/changes/archive/2026-11-02-baseline-pedidos" {
		t.Fatalf("migrated = %+v", res)
	}
	got := read(t, root, "specs/pedidos/spec.md")
	for _, w := range []string{
		"tipo: spec\nformato: 3\ncontexto: pedidos\n", `versao: "1.2"`, "depende_de: [clientes]", "entidades: [pedido_pedido]", "status: proposto",
		"## Propósito\n\nPedidos do cliente, do carrinho ao envio.",
		"### Requisito: REQ-01 — Total positivo\nO sistema DEVE recusar pedido com total zero.\n",
		"### Requisito: REQ-04 — Pedido enviado não pode ser cancelado",
		"#### Cenário: REQ-04.1 — Pedido enviado não pode ser cancelado",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("living spec lacks %q:\n%s", w, got)
		}
	}
	// The how stays with the old document, not in a requirement.
	for _, gone := range []string{"Implementação", "ErrPedidosValidation", "CREATE TABLE", "Go module"} {
		if strings.Contains(got, gone) {
			t.Errorf("living spec carries the how (%q):\n%s", gone, got)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "specs/pedidos/sdd-pedidos.md")); !os.IsNotExist(err) {
		t.Error("the format-2 spec is still in specs/pedidos/")
	}
	if a := read(t, root, res.Archive+"/sdd-pedidos.md"); !strings.Contains(a, "status: arquivado") || !strings.Contains(a, "CREATE TABLE") {
		t.Errorf("archived:\n%s", a)
	}
	if m := read(t, root, ".claude/memory/contexts/pedidos.md"); !strings.Contains(m, "spec: specs/pedidos/spec.md") {
		t.Errorf("memory:\n%s", m)
	}

	// Not approvable until the scenarios are written — and only for that.
	all := corpus(t, root)
	living, _ := Living(all, "pedidos")
	findings := Check(root, all, living, mustPlaceholders(t, root))
	for _, f := range findings {
		if f.Error && !strings.Contains(f.Message, "dúvida em aberto") {
			t.Errorf("unexpected error: %s", f.Message)
		}
	}
	if Errors(findings) != 2 {
		t.Errorf("findings:\n%s", messages(findings))
	}
	written := strings.NewReplacer(
		"- DADO [PRECISA ESCLARECER: cenário de REQ-01 — a regra antiga não tinha critério verificável]\n- QUANDO …\n- ENTÃO …",
		"- DADO um carrinho com total zero\n- QUANDO o cliente fecha\n- ENTÃO recebe recusa",
		"- DADO [PRECISA ESCLARECER: cenário de REQ-04 — a regra antiga não tinha critério verificável]\n- QUANDO …\n- ENTÃO …",
		"- DADO um pedido enviado\n- QUANDO o cliente cancela\n- ENTÃO recebe recusa",
	).Replace(read(t, root, "specs/pedidos/spec.md"))
	write(t, root, "specs/pedidos/spec.md", written)
	all = corpus(t, root)
	living, _ = Living(all, "pedidos")
	if got := Check(root, all, living, mustPlaceholders(t, root)); Errors(got) != 0 {
		t.Errorf("with the scenarios written:\n%s", messages(got))
	}

	if _, err := Migrate(root, corpus(t, root), "pedidos", "2026-11-02"); err == nil {
		t.Error("a context was migrated twice")
	}
}

// Several format-2 specs in one context become one living spec: ids are
// unique per context, so the rules are renumbered in order, and the map says
// which became which.
func TestMigrateRenumbersAcrossSubmodules(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/sdd-pedidos.md", sddV2)
	write(t, root, "specs/pedidos/sdd-pedidos-envio.md", strings.Replace(sddV2, "Pedidos do cliente, do carrinho ao envio.", "O envio dos pedidos.", 1))
	res, err := Migrate(root, corpus(t, root), "pedidos", "2026-11-02")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"specs/pedidos/sdd-pedidos-envio.md#RN-01": "REQ-01", "specs/pedidos/sdd-pedidos-envio.md#RN-04": "REQ-02",
		"specs/pedidos/sdd-pedidos.md#RN-01": "REQ-03", "specs/pedidos/sdd-pedidos.md#RN-04": "REQ-04",
	}
	for k, v := range want {
		if res.Renamed[k] != v {
			t.Errorf("%s → %s, want %s (all: %v)", k, res.Renamed[k], v, res.Renamed)
		}
	}
	got := read(t, root, "specs/pedidos/spec.md")
	if strings.Count(got, "### Requisito:") != 4 || !strings.Contains(got, "O envio dos pedidos.") {
		t.Errorf("living spec:\n%s", got)
	}
}

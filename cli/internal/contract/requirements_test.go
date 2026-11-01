package contract

import (
	"reflect"
	"strings"
	"testing"
)

const livingSpec = `---
tipo: spec
formato: 3
contexto: pedidos
versao: "1.0"
status: proposto
keywords: [pedido]
---
# Spec — pedidos

## Propósito

Pedidos do cliente.

## Requisitos

### Requisito: REQ-01 — Total positivo
O sistema DEVE recusar pedido com total zero.

#### Cenário: REQ-01.1 — Total zero
- DADO um carrinho com total zero
- QUANDO o cliente fecha o pedido
- ENTÃO recebe recusa
- E nenhum pedido é criado

` + "```" + `
### Requisito: REQ-99 — dentro de código, não conta
` + "```" + `

### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
O sistema NÃO DEVE cancelar um pedido QUANDO ele já foi enviado.

#### Cenário: REQ-04.1 – Cancelar enviado
- Dado um pedido enviado
- Quando o cliente cancela
- Entao recebe recusa
Nota solta.

### Requisito: sem id
Texto.
`

const proposal = `---
tipo: mudanca
formato: 3
contextos: [pedidos, envio]
versao: "1.0"
status: proposto
keywords: [rate-limit]
---
# Mudança — adicionar-rate-limit

## Por quê
`

const delta = `---
tipo: delta
formato: 3
contexto: pedidos
versao: "1.0"
status: proposto
keywords: [rate-limit]
---
# Delta — pedidos

## ADICIONADO

### Requisito: REQ-07 — Limite por cliente
O sistema NÃO DEVE aceitar mais de 10 pedidos por minuto de um cliente.

#### Cenário: REQ-07.1 — Décimo primeiro pedido
- DADO dez pedidos no último minuto
- QUANDO o cliente faz o décimo primeiro
- ENTÃO recebe recusa

## Modificados

### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
Texto completo.

## Notas

### Requisito: REQ-50 — fora de uma seção de operação

## REMOVIDO

### Requisito: REQ-02 — Desconto manual
**Por quê:** saiu do produto.
`

// changeLayout writes a living spec, an open change touching two contexts,
// its tasks (derived, not contract) and an archived change (history).
func changeLayout(t *testing.T) string {
	t.Helper()
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", livingSpec)
	write(t, root, "specs/changes/adicionar-rate-limit/proposal.md", proposal)
	write(t, root, "specs/changes/adicionar-rate-limit/deltas/pedidos.md", delta)
	write(t, root, "specs/changes/adicionar-rate-limit/design.md", "---\ntipo: design\nformato: 3\ncontexto: pedidos\n---\n# Design\n")
	write(t, root, "specs/changes/adicionar-rate-limit/tasks.md", "---\ntipo: tarefas\nformato: 3\ncontexto: pedidos\n---\n- [ ] 1.1\n")
	write(t, root, "specs/changes/archive/2026-10-01-criar-pedidos/proposal.md", proposal)
	return root
}

func TestDocsReadsLivingSpecsAndOpenChanges(t *testing.T) {
	all := corpus(t, changeLayout(t))
	var got []string
	for _, d := range all {
		got = append(got, d.Kind+" "+d.Path+" change="+d.Change+" ctx="+d.Context+" "+strings.Join(d.Contexts, ","))
	}
	want := []string{
		"mudanca specs/changes/adicionar-rate-limit/proposal.md change=adicionar-rate-limit ctx=pedidos pedidos,envio",
		"delta specs/changes/adicionar-rate-limit/deltas/pedidos.md change=adicionar-rate-limit ctx=pedidos pedidos",
		"design specs/changes/adicionar-rate-limit/design.md change=adicionar-rate-limit ctx=pedidos pedidos",
		"spec specs/pedidos/spec.md change= ctx=pedidos pedidos",
	}
	// Sorted by path: deltas/ comes before design.md.
	want[0], want[1], want[2] = want[1], want[2], want[0]
	if !reflect.DeepEqual(got, want) {
		t.Errorf("docs:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if all[0].Format != 3 {
		t.Errorf("format = %d", all[0].Format)
	}
}

func TestSelectTakesAContextWithItsChanges(t *testing.T) {
	all := corpus(t, changeLayout(t))
	byContext, _ := Select(all, []string{"envio"})
	if len(byContext) != 1 || byContext[0].Kind != KindChange {
		t.Errorf("envio = %+v: the proposal names it, nothing else does", byContext)
	}
	byChange, _ := Select(all, []string{"adicionar-rate-limit"})
	if len(byChange) != 3 {
		t.Errorf("change = %d docs, want proposal, delta and design", len(byChange))
	}
	if living, ok := Living(all, "pedidos"); !ok || living.Path != "specs/pedidos/spec.md" {
		t.Errorf("living = %+v, %v", living, ok)
	}
}

func TestRequirementsAreReadByID(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", livingSpec)
	reqs := Requirements(only(t, root))
	if len(reqs) != 3 {
		t.Fatalf("requirements = %+v", reqs)
	}
	r1, r4, bad := reqs[0], reqs[1], reqs[2]
	if r1.ID != "REQ-01" || r1.Title != "Total positivo" || r1.Line != 17 || len(r1.Text) != 1 {
		t.Errorf("REQ-01 = %+v", r1)
	}
	if len(r1.Scenarios) != 1 || r1.Scenarios[0].ID != "REQ-01.1" {
		t.Fatalf("REQ-01 scenarios = %+v", r1.Scenarios)
	}
	var kw []string
	for _, s := range r1.Scenarios[0].Steps {
		kw = append(kw, s.Keyword)
	}
	if strings.Join(kw, " ") != "DADO QUANDO ENTAO E" {
		t.Errorf("steps = %v", kw)
	}
	// Lower case and an unaccented ENTAO are the same keywords; an en dash
	// separates the title as an em dash does.
	s4 := r4.Scenarios[0]
	if r4.ID != "REQ-04" || s4.ID != "REQ-04.1" || s4.Title != "Cancelar enviado" || len(s4.Steps) != 3 || len(s4.Other) != 1 {
		t.Errorf("REQ-04 = %+v", r4)
	}
	if bad.ID != "" || bad.Title != "sem id" {
		t.Errorf("a heading without an id = %+v", bad)
	}
	if Parent("REQ-04.1") != "REQ-04" || Parent("REQ-04") != "" {
		t.Error("Parent")
	}
}

func TestDeltaSections(t *testing.T) {
	root := project(t)
	write(t, root, "specs/changes/x/deltas/pedidos.md", delta)
	sections := DeltaSections(only(t, root))
	var got []string
	for _, s := range sections {
		ids := []string{}
		for _, r := range s.Requirements {
			ids = append(ids, r.ID)
		}
		got = append(got, s.Op+":"+strings.Join(ids, ","))
	}
	// A plural reads as the singular; a section that is not an operation is
	// not one, and what it holds belongs to none.
	want := []string{"ADICIONADO:REQ-07", "MODIFICADO:REQ-04", "REMOVIDO:REQ-02"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %v, want %v", got, want)
	}
	if len(sections[0].Requirements[0].Scenarios) != 1 {
		t.Errorf("added = %+v", sections[0].Requirements[0])
	}
}

// A format-3 document is not checked by the format-2 rules: it has no
// numbered sections and no RN-xx, and that is not an error. What it has is
// held to its own.
func TestFormatThreeIsNotHeldToTheFormatTwoRules(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", livingSpec)
	got := messages(Check(root, corpus(t, root), only(t, root), mustPlaceholders(t, root)))
	want := "error L39: requisito sem ID: escreva `### Requisito: REQ-NN — título` (\"sem id\")\n" +
		"warn L26: bloco de código na spec: o como vai para o design da mudança\n"
	if got != want {
		t.Errorf("findings:\n%s\nwant:\n%s", got, want)
	}
}

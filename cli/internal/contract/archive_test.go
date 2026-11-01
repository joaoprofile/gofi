package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const livingBefore = `---
tipo: spec
formato: 3
contexto: pedidos
versao: "1.3"
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

### Requisito: REQ-02 — Desconto manual
O sistema DEVE aceitar desconto manual.

#### Cenário: REQ-02.1 — Desconto
- DADO um pedido
- QUANDO o atendente aplica desconto
- ENTÃO o total cai

### Requisito: REQ-04 — Cancelamento
O sistema DEVE cancelar pedido.

#### Cenário: REQ-04.1 — Cancelar
- DADO um pedido
- QUANDO o cliente cancela
- ENTÃO o pedido é cancelado
`

const deltaPedidos = `---
tipo: delta
formato: 3
contexto: pedidos
versao: "1.0"
status: proposto
depende_de: [clientes]
keywords: [pedido]
---
# Delta — pedidos

## ADICIONADO

### Requisito: REQ-07 — Limite por cliente
O sistema NÃO DEVE aceitar mais de 10 pedidos por minuto de um cliente.

#### Cenário: REQ-07.1 — Décimo primeiro
- DADO dez pedidos no último minuto
- QUANDO o cliente faz o décimo primeiro
- ENTÃO recebe recusa

## MODIFICADO

### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
O sistema NÃO DEVE cancelar um pedido QUANDO ele já foi enviado.

#### Cenário: REQ-04.1 — Cancelar enviado
- DADO um pedido enviado
- QUANDO o cliente cancela
- ENTÃO recebe recusa

## REMOVIDO

### Requisito: REQ-02 — Desconto manual
**Por quê:** saiu do produto.
`

const livingAfterBody = `# Spec — pedidos

## Propósito

Pedidos do cliente.

## Requisitos

### Requisito: REQ-01 — Total positivo
O sistema DEVE recusar pedido com total zero.

#### Cenário: REQ-01.1 — Total zero
- DADO um carrinho com total zero
- QUANDO o cliente fecha o pedido
- ENTÃO recebe recusa

### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
O sistema NÃO DEVE cancelar um pedido QUANDO ele já foi enviado.

#### Cenário: REQ-04.1 — Cancelar enviado
- DADO um pedido enviado
- QUANDO o cliente cancela
- ENTÃO recebe recusa

### Requisito: REQ-07 — Limite por cliente
O sistema NÃO DEVE aceitar mais de 10 pedidos por minuto de um cliente.

#### Cenário: REQ-07.1 — Décimo primeiro
- DADO dez pedidos no último minuto
- QUANDO o cliente faz o décimo primeiro
- ENTÃO recebe recusa
`

const designRateLimit = `---
tipo: design
formato: 3
contexto: pedidos
versao: "1.0"
status: proposto
keywords: [pedido]
---
# Design

## Abordagem

Contador por cliente.

## Mapa de requisitos

| REQ-04 | Service.Cancel |
| REQ-07 | Limiter |
| estoque/REQ-01 | Estoque.Baixa |
`

const proposalRateLimit = `---
tipo: mudanca
formato: 3
contextos: [pedidos, estoque]
versao: "1.0"
status: proposto
keywords: [pedido]
---
# Mudança — adicionar-rate-limit

## Por quê

Abuso.

## O que muda

REQ-07, REQ-04, REQ-02.
`

const deltaEstoque = `---
tipo: delta
formato: 3
contexto: estoque
versao: "1.0"
status: proposto
keywords: [estoque]
---
# Delta — estoque

## Propósito

Saldo de produtos.

## ADICIONADO

### Requisito: REQ-01 — Saldo nunca negativo
O sistema NÃO DEVE deixar o saldo de um produto abaixo de zero.

#### Cenário: REQ-01.1 — Baixa maior que o saldo
- DADO um produto com saldo 1
- QUANDO chega uma baixa de 2
- ENTÃO a baixa é recusada
`

// archiveFixture is a living spec of pedidos and an approved change that
// adds, modifies and removes requirements in it and creates estoque.
func archiveFixture(t *testing.T, approve bool) string {
	t.Helper()
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", livingBefore)
	dir := "specs/changes/adicionar-rate-limit/"
	write(t, root, dir+"proposal.md", proposalRateLimit)
	write(t, root, dir+"deltas/pedidos.md", deltaPedidos)
	write(t, root, dir+"deltas/estoque.md", deltaEstoque)
	write(t, root, dir+"design.md", designRateLimit)
	write(t, root, dir+"tasks.md", "---\ntipo: tarefas\nformato: 3\ncontexto: pedidos\n---\n- [x] 1.1\n")
	write(t, root, "specs/clientes/spec.md", strings.ReplaceAll(livingBefore, "pedidos", "clientes"))
	if approve {
		for _, d := range corpus(t, root) {
			if d.Change != "" {
				if _, err := Approve(root, d); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	return root
}

func TestArchiveAppliesTheDeltas(t *testing.T) {
	root := archiveFixture(t, true)
	res, err := Archive(root, corpus(t, root), "adicionar-rate-limit", "2026-11-01")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Specs, " ") != "specs/estoque/spec.md specs/pedidos/spec.md" ||
		res.Dir != "specs/changes/archive/2026-11-01-adicionar-rate-limit" {
		t.Fatalf("archived = %+v", res)
	}

	got := read(t, root, "specs/pedidos/spec.md")
	front, body := splitFront(got)
	if strings.Join(body, "\n") != livingAfterBody {
		t.Errorf("body:\n%s\nwant:\n%s", strings.Join(body, "\n"), livingAfterBody)
	}
	for _, w := range []string{`versao: "1.4"`, "status: aprovado", "atualizado: 2026-11-01", "depende_de: [clientes]", `aprovacao_versao: "1.4"`, `aprovacao_sha: "sha256:`} {
		if !strings.Contains(strings.Join(front, "\n"), w) {
			t.Errorf("frontmatter lacks %q:\n%s", w, strings.Join(front, "\n"))
		}
	}

	all := corpus(t, root)
	pedidos, _ := Living(all, "pedidos")
	if state, reason := SealOf(pedidos); state != Sealed {
		t.Errorf("pedidos seal = %v (%s)", state, reason)
	}
	estoque, ok := Living(all, "estoque")
	if !ok || value(estoque.Front, "versao") != "1.0" || len(Requirements(estoque)) != 1 {
		t.Fatalf("estoque = %+v", estoque)
	}
	if !strings.Contains(strings.Join(estoque.Body, "\n"), "## Propósito\n\nSaldo de produtos.") {
		t.Errorf("estoque body:\n%s", strings.Join(estoque.Body, "\n"))
	}
	for _, d := range []Doc{pedidos, estoque} {
		if got := Check(root, all, d, mustPlaceholders(t, root)); Errors(got) > 0 {
			t.Errorf("%s does not pass the check:\n%s", d.Path, messages(got))
		}
	}

	if _, err := os.Stat(filepath.Join(root, "specs/changes/adicionar-rate-limit")); !os.IsNotExist(err) {
		t.Error("the change folder is still open")
	}
	archived := read(t, root, res.Dir+"/proposal.md")
	if !strings.Contains(archived, "status: arquivado") {
		t.Errorf("archived proposal:\n%s", archived)
	}
	if _, err := os.Stat(filepath.Join(root, res.Dir, "tasks.md")); err != nil {
		t.Error("the tasks did not go with the change")
	}
	if Changes(all) != nil && len(Changes(all)) != 0 {
		t.Errorf("open changes after archiving = %+v", Changes(all))
	}
}

// Archiving twice is the same as once: the same input gives the same bytes.
func TestArchiveIsDeterministic(t *testing.T) {
	a, b := archiveFixture(t, true), archiveFixture(t, true)
	for _, root := range []string{a, b} {
		if _, err := Archive(root, corpus(t, root), "adicionar-rate-limit", "2026-11-01"); err != nil {
			t.Fatal(err)
		}
	}
	if read(t, a, "specs/pedidos/spec.md") != read(t, b, "specs/pedidos/spec.md") {
		t.Error("two archives of the same change differ")
	}
}

func TestArchiveRefusesWhatWasNotApproved(t *testing.T) {
	root := archiveFixture(t, false)
	before := read(t, root, "specs/pedidos/spec.md")
	_, err := Archive(root, corpus(t, root), "adicionar-rate-limit", "2026-11-01")
	if err == nil || !strings.Contains(err.Error(), "não está aprovado") {
		t.Fatalf("err = %v", err)
	}
	if read(t, root, "specs/pedidos/spec.md") != before {
		t.Error("a refused archive wrote the spec")
	}
	if _, err := Archive(root, corpus(t, root), "sumida", "2026-11-01"); err == nil {
		t.Error("an unknown change was archived")
	}

	// Approved, then edited: the seal is broken and the archive refused.
	root = archiveFixture(t, true)
	d := read(t, root, "specs/changes/adicionar-rate-limit/design.md")
	write(t, root, "specs/changes/adicionar-rate-limit/design.md", strings.Replace(d, "Contador por cliente.", "Outro.", 1))
	if _, err := Archive(root, corpus(t, root), "adicionar-rate-limit", "2026-11-01"); err == nil {
		t.Error("an edited change was archived")
	}
}

func TestBump(t *testing.T) {
	for in, want := range map[string]string{"1.3": "1.4", "2": "2.1", "1.9": "1.10", "": "1.0", "x.y": "1.0"} {
		if got := bump(in); got != want {
			t.Errorf("bump(%q) = %q, want %q", in, got, want)
		}
	}
}

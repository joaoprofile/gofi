package retrieval

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

const reqSpec = `---
tipo: spec
formato: 3
contexto: pedidos
versao: "1.0"
status: aprovado
keywords: [pedido]
---
# Spec — pedidos

## Propósito

Pedidos.

## Requisitos

### Requisito: REQ-04 — Pedido enviado não pode ser cancelado
O sistema NÃO DEVE cancelar um pedido enviado.

#### Cenário: REQ-04.1 — Cancelar enviado
- DADO um pedido enviado
- QUANDO o cliente cancela
- ENTÃO recebe recusa

#### Cenário: REQ-04.2 — Cancelar em separação
- DADO um pedido em separação
- QUANDO o cliente cancela
- ENTÃO o pedido é cancelado

### Requisito: REQ-040 — Outro requisito
O sistema DEVE outra coisa.
`

// An id names one section exactly: the requirement with its scenarios, or
// one scenario — what goes in an agent's turn instead of the whole spec.
func TestShowReadsARequirementByID(t *testing.T) {
	root := t.TempDir()
	write(t, root, "specs/pedidos/spec.md", reqSpec)
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	e, err := Open(root, "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := e.Show("specs/pedidos/spec.md#REQ-04", 50)
	if err != nil {
		t.Fatal(err)
	}
	d := v.Data.(SectionData)
	if !strings.HasPrefix(d.Heading, "Requisito: REQ-04 —") || !strings.Contains(d.Text, "REQ-04.2") || strings.Contains(d.Text, "REQ-040") {
		t.Errorf("REQ-04 = %+v\n%s", d, d.Text)
	}
	v, _ = e.Show("specs/pedidos/spec.md#req-04.1", 50)
	if d := v.Data.(SectionData); !strings.HasPrefix(d.Heading, "Cenário: REQ-04.1") || strings.Contains(d.Text, "REQ-04.2") {
		t.Errorf("REQ-04.1 = %+v\n%s", d, d.Text)
	}
	v, _ = e.Show("specs/pedidos/spec.md#REQ-040", 50)
	if d := v.Data.(SectionData); !strings.HasPrefix(d.Heading, "Requisito: REQ-040") {
		t.Errorf("REQ-040 = %+v", d)
	}
}

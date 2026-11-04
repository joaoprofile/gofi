package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

const sddPedidos = "---\ntipo: spec\nformato: 2\ncontexto: pedidos\nversao: \"1.45\"\nstatus: aprovado\nkeywords: [pedido]\n---\n# SDD — pedidos\n\n## 1. Visão\n\nPedidos.\n\n## 2. Regras\n\n- **RN-01** — o total não pode ser zero.\n"

// The pricing context had only format-2 specs (sdd-*.md) and a format-3
// change was created and run against it as if the context were new:
// archived, it would have started a spec.md at REQ-01 beside the old ones.
// A change of a context still in format 2 is refused where it is born and
// where it is checked.
func TestAChangeOfAContextStillInFormatTwoIsRefused(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs/pedidos/sdd-pedidos.md"), sddPedidos)

	if out, err := execCmd(newChangeCmd, "new", "ajustar-total", "--context", "pedidos"); err == nil || !strings.Contains(err.Error(), "gofi spec migrate pedidos") {
		t.Fatalf("a change was born on a format-2 context: %v\n%s", err, out)
	}
	if exists(filepath.Join(root, "specs/pedidos/changes/ajustar-total")) {
		t.Error("the refused change left a folder")
	}

	dir := filepath.Join(root, "specs/pedidos/changes/a-mao")
	writeFile(t, filepath.Join(dir, "proposal.md"), proposalFor("pedidos"))
	writeFile(t, filepath.Join(dir, "design.md"), minimalDesign("pedidos"))
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")
	out, err := execCmd(newSpecCmd, "check", "a-mao")
	if err == nil || !strings.Contains(out, "gofi spec migrate pedidos") {
		t.Fatalf("a change written by hand on a format-2 context passed the check: %v\n%s", err, out)
	}
	if _, err := execCmd(newChangeCmd, "approve", "a-mao"); err == nil || sealed(t, root, "a-mao") {
		t.Error("a change on a format-2 context was approved")
	}
}

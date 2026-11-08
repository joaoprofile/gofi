package intake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
)

func writeDoc(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const sealProposal = "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nx\n"

const sealDesign = "---\ntipo: design\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nentidades: []\nkeywords: [pedido]\n---\n# Design\n\n## Abordagem\n\nUma função.\n\n## Mapa de requisitos\n\n| Requisito | Componente |\n|---|---|\n| REQ-01 | `pedidos.Total` |\n"

const sealDelta = "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n"

// Past the review stop on a spec, the conductor seals the context's open
// change; one that does not pass seals nothing and says why.
func TestSealContract(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "specs/pedidos/changes/criar-pedidos/proposal.md", sealProposal)
	writeDoc(t, root, "specs/pedidos/changes/criar-pedidos/deltas/pedidos.md", sealDelta)
	writeDoc(t, root, "specs/pedidos/changes/criar-pedidos/design.md", sealDesign)
	r := &Result{Context: &Context{Name: "pedidos"}, Plan: []Phase{{Role: "gofi-spec", Produces: "spec"}, {Role: "gofi-eng", Produces: "code"}}}

	if sealed, err := SealContract(root, r, 1); err != nil || sealed != nil {
		t.Errorf("after the code phase: sealed=%v err=%v", sealed, err)
	}
	sealed, err := SealContract(root, r, 0)
	if err != nil || len(sealed) != 3 {
		t.Fatalf("sealed=%v err=%v", sealed, err)
	}
	all, _ := contract.Docs(root)
	for _, d := range all {
		if state, _ := contract.SealOf(d); state != contract.Sealed {
			t.Errorf("%s not sealed", d.Path)
		}
	}
	if again, err := SealContract(root, r, 0); err != nil || len(again) != 0 {
		t.Errorf("sealing twice: %v %v", again, err)
	}

	writeDoc(t, root, "specs/pedidos/changes/outra/proposal.md", strings.Replace(sealProposal, "## O que muda\n\nx\n", "", 1))
	writeDoc(t, root, "specs/pedidos/changes/outra/deltas/pedidos.md", strings.ReplaceAll(sealDelta, "REQ-01", "REQ-02"))
	_, err = SealContract(root, r, 0)
	if err == nil || !strings.Contains(err.Error(), "falta a seção ## O que muda") {
		t.Fatalf("err = %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(root, "specs/pedidos/changes/outra/deltas/pedidos.md"))
	if strings.Contains(string(b), "aprovacao_sha") {
		t.Error("a refused contract was partly sealed")
	}
}

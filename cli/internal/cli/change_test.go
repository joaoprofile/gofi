package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runChange(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newChangeCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// The life of a change up to its approval: created from the templates,
// refused while it is a skeleton, sealed once written, broken by an edit.
func TestChangeNewCheckApprove(t *testing.T) {
	root := configuredProject(t)
	tmpl := filepath.Join(root, ".claude", "templates")
	writeFile(t, filepath.Join(tmpl, "proposal-template.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [{contextos}]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Mudança — {mudanca}\n\n## Por quê\n\n## O que muda\n")
	writeFile(t, filepath.Join(tmpl, "delta-template.md"), "---\ntipo: delta\nformato: 3\ncontexto: {contexto}\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Delta — {contexto}\n\n## ADICIONADO\n\n### Requisito: REQ-01 — {Título}\n")
	writeFile(t, filepath.Join(tmpl, "design-template.md"), "---\ntipo: design\nformato: 3\ncontexto: {contexto}\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Design\n\n## Abordagem\n\n## Mapa de requisitos\n")

	out, err := runChange(t, root, "new", "criar-pedidos", "--context", "pedidos")
	if err != nil || !strings.Contains(out, "specs/changes/criar-pedidos/proposal.md") {
		t.Fatalf("new: err=%v\n%s", err, out)
	}

	// The skeleton is not approvable: placeholders, a requirement with no scenario.
	out, err = runChange(t, root, "approve", "criar-pedidos")
	var exit *ExitCodeError
	if !errors.As(err, &exit) || exit.Code != 1 || !strings.Contains(out, "placeholder do template {Título}") {
		t.Fatalf("approve of a skeleton: err=%v\n%s", err, out)
	}

	dir := filepath.Join(root, "specs", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# Mudança — criar-pedidos\n\n## Por quê\n\nNão há pedidos.\n\n## O que muda\n\nREQ-01 novo.\n")
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# Delta — pedidos\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total positivo\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Total zero\n- DADO um carrinho vazio\n- QUANDO o cliente fecha\n- ENTÃO recebe recusa\n")
	writeFile(t, filepath.Join(dir, "design.md"), "---\ntipo: design\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# Design\n\n## Abordagem\n\nUm serviço.\n\n## Mapa de requisitos\n\n| REQ-01 | Service.Create |\n")

	if out, err := runChange(t, root, "approve", "criar-pedidos"); err != nil || strings.Count(out, "sha256:") != 3 {
		t.Fatalf("approve: err=%v\n%s", err, out)
	}
	if out, _ := runChange(t, root, "list", "--json"); !strings.Contains(out, `"selada": true`) || !strings.Contains(out, `"status": "aprovado"`) {
		t.Errorf("list after approval:\n%s", out)
	}
	sealed, _ := os.ReadFile(filepath.Join(dir, "design.md"))
	writeFile(t, filepath.Join(dir, "design.md"), strings.Replace(string(sealed), "Um serviço.", "Dois serviços.", 1))
	if out, _ := runChange(t, root, "list"); !strings.Contains(out, "approval broken in specs/changes/criar-pedidos/design.md") {
		t.Errorf("list after an edit:\n%s", out)
	}
	if _, err := runChange(t, root, "approve", "sumida"); err == nil {
		t.Error("approving an unknown change passed")
	}
}

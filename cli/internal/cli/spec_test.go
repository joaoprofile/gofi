package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const specFront = "---\ntipo: spec\nformato: 2\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n"

func runSpec(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newSpecCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// A spec missing what the loop works from fails the check with exit code 1,
// naming what to fix; a ready one passes.
func TestSpecCheck(t *testing.T) {
	root := configuredProject(t)
	spec := filepath.Join(root, "specs", "pedidos", "sdd-pedidos.md")
	writeFile(t, spec, specFront+"# pedidos\n\n## 1. Visão Geral\n\nx\n\n## 5. Regras de Negócio\n\n### RN-01 — Total\n> Maior que zero.\n")

	out, err := runSpec(t, root, "check", "pedidos")
	var exit *ExitCodeError
	if !errors.As(err, &exit) || exit.Code != 1 || !strings.Contains(out, "falta a seção 4 (Operações)") {
		t.Fatalf("err=%v output:\n%s", err, out)
	}

	writeFile(t, spec, specFront+"# pedidos\n\n## 1. Visão Geral\n\nx\n\n## 4. Operações\n\nx\n\n## 5. Regras de Negócio\n\n### RN-01 — Total\n> Maior que zero.\n")
	if out, err := runSpec(t, root, "check"); err != nil || !strings.Contains(out, "1 specs: 0") {
		t.Errorf("err=%v output:\n%s", err, out)
	}
	if _, err := runSpec(t, root, "check", "estoque"); err == nil {
		t.Error("an unknown context passed")
	}
}

// Approval is all or nothing: one spec with an error and none is sealed.
func TestSpecApprove(t *testing.T) {
	root := configuredProject(t)
	ready := specFront + "# pedidos\n\n## 1. Visão Geral\n\nx\n\n## 4. Operações\n\nx\n\n## 5. Regras de Negócio\n\n### RN-01 — Total\n> Maior que zero.\n"
	good := filepath.Join(root, "specs", "pedidos", "sdd-pedidos.md")
	bad := filepath.Join(root, "specs", "clientes", "sdd-clientes.md")
	writeFile(t, good, ready)
	writeFile(t, bad, strings.Replace(strings.Replace(ready, "contexto: pedidos", "contexto: clientes", 1), "## 4. Operações", "## 6. Outra", 1))

	if _, err := runSpec(t, root, "approve"); err == nil {
		t.Error("approve with nothing named passed")
	}
	out, err := runSpec(t, root, "approve", "--all")
	var exit *ExitCodeError
	if !errors.As(err, &exit) || exit.Code != 1 || !strings.Contains(out, "sdd-clientes.md") {
		t.Fatalf("err=%v output:\n%s", err, out)
	}
	if b, _ := os.ReadFile(good); strings.Contains(string(b), "aprovacao_sha") {
		t.Error("a refused approval sealed the good spec")
	}

	if out, err := runSpec(t, root, "approve", "pedidos"); err != nil || !strings.Contains(out, "sha256:") {
		t.Fatalf("err=%v output:\n%s", err, out)
	}
	if out, _ := runSpec(t, root, "check", "pedidos"); !strings.Contains(out, "1 specs: 0") {
		t.Errorf("an approved spec does not pass the check:\n%s", out)
	}
	b, _ := os.ReadFile(good)
	writeFile(t, good, strings.Replace(string(b), "Maior que zero.", "Maior que um.", 1))
	if out, err := runSpec(t, root, "check", "pedidos"); err == nil || !strings.Contains(out, "aprovação vencida") {
		t.Errorf("an edited approved spec passed: err=%v\n%s", err, out)
	}
}

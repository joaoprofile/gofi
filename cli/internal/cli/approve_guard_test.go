package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
)

func withApprovePorts(t *testing.T, agent, tty bool) {
	t.Helper()
	prev := approvePorts
	approvePorts.agent = func() bool { return agent }
	approvePorts.tty = func() bool { return tty }
	t.Cleanup(func() { approvePorts = prev })
}

func sealed(t *testing.T, root, change string) bool {
	t.Helper()
	all, err := contract.Docs(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range contract.Changes(all) {
		if c.ID == change {
			return c.Sealed
		}
	}
	t.Fatalf("no change %s", change)
	return false
}

// openChangeProject is a project with the change criar-pedidos written and
// not approved.
func openChangeProject(t *testing.T) string {
	t.Helper()
	root := configuredProject(t)
	t.Chdir(root)
	dir := filepath.Join(root, "specs", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "design.md"), minimalDesign("pedidos"))
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")
	return root
}

// An agent ran gofi change approve from its shell and sealed a change the
// person never approved: the seal then proved nothing. The command an agent
// runs is refused, and nothing is sealed.
func TestAnAgentCannotApproveFromTheShell(t *testing.T) {
	root := openChangeProject(t)
	withApprovePorts(t, true, false)
	out, err := execCmd(newChangeCmd, "approve", "criar-pedidos")
	if err == nil || !strings.Contains(err.Error(), "gofi") {
		t.Fatalf("an agent approved:\n%s", out)
	}
	if sealed(t, root, "criar-pedidos") {
		t.Error("the change was sealed")
	}
	if _, err := execCmd(newSpecCmd, "approve", "pedidos"); err == nil {
		t.Error("an agent approved through gofi spec approve")
	}
}

// At a terminal the person sees what is sealed and confirms it; anything but
// yes seals nothing.
func TestAtATerminalThePersonConfirmsTheSeal(t *testing.T) {
	root := openChangeProject(t)
	withApprovePorts(t, false, true)
	run := func(answer string) (string, error) {
		c := newChangeCmd()
		var b strings.Builder
		c.SetOut(&b)
		c.SetErr(&b)
		c.SetIn(strings.NewReader(answer))
		c.SetArgs([]string{"approve", "criar-pedidos"})
		err := c.Execute()
		return b.String(), err
	}
	if out, err := run("\n"); err == nil || sealed(t, root, "criar-pedidos") {
		t.Fatalf("approved without a yes:\n%s", out)
	}
	out, err := run("s\n")
	if err != nil || !sealed(t, root, "criar-pedidos") {
		t.Fatalf("a yes did not approve: %v\n%s", err, out)
	}
	if !strings.Contains(out, "proposal.md") {
		t.Errorf("the person must see what is sealed:\n%s", out)
	}
}

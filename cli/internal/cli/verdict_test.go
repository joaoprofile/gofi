package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerdictWrite(t *testing.T) {
	root := configuredProject(t)
	mem := filepath.Join(root, ".claude", "memory", "contexts", "pedidos.md")
	writeFile(t, mem, "---\ncontexto: pedidos\nversao: \"1.0\"\nstatus: implementado\nkeywords: [pedido]\n---\n# pedidos\n")
	t.Chdir(root)
	run := func(stdin string, args ...string) (string, error) {
		var out bytes.Buffer
		cmd := newVerdictCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}

	out, err := run(`{"result":"aprovado","findings":[{"severity":"major","rule":"pedidos/REQ-04.1","owner":"eng","message":"m"}]}`, "write", "pedidos", "--run", "r-1", "--iter", "2")
	if err != nil || !strings.Contains(out, ".gofi/verdict/pedidos.json") {
		t.Fatalf("err=%v\n%s", err, out)
	}
	b, _ := os.ReadFile(filepath.Join(root, ".gofi", "verdict", "pedidos.json"))
	if !strings.Contains(string(b), `"run": "r-1"`) || !strings.Contains(string(b), `"iter": 2`) {
		t.Errorf("verdict:\n%s", b)
	}
	// "aprovado" is the person's seal on a contract: a passing audit leaves
	// auditado in the memory, so the two are never read as one.
	m, _ := os.ReadFile(mem)
	if !strings.Contains(string(m), "status: auditado") || !strings.Contains(string(m), "ressalvas: 1") || strings.Contains(string(m), "status: aprovado") {
		t.Errorf("memory not carried:\n%s", m)
	}

	_, err = run(`{"result":"aprovado","findings":[{"severity":"blocker","rule":"r","owner":"eng","message":"m"}]}`, "write", "pedidos")
	var exit *ExitCodeError
	if !errors.As(err, &exit) || exit.Code != 1 {
		t.Errorf("an inconsistent verdict was written: %v", err)
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/sensor"
)

const pontosDelta = `---
tipo: delta
formato: 3
contexto: pontos
versao: "1.0"
status: proposto
keywords: [pontos]
---
# Delta — pontos — expor-pontos

## Propósito

Expõe os pontos.

## ADICIONADO

### Requisito: REQ-03 — Consulta
O sistema DEVE responder a consulta.

#### Cenário: REQ-03.1 — Consulta válida
- DADO uma consulta
- QUANDO a rota é chamada
- ENTÃO responde 200

#### Cenário: REQ-03.2 — Valor inválido
- DADO um valor que não é número
- QUANDO a rota é chamada
- ENTÃO responde 400
`

func runTraceIn(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	t.Chdir(root)
	var out bytes.Buffer
	cmd := newTraceCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// A change is traced from its deltas to the code's directives: a scenario
// with no test fails the trace, at the scenario, in the SARIF the loop's
// trace stage reads.
func TestTraceFindsTheScenarioWithNoTest(t *testing.T) {
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, "specs/pontos/changes/expor-pontos/deltas/pontos.md"), pontosDelta)
	writeFile(t, filepath.Join(root, "src/domain/pontos/handler/h.go"), "package handler\n\n// gofi:implements REQ-03\nfunc Get() {}\n")
	writeFile(t, filepath.Join(root, "src/domain/pontos/handler/h_test.go"), "package handler\n\n// gofi:tests REQ-03.1\nfunc TestGet() {}\n")
	out, err := runTraceIn(t, root, "expor-pontos", "--report", ".gofi/runs/r-1/trace.sarif")
	if err == nil || !strings.Contains(out, "REQ-03.2") || !strings.Contains(out, "1 de 3 sem resposta") {
		t.Fatalf("err %v, out:\n%s", err, out)
	}
	report, err := os.ReadFile(filepath.Join(root, ".gofi/runs/r-1/trace.sarif"))
	if err != nil {
		t.Fatal(err)
	}
	got := sensor.Read("trace", pack.FormatSARIF, report, nil, "")
	if len(got) != 1 || got[0].Rule != "gofi:tests pontos/REQ-03.2" || got[0].File != "specs/pontos/changes/expor-pontos/deltas/pontos.md" || got[0].Line != 25 {
		t.Errorf("findings %+v", got)
	}
	// With its test, the change is traced.
	writeFile(t, filepath.Join(root, "src/domain/pontos/handler/h_test.go"), "package handler\n\n// gofi:tests REQ-03.1\nfunc TestGet() {}\n\n// gofi:tests REQ-03.2\nfunc TestBad() {}\n")
	if out, err := runTraceIn(t, root, "expor-pontos"); err != nil || !strings.Contains(out, "0 de 3") {
		t.Errorf("err %v, out:\n%s", err, out)
	}
	if _, err := runTraceIn(t, root, "nada"); err == nil {
		t.Error("an unknown change traced")
	}
}

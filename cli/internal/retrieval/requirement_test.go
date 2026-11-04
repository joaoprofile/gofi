package retrieval

import (
	"strings"
	"testing"
)

// A requirement is shown traced: where the living spec says it, the symbol
// that implements it and, scenario by scenario, the test that tests it — a
// scenario with none said so.
func TestARequirementIsShownTraced(t *testing.T) {
	root := fixture(t, true)
	write(t, root, "specs/order/spec.md", "---\ntipo: spec\nformato: 3\ncontexto: order\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [pedido]\n---\n# Spec — order\n\n## Requisitos\n\n### Requisito: REQ-01 — Faturar\nO sistema DEVE faturar.\n\n#### Cenário: REQ-01.1 — Faturado\n- DADO um pedido\n- QUANDO fatura\n- ENTÃO fica faturado\n\n#### Cenário: REQ-01.2 — Cancelado\n- DADO um pedido cancelado\n- QUANDO fatura\n- ENTÃO recusa\n")
	write(t, root, "domain/order/service/order_service.go", "package service\n\n// gofi:implements REQ-01\nfunc BillOrder() error { return nil }\n")
	write(t, root, "domain/order/service/order_service_test.go", "package service\n\n// gofi:tests REQ-01.1\nfunc TestBill(t *testing.T) {}\n")
	view, err := open(t, root).Show("req:order/REQ-01", 10)
	if err != nil {
		t.Fatal(err)
	}
	d := view.Data.(RequirementData)
	if d.Line != 13 || len(d.Implements) != 1 || d.Implements[0].Path != "domain/order/service/order_service.go" || d.Implements[0].Line != 3 {
		t.Fatalf("requirement %+v", d)
	}
	if len(d.Scenarios) != 2 || len(d.Scenarios[0].Tests) != 1 || len(d.Scenarios[1].Tests) != 0 {
		t.Errorf("scenarios %+v", d.Scenarios)
	}
	if !strings.Contains(view.Text, "REQ-01.2 — Cancelado\n    testa: — nenhum teste") {
		t.Errorf("text:\n%s", view.Text)
	}
	if _, err := open(t, root).Show("req:order/REQ-09", 10); err == nil {
		t.Error("an unknown requirement was shown")
	}
}

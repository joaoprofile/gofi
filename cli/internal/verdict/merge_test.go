package verdict

import (
	"encoding/json"
	"testing"
)

func lens(t *testing.T, name, body string) *Verdict {
	t.Helper()
	v, err := Parse([]byte(body), "pedidos")
	if err != nil {
		t.Fatal(err)
	}
	v.Lens = name
	return v
}

// The lenses of one audit merge into one verdict, the same bytes whatever
// order they come in: one finding per fingerprint at its highest severity,
// the contract's owner when they disagree, rejected when any lens rejects.
func TestTheLensesMergeDeterministically(t *testing.T) {
	sec := `{"result":"reprovado","findings":[
	  {"severity":"blocker","owner":"eng","rule":"pedidos/REQ-01","symbol":"pedidos.Cancel","message":"sem autorização"},
	  {"severity":"minor","owner":"eng","rule":"naming","file":"a.go","message":"nome"}]}`
	conf := `{"result":"aprovado","findings":[
	  {"severity":"major","owner":"spec","rule":"pedidos/REQ-01","symbol":"pedidos.Cancel","message":"o requisito não diz quem"}]}`
	clean := `{"result":"aprovado"}`
	a, conflicts := Merge("pedidos", "r-1", 4, []*Verdict{lens(t, "gofi-qa-conformance", conf), lens(t, "gofi-qa-security", sec), lens(t, "gofi-qa", clean)})
	b, _ := Merge("pedidos", "r-1", 4, []*Verdict{lens(t, "gofi-qa", clean), lens(t, "gofi-qa-security", sec), lens(t, "gofi-qa-conformance", conf)})
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Errorf("the order of the lenses changed the merge:\n%s\n%s", ja, jb)
	}
	if a.Result != Rejected || len(a.Findings) != 2 || a.Run != "r-1" || a.Iter != 4 {
		t.Fatalf("merged = %s", ja)
	}
	f := a.Findings[0]
	if f.ID != "F-1" || f.Severity != "blocker" || f.Owner != "spec" || f.Category != "gofi-qa-security" {
		t.Errorf("the shared finding = %+v", f)
	}
	if len(conflicts) != 1 || conflicts[0] != f.Fingerprint {
		t.Errorf("conflicts = %v", conflicts)
	}
	if len(a.Lenses) != 3 {
		t.Errorf("lenses = %v", a.Lenses)
	}

	c, _ := Merge("pedidos", "r-1", 4, []*Verdict{lens(t, "gofi-qa", clean), lens(t, "gofi-qa-tests", clean)})
	if !c.Clean() {
		t.Errorf("two clean lenses = %+v", c)
	}
}

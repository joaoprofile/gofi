package verdict

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseFillsAndOrders(t *testing.T) {
	v, err := Parse([]byte(`{"result": "reprovado", "findings": [
		{"severity": "MINOR", "rule": "pack/naming", "file": "a.go", "line": 3, "owner": "eng", "message": "m"},
		{"severity": "blocker", "rule": "pedidos/REQ-04.1", "symbol": "pedidos/service.Cancel", "line": 88, "owner": "eng", "message": "b"},
		{"severity": "suggestion", "rule": "x", "owner": "spec", "message": "s"}
	]}`), "pedidos")
	if err != nil {
		t.Fatal(err)
	}
	if v.Schema != Schema || v.Context != "pedidos" {
		t.Errorf("verdict = %+v", v)
	}
	var order []string
	for _, f := range v.Findings {
		order = append(order, f.Severity+":"+f.ID)
		if len(f.Fingerprint) != 12 {
			t.Errorf("fingerprint %q", f.Fingerprint)
		}
	}
	if strings.Join(order, " ") != "blocker:F-2 minor:F-1 suggestion:F-3" {
		t.Errorf("order = %v", order)
	}
	if v.Caveats() != 1 || v.Clean() {
		t.Errorf("caveats = %d, clean = %v", v.Caveats(), v.Clean())
	}
}

// The same defect is the same finding when the code above it moves: the
// fingerprint is the rule, the symbol and the category — never the line.
func TestFingerprintSurvivesAMovedLine(t *testing.T) {
	a := Finding{Rule: "pedidos/REQ-04.1", Symbol: "pedidos/service.Cancel", Line: 88, Category: "conformidade"}
	b := a
	b.Line, b.Message = 120, "reworded"
	if Fingerprint(a) != Fingerprint(b) {
		t.Error("a moved line changed the fingerprint")
	}
	c := a
	c.Rule = "pedidos/REQ-04.2"
	if Fingerprint(a) == Fingerprint(c) {
		t.Error("another rule kept the fingerprint")
	}
}

func TestParseRefusesWhatDoesNotHoldTogether(t *testing.T) {
	for name, tc := range map[string]struct{ json, want string }{
		"blocker approved":     {`{"result":"aprovado","findings":[{"severity":"blocker","rule":"r","owner":"eng","message":"m"}]}`, "a blocker rejects"},
		"rejected for nothing": {`{"result":"reprovado","findings":[{"severity":"suggestion","rule":"r","owner":"eng","message":"m"}]}`, "nothing rejects it"},
		"bad severity":         {`{"result":"aprovado","findings":[{"severity":"grave","rule":"r","owner":"eng","message":"m"}]}`, "severity \"grave\""},
		"bad owner":            {`{"result":"aprovado","findings":[{"severity":"minor","rule":"r","owner":"qa","message":"m"}]}`, "owner \"qa\""},
		"no rule":              {`{"result":"aprovado","findings":[{"severity":"minor","owner":"eng","message":"m"}]}`, "rule and message"},
		"bad result":           {`{"result":"ok"}`, "result \"ok\""},
		"unknown field":        {`{"result":"aprovado","nota":1}`, "unknown field"},
		"other context":        {`{"contexto":"envio","result":"aprovado"}`, "about \"envio\""},
	} {
		if _, err := Parse([]byte(tc.json), "pedidos"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", name, err, tc.want)
		}
	}
	if v, err := Parse([]byte(`{"result":"aprovado"}`), "pedidos"); err != nil || !v.Clean() {
		t.Errorf("a clean approval: %+v, %v", v, err)
	}
}

func TestWriteAndRead(t *testing.T) {
	root := t.TempDir()
	if v, _ := Read(root, "pedidos"); v != nil {
		t.Error("a verdict out of nowhere")
	}
	v, _ := Parse([]byte(`{"result":"aprovado","findings":[{"severity":"minor","rule":"r","owner":"eng","message":"m"}]}`), "pedidos")
	if _, err := Write(root, v); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root, "pedidos")
	if err != nil || got.Result != Approved || len(got.Findings) != 1 || got.Findings[0].Fingerprint == "" {
		t.Errorf("read = %+v, %v", got, err)
	}
}

// A finding placed by line is keyed by the symbol that holds the line: the
// same defect, with code added above it, is the same finding.
func TestLocateKeysAFindingByItsSymbol(t *testing.T) {
	at := func(file string, line int) string {
		if file == "backend/x.go" && line >= 10 && line <= 30 {
			return "x.Bill"
		}
		return ""
	}
	parse := func(line int) Finding {
		v, err := Parse([]byte(fmt.Sprintf(`{"result":"reprovado","findings":[{"severity":"major","owner":"eng","rule":"x/REQ-01.1","file":"backend/x.go","line":%d,"message":"m"}]}`, line)), "x")
		if err != nil {
			t.Fatal(err)
		}
		v.Locate(at)
		return v.Findings[0]
	}
	a, b := parse(12), parse(25)
	if a.Symbol != "x.Bill" || a.Fingerprint != b.Fingerprint {
		t.Errorf("a %+v, b %+v", a, b)
	}
	if c := parse(50); c.Symbol != "" || c.Fingerprint == a.Fingerprint {
		t.Errorf("a line no symbol holds: %+v", c)
	}
}

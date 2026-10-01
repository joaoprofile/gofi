package retrieval

import (
	"strings"
	"testing"
)

// textFixture adds the source file the graph fixture describes, so a line can
// be traced back to the symbol that holds it.
func textFixture(t *testing.T) string {
	t.Helper()
	root := fixture(t, true)
	var b strings.Builder
	for i := 1; i <= 25; i++ {
		switch i {
		case 12:
			b.WriteString("func BillOrder(o *Order) error {\n")
		case 14:
			b.WriteString("\treturn ErrOrderCancelled // conflito\n")
		default:
			b.WriteString("// filler\n")
		}
	}
	write(t, root, "domain/order/service/order_service.go", b.String())
	return root
}

func TestTextNamesTheSymbolHoldingTheLine(t *testing.T) {
	res, err := open(t, textFixture(t)).Text(TextQuery{Pattern: "ErrOrderCancelled"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.Matches) != 1 {
		t.Fatalf("got %+v", res)
	}
	m := res.Matches[0]
	if m.Path != "domain/order/service/order_service.go" || m.Line != 14 || m.Area != AreaCode {
		t.Errorf("match = %+v", m)
	}
	if m.In == nil || m.In.Kind != KindSymbol || !strings.Contains(m.In.Title, "BillOrder") {
		t.Errorf("enclosing = %+v, want BillOrder", m.In)
	}
}

func TestTextNamesTheSectionHoldingTheLine(t *testing.T) {
	res, err := open(t, textFixture(t)).Text(TextQuery{Pattern: "recusado com erro de conflito"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Matches) != 1 {
		t.Fatalf("got %+v", res)
	}
	if in := res.Matches[0].In; in == nil || !strings.HasPrefix(in.Title, "RN-01") {
		t.Errorf("enclosing = %+v, want RN-01", in)
	}
}

func TestTextRegexAreasAndLimit(t *testing.T) {
	e := open(t, textFixture(t))
	res, err := e.Text(TextQuery{Pattern: `conflit(o)?`, Regex: true, IgnoreCase: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || res.Files != 2 {
		t.Fatalf("regex: total %d in %d files, want 2 in 2", res.Total, res.Files)
	}
	if res, _ = e.Text(TextQuery{Pattern: "conflito", Areas: []string{AreaCode}}); res.Total != 1 || res.Matches[0].Area != AreaCode {
		t.Errorf("--in code: %+v", res)
	}
	if res, _ = e.Text(TextQuery{Pattern: "filler", Limit: 3}); len(res.Matches) != 3 || res.Total != 23 {
		t.Errorf("limit: %d shown of %d, want 3 of 23", len(res.Matches), res.Total)
	}
}

// The index is derived from the files; searching it would answer every query
// twice, once from the source and once from its copy.
func TestTextSkipsTheIndex(t *testing.T) {
	res, err := open(t, textFixture(t)).Text(TextQuery{Pattern: "charges an order"})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range res.Matches {
		if strings.HasPrefix(m.Path, ".gofi/") {
			t.Errorf("searched the index: %s", m.Path)
		}
	}
}

func TestTextRejectsAnEmptyPattern(t *testing.T) {
	if _, err := open(t, fixture(t, false)).Text(TextQuery{Pattern: "  "}); err == nil {
		t.Error("an empty pattern matched every line")
	}
}

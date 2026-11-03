package trace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheMatrixLinesUpRequirementsCodeAndTests(t *testing.T) {
	root := t.TempDir()
	write(t, root, "backend/domain/pontos/service/s.go", "package service\n\n// gofi:implements REQ-01\nfunc A() {}\n\n// gofi:implements REQ-02.1, faturas/REQ-09\nfunc B() {}\n// gofi:tests REQ-02.1\n")
	write(t, root, "backend/domain/pontos/service/s_test.go", "package service\n\n// gofi:tests REQ-01.1\nfunc TestA() {}\n")
	write(t, root, "backend/README.md", "gofi:implements REQ-03\n")
	lp, _ := pack.Load("go")
	marks, err := Scan(root, "backend", lp)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range marks {
		got = append(got, m.Kind+" "+m.Context+"/"+m.ID)
	}
	// A gofi:tests in production code does not count; a file of no source
	// language is not read.
	if strings.Join(got, ",") != "implements pontos/REQ-01,implements pontos/REQ-02.1,implements faturas/REQ-09,tests pontos/REQ-01.1" {
		t.Fatalf("marks = %v", got)
	}
	reqs := []docs.Requirement{
		{ID: "REQ-01", Title: "a", Line: 10, Scenarios: []docs.Scenario{{ID: "REQ-01.1", Line: 12}, {ID: "REQ-01.2", Line: 16}}},
		{ID: "REQ-02", Title: "b", Line: 20, Scenarios: []docs.Scenario{{ID: "REQ-02.1", Line: 22}}},
		{ID: "REQ-03", Title: "c", Line: 30},
	}
	rows := Matrix([]Source{{Context: "pontos", Doc: "specs/changes/x/deltas/pontos.md", Requirements: reqs}}, marks)
	var gaps []string
	for _, r := range Gaps(rows) {
		gaps = append(gaps, r.Need()+" "+r.ID)
	}
	// REQ-02 is implemented through its scenario's mark.
	if strings.Join(gaps, ",") != "tests REQ-01.2,tests REQ-02.1,implements REQ-03" {
		t.Errorf("gaps = %v", gaps)
	}
}

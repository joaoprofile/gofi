package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// Every suite gofi ships is a contract gofi spec check passes: a suite with
// an error of its own would measure the error, not the loop.
func TestTheSuitesPassTheContractCheck(t *testing.T) {
	entries, err := suites.ReadDir("suites")
	if err != nil || len(entries) == 0 {
		t.Fatalf("no suite: %v", err)
	}
	for _, e := range entries {
		s, ok, err := Load(e.Name())
		if err != nil || !ok || len(s.Changes) == 0 {
			t.Fatalf("suite %s: %v", e.Name(), err)
		}
		root := t.TempDir()
		if err := s.Write(root); err != nil {
			t.Fatal(err)
		}
		all, err := contract.Docs(root)
		if err != nil {
			t.Fatal(err)
		}
		placeholders, _ := contract.Placeholders(root)
		for _, d := range all {
			for _, f := range contract.Blocking(root, all, d, placeholders) {
				t.Errorf("suite %s: %s: %s", e.Name(), f.Path, f.Message)
			}
		}
		for _, c := range s.Changes {
			found := false
			for _, ch := range contract.Changes(all) {
				found = found || ch.ID == c.ID
			}
			if !found {
				t.Errorf("suite %s: %s is not written", e.Name(), c.ID)
			}
		}
	}
}

// The result is read from the runs' folders: each change's last run, how it
// ended, its rounds, caveats and cost by role.
func TestCollectReadsTheRuns(t *testing.T) {
	dir := t.TempDir()
	s := &Suite{Language: "go", Changes: []Change{{ID: "a", Kind: "facil"}, {ID: "b", Kind: "contrato"}, {ID: "c", Kind: "armadilha"}}}
	save := func(st *loop.State, agents ...loop.AgentState) {
		_ = os.MkdirAll(loop.Dir(dir, st.Run), 0o755)
		if err := loop.SaveState(dir, st); err != nil {
			t.Fatal(err)
		}
		for _, a := range agents {
			p := filepath.Join(loop.Dir(dir, st.Run), loop.AgentsDir, a.ID+".json")
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			data, _ := json.Marshal(a)
			_ = os.WriteFile(p, data, 0o644)
		}
	}
	save(&loop.State{Schema: loop.StateSchema, Run: "r-1", Change: "a", Outcome: "impasse", CostUSD: 1})
	save(&loop.State{Schema: loop.StateSchema, Run: "r-2", Change: "a", Outcome: "accepted", CostUSD: 2},
		loop.AgentState{ID: "implement-1", Node: "implement", Role: "gofi-eng", CostUSD: 1.5}, loop.AgentState{ID: "audit-2", Node: "audit", Role: "gofi-qa", CostUSD: 0.5})
	save(&loop.State{Schema: loop.StateSchema, Run: "r-3", Change: "b", Outcome: "accepted", CostUSD: 1, Notes: []loop.Finding{{Rule: "b/REQ-02"}}})
	r, err := Collect(dir, "b-1", s)
	if err != nil {
		t.Fatal(err)
	}
	if r.Accepted != 2 || r.FirstPass != 2 || r.Runs[0].Run != "r-2" || r.Runs[1].Notes != 1 || r.Runs[2].Outcome != "não rodou" ||
		r.ByRole["gofi-eng"] != 1.5 || r.CostUSD != 3 || strings.Join(r.Runs[0].Roles, ",") != "gofi-eng" {
		t.Errorf("result %+v", r)
	}
	if rep := r.Report(); !strings.Contains(rep, "2 de 3 aceitas") || !strings.Contains(rep, "| gofi-eng | 1.50 |") {
		t.Errorf("report:\n%s", rep)
	}
}

// An audit case is a contract the check passes and code that builds; a
// verdict finds a planted defect only where it is and saying what it is.
func TestAnAuditCaseIsJudgedByWhereAndWhat(t *testing.T) {
	cases, err := AuditCases("go")
	if err != nil || len(cases) == 0 {
		t.Fatalf("cases %v, %v", cases, err)
	}
	for _, c := range cases {
		root := t.TempDir()
		// The case's code imports its own packages by the module: a backend
		// with a go.mod of its own, as gofi init writes it.
		if err := os.MkdirAll(filepath.Join(root, "backend"), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module example.com/bench\n\ngo 1.22\n"), 0o644)
		if err := c.Write(root, "backend"); err != nil {
			t.Fatal(err)
		}
		build := exec.Command("go", "build", "./...")
		build.Dir = filepath.Join(root, "backend")
		build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
		if out, err := build.CombinedOutput(); err != nil {
			t.Errorf("case %s: its code does not build: %v\n%s", c.Name, err, out)
		}
		all, _ := contract.Docs(root)
		placeholders, _ := contract.Placeholders(root)
		for _, d := range all {
			for _, f := range contract.Blocking(root, all, d, placeholders) {
				t.Errorf("case %s: %s: %s", c.Name, f.Path, f.Message)
			}
		}
		if len(c.Defects) == 0 {
			t.Errorf("case %s plants no defect", c.Name)
		}
		for _, d := range c.Defects {
			if d.File == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, strings.ReplaceAll(d.File, "{src}", "backend"))); err != nil {
				t.Errorf("case %s: defect %s is in a file the case does not write: %v", c.Name, d.ID, err)
			}
		}
	}
	var c *AuditCase
	for _, x := range cases {
		if x.Name == "reembolso" {
			c = x
		}
	}
	file := "backend/domain/benchreembolso/service/benchreembolso_service.go"
	v := &verdict.Verdict{Findings: []verdict.Finding{
		{Severity: "blocker", Rule: "benchreembolso/REQ-01.1", File: file, Line: 22, Message: "a taxa é 3%, o requisito diz 2%"},
		{Severity: "major", Rule: "benchreembolso/REQ-03", File: file, Line: 40, Message: "o prazo usa 31"},
		{Severity: "major", Rule: "benchreembolso/REQ-02.1", Message: "o cenário REQ-02.1 não tem teste"},
	}}
	s := c.Judge(v, "backend")
	// The second is in D2's file, off its line, but names REQ-03 and says it.
	if strings.Join(s.Found, ",") != "D1,D2,D5" || strings.Join(s.Missed, ",") != "D3,D4" || s.Extra != 0 {
		t.Errorf("score %+v", s)
	}
}

// A finding kept on another file finds a defect by the place it names, and
// one with no file by the requirement it breaks — never by a scenario of it,
// whose missing test is a defect of its own.
func TestAFindingFindsADefectByThePlaceItNamesOrItsRequirement(t *testing.T) {
	cases, _ := AuditCases("go")
	byName := map[string]*AuditCase{}
	for _, c := range cases {
		byName[c.Name] = c
	}
	judge := func(name string, fs ...verdict.Finding) Score {
		c := *byName[name]
		c.Defects = append([]Defect{}, c.Defects...)
		root := t.TempDir()
		_ = os.MkdirAll(filepath.Join(root, "backend"), 0o755)
		_ = os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module example.com/bench\n\ngo 1.22\n"), 0o644)
		if err := c.Write(root, "backend"); err != nil {
			t.Fatal(err)
		}
		return c.Judge(&verdict.Verdict{Findings: fs}, "backend")
	}
	data, _ := os.ReadFile(filepath.Join("suites", "go", "audit", "pedido", "code", "domain", "benchpedido", "handler", "benchpedido_handler.go.tmpl"))
	line := 0
	for n, l := range strings.Split(string(data), "\n") {
		if strings.Contains(l, "http.StatusInternalServerError") {
			line = n + 1
		}
	}
	s := judge("pedido",
		verdict.Finding{Severity: "major", Rule: "layers.md §Erros", File: "backend/domain/benchpedido/service/benchpedido_service.go", Line: 12,
			Message: fmt.Sprintf("qualquer err vira 500 fixo (benchpedido_handler.go:%d) em vez do 404", line)},
		verdict.Finding{Severity: "major", Rule: "benchpedido/REQ-03.2", Message: "Cenário REQ-03.2 (pedido inexistente responde 404) sem teste"},
	)
	if strings.Join(s.Found, ",") != "D3" || s.Extra != 1 {
		t.Errorf("pedido %+v", s)
	}
	s = judge("transferencia", verdict.Finding{Severity: "blocker", Rule: "benchconta/REQ-01",
		Message: "debita e credita em duas chamadas soltas, sem transação nem bloqueio — duas transferências concorrentes leem o mesmo saldo"})
	if strings.Join(s.Found, ",") != "D1,D4" || s.Extra != 0 {
		t.Errorf("transferencia %+v", s)
	}
}

func TestARequirementIsNotOneOfItsScenarios(t *testing.T) {
	for rule, want := range map[string]bool{"ctx/REQ-01": true, "REQ-01 e REQ-01.2": true, "ctx/REQ-01.2": false, "REQ-012": false, "req-01": true} {
		if namesRequirement(rule, "REQ-01") != want {
			t.Errorf("%q names REQ-01: %v", rule, !want)
		}
	}
}

// A finding placed on the defect's function finds it when it names one of
// the defect's lines; there, saying only what it is about is not enough.
func TestAFindingOnTheDefectsFunctionFindsItByTheLineItNames(t *testing.T) {
	cases, _ := AuditCases("go")
	for _, c := range cases {
		if c.Name != "pedido" {
			continue
		}
		cc := *c
		cc.Defects = append([]Defect{}, c.Defects...)
		root := t.TempDir()
		if err := cc.Write(root, "backend"); err != nil {
			t.Fatal(err)
		}
		file := "backend/domain/benchpedido/handler/benchpedido_handler.go"
		var d2 Defect
		for _, d := range cc.Defects {
			if d.ID == "D2" {
				d2 = d
			}
		}
		decl := d2.Lines[0] - 3
		judge := func(msg string) []string {
			return cc.Judge(&verdict.Verdict{Findings: []verdict.Finding{{Severity: "major", Rule: "qa-checklist.md §10", File: file, Line: decl, Message: msg}}}, "backend").Found
		}
		if got := judge(fmt.Sprintf("o handler usa o repositório direto (h.repo.Buscar(id), linha %d)", d2.Lines[0])); !slices.Contains(got, "D2") {
			t.Errorf("naming line %d: found %v", d2.Lines[0], got)
		}
		if got := judge("o handler usa o repositório direto"); slices.Contains(got, "D2") {
			t.Errorf("naming no line: found %v", got)
		}
	}
}

// In the defect's own file, a finding naming a scenario of the defect's
// requirement finds it; with no file, the scenario is a missing test.
func TestInItsFileAScenarioIsOfTheRequirement(t *testing.T) {
	d := Defect{ID: "D3", File: "{src}/s.go", Lines: [2]int{39, 39}, Rule: "REQ-01", Keywords: []string{"_ ="}}
	in := verdict.Finding{File: "b/s.go", Line: 30, Rule: "ctx/REQ-01.2", Message: "descarta o erro com `_ =`"}
	if !d.finds(in, "b") {
		t.Error("in its file, REQ-01.2 is of REQ-01")
	}
	in.File = ""
	if d.finds(in, "b") {
		t.Error("with no file, REQ-01.2 is a scenario's missing test")
	}
	if namesRequirementOrScenario("ctx/REQ-012", "REQ-01") {
		t.Error("REQ-012 is another requirement")
	}
}

// A keyword is said at the start of a word, and a number as a whole one.
func TestAKeywordIsSaidAsAWord(t *testing.T) {
	for _, c := range []struct {
		text, k string
		want    bool
	}{
		{"o handler alcança o repositório", "repositor", true},
		{"um prerepositorio", "repositor", false},
		{"responde 500 em vez de 404", "500", true},
		{"o total 5000", "500", false},
		{"descarta com `_ =`", "_ =", true},
	} {
		if saysWord(fold(c.text), fold(c.k)) != c.want {
			t.Errorf("%q says %q: %v", c.text, c.k, !c.want)
		}
	}
}

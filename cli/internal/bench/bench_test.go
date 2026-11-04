package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
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
		if err := c.Write(root, "backend"); err != nil {
			t.Fatal(err)
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
	c := cases[0]
	file := "backend/domain/benchreembolso/service/benchreembolso_service.go"
	v := &verdict.Verdict{Findings: []verdict.Finding{
		{Severity: "blocker", Rule: "benchreembolso/REQ-01.1", File: file, Line: 22, Message: "a taxa é 3%, o requisito diz 2%"},
		{Severity: "major", Rule: "benchreembolso/REQ-03", File: file, Line: 40, Message: "o prazo usa 31"},
		{Severity: "major", Rule: "benchreembolso/REQ-02.1", Message: "o cenário REQ-02.1 não tem teste"},
	}}
	s := c.Judge(v, "backend")
	if strings.Join(s.Found, ",") != "D1,D5" || strings.Join(s.Missed, ",") != "D2,D3,D4" || s.Extra != 1 {
		t.Errorf("score %+v", s)
	}
}

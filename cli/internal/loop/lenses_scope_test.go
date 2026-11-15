package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// With the budget short of what the lenses need to reach a verdict, none is
// started: the run stops at the audit, resumable, having spent nothing on it.
func TestAnAuditTheBudgetCannotFinishStartsNoLens(t *testing.T) {
	w := lensWorld(t)
	w.runner.Policy.BudgetUSD = 0.5 // two lenses need 0.70
	_, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos")
	if err == nil || !strings.Contains(err.Error(), "orçamento insuficiente") {
		t.Fatalf("err = %v", err)
	}
	for _, turn := range w.agent.turns {
		if turn.Node == "audit" {
			t.Errorf("a lens ran: %+v", turn)
		}
	}
}

func lens(name string, scope ...string) role.Role {
	all := []string{"sensor.green"}
	for _, l := range scope {
		all = append(all, "diff.touches("+l+")")
	}
	return role.Role{Name: name, Contract: role.Contract{Triggers: []role.Trigger{{All: all}}}}
}

func names(roles []role.Role) string {
	var out []string
	for _, r := range roles {
		out = append(out, r.Name)
	}
	return strings.Join(out, ",")
}

// One lens of five found something: the other four run again only when what
// the rework changed is in their scope, and a lens of the whole change only
// when anything changed.
func TestOnlyTheLensesTheReworkConcernsRunAgain(t *testing.T) {
	w := newWorld(t)
	r := w.runner
	if err := os.MkdirAll(Dir(w.root, "r-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"data.go", "api.go"} {
		if err := os.WriteFile(filepath.Join(w.root, f), []byte("v1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &State{Run: "r-1", Diff: &GraphDiff{Touched: []Touch{{File: "data.go", Layer: "data"}, {File: "api.go", Layer: "api"}}}}
	s.Audited = &AuditMark{Clean: []string{"data", "api", "integ", "whole"}, Files: map[string]string{
		"data.go": fileHash(filepath.Join(w.root, "data.go")), "api.go": fileHash(filepath.Join(w.root, "api.go"))}}
	roles := []role.Role{lens("found", "data"), lens("data", "data"), lens("api", "api"), lens("integ", "integration"), lens("whole")}

	if run, kept := r.affected("audit", roles, s); names(run) != "found" || strings.Join(kept, ",") != "data,api,integ,whole" {
		t.Errorf("nothing changed: run %s, kept %v", names(run), kept)
	}
	if err := os.WriteFile(filepath.Join(w.root, "api.go"), []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if run, kept := r.affected("audit", roles, s); names(run) != "found,api,whole" || strings.Join(kept, ",") != "data,integ" {
		t.Errorf("the api changed: run %s, kept %v", names(run), kept)
	}
	if evs, _ := Events(w.root, "r-1"); !strings.Contains(evs[len(evs)-1].Why, "escopo") {
		t.Errorf("a lens spared does not say why: %+v", evs[len(evs)-1])
	}
}

// The first audit, or one with no graph to judge by, runs every lens.
func TestWithoutAnAuditBeforeEveryLensRuns(t *testing.T) {
	w := newWorld(t)
	roles := []role.Role{lens("a", "data"), lens("b")}
	if run, _ := w.runner.affected("audit", roles, &State{Run: "r-1", Diff: &GraphDiff{}}); names(run) != "a,b" {
		t.Errorf("run %s", names(run))
	}
	if run, _ := w.runner.affected("audit", roles, &State{Run: "r-1", Audited: &AuditMark{Clean: []string{"a"}}}); names(run) != "a,b" {
		t.Errorf("no graph: run %s", names(run))
	}
}

// What each audit found stays in the run, though the next audit rewrites the
// context's verdict, and says how it ended.
func TestTheFindingsOfEachAuditStayInTheRun(t *testing.T) {
	w := lensWorld(t)
	w.agent.steps["audit/gofi-qa"] = []step{lensAudit(w.root, "aprovado"), lensAudit(w.root, "aprovado")}
	w.agent.steps["audit/gofi-qa-security"] = []step{lensAudit(w.root, "reprovado", "blocker:eng:pedidos/REQ-01:backend/pedidos/handler.go"), lensAudit(w.root, "aprovado")}
	s := w.run(t)
	found := ReadFindings(w.root, "r-1")
	if len(found) != 1 || found[0].Rule != "pedidos/REQ-01" {
		t.Fatalf("recorded %+v", found)
	}
	if got := Situation(s, found[0]); got != "resolvido" {
		t.Errorf("situation %q", got)
	}
}

// The lenses wait for the first of them to be answered, and no longer: what
// they open with is then read from the cache, not written by each.
func TestTheLensesStartOnceTheFirstIsAnswered(t *testing.T) {
	w := lensWorld(t)
	w.runner.LensWarm = time.Minute
	var mu sync.Mutex
	var order []string
	seen := func(next step) step {
		return func(turn Turn) {
			mu.Lock()
			order = append(order, turn.Role)
			mu.Unlock()
			if turn.Warmed != nil {
				turn.Warmed()
			}
			next(turn)
		}
	}
	w.agent.steps["audit/gofi-qa"] = []step{seen(lensAudit(w.root, "aprovado"))}
	w.agent.steps["audit/gofi-qa-security"] = []step{seen(lensAudit(w.root, "aprovado"))}
	start := time.Now()
	if s := w.run(t); s.Outcome != "accepted" {
		t.Fatalf("outcome %s", s.Outcome)
	}
	if len(order) != 2 || order[0] != "gofi-qa" {
		t.Errorf("order %v", order)
	}
	if time.Since(start) > 20*time.Second {
		t.Error("the lenses waited out the limit instead of the first answer")
	}
}

// The lens that goes first is one whose tools most lenses have.
func TestTheLeadLensHasTheToolsOfMost(t *testing.T) {
	tools := func(name string, t ...string) role.Role {
		return role.Role{Name: name, Contract: role.Contract{Tools: t}}
	}
	roles := []role.Role{tools("general", "read", "verdict", "memory"), tools("a", "verdict", "read"), tools("b", "read", "verdict")}
	if got := leadLens(roles); got != 1 {
		t.Errorf("lead = %d", got)
	}
}

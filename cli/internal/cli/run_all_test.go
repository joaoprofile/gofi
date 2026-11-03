package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// change writes an open change of one context, depending on deps.
func change(t *testing.T, root, id, deps string) {
	t.Helper()
	dir := filepath.Join(root, "specs", "changes", id)
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: ["+id+"]\nversao: \"1.0\"\nstatus: proposto\ndepende_de: ["+deps+"]\ndesign: n/a\nkeywords: [x]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "deltas", id+".md"), "---\ntipo: delta\nformato: 3\ncontexto: "+id+"\nversao: \"1.0\"\nstatus: proposto\nkeywords: [x]\n---\n# D\n\n## Propósito\n\nx.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — X\nO sistema DEVE x.\n\n#### Cenário: REQ-01.1 — X\n- DADO x\n- QUANDO y\n- ENTÃO z\n")
}

// A batch delivers the open changes by their depende_de: a change whose
// dependency was not accepted is skipped, the ones that do not depend on
// it go on, one not approved is not run; the summary says each.
func TestRunAllDeliversInOrderAndSkipsWhatDependsOnAFailure(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	change(t, root, "a", "")
	change(t, root, "b", "a")
	change(t, root, "c", "")
	change(t, root, "d", "")
	for _, id := range []string{"a", "b", "c"} {
		c := newChangeCmd()
		c.SetOut(io.Discard)
		c.SetArgs([]string{"approve", id})
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var started []string
	orig := startChange
	t.Cleanup(func() { startChange = orig })
	startChange = func(_ context.Context, root, change, run string, worktree bool, _ float64, _ allOpts, _ io.Writer) (*loop.State, error) {
		mu.Lock()
		started = append(started, change)
		mu.Unlock()
		s := &loop.State{Schema: loop.StateSchema, Run: run, Change: change, Outcome: "accepted", CostUSD: 1}
		if change == "a" {
			s.Outcome, s.Why = "impasse", "achado do contrato"
		}
		return s, loop.SaveState(root, s)
	}

	var out bytes.Buffer
	cmd := newRunCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--all"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "3 of 4") {
		t.Fatalf("err %v\n%s", err, out.String())
	}
	if strings.Join(started, ",") != "a,c" {
		t.Errorf("started %v", started)
	}
	for _, want := range []string{"| 0 | a | impasse |", "| 1 | b | skipped |", "depends on a, which ended impasse", "| 0 | c | accepted |", "| 0 | d | skipped |", "not approved"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
	batches, _ := os.ReadDir(filepath.Join(root, ".gofi", "batches"))
	if len(batches) != 1 {
		t.Errorf("batch summaries: %d", len(batches))
	}
}

// The batch's budget bounds the changes together: spent, the rest is not
// started.
func TestRunAllStopsStartingWhenTheBudgetIsSpent(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  run_budget_usd: 1\n")
	for _, id := range []string{"a", "b"} {
		change(t, root, id, "")
		c := newChangeCmd()
		c.SetOut(io.Discard)
		c.SetArgs([]string{"approve", id})
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	orig := startChange
	t.Cleanup(func() { startChange = orig })
	var budgets []float64
	startChange = func(_ context.Context, root, change, run string, _ bool, budget float64, _ allOpts, _ io.Writer) (*loop.State, error) {
		budgets = append(budgets, budget)
		s := &loop.State{Schema: loop.StateSchema, Run: run, Change: change, Outcome: "accepted", CostUSD: 1.2}
		return s, loop.SaveState(root, s)
	}
	var out bytes.Buffer
	cmd := newRunCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--all"})
	_ = cmd.Execute()
	if len(budgets) != 1 || budgets[0] != 1 || !strings.Contains(out.String(), "| 0 | b | skipped |") || !strings.Contains(out.String(), "budget") {
		t.Errorf("budgets %v\n%s", budgets, out.String())
	}
}

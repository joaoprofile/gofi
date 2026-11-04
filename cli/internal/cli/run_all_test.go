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
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: ["+id+"]\nversao: \"1.0\"\nstatus: proposto\ndepende_de: ["+deps+"]\nkeywords: [x]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "design.md"), minimalDesign(id))
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
	startChange = func(_ context.Context, root string, j job, _ allOpts, _ io.Writer) (*loop.State, error) {
		change, run := j.Change, j.Run
		mu.Lock()
		started = append(started, change)
		mu.Unlock()
		s := &loop.State{Schema: loop.StateSchema, Run: run, Change: change, Outcome: "accepted", CostUSD: 1}
		if change == "a" {
			s.Outcome, s.Proposal = "impasse", ".gofi/proposals/a-1.md"
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
	for _, want := range []string{"| 0 | a | impasse |", "proposta: .gofi/proposals/a-1.md", "| 1 | b | skipped |", "depends on a, which ended impasse", "| 0 | c | accepted |", "| 0 | d | skipped |", "not approved"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary lacks %q:\n%s", want, out.String())
		}
	}
	batches, _ := os.ReadDir(filepath.Join(root, ".gofi", "batches"))
	if len(batches) != 2 {
		t.Errorf("batch summary and state: %d", len(batches))
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
	startChange = func(_ context.Context, root string, j job, _ allOpts, _ io.Writer) (*loop.State, error) {
		change, run := j.Change, j.Run
		budgets = append(budgets, j.Budget)
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

// A batch goes on with --resume: what it accepted is not run again, a run it
// left midway goes on — the same run —, one not accepted runs anew, and the
// budget is what every run of the batch spent together.
func TestRunAllResumesABatch(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  run_budget_usd: 10\n")
	change(t, root, "a", "")
	change(t, root, "b", "a")
	change(t, root, "c", "")
	for _, id := range []string{"a", "b", "c"} {
		c := newChangeCmd()
		c.SetOut(io.Discard)
		c.SetArgs([]string{"approve", id})
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	orig := startChange
	t.Cleanup(func() { startChange = orig })
	var mu sync.Mutex
	first := map[string]string{}
	startChange = func(_ context.Context, root string, j job, _ allOpts, _ io.Writer) (*loop.State, error) {
		mu.Lock()
		first[j.Change] = j.Run
		mu.Unlock()
		s := &loop.State{Schema: loop.StateSchema, Run: j.Run, Change: j.Change, Outcome: "accepted", CostUSD: 1}
		switch j.Change {
		case "b":
			s.Outcome = "impasse"
		case "c":
			// Cut short in the middle of its work.
			s.Outcome, s.Node, s.CostUSD = "", "implement", 0.5
		}
		return s, loop.SaveState(root, s)
	}
	run := func(args ...string) string {
		var out bytes.Buffer
		cmd := newRunCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		_ = cmd.Execute()
		return out.String()
	}
	out := run("--all")
	var id string
	entries, _ := os.ReadDir(filepath.Join(root, ".gofi", "batches"))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			id = strings.TrimSuffix(e.Name(), ".json")
		}
	}
	if id == "" || !strings.Contains(out, "--all --resume "+id) {
		t.Fatalf("no batch to resume (%q):\n%s", id, out)
	}

	var jobs []job
	startChange = func(_ context.Context, root string, j job, _ allOpts, _ io.Writer) (*loop.State, error) {
		mu.Lock()
		jobs = append(jobs, j)
		mu.Unlock()
		s := &loop.State{Schema: loop.StateSchema, Run: j.Run, Change: j.Change, Outcome: "accepted", CostUSD: 0.3}
		if j.Change == "c" {
			s.CostUSD = 0.8 // what it had spent, and more
		}
		return s, loop.SaveState(root, s)
	}
	out = run("--all", "--resume", id)
	got := map[string]job{}
	for _, j := range jobs {
		got[j.Change] = j
	}
	if _, ran := got["a"]; ran || len(jobs) != 2 {
		t.Errorf("jobs %+v — a was accepted", jobs)
	}
	if c := got["c"]; !c.Resume || c.Run != first["c"] || c.Budget != 10-2.5+0.5 {
		t.Errorf("c = %+v, want the same run %s going on with US$ 8.00", c, first["c"])
	}
	if b := got["b"]; b.Resume || b.Run == first["b"] {
		t.Errorf("b = %+v, want a new run", b)
	}
	// a 1 + b's first run 1 + c 0.8 + b's new run 0.3.
	if !strings.Contains(out, "US$ 3.10 de US$ 10.00") || !strings.Contains(out, "| 0 | a | accepted |") {
		t.Errorf("summary:\n%s", out)
	}
	entries, _ = os.ReadDir(filepath.Join(root, ".gofi", "batches"))
	if len(entries) != 2 {
		t.Errorf("a resumed batch writes its own files again, not new ones: %d", len(entries))
	}
	if out := run("--all", "--resume", "b-nada"); !strings.Contains(out, "b-nada") {
		t.Errorf("an unknown batch: %s", out)
	}
}

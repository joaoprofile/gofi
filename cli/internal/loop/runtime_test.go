package loop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// A scripted world: what each agent turn does, what each sensor run says,
// whether the person answers. The runtime under test only decides.

type step func(t Turn) // what an agent turn does on disk

type fakeAgent struct {
	root  string
	steps map[string][]step // by node, consumed in order
	turns []Turn
}

func (a *fakeAgent) Run(_ context.Context, t Turn) (TurnResult, error) {
	a.turns = append(a.turns, t)
	if q := a.steps[t.Node]; len(q) > 0 {
		a.steps[t.Node] = q[1:]
		if q[0] != nil {
			q[0](t)
		}
	}
	return TurnResult{CostUSD: 0.01}, nil
}

type fakeSensors struct{ results []SensorResult }

func (s *fakeSensors) Run(context.Context, []string) SensorResult {
	if len(s.results) == 0 {
		return SensorResult{}
	}
	r := s.results[0]
	s.results = s.results[1:]
	return r
}

type diskVerdicts struct{ root string }

func (d diskVerdicts) Read(ctx string) (*verdict.Verdict, error) { return verdict.Read(d.root, ctx) }

type fakeGate struct {
	answer bool
	asked  int
}

func (g *fakeGate) Ask(_ context.Context, e *intake.Elicitation) (map[string]string, bool) {
	g.asked++
	if !g.answer {
		return nil, false
	}
	out := map[string]string{}
	for _, q := range e.Questions {
		out[q.ID] = "sim"
	}
	return out, true
}

// audit writes the verdict of this run's turn, with the findings given as
// severity:owner:rule.
func audit(root string, result string, findings ...string) step {
	return func(t Turn) {
		v := &verdict.Verdict{Schema: verdict.Schema, Context: "pedidos", Run: "r-1", Iter: t.Iter, Result: result, Findings: []verdict.Finding{}}
		for _, f := range findings {
			p := strings.SplitN(f, ":", 3)
			v.Findings = append(v.Findings, verdict.Finding{Severity: p[0], Owner: p[1], Rule: p[2], Message: "m"})
		}
		data, _ := json.Marshal(v)
		parsed, err := verdict.Parse(data, "pedidos")
		if err != nil {
			panic(err)
		}
		parsed.Run, parsed.Iter = v.Run, v.Iter
		if _, err := verdict.Write(root, parsed); err != nil {
			panic(err)
		}
	}
}

// block writes a decision the implementation stopped on.
func block(root string) step {
	return func(t Turn) {
		e := intake.Elicitation{Schema: intake.ElicitSchema, Questions: []intake.ElicitQuest{{ID: "q1", Text: "Arredonda?", Options: []string{"sim", "não"}}}}
		data, _ := json.Marshal(e)
		p := filepath.Join(root, filepath.FromSlash(t.ElicitFile))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, data, 0o644)
	}
}

type world struct {
	root    string
	agent   *fakeAgent
	sensors *fakeSensors
	gate    *fakeGate
	runner  *Runner
}

func newWorld(t *testing.T) *world {
	t.Helper()
	root := t.TempDir()
	f, err := Load(root, "delivery")
	if err != nil {
		t.Fatal(err)
	}
	w := &world{root: root, agent: &fakeAgent{root: root, steps: map[string][]step{}}, sensors: &fakeSensors{}, gate: &fakeGate{answer: true}}
	clock := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	w.runner = &Runner{Root: root, Flow: f, Agent: w.agent, Sensors: w.sensors, Verdicts: diskVerdicts{root}, Gate: w.gate,
		Brief: DefaultBrief, Now: func() time.Time { clock = clock.Add(time.Second); return clock }}
	return w
}

func (w *world) run(t *testing.T) *State {
	t.Helper()
	s, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return s
}

// path is the nodes the run went through, in order.
func path(t *testing.T, root string) string {
	t.Helper()
	evs, err := Events(root, "r-1")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range evs {
		if e.Ev == "node.start" {
			out = append(out, e.Node)
		}
	}
	return strings.Join(out, " ")
}

func TestGreenOnTheFirstPass(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement sense audit policy accepted" {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

// Code that does not build goes back to the implementation with the output,
// and never reaches the audit.
func TestAFailingSensorGoesBackWithItsOutput(t *testing.T) {
	w := newWorld(t)
	w.sensors.results = []SensorResult{{Failed: []string{"build"}, Output: "service.go:12: undefined: Total"}, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement sense implement sense audit policy accepted" {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if !strings.Contains(w.agent.turns[1].Prompt, "undefined: Total") {
		t.Errorf("the second turn does not carry the build output:\n%s", w.agent.turns[1].Prompt)
	}
}

// A spent limit ends the run in on_limit: failing code is never audited.
func TestASpentLimitEndsTheRunInsteadOfFallingThrough(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 10; i++ {
		w.sensors.results = append(w.sensors.results, SensorResult{Failed: []string{"test"}})
	}
	s := w.run(t)
	if s.Outcome != "exhausted" || strings.Contains(path(t, w.root), "audit") {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if s.Why == "" && !strings.Contains(lastEdgeWhy(t, w.root), "esgotado") {
		t.Errorf("the run does not say the limit was spent: %s", lastEdgeWhy(t, w.root))
	}
}

func TestFindingsOfTheImplementationAreFixedAndAuditedAgain(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "blocker:eng:pedidos/REQ-04.1"), audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement sense audit policy implement sense audit policy accepted" {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if !strings.Contains(w.agent.turns[2].Prompt, "pedidos/REQ-04.1") {
		t.Errorf("the fix turn does not carry the finding:\n%s", w.agent.turns[2].Prompt)
	}
}

// Caveats are not acceptance: a minor reopens the implementation.
func TestCaveatsReopenTheImplementation(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado", "minor:eng:pack/naming"), audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || strings.Count(path(t, w.root), "implement") != 2 {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

// The contract is wrong: not the loop's to fix.
func TestAFindingOfTheContractIsAnImpasse(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "major:spec:pedidos/REQ-04.1")}
	if s := w.run(t); s.Outcome != "impasse" || strings.Count(path(t, w.root), "implement") != 1 {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

func TestTheSameFindingTwiceIsNoProgress(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "blocker:eng:r1"), audit(w.root, "reprovado", "blocker:eng:r1")}
	if s := w.run(t); s.Outcome != "impasse" || strings.Count(path(t, w.root), "audit") != 2 {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

// A finding fixed and then broken again is an oscillation: the run stops.
func TestAFindingThatComesBackIsAnOscillation(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["audit"] = []step{
		audit(w.root, "reprovado", "blocker:eng:r1"),
		audit(w.root, "aprovado", "minor:eng:r2"),
		audit(w.root, "reprovado", "blocker:eng:r1"),
	}
	s := w.run(t)
	if s.Outcome != "impasse" || !strings.Contains(lastEdgeWhy(t, w.root), "oscilação true") {
		t.Errorf("outcome %s, last edge %q, path %s", s.Outcome, lastEdgeWhy(t, w.root), path(t, w.root))
	}
}

// An implementation that stops on a decision gets the person's answer and
// runs again; nobody to answer is an impasse.
func TestABlockedImplementationAsksAndResumes(t *testing.T) {
	w := newWorld(t)
	w.agent.steps["implement"] = []step{block(w.root), nil}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement decide implement sense audit policy accepted" || w.gate.asked != 1 {
		t.Errorf("outcome %s, path %s, asked %d", s.Outcome, path(t, w.root), w.gate.asked)
	}
	if !strings.Contains(w.agent.turns[1].Prompt, "Arredonda?") {
		t.Errorf("the resumed turn does not carry the decision:\n%s", w.agent.turns[1].Prompt)
	}

	w = newWorld(t)
	w.gate.answer = false
	w.agent.steps["implement"] = []step{block(w.root)}
	if s := w.run(t); s.Outcome != "impasse" {
		t.Errorf("unanswered: outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

// An audit that wrote no verdict for this run stops the run, never read as
// an earlier one; the run resumes where it stopped once the verdict is there.
func TestAMissingVerdictStopsAndTheRunResumes(t *testing.T) {
	w := newWorld(t)
	audit(w.root, "aprovado")(Turn{Iter: 99}) // a stale verdict, of another iteration
	_, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos")
	if err == nil || !strings.Contains(err.Error(), "não gravou o veredicto desta execução") {
		t.Fatalf("err = %v", err)
	}
	s, _ := LoadState(w.root, "r-1")
	if s.Node != "policy" {
		t.Fatalf("stopped at %q, want policy", s.Node)
	}
	audit(w.root, "aprovado")(Turn{Iter: s.Iter})
	s, err = w.runner.Resume(context.Background(), "r-1")
	if err != nil || s.Outcome != "accepted" {
		t.Errorf("resume: %+v, %v", s, err)
	}
	if got := path(t, w.root); got != "implement sense audit policy policy accepted" {
		t.Errorf("path %s", got)
	}
}

// The same world gives the same log, byte for byte.
func TestTheRunIsDeterministic(t *testing.T) {
	logs := []string{}
	for i := 0; i < 2; i++ {
		w := newWorld(t)
		w.sensors.results = []SensorResult{{Failed: []string{"build"}, Output: "x"}, {}}
		w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "major:eng:r1"), audit(w.root, "aprovado")}
		w.run(t)
		b, _ := os.ReadFile(filepath.Join(Dir(w.root, "r-1"), "events.ndjson"))
		logs = append(logs, string(b))
	}
	if logs[0] != logs[1] {
		t.Error("two runs of the same world logged differently")
	}
}

func lastEdgeWhy(t *testing.T, root string) string {
	evs, _ := Events(root, "r-1")
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Ev == "edge" {
			return evs[i].Why
		}
	}
	return ""
}

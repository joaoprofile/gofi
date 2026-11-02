package loop

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/role"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// A scripted world: what each agent turn does, what each sensor run says,
// whether the person answers. The runtime under test only decides.

type step func(t Turn) // what an agent turn does on disk

type fakeAgent struct {
	mu    sync.Mutex // lenses run at once
	root  string
	steps map[string][]step // by node — or node/role for a lens — consumed in order
	turns []Turn
}

func (a *fakeAgent) Run(_ context.Context, t Turn) (TurnResult, error) {
	a.mu.Lock()
	a.turns = append(a.turns, t)
	key := t.Node
	if _, ok := a.steps[t.Node+"/"+t.Role]; ok {
		key = t.Node + "/" + t.Role
	}
	var s step
	if q := a.steps[key]; len(q) > 0 {
		a.steps[key], s = q[1:], q[0]
	}
	a.mu.Unlock()
	if s != nil {
		s(t)
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

func (d diskVerdicts) ReadLens(ctx, lens string) (*verdict.Verdict, error) {
	return verdict.ReadLens(d.root, ctx, lens)
}

func (d diskVerdicts) Write(v *verdict.Verdict) error { _, err := verdict.Write(d.root, v); return err }

type fakeGate struct {
	answer bool
	asked  int
}

func (g *fakeGate) Ask(_ context.Context, e Question) (map[string]string, bool) {
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
		Roles: shippedRoles(), Facts: role.Facts{"change.surface(backend)": true},
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

// path is the nodes the run went through, in order — an optional stage
// that dispatched nobody left out.
func path(t *testing.T, root string) string {
	t.Helper()
	evs, err := Events(root, "r-1")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range evs {
		switch e.Ev {
		case "node.start":
			out = append(out, e.Node)
		case "skip": // an optional stage nobody answers did not run
			out = out[:len(out)-1]
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

// Code that does not build goes to repair with the sensor's findings placed
// by file and line, and never reaches the audit; a failing test goes back to
// the implementation (0002, D6).
func TestAFailingSensorGoesBackWithItsFindings(t *testing.T) {
	w := newWorld(t)
	w.sensors.results = []SensorResult{{Failed: []string{"build"}, Findings: []SensorFinding{
		{Stage: "build", File: "backend/pedidos/service.go", Line: 12, Severity: "blocker", Message: "undefined: Total"}}}, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement sense repair sense audit policy accepted" {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if w.agent.turns[1].Role != "gofi-repair" || !strings.Contains(w.agent.turns[1].Prompt, "`backend/pedidos/service.go:12` undefined: Total") {
		t.Errorf("the repair turn does not carry the finding:\n%s", w.agent.turns[1].Prompt)
	}

	w = newWorld(t)
	w.sensors.results = []SensorResult{{Failed: []string{"test"}, Findings: []SensorFinding{
		{Stage: "test", Rule: "TestTotal", Severity: "blocker", Message: "TestTotal falhou"}}}, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || path(t, w.root) != "implement sense implement sense audit policy accepted" {
		t.Errorf("a failing test: outcome %s, path %s", s.Outcome, path(t, w.root))
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

// propose writes the amendment the triage turn is told to write.
func propose(dir string) step {
	return func(t Turn) {
		m := regexp.MustCompile("`(" + regexp.QuoteMeta(ProposalsDir) + "/[^`]+)`").FindStringSubmatch(t.Prompt)
		if m == nil {
			panic("the triage turn names no proposal:\n" + t.Prompt)
		}
		p := filepath.Join(dir, filepath.FromSlash(m[1]))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("# Proposta de emenda\n"), 0o644)
	}
}

// tiers are the tiers a node's turns ran on, in order.
func tiers(a *fakeAgent, node string) string {
	var out []string
	for _, t := range a.turns {
		if t.Node == node {
			out = append(out, string(t.Tier))
		}
	}
	return strings.Join(out, " ")
}

// The contract is wrong: not the loop's to fix. Triage proposes the
// amendment, numbered after the context's last, and the run ends in impasse
// (0002, D9).
func TestAFindingOfTheContractIsTriagedIntoAProposal(t *testing.T) {
	w := newWorld(t)
	_ = os.MkdirAll(filepath.Join(w.root, ".gofi", "proposals"), 0o755)
	_ = os.WriteFile(filepath.Join(w.root, ".gofi", "proposals", "pedidos-1.md"), []byte("x"), 0o644)
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "major:spec:pedidos/REQ-04.1")}
	w.agent.steps["triage"] = []step{propose(w.root)}
	s := w.run(t)
	if s.Outcome != "impasse" || path(t, w.root) != "implement sense audit policy triage impasse" {
		t.Fatalf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	last := w.agent.turns[len(w.agent.turns)-1]
	if last.Role != "gofi-triage" || !strings.Contains(last.Prompt, "pedidos/REQ-04.1") || s.Proposal != ".gofi/proposals/pedidos-2.md" {
		t.Errorf("triage %s, proposal %q:\n%s", last.Role, s.Proposal, last.Prompt)
	}
	if !hasEvent(t, w.root, "proposal", s.Proposal) {
		t.Error("the proposal is not in the log")
	}
}

// Without progress the retry runs on the same tier; two iterations without
// progress raise the implementation one tier, and on the highest tier the
// run is stuck: triaged, then impasse (0002, D8).
func TestNoProgressRaisesTheTierBeforeTheImpasse(t *testing.T) {
	w := newWorld(t)
	for i := 0; i < 5; i++ {
		w.agent.steps["audit"] = append(w.agent.steps["audit"], audit(w.root, "reprovado", "blocker:eng:r1"))
	}
	s := w.run(t)
	if s.Outcome != "impasse" || !strings.HasSuffix(path(t, w.root), "policy triage impasse") {
		t.Fatalf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if got := tiers(w.agent, "implement"); got != "standard standard standard deep deep" {
		t.Errorf("implementation tiers = %s", got)
	}
	if got := tiers(w.agent, "audit"); got != "standard standard standard standard standard" {
		t.Errorf("the audit was raised: %s", got)
	}
	if !hasEvent(t, w.root, "escalate", "gofi-eng") {
		t.Error("the escalation is not in the log")
	}
}

// loop.escalate: never raises nothing, and stops at the second iteration
// without progress; on_first_reject raises at the first refusal.
func TestTheEscalationFollowsThePolicy(t *testing.T) {
	w := newWorld(t)
	w.runner.Policy.Escalate = EscalateNever
	for i := 0; i < 3; i++ {
		w.agent.steps["audit"] = append(w.agent.steps["audit"], audit(w.root, "reprovado", "blocker:eng:r1"))
	}
	if s := w.run(t); s.Outcome != "impasse" || tiers(w.agent, "implement") != "standard standard standard" || !strings.Contains(path(t, w.root), "triage") {
		t.Errorf("never: outcome %s, tiers %s, path %s", s.Outcome, tiers(w.agent, "implement"), path(t, w.root))
	}

	w = newWorld(t)
	w.runner.Policy.Escalate = EscalateOnFirstReject
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "blocker:eng:r1"), audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || tiers(w.agent, "implement") != "standard deep" {
		t.Errorf("on_first_reject: outcome %s, tiers %s", s.Outcome, tiers(w.agent, "implement"))
	}
}

// The repair starts light; two repairs that leave the same failure raise
// it. A failure further on — a later stage — is progress.
func TestTheRepairStartsLightAndRisesWhenItDoesNotProgress(t *testing.T) {
	w := newWorld(t)
	broken := SensorResult{Failed: []string{"build"}, Findings: []SensorFinding{{Stage: "build", File: "a.go", Line: 1, Severity: "blocker", Message: "undefined: X"}}}
	w.sensors.results = []SensorResult{broken, broken, broken, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || tiers(w.agent, "repair") != "light light standard" {
		t.Errorf("outcome %s, repair tiers %s, path %s", s.Outcome, tiers(w.agent, "repair"), path(t, w.root))
	}

	w = newWorld(t)
	w.sensors.results = []SensorResult{broken, {Failed: []string{"static"}, Findings: []SensorFinding{{Stage: "static", Severity: "blocker", Message: "vet"}}}, broken, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	w.runner.Flow.Nodes["sense"] = Node{Kind: KindSensor, Stages: []string{"build", "static"}}
	if s := w.run(t); s.Outcome != "accepted" || tiers(w.agent, "repair") != "light light light" {
		t.Errorf("progress across stages: outcome %s, repair tiers %s", s.Outcome, tiers(w.agent, "repair"))
	}
}

// A spent budget runs no agent again: the run ends exhausted, saying why.
func TestASpentBudgetRunsNoAgentAgain(t *testing.T) {
	w := newWorld(t)
	w.runner.Policy.BudgetUSD = 0.025 // each fake turn costs 0.01
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "blocker:eng:r1"), audit(w.root, "reprovado", "major:eng:r1")}
	s := w.run(t)
	if s.Outcome != "exhausted" || len(w.agent.turns) != 3 || !strings.Contains(s.Why, "orçamento de US$ 0.03") {
		t.Errorf("outcome %s, turns %d, why %q, path %s", s.Outcome, len(w.agent.turns), s.Why, path(t, w.root))
	}
}

// loop.max_iterations bounds every way back to work together — a failing
// sensor and a refused audit alike.
func TestMaxIterationsBoundsTheReworkTogether(t *testing.T) {
	w := newWorld(t)
	w.runner.Policy.MaxIterations = 2
	w.sensors.results = []SensorResult{{Failed: []string{"test"}}, {}}
	w.agent.steps["audit"] = []step{audit(w.root, "reprovado", "blocker:eng:r1"), audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "exhausted" || !strings.Contains(lastEdgeWhy(t, w.root), "loop.max_iterations esgotado (2)") {
		t.Errorf("outcome %s, last edge %q, path %s", s.Outcome, lastEdgeWhy(t, w.root), path(t, w.root))
	}
	if path(t, w.root) != "implement sense implement sense audit policy exhausted" {
		t.Errorf("path %s", path(t, w.root))
	}
}

// loop.accept: ressalvas accepts an approval with caveats open.
func TestRessalvasAcceptsCaveats(t *testing.T) {
	w := newWorld(t)
	w.runner.Policy.Accept = AcceptCaveats
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado", "minor:eng:pack/naming")}
	if s := w.run(t); s.Outcome != "accepted" || strings.Count(path(t, w.root), "implement") != 1 {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
}

// A run in a worktree leaves its proposal where the person reads it: at the
// project root.
func TestTheProposalOfAWorktreeRunIsKeptAtTheRoot(t *testing.T) {
	w := newWorld(t)
	work := t.TempDir()
	w.runner.Work = work
	w.runner.Verdicts = diskVerdicts{work}
	w.agent.steps["audit"] = []step{audit(work, "reprovado", "major:pd:pedidos/REQ-01")}
	w.agent.steps["triage"] = []step{propose(work)}
	s := w.run(t)
	if _, err := os.Stat(filepath.Join(w.root, filepath.FromSlash(s.Proposal))); s.Outcome != "impasse" || err != nil {
		t.Errorf("outcome %s, proposal %q at the root: %v", s.Outcome, s.Proposal, err)
	}
}

func hasEvent(t *testing.T, root, ev, to string) bool {
	t.Helper()
	evs, _ := Events(root, "r-1")
	for _, e := range evs {
		if e.Ev == ev && e.To == to {
			return true
		}
	}
	return false
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

// A decision written as an agent tends to — options as objects, the schema
// left out — still stops the implementation at the gate; one gofi cannot
// read stops the run, never read as no decision.
func TestADecisionIsNeverLostToItsShape(t *testing.T) {
	writeRaw := func(root, body string) step {
		return func(t Turn) {
			p := filepath.Join(root, filepath.FromSlash(t.ElicitFile))
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte(body), 0o644)
		}
	}
	w := newWorld(t)
	w.agent.steps["implement"] = []step{writeRaw(w.root, `{"format":"gofi.elicit/v1","questions":[{"id":"resto","text":"Quem absorve o resto?","options":[{"id":"ultima","label":"A última"},{"id":"primeira","label":"A primeira"}],"recommended":"ultima"}]}`), nil}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || w.gate.asked != 1 || !strings.HasPrefix(path(t, w.root), "implement decide implement") {
		t.Errorf("outcome %s, asked %d, path %s", s.Outcome, w.gate.asked, path(t, w.root))
	}

	w = newWorld(t)
	w.agent.steps["implement"] = []step{writeRaw(w.root, `{"questions": "?"}`)}
	_, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos")
	if err == nil || !strings.Contains(err.Error(), "não lê") {
		t.Errorf("err = %v", err)
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

// Resumed still without its verdict, the run goes back to the audit, which
// writes it this time: reading the policy again would only fail again.
func TestAResumeWithoutTheVerdictRunsTheAuditAgain(t *testing.T) {
	w := newWorld(t)
	if _, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos"); err == nil {
		t.Fatal("the run went on without a verdict")
	}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s, err := w.runner.Resume(context.Background(), "r-1")
	if err != nil || s.Outcome != "accepted" {
		t.Fatalf("resume: %+v, %v", s, err)
	}
	if got := path(t, w.root); got != "implement sense audit policy audit policy accepted" {
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

// The roles that implement and audit are dispatched by the change's facts,
// and the log says which and why (0002, D18).
func TestTheStageDispatchesByTheChangesFacts(t *testing.T) {
	w := newWorld(t)
	w.runner.Facts = role.Facts{"change.surface(web)": true}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" {
		t.Fatalf("outcome %s", s.Outcome)
	}
	if w.agent.turns[0].Role != "gofi-ui" || !strings.HasPrefix(w.agent.turns[0].Prompt, "Siga o papel descrito em ") {
		t.Errorf("first turn: %s\n%s", w.agent.turns[0].Role, w.agent.turns[0].Prompt)
	}
	evs, _ := Events(w.root, "r-1")
	var got []string
	for _, e := range evs {
		if e.Ev == "dispatch" {
			got = append(got, e.Node+">"+e.To+" ("+e.Why+")")
		}
	}
	if strings.Join(got, "; ") != "implement>gofi-ui (change.surface(web)); audit>gofi-qa (sensor.green)" {
		t.Errorf("dispatch events = %v", got)
	}
}

// A change no role implements stops the run, saying what each role lacked.
func TestAStageNobodyAnswersStopsTheRun(t *testing.T) {
	w := newWorld(t)
	w.runner.Facts = role.Facts{"change.surface(infra)": true}
	_, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos")
	if err == nil || !strings.Contains(err.Error(), "nenhum papel do estágio implement") || !strings.Contains(err.Error(), "✗ gofi-eng") {
		t.Fatalf("err = %v", err)
	}
	if len(w.agent.turns) != 0 {
		t.Error("a turn ran with nobody dispatched")
	}
}

// planned writes the plan the plan turn is told to write.
func planned(dir, body string) step {
	return func(t Turn) {
		p := filepath.Join(dir, filepath.FromSlash(planFile("r-1")))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
}

const threeLayers = `{"schema":"gofi-plan/v1","tasks":[
 {"id":"T1","layer":"data","summary":"tabela","reqs":["REQ-01"],"files":["backend/pedidos/repository/"]},
 {"id":"T2","layer":"domain","summary":"serviço","reqs":["REQ-01"],"files":["backend/pedidos/service/"],"depends":["T1"]},
 {"id":"T3","layer":"api","summary":"rota","reqs":["REQ-01"],"files":["backend/pedidos/handler/"],"depends":["T2"]}]}`

// auditAt writes a verdict with findings placed in a file, given as
// severity:owner:rule:file.
func auditAt(root, result string, findings ...string) step {
	return func(t Turn) {
		v := &verdict.Verdict{Schema: verdict.Schema, Context: "pedidos", Run: "r-1", Iter: t.Iter, Result: result, Findings: []verdict.Finding{}}
		for _, f := range findings {
			p := strings.SplitN(f, ":", 4)
			v.Findings = append(v.Findings, verdict.Finding{Severity: p[0], Owner: p[1], Rule: p[2], File: p[3], Message: "m"})
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

func layeredWorld(t *testing.T) *world {
	w := newWorld(t)
	w.runner.Roles = layeredRoles()
	w.runner.IsTest = func(f string) bool { return strings.HasSuffix(f, "_test.go") }
	w.agent.steps["plan"] = []step{planned(w.root, threeLayers)}
	return w
}

// roles are the roles a node's turns ran, in order.
func roles(a *fakeAgent, node string) string {
	var out []string
	for _, t := range a.turns {
		if t.Node == node {
			out = append(out, t.Role)
		}
	}
	return strings.Join(out, " ")
}

// The plan decomposes the change by layer, and the implementation is one
// role per layer with work, in the plan's order, each told its tasks; the
// tests are written by a role of their own (0002, D17).
func TestThePlanDispatchesOneRolePerLayerInItsOrder(t *testing.T) {
	w := layeredWorld(t)
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "plan implement test sense audit policy accepted" {
		t.Fatalf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if got := roles(w.agent, "implement"); got != "gofi-eng-data gofi-eng gofi-eng-api" {
		t.Errorf("implementation = %s", got)
	}
	if got := roles(w.agent, "test"); got != "gofi-test" {
		t.Errorf("tests = %q", got)
	}
	for _, turn := range w.agent.turns {
		if turn.Role == "gofi-eng-data" && (!strings.Contains(turn.Prompt, "T1 — tabela") || strings.Contains(turn.Prompt, "T2")) {
			t.Errorf("the data turn does not carry its tasks alone:\n%s", turn.Prompt)
		}
		if turn.Role == "gofi-plan" && !strings.Contains(turn.Prompt, planFile("r-1")) {
			t.Errorf("the plan turn does not say where the plan goes:\n%s", turn.Prompt)
		}
	}
}

// A retry goes to the layers the findings point at: an audit finding in the
// handler to the API alone, a failing test to the code it tests; the tests
// are not written again.
func TestARetryGoesToTheLayersTheFindingsPointAt(t *testing.T) {
	w := layeredWorld(t)
	w.sensors.results = []SensorResult{{}, {Failed: []string{"test"}, Findings: []SensorFinding{{Stage: "test", File: "backend/pedidos/service/service_test.go", Severity: "blocker", Message: "TestTotal falhou"}}}, {}}
	w.agent.steps["audit"] = []step{auditAt(w.root, "reprovado", "blocker:eng:pedidos/REQ-01:backend/pedidos/handler/h.go"), audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" {
		t.Fatalf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if got := roles(w.agent, "implement"); got != "gofi-eng-data gofi-eng gofi-eng-api gofi-eng-api gofi-eng" {
		t.Errorf("implementation = %s", got)
	}
	if got := roles(w.agent, "test"); got != "gofi-test" {
		t.Errorf("tests = %q", got)
	}

	// A finding nobody's files hold goes back to every layer.
	w = layeredWorld(t)
	w.agent.steps["audit"] = []step{auditAt(w.root, "reprovado", "blocker:eng:pedidos/REQ-01:backend/main.go"), audit(w.root, "aprovado")}
	w.run(t)
	if got := roles(w.agent, "implement"); got != "gofi-eng-data gofi-eng gofi-eng-api gofi-eng-data gofi-eng gofi-eng-api" {
		t.Errorf("an unplaced finding: implementation = %s", got)
	}
}

// What the audit found only in the tests goes to the test role.
func TestAFindingInTheTestsGoesToTheTestRole(t *testing.T) {
	w := layeredWorld(t)
	w.agent.steps["audit"] = []step{auditAt(w.root, "reprovado", "major:eng:pedidos/REQ-01.1:backend/pedidos/service/service_test.go"), audit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "plan implement test sense audit policy test sense audit policy accepted" {
		t.Errorf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	if got := roles(w.agent, "test"); got != "gofi-test gofi-test" {
		t.Errorf("tests = %q", got)
	}
}

// A plan gofi cannot read, or none, stops the run.
func TestAPlanThatCannotBeReadStopsTheRun(t *testing.T) {
	w := layeredWorld(t)
	w.agent.steps["plan"] = []step{planned(w.root, `{"tasks":[{"id":"T1","layer":"ui","files":["x"]}]}`)}
	if _, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos"); err == nil || !strings.Contains(err.Error(), `layer "ui"`) {
		t.Errorf("err = %v", err)
	}
	w = layeredWorld(t)
	w.agent.steps["plan"] = []step{nil}
	if _, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos"); err == nil || !strings.Contains(err.Error(), "não foi gravado") {
		t.Errorf("err = %v", err)
	}
}

// A change of a screen has no plan: the plan and test stages pass, and the
// surface's role implements.
func TestAChangeWithNoPlanPassesThePlanStage(t *testing.T) {
	w := layeredWorld(t)
	for _, r := range shippedRoles() {
		if r.Name == "gofi-ui" {
			w.runner.Roles = append(w.runner.Roles, r)
		}
	}
	w.runner.Facts = role.Facts{"change.surface(web)": true}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" || path(t, w.root) != "implement sense audit policy accepted" || roles(w.agent, "implement") != "gofi-ui" {
		t.Errorf("outcome %s, path %s, implementation %s", s.Outcome, path(t, w.root), roles(w.agent, "implement"))
	}
}

// Who writes the code never writes the test that judges it: what an
// implementer changes in a test file, or the test role in the code, is
// undone and logged (0002, D12, D17).
func TestTheAuthorshipOfCodeAndTestsIsGuarded(t *testing.T) {
	w := layeredWorld(t)
	w.runner.IsSource = func(f string) bool { return strings.HasSuffix(f, ".go") }
	put := func(rel, body string) step {
		return func(Turn) {
			p := filepath.Join(w.root, filepath.FromSlash(rel))
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte(body), 0o644)
		}
	}
	put("backend/pedidos/service/service_test.go", "teste do gofi-test\n")(Turn{})
	both := func(a, b step) step { return func(t Turn) { a(t); b(t) } }
	w.agent.steps["implement"] = []step{nil, both(put("backend/pedidos/service/service.go", "código\n"), put("backend/pedidos/service/service_test.go", "teste do eng\n")), nil}
	w.agent.steps["test"] = []step{put("backend/pedidos/service/service.go", "código do gofi-test\n")}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	if s := w.run(t); s.Outcome != "accepted" {
		t.Fatalf("outcome %s", s.Outcome)
	}
	read := func(rel string) string {
		b, _ := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(rel)))
		return string(b)
	}
	if got := read("backend/pedidos/service/service_test.go"); got != "teste do gofi-test\n" {
		t.Errorf("the implementer's test stayed: %q", got)
	}
	if got := read("backend/pedidos/service/service.go"); got != "código\n" {
		t.Errorf("the test role's code stayed: %q", got)
	}
	if !hasEvent(t, w.root, "guard", "gofi-eng") || !hasEvent(t, w.root, "guard", "gofi-test") {
		t.Error("the guard is not in the log")
	}
}

// lensAudit writes one lens's verdict of this turn, its findings as
// severity:owner:rule:file.
func lensAudit(root, result string, findings ...string) step {
	return func(t Turn) {
		v := &verdict.Verdict{Schema: verdict.Schema, Context: "pedidos", Result: result, Findings: []verdict.Finding{}}
		for _, f := range findings {
			p := strings.SplitN(f, ":", 4)
			v.Findings = append(v.Findings, verdict.Finding{Severity: p[0], Owner: p[1], Rule: p[2], File: p[3], Message: "m"})
		}
		data, _ := json.Marshal(v)
		parsed, err := verdict.Parse(data, "pedidos")
		if err != nil {
			panic(err)
		}
		parsed.Run, parsed.Iter, parsed.Lens = "r-1", t.Iter, t.Role
		if _, err := verdict.Write(root, parsed); err != nil {
			panic(err)
		}
	}
}

// lensWorld audits in lenses: the general one and security, both reading
// only.
func lensWorld(t *testing.T) *world {
	w := newWorld(t)
	reads := []string{"read", "search", "gofi", "verify", "verdict"}
	var roles []role.Role
	for _, r := range shippedRoles() {
		if r.Contract.Stage != "audit" {
			roles = append(roles, r)
		}
	}
	w.runner.Roles = append(roles,
		role.Role{Name: "gofi-qa", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "audit", Tools: reads,
			Triggers: []role.Trigger{{All: []string{"sensor.green"}}}}},
		role.Role{Name: "gofi-qa-security", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "audit", Tools: reads,
			Triggers: []role.Trigger{{All: []string{"sensor.green", "change.surface(backend)"}}}}})
	return w
}

// The audit runs in lenses, each in its own session on the same iteration,
// told it is a lens; gofi merges their verdicts, and the policy reads the
// merge (0002, D17).
func TestTheAuditRunsInLensesAndTheirVerdictsMerge(t *testing.T) {
	w := lensWorld(t)
	w.agent.steps["audit/gofi-qa"] = []step{lensAudit(w.root, "aprovado"), lensAudit(w.root, "aprovado")}
	w.agent.steps["audit/gofi-qa-security"] = []step{lensAudit(w.root, "reprovado", "blocker:eng:pedidos/REQ-01:backend/pedidos/handler/h.go"), lensAudit(w.root, "aprovado")}
	s := w.run(t)
	if s.Outcome != "accepted" || path(t, w.root) != "implement sense audit policy implement sense audit policy accepted" {
		t.Fatalf("outcome %s, path %s", s.Outcome, path(t, w.root))
	}
	var audits []Turn
	for _, turn := range w.agent.turns {
		if turn.Node == "audit" {
			audits = append(audits, turn)
		}
	}
	if len(audits) != 4 || audits[0].Iter != audits[1].Iter || audits[0].Agent == audits[1].Agent {
		t.Fatalf("lens turns = %+v", audits)
	}
	if !strings.Contains(audits[0].Prompt, "lens "+audits[0].Role) {
		t.Errorf("a lens is not told its lens:\n%s", audits[0].Prompt)
	}
	var fixes []Turn
	for _, turn := range w.agent.turns {
		if turn.Node == "implement" {
			fixes = append(fixes, turn)
		}
	}
	if len(fixes) != 2 || !strings.Contains(fixes[1].Prompt, "pedidos/REQ-01") {
		t.Errorf("the fix turn does not carry the security lens's finding: %+v", fixes)
	}
	v, _ := verdict.Read(w.root, "pedidos")
	if v == nil || strings.Join(v.Lenses, ",") != "gofi-qa,gofi-qa-security" || !v.Clean() {
		t.Errorf("merged verdict = %+v", v)
	}
	agents, _ := Agents(w.root, "r-1")
	if len(agents) != 6 {
		t.Errorf("agents = %d, want an agent per lens per audit", len(agents))
	}
}

// A lens that wrote no verdict stops the run, as an audit that wrote none.
func TestALensWithoutItsVerdictStopsTheRun(t *testing.T) {
	w := lensWorld(t)
	w.agent.steps["audit/gofi-qa"] = []step{lensAudit(w.root, "aprovado")}
	w.agent.steps["audit/gofi-qa-security"] = []step{nil}
	if _, err := w.runner.Start(context.Background(), "r-1", "criar-pedidos", "pedidos"); err == nil || !strings.Contains(err.Error(), "a lente gofi-qa-security não gravou") {
		t.Errorf("err = %v", err)
	}
}

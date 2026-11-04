package loop

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/role"
	"github.com/joaoprofile/gofi/cli/internal/runs"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// StateSchema names the format of a run's state file.
const StateSchema = "gofi-run-state/v1"

// RunsDir is where runs keep their state and events, under the project root:
// beside the run records, one folder per run.
const RunsDir = runs.Dir

// The ports the runtime reaches the world through. The runtime decides; they
// only do — and a test gives fakes.
type (
	// Agent runs one turn of a role in a session of its own.
	Agent interface {
		Run(ctx context.Context, t Turn) (TurnResult, error)
	}
	// Sensors runs the deterministic checks of a sensor node.
	Sensors interface {
		Run(ctx context.Context, stages []string) SensorResult
	}
	// Verdicts reads the audit's last verdict of a context — or one lens's
	// of it — and writes the one merged from the lenses.
	Verdicts interface {
		Read(ctx string) (*verdict.Verdict, error)
		ReadLens(ctx, lens string) (*verdict.Verdict, error)
		Write(v *verdict.Verdict) error
	}
	// Gate asks the person a decision an agent stopped on. ok is false when
	// nobody answered in time: the run cannot go on on its own.
	Gate interface {
		Ask(ctx context.Context, q Question) (answers map[string]string, ok bool)
	}
)

// Turn is what an agent node sends.
type Turn struct {
	Node, Role string
	// Agent is the agent's id, where its transcript goes.
	Agent string
	// Tier is the role's, for the engine to pick its model; Tools what its
	// contract grants, nil for the engine's defaults.
	Tier   host.Tier
	Tools  []string
	Iter   int
	Prompt string
	// ElicitFile is where the agent writes a decision it cannot find, relative
	// to the project root.
	ElicitFile string
}

// TurnResult is what the turn cost.
type TurnResult struct {
	CostUSD float64
	Seconds float64
}

// SensorResult is one sensor run: the stages that failed, and what they said.
type SensorResult struct {
	Failed []string `json:"failed,omitempty"`
	Output string   `json:"output,omitempty"`
	// Findings are the failure read into places — file, line, what is wrong
	// — from the tool's own report: what the next turn is given instead of
	// the raw output (0002, D6).
	Findings []SensorFinding `json:"findings,omitempty"`
}

// SensorFinding is one thing a sensor found.
type SensorFinding struct {
	Stage string `json:"stage"`
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
	// Symbol is the code graph's symbol the finding is at, when the tool
	// names one.
	Symbol   string `json:"symbol,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity"` // blocker | major
	Message  string `json:"message"`
}

// Finding is the part of a verdict's finding the policy keeps.
type Finding struct {
	Fingerprint string `json:"fingerprint"`
	Severity    string `json:"severity"`
	Owner       string `json:"owner"`
	Rule        string `json:"rule"`
	File        string `json:"file,omitempty"`
	Message     string `json:"message"`
}

// State is a run, as the runtime leaves it after each node: enough to resume
// from the node it points at, and nothing a model wrote.
type State struct {
	Schema string `json:"schema"`
	Run    string `json:"run"`
	Flow   string `json:"flow"`
	Change string `json:"mudanca"`
	// ChangeDir is the change's folder: in its context's folder, or in
	// specs/changes/ when it changes several.
	ChangeDir string `json:"mudanca_dir,omitempty"`
	Context   string `json:"contexto"`
	// Node is the next node to run; empty once the run has ended.
	Node    string `json:"node"`
	Outcome string `json:"outcome,omitempty"`
	Why     string `json:"why,omitempty"`
	// Taken counts how many times each edge was taken ("from>to").
	Taken map[string]int `json:"taken"`
	Iter  int            `json:"iter"`
	// Agent is the last agent node run: where a resume goes back to when its
	// turn did not leave what the node after it reads.
	Agent string `json:"agent,omitempty"`

	Sensor   *SensorResult `json:"sensor,omitempty"`
	Verdict  string        `json:"verdict,omitempty"` // clean | caveats | rejected
	Open     []Finding     `json:"open,omitempty"`
	Score    *int          `json:"score,omitempty"`
	Previous *int          `json:"previous,omitempty"`
	Resolved []string      `json:"resolved,omitempty"`
	// Rounds is how many times the work went back after a sensor or the
	// audit refused it; Stalled and SensorStalled the audit's and the
	// sensors' judgments in a row without progress, SensorMark how far the
	// last failing sensor run got. Raised is how many tiers each role runs
	// above its own, Worked the turns the next audit judges, and Stuck that
	// nothing progresses and no tier is left to raise (0002, D8).
	Rounds        int            `json:"rounds,omitempty"`
	Stalled       int            `json:"stalled,omitempty"`
	SensorStalled int            `json:"sensor_stalled,omitempty"`
	SensorMark    *int           `json:"sensor_mark,omitempty"`
	Raised        map[string]int `json:"raised,omitempty"`
	Worked        []Ran          `json:"worked,omitempty"`
	Stuck         bool           `json:"stuck,omitempty"`
	// Plan is what the plan stage decomposed the change into; Tested that
	// the tests were written by the test stage once.
	Plan   *Plan `json:"plan,omitempty"`
	Tested bool  `json:"tested,omitempty"`
	// Seen is every finding the audit raised in the run, as first raised, and
	// SensorFailures how many times each sensor stage refused the work:
	// what the run had to fix, for the lessons proposed of it (0002, §7).
	Seen []Finding `json:"seen,omitempty"`
	// Notes are the minors of the contract the last audit raised: kept for
	// the person, they stop nothing.
	Notes          []Finding      `json:"notes,omitempty"`
	SensorFailures map[string]int `json:"sensor_failures,omitempty"`
	// Diff is what the graph stage last said the change did to the code;
	// Reach the contexts besides the change's own that depend on what it
	// touches — or, before any code, on what its plan writes (0002, D11).
	Diff  *GraphDiff `json:"diff,omitempty"`
	Reach []string   `json:"reach,omitempty"`
	// Lenses are the roles auditing this iteration as lenses; Ambiguous the
	// findings whose owner they disagreed on.
	Lenses    []string `json:"lenses,omitempty"`
	Ambiguous []string `json:"ambiguous,omitempty"`
	// Proposal is the amendment the run proposes when it cannot go on — the
	// contract is wrong, or the work is stuck — relative to the root (D9).
	Proposal string `json:"proposal,omitempty"`
	Blocked  bool   `json:"blocked,omitempty"`
	Answered bool   `json:"answered,omitempty"`
	// Elicit is the file the last blocked agent wrote its decisions to,
	// BlockedBy that agent, and Answers what the person answered at the gate.
	Elicit    string            `json:"elicit,omitempty"`
	BlockedBy string            `json:"blocked_by,omitempty"`
	Answers   map[string]string `json:"answers,omitempty"`
	// Worktree, Branch and Base say where a run in a worktree works and
	// lands — Base alone, the branch a run in the project's tree commits on;
	// Landing is how that went: landed, waiting, conflict.
	Worktree string `json:"worktree,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Base     string `json:"base,omitempty"`
	Landing  string `json:"landing,omitempty"`
	// Kept is the branch the work of a run not accepted in the project's tree
	// was kept on, the tree put back where it started.
	Kept    string    `json:"kept,omitempty"`
	CostUSD float64   `json:"cost_usd"`
	Updated time.Time `json:"updated"`
}

// Event is one line of a run's log.
type Event struct {
	Seq  int       `json:"seq"`
	T    time.Time `json:"t"`
	Ev   string    `json:"ev"`
	Node string    `json:"node,omitempty"`
	To   string    `json:"to,omitempty"`
	Why  string    `json:"why,omitempty"`
	// CostUSD is what the run had spent when the event happened: what a
	// batch of runs side by side adds up as they go.
	CostUSD float64 `json:"cost_usd,omitempty"`
	// Run is the run the event is of: what a chat following a delivery of
	// several changes answers a gate of, or stops.
	Run string `json:"run,omitempty"`
}

// Runner carries out a flow over one change.
type Runner struct {
	// Root is the project root, where the run's state, events and agents are
	// kept for gofi ps and gofi watch; Work is where the agents work — the
	// run's worktree, or the root when it has none.
	Root     string
	Work     string
	Flow     *Flow
	Agent    Agent
	Sensors  Sensors
	Verdicts Verdicts
	Gate     Gate
	// Brief writes the prompt of a role's turn from the state.
	Brief func(rl role.Role, s *State, elicitFile string) string
	// Roles are the installed roles a stage dispatches from; Facts what
	// holds about the change, computed when the run starts.
	Roles []role.Role
	Facts role.Facts
	// Policy is the project's convergence policy: budget, iterations,
	// acceptance, escalation.
	Policy Policy
	// Reach is the contexts that depend on the code of some files, from the
	// graph the run started from; nil when the project has no graph.
	Reach func(files []string) []string
	// IsTest says a file is a test of the project's language, IsSource that
	// it is code or test, from the language's pack.
	IsTest   func(file string) bool
	IsSource func(file string) bool
	// Now is the clock; tests fix it.
	Now func() time.Time
	// OnEvent sees every event as it is written, for a live view.
	OnEvent func(Event)

	seq int
}

// SaveState writes a run's state as the runtime does after each node: for
// what happens to a run past its terminal node — archived, landed.
func SaveState(root string, s *State) error {
	return (&Runner{Root: root}).save(s)
}

// Dir is where a run keeps its files.
func Dir(root, run string) string { return filepath.Join(root, filepath.FromSlash(RunsDir), run) }

// Start begins a run of the flow over a change.
func (r *Runner) Start(ctx context.Context, run, change, context string) (*State, error) {
	return r.StartIn(ctx, run, change, context, nil)
}

// StartIn is Start with setup filling the state before it is first saved:
// where a run in a worktree works is on record from its first node.
func (r *Runner) StartIn(ctx context.Context, run, change, context string, setup func(*State)) (*State, error) {
	s := &State{Schema: StateSchema, Run: run, Flow: r.Flow.Name, Change: change, Context: context,
		Node: r.Flow.Start, Taken: map[string]int{}}
	if dir, ok := contract.FindChange(r.work(), change); ok {
		s.ChangeDir = dir
	}
	if setup != nil {
		setup(s)
	}
	if err := os.MkdirAll(Dir(r.Root, run), 0o755); err != nil {
		return nil, err
	}
	r.saveFlow(run)
	r.emit(s, Event{Ev: "run.start", Node: r.Flow.Start, Why: "mudança " + change})
	if err := r.save(s); err != nil {
		return nil, err
	}
	return r.Continue(ctx, s)
}

// Resume reads a run's state and carries on from the node it points at.
func (r *Runner) Resume(ctx context.Context, run string) (*State, error) {
	s, err := LoadState(r.Root, run)
	if err != nil {
		return nil, err
	}
	if s.Node == "" {
		return s, fmt.Errorf("run %s ended (%s)", run, s.Outcome)
	}
	r.seq = countEvents(r.Root, run)
	if n := r.Flow.Nodes[s.Node]; n.Kind == KindPolicy && s.Agent != "" {
		// Stopped for a verdict the audit never wrote: reading it again finds
		// nothing, so the audit runs again.
		if v, _ := r.Verdicts.Read(s.Context); v == nil || v.Run != s.Run || v.Iter != s.Iter {
			r.emit(s, Event{Ev: "run.resume", Node: s.Agent, Why: "sem veredicto desta iteração: " + s.Agent + " roda de novo"})
			s.Node = s.Agent
			return r.Continue(ctx, s)
		}
	}
	r.emit(s, Event{Ev: "run.resume", Node: s.Node})
	return r.Continue(ctx, s)
}

// Continue runs nodes from s.Node until a terminal, checkpointing after each.
func (r *Runner) Continue(ctx context.Context, s *State) (*State, error) {
	for s.Node != "" {
		if err := ctx.Err(); err != nil {
			r.emit(s, Event{Ev: "run.cancelled", Node: s.Node})
			_ = r.save(s)
			return s, err
		}
		name := s.Node
		n, ok := r.Flow.Nodes[name]
		if !ok {
			return s, fmt.Errorf("node %q is not in flow %s", name, r.Flow.Name)
		}
		r.emit(s, Event{Ev: "node.start", Node: name})
		if n.Kind == KindTerminal {
			s.Node, s.Outcome = "", n.Outcome
			r.emit(s, Event{Ev: "run.end", Node: name, Why: n.Outcome})
			return s, r.save(s)
		}
		// A spent budget runs no agent again: the run ends where a spent
		// limit ends it.
		if why, spent := r.spent(s); spent && n.Kind == KindAgent {
			to := r.Flow.OnLimit
			r.emit(s, Event{Ev: "edge", Node: name, To: to, Why: why})
			s.Why = why
			if to == "" {
				s.Node, s.Outcome = "", "exhausted"
				r.emit(s, Event{Ev: "run.end", Node: name, Why: s.Outcome})
				return s, r.save(s)
			}
			s.Node = to
			if err := r.save(s); err != nil {
				return s, err
			}
			continue
		}
		if err := r.runNode(ctx, name, n, s); err != nil {
			r.emit(s, Event{Ev: "node.failed", Node: name, Why: err.Error()})
			_ = r.save(s)
			return s, err
		}
		to, why := r.next(name, s)
		r.emit(s, Event{Ev: "edge", Node: name, To: to, Why: why})
		s.Node = to
		if err := r.save(s); err != nil {
			return s, err
		}
	}
	return s, nil
}

func (r *Runner) runNode(ctx context.Context, name string, n Node, s *State) error {
	switch n.Kind {
	case KindAgent:
		roles, err := r.dispatch(name, n, s)
		if err != nil {
			return err
		}
		if lensable(n, roles) {
			return r.runLenses(ctx, name, roles, s)
		}
		for _, rl := range roles {
			if err := r.runAgent(ctx, name, rl, s); err != nil {
				return err
			}
			if s.Blocked {
				break // the person answers before the next role builds on it
			}
		}
	case KindSensor:
		res := r.Sensors.Run(ctx, n.Stages)
		s.Sensor = &res
		for _, st := range res.Failed {
			if s.SensorFailures == nil {
				s.SensorFailures = map[string]int{}
			}
			s.SensorFailures[st]++
		}
		r.weighSensors(s, n.Stages)
		r.readDiff(s, n.Stages)
	case KindPolicy:
		v, err := r.Verdicts.Read(s.Context)
		if err != nil {
			return err
		}
		// Only the verdict of this run's audit counts: one left by an earlier
		// run, or none, is never read as this audit's.
		if v == nil || v.Run != s.Run || v.Iter != s.Iter {
			return fmt.Errorf("a auditoria não gravou o veredicto desta execução (gofi verdict write %s --run %s --iter %d)", s.Context, s.Run, s.Iter)
		}
		r.judge(s, v)
		r.weighVerdict(s)
	case KindGate:
		e, err := intake.ReadElicitation(r.work(), s.Elicit)
		if err != nil || e == nil {
			s.Answered = false
			break
		}
		r.emit(s, Event{Ev: "gate.open", Node: name, Why: s.BlockedBy + ": gofi answer " + s.Run})
		answers, ok := r.Gate.Ask(ctx, Question{Run: s.Run, Node: name, Agent: s.BlockedBy, Questions: e.Questions})
		s.Answered = ok
		if ok {
			s.Answers = answers
			r.emit(s, Event{Ev: "gate.answered", Node: name})
			r.unblock(s)
		} else {
			r.emit(s, Event{Ev: "gate.unanswered", Node: name})
		}
		s.Blocked = false
	}
	return nil
}

// unblock marks the agent that stopped as done once its decisions are
// answered: what it left is now the next agent's to carry on.
func (r *Runner) unblock(s *State) {
	if s.BlockedBy == "" {
		return
	}
	agents, _ := Agents(r.Root, s.Run)
	for _, a := range agents {
		if a.ID == s.BlockedBy && a.Status == AgentBlocked {
			a.Status = AgentDone
			_ = saveAgent(r.Root, s.Run, &a)
		}
	}
	s.BlockedBy = ""
}

// dispatch is the roles an agent node runs: the one it names, or those of
// its stage whose triggers hold on the run's facts (0002, D18) — recorded,
// with why, in the run's log. A stage nobody answers stops the run: a change
// nobody can implement is not accepted by skipping it.
func (r *Runner) dispatch(name string, n Node, s *State) ([]role.Role, error) {
	if n.Role != "" {
		for _, rl := range r.Roles {
			if rl.Name == n.Role {
				return []role.Role{rl}, nil
			}
		}
		return []role.Role{{Name: n.Role}}, nil
	}
	facts := r.facts(s)
	choices := role.Dispatch(r.Roles, n.Stage, facts)
	if len(choices) == 0 && n.Optional {
		r.emit(s, Event{Ev: "skip", Node: name, Why: "nenhum papel do estágio " + n.Stage + " dispara: o nó é opcional"})
		return nil, nil
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("nenhum papel do estágio %s dispara nesta mudança:\n  %s", n.Stage, strings.Join(role.Explain(r.Roles, n.Stage, facts), "\n  "))
	}
	var out []role.Role
	for _, c := range choices {
		r.emit(s, Event{Ev: "dispatch", Node: name, To: c.Role, Why: c.Why})
		for _, rl := range r.Roles {
			if rl.Name == c.Role {
				out = append(out, rl)
			}
		}
	}
	byLayer(out, s.Plan)
	return out, nil
}

// facts are the run's facts now: the change's, given when the run started,
// and the run's own — the last sensor run, the verdict, the open findings'
// owners. Computed, never read from what a model said.
func (r *Runner) facts(s *State) role.Facts {
	f := role.Facts{}
	for k, v := range r.Facts {
		f[k] = v
	}
	if s.Sensor != nil {
		f["sensor.green"] = len(s.Sensor.Failed) == 0
		for _, st := range s.Sensor.Failed {
			f["sensor.failed("+st+")"] = true
		}
	}
	if s.Verdict != "" {
		f["verdict("+s.Verdict+")"] = true
	}
	for _, o := range s.Open {
		f["owner("+o.Owner+")"] = true
	}
	if s.Stuck {
		f["stuck"] = true
	}
	if s.Plan == nil {
		f["plan.none"] = true
	} else {
		for _, l := range r.due(s) {
			f["layer("+l+")"] = true
		}
	}
	if r.testsDue(s) {
		f["tests.due"] = true
	}
	if s.Rounds > 0 {
		f["run.reworked"] = true
	}
	if s.Diff != nil {
		for _, l := range s.Diff.Layers {
			f["diff.touches("+l+")"] = true
		}
	}
	return f
}

// runAgent runs one role's turn in a session of its own and records the
// agent: its state, its cost, whether it stopped on a decision.
func (r *Runner) runAgent(ctx context.Context, name string, rl role.Role, s *State) error {
	s.Iter++
	s.Agent = name
	s.Lenses = nil
	elicit := filepath.ToSlash(filepath.Join(RunsDir, s.Run, fmt.Sprintf("elicit-%s-%d.json", name, s.Iter)))
	prompt := r.Brief(rl, s, elicit)
	if s.Answered && s.Elicit != "" {
		if e, err := intake.ReadElicitation(r.work(), s.Elicit); err == nil && e != nil {
			prompt = intake.WithDecisions(prompt, e, s.Answers)
		}
	}
	tier := r.tierOf(rl.Name, s)
	if wide := r.wide(rl, s); wide != "" {
		r.emit(s, Event{Ev: "escalate", Node: name, To: rl.Name, Why: wide})
	}
	s.Worked = append(s.Worked, Ran{Node: name, Role: rl.Name})
	a := &AgentState{ID: AgentID(name, s.Iter), Node: name, Role: rl.Name, Tier: tier, Iter: s.Iter, Status: AgentWorking, Started: r.now()}
	_ = saveAgent(r.Root, s.Run, a)
	guard := r.guarded(rl)
	var before map[string][]byte
	if guard != nil {
		before = snapshot(r.work(), guard)
	}
	res, err := r.Agent.Run(ctx, Turn{Node: name, Role: rl.Name, Agent: a.ID, Iter: s.Iter, Prompt: prompt, ElicitFile: elicit, Tier: tier, Tools: rl.Contract.Tools})
	s.CostUSD += res.CostUSD
	a.Ended, a.CostUSD, a.Seconds = r.now(), res.CostUSD, res.Seconds
	if err != nil {
		a.Status, a.Error = AgentFailed, err.Error()
		_ = saveAgent(r.Root, s.Run, a)
		return err
	}
	if guard != nil {
		if undone := restore(r.work(), before, guard); len(undone) > 0 {
			r.emit(s, Event{Ev: "guard", Node: name, To: rl.Name, Why: "desfeito — não é deste papel: " + joinFiles(undone)})
		}
	}
	r.keepProposal(s)
	// A decision the agent wrote and gofi cannot read stops the run: read as
	// nothing, the question would be lost and the work go on without it.
	e, err := intake.ReadElicitation(r.work(), elicit)
	if err != nil {
		a.Status, a.Error = AgentFailed, err.Error()
		_ = saveAgent(r.Root, s.Run, a)
		return fmt.Errorf("%s gravou uma decisão que o gofi não lê: %w", rl.Name, err)
	}
	s.Blocked = e != nil && len(e.Questions) > 0
	switch rl.Contract.Stage {
	case "plan":
		// The plan is what the next stages dispatch on: one gofi cannot read,
		// or none, stops the run.
		p, err := readPlan(r.work(), planFile(s.Run))
		if err == nil && p == nil && !s.Blocked {
			err = fmt.Errorf("o plano não foi gravado em %s", planFile(s.Run))
		}
		if err != nil {
			a.Status, a.Error = AgentFailed, err.Error()
			_ = saveAgent(r.Root, s.Run, a)
			return fmt.Errorf("%s: %w", rl.Name, err)
		}
		if p != nil {
			s.Plan = p
			r.emit(s, Event{Ev: "plan", Node: name, Why: fmt.Sprintf("%d tarefas: %s", len(p.Tasks), strings.Join(p.layers(), ", "))})
			if r.Reach != nil {
				var files []string
				for _, t := range p.Tasks {
					files = append(files, t.Files...)
				}
				r.reach(s, r.Reach(files))
			}
		}
	case "test":
		s.Tested = true
	case "learn":
		if err := r.keepLessons(s); err != nil {
			a.Status, a.Error = AgentFailed, err.Error()
			_ = saveAgent(r.Root, s.Run, a)
			return fmt.Errorf("%s: %w", rl.Name, err)
		}
	}
	s.Answered, s.Answers = false, nil
	a.Status = AgentDone
	if s.Blocked {
		s.Elicit, s.BlockedBy, a.Status = elicit, a.ID, AgentBlocked
	}
	_ = saveAgent(r.Root, s.Run, a)
	return nil
}

var weight = map[string]int{"blocker": 8, "major": 4, "minor": 1}

// judge reads the verdict into the facts the policy's edges are decided on:
// the verdict's category, the open findings, their weight against the
// iteration before, and the findings resolved earlier that are open again.
func (r *Runner) judge(s *State, v *verdict.Verdict) {
	var open []Finding
	s.Notes = nil
	score := 0
	for _, f := range v.Findings {
		if f.Severity == "suggestion" {
			continue
		}
		found := Finding{Fingerprint: f.Fingerprint, Severity: f.Severity, Owner: f.Owner, Rule: f.Rule, File: f.File, Message: f.Message}
		// A minor of the contract — a scenario the change could also have
		// had — is the next change's to fix, not this run's: the code is
		// right, and no role of the run may change the contract. It is kept
		// for the person, and stops nothing.
		if f.Severity == "minor" && (f.Owner == "spec" || f.Owner == "pd") {
			s.Notes = append(s.Notes, found)
			continue
		}
		open = append(open, found)
		score += weight[f.Severity]
	}
	now := map[string]bool{}
	for _, f := range open {
		now[f.Fingerprint] = true
	}
	for _, f := range open {
		if !slices.ContainsFunc(s.Seen, func(x Finding) bool { return x.Fingerprint == f.Fingerprint }) {
			s.Seen = append(s.Seen, f)
		}
	}
	resolved := map[string]bool{}
	for _, fp := range s.Resolved {
		resolved[fp] = true
	}
	for _, f := range s.Open {
		if !now[f.Fingerprint] {
			resolved[f.Fingerprint] = true
		}
	}
	s.Previous, s.Score = s.Score, &score
	s.Open = open
	s.Resolved = sortedSet(resolved)
	switch {
	case v.Result == verdict.Approved && len(open) == 0:
		s.Verdict = "clean"
	case v.Result == verdict.Approved:
		s.Verdict = "caveats"
	default:
		s.Verdict = "rejected"
	}
}

// next decides the edge out of a node: the first whose condition holds, or
// on_limit when that edge's limit is spent.
func (r *Runner) next(from string, s *State) (to, why string) {
	for _, e := range r.Flow.outgoing(from) {
		ok, why := r.holds(e.When, s)
		if !ok {
			continue
		}
		key := from + ">" + e.To
		if max := r.Flow.limit(from, e.To); max > 0 && s.Taken[key] >= max {
			return r.Flow.OnLimit, fmt.Sprintf("limite de %s → %s esgotado (%d)", from, e.To, max)
		}
		if r.rework(from, e.To) {
			// The first pass is iteration 1: max_iterations passes in all.
			if max := r.Policy.MaxIterations; max > 0 && s.Rounds+1 >= max {
				return r.Flow.OnLimit, fmt.Sprintf("loop.max_iterations esgotado (%d)", max)
			}
			s.Rounds++
		}
		s.Taken[key]++
		return e.To, why
	}
	// Check guarantees a default edge; this is a flow that skipped it.
	return r.Flow.OnLimit, "nenhuma aresta se aplica"
}

// holds evaluates a condition against the facts, and says which held.
func (r *Runner) holds(w When, s *State) (bool, string) {
	var why []string
	if w.SensorFailed != nil {
		if s.Sensor == nil || len(s.Sensor.Failed) == 0 {
			return false, ""
		}
		if len(*w.SensorFailed) > 0 && !overlaps(*w.SensorFailed, s.Sensor.Failed) {
			return false, ""
		}
		why = append(why, "sensor falhou: "+strings.Join(s.Sensor.Failed, ", "))
	}
	if w.Verdict != "" {
		if s.Verdict != w.Verdict {
			return false, ""
		}
		why = append(why, "veredicto "+s.Verdict)
	}
	if len(w.Owner) > 0 {
		var owners []string
		for _, f := range s.Open {
			owners = append(owners, f.Owner)
		}
		if !overlaps(w.Owner, owners) {
			return false, ""
		}
		why = append(why, "achado do dono "+strings.Join(w.Owner, "/"))
	}
	if w.Progress != nil {
		progress := s.Score != nil && (s.Previous == nil || *s.Score < *s.Previous)
		if progress != *w.Progress {
			return false, ""
		}
		why = append(why, fmt.Sprintf("progresso %v", progress))
	}
	if w.Oscillation != nil {
		osc := oscillates(s)
		if osc != *w.Oscillation {
			return false, ""
		}
		why = append(why, fmt.Sprintf("oscilação %v", osc))
	}
	if w.Accept != nil {
		ok := r.Policy.accepts(s.Verdict)
		if ok != *w.Accept {
			return false, ""
		}
		why = append(why, fmt.Sprintf("aceite %v (veredicto %s, accept %s)", ok, s.Verdict, r.Policy.acceptName()))
	}
	if w.TestsOnly != nil {
		ok := r.testsOnly(s)
		if ok != *w.TestsOnly {
			return false, ""
		}
		why = append(why, fmt.Sprintf("só testes %v", ok))
	}
	if w.Stuck != nil {
		if s.Stuck != *w.Stuck {
			return false, ""
		}
		why = append(why, fmt.Sprintf("sem saída %v", s.Stuck))
	}
	if w.Blocked != nil {
		if s.Blocked != *w.Blocked {
			return false, ""
		}
		why = append(why, fmt.Sprintf("bloqueado %v", s.Blocked))
	}
	if w.Answered != nil {
		if s.Answered != *w.Answered {
			return false, ""
		}
		why = append(why, fmt.Sprintf("respondido %v", s.Answered))
	}
	if len(why) == 0 {
		return true, "padrão"
	}
	return true, strings.Join(why, " · ")
}

// oscillates reports whether a finding resolved before is open again.
func oscillates(s *State) bool {
	resolved := map[string]bool{}
	for _, fp := range s.Resolved {
		resolved[fp] = true
	}
	for _, f := range s.Open {
		if resolved[f.Fingerprint] {
			return true
		}
	}
	return false
}

func (r *Runner) emit(s *State, e Event) {
	r.seq++
	e.Seq = r.seq
	e.T = r.now()
	e.CostUSD, e.Run = s.CostUSD, s.Run
	line, _ := json.Marshal(e)
	p := filepath.Join(Dir(r.Root, s.Run), "events.ndjson")
	if f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.Write(append(line, '\n'))
		_ = f.Close()
	}
	if r.OnEvent != nil {
		r.OnEvent(e)
	}
}

func (r *Runner) save(s *State) error {
	s.Updated = r.now()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(Dir(r.Root, s.Run), "state.json")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (r *Runner) work() string {
	if r.Work != "" {
		return r.Work
	}
	return r.Root
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now().UTC()
}

// LoadState reads a run's state.
func LoadState(root, run string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), "state.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no run %s", run)
		}
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Schema != StateSchema {
		return nil, errors.New("run " + run + ": state of another schema")
	}
	if s.Taken == nil {
		s.Taken = map[string]int{}
	}
	return &s, nil
}

// Events reads a run's log, in order.
func Events(root, run string) ([]Event, error) {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), "events.ndjson"))
	if err != nil {
		return nil, err
	}
	var out []Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e Event
		if json.Unmarshal([]byte(line), &e) == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// AppendEvent writes one more event to a run's log, after the runtime's:
// what happens to a run past its terminal node — landing — is in the same
// log gofi logs and gofi watch read.
func AppendEvent(root, run string, e Event) Event {
	e.Seq, e.Run = countEvents(root, run)+1, run
	if e.T.IsZero() {
		e.T = time.Now().UTC()
	}
	line, _ := json.Marshal(e)
	if f, err := os.OpenFile(filepath.Join(Dir(root, run), "events.ndjson"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.Write(append(line, '\n'))
		_ = f.Close()
	}
	return e
}

func countEvents(root, run string) int {
	evs, _ := Events(root, run)
	if len(evs) == 0 {
		return 0
	}
	return evs[len(evs)-1].Seq
}

func overlaps(a, b []string) bool {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	for _, y := range b {
		if set[y] {
			return true
		}
	}
	return false
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Stream reads a run's events as gofi run --events writes them, one JSON
// line each, until the reader ends — how a conductor follows a delivery it
// started as a subprocess.
func Stream(r io.Reader) <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for sc.Scan() {
			var e Event
			if json.Unmarshal(sc.Bytes(), &e) == nil && e.Ev != "" {
				ch <- e
			}
		}
	}()
	return ch
}

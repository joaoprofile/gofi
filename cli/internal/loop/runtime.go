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
	"sort"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
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
	// Verdicts reads the audit's last verdict of a context.
	Verdicts interface {
		Read(ctx string) (*verdict.Verdict, error)
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
	Iter       int
	Prompt     string
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
}

// Finding is the part of a verdict's finding the policy keeps.
type Finding struct {
	Fingerprint string `json:"fingerprint"`
	Severity    string `json:"severity"`
	Owner       string `json:"owner"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
}

// State is a run, as the runtime leaves it after each node: enough to resume
// from the node it points at, and nothing a model wrote.
type State struct {
	Schema  string `json:"schema"`
	Run     string `json:"run"`
	Flow    string `json:"flow"`
	Change  string `json:"mudanca"`
	Context string `json:"contexto"`
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
	Blocked  bool          `json:"blocked,omitempty"`
	Answered bool          `json:"answered,omitempty"`
	// Elicit is the file the last blocked agent wrote its decisions to,
	// BlockedBy that agent, and Answers what the person answered at the gate.
	Elicit    string            `json:"elicit,omitempty"`
	BlockedBy string            `json:"blocked_by,omitempty"`
	Answers   map[string]string `json:"answers,omitempty"`
	// Worktree, Branch and Base say where a run in a worktree works and
	// lands; Landing is how that went: landed, waiting, conflict.
	Worktree string    `json:"worktree,omitempty"`
	Branch   string    `json:"branch,omitempty"`
	Base     string    `json:"base,omitempty"`
	Landing  string    `json:"landing,omitempty"`
	CostUSD  float64   `json:"cost_usd"`
	Updated  time.Time `json:"updated"`
}

// Event is one line of a run's log.
type Event struct {
	Seq  int       `json:"seq"`
	T    time.Time `json:"t"`
	Ev   string    `json:"ev"`
	Node string    `json:"node,omitempty"`
	To   string    `json:"to,omitempty"`
	Why  string    `json:"why,omitempty"`
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
	// Brief writes the prompt of an agent node from the state.
	Brief func(n Node, s *State, elicitFile string) string
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
	if setup != nil {
		setup(s)
	}
	if err := os.MkdirAll(Dir(r.Root, run), 0o755); err != nil {
		return nil, err
	}
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
		s.Iter++
		s.Agent = name
		elicit := filepath.ToSlash(filepath.Join(RunsDir, s.Run, fmt.Sprintf("elicit-%s-%d.json", name, s.Iter)))
		prompt := r.Brief(n, s, elicit)
		if s.Answered && s.Elicit != "" {
			if e, err := intake.ReadElicitation(r.work(), s.Elicit); err == nil && e != nil {
				prompt = intake.WithDecisions(prompt, e, s.Answers)
			}
		}
		a := &AgentState{ID: AgentID(name, s.Iter), Node: name, Role: n.Role, Iter: s.Iter, Status: AgentWorking, Started: r.now()}
		_ = saveAgent(r.Root, s.Run, a)
		res, err := r.Agent.Run(ctx, Turn{Node: name, Role: n.Role, Iter: s.Iter, Prompt: prompt, ElicitFile: elicit})
		s.CostUSD += res.CostUSD
		a.Ended, a.CostUSD, a.Seconds = r.now(), res.CostUSD, res.Seconds
		if err != nil {
			a.Status, a.Error = AgentFailed, err.Error()
			_ = saveAgent(r.Root, s.Run, a)
			return err
		}
		e, _ := intake.ReadElicitation(r.work(), elicit)
		s.Blocked = e != nil && len(e.Questions) > 0
		s.Answered, s.Answers = false, nil
		a.Status = AgentDone
		if s.Blocked {
			s.Elicit, s.BlockedBy, a.Status = elicit, a.ID, AgentBlocked
		}
		_ = saveAgent(r.Root, s.Run, a)
	case KindSensor:
		res := r.Sensors.Run(ctx, n.Stages)
		s.Sensor = &res
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

var weight = map[string]int{"blocker": 8, "major": 4, "minor": 1}

// judge reads the verdict into the facts the policy's edges are decided on:
// the verdict's category, the open findings, their weight against the
// iteration before, and the findings resolved earlier that are open again.
func (r *Runner) judge(s *State, v *verdict.Verdict) {
	var open []Finding
	score := 0
	for _, f := range v.Findings {
		if f.Severity == "suggestion" {
			continue
		}
		open = append(open, Finding{Fingerprint: f.Fingerprint, Severity: f.Severity, Owner: f.Owner, Rule: f.Rule, Message: f.Message})
		score += weight[f.Severity]
	}
	now := map[string]bool{}
	for _, f := range open {
		now[f.Fingerprint] = true
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
	case v.Clean():
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
	e.Seq = countEvents(root, run) + 1
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

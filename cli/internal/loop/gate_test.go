package loop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
)

func question() Question {
	return Question{Node: "decide", Agent: "implement-1", Questions: []intake.ElicitQuest{
		{ID: "q1", Text: "Arredonda?", Options: []string{"sim", "não"}, Recommended: "sim"},
	}}
}

// waitOpen waits for the gate of a run to open, as another terminal would.
func waitOpen(t *testing.T, root, run string) *Question {
	t.Helper()
	for range 200 {
		if q, ok := OpenGate(root, run); ok {
			return q
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the gate never opened")
	return nil
}

// The question goes to gate.json; an answer written by any other process is
// what the gate returns, and both files are gone after it.
func TestTheGateIsAnsweredFromAnotherTerminal(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(Dir(root, "r-1"), 0o755)
	g := &FileGate{Root: root, Run: "r-1", Timeout: time.Minute, Poll: 5 * time.Millisecond}
	go func() {
		q := waitOpen(t, root, "r-1")
		if q.Agent != "implement-1" || q.Deadline.IsZero() || q.Questions[0].ID != "q1" {
			t.Errorf("gate = %+v", q)
		}
		if err := Answer(root, "r-1", map[string]string{"q1": "não"}); err != nil {
			t.Error(err)
		}
	}()
	a, ok := g.Ask(context.Background(), question())
	if !ok || a["q1"] != "não" {
		t.Fatalf("answers %v, ok %v", a, ok)
	}
	for _, f := range []string{GateFile, AnswerFile} {
		if _, err := os.Stat(filepath.Join(Dir(root, "r-1"), f)); err == nil {
			t.Errorf("%s left behind", f)
		}
	}
}

// Unanswered by the deadline, the run goes on without an answer; with no
// wait at all, nobody is expected.
func TestTheGateGivesUpAtItsDeadline(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(Dir(root, "r-1"), 0o755)
	g := &FileGate{Root: root, Run: "r-1", Timeout: 30 * time.Millisecond, Poll: 5 * time.Millisecond}
	if _, ok := g.Ask(context.Background(), question()); ok {
		t.Error("answered with nobody there")
	}
	g.Timeout = 0
	if _, ok := g.Ask(context.Background(), question()); ok {
		t.Error("a gate that does not wait answered")
	}
}

// The terminal that runs the supervisor is one more answerer: an answer from
// elsewhere takes its question off the screen.
func TestAnAnswerFromElsewhereCancelsTheTerminalPrompt(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(Dir(root, "r-1"), 0o755)
	cancelled := make(chan bool, 1)
	g := &FileGate{Root: root, Run: "r-1", Timeout: time.Minute, Poll: 5 * time.Millisecond,
		Local: func(ctx context.Context, _ *intake.Elicitation) (map[string]string, bool) {
			<-ctx.Done()
			cancelled <- true
			return nil, false
		}}
	go func() {
		waitOpen(t, root, "r-1")
		_ = Answer(root, "r-1", map[string]string{"q1": "sim"})
	}()
	if _, ok := g.Ask(context.Background(), question()); !ok {
		t.Fatal("not answered")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Error("the terminal prompt was left up")
	}

	// Answered at the terminal, the gate does not wait for anyone else.
	g.Local = func(context.Context, *intake.Elicitation) (map[string]string, bool) {
		return map[string]string{"q1": "não"}, true
	}
	if a, ok := g.Ask(context.Background(), question()); !ok || a["q1"] != "não" {
		t.Errorf("answers %v, ok %v", a, ok)
	}
}

// An answer is checked against what the gate asked.
func TestAnAnswerIsWhatTheGateAsked(t *testing.T) {
	root := t.TempDir()
	if err := Answer(root, "r-1", nil); err == nil || !strings.Contains(err.Error(), "não espera") {
		t.Errorf("no gate: %v", err)
	}
	_ = os.MkdirAll(Dir(root, "r-1"), 0o755)
	q := question()
	q.Schema = GateSchema
	_ = writeJSON(filepath.Join(Dir(root, "r-1"), GateFile), q)
	for in, want := range map[string]string{"q9=sim": "não perguntou", "q1=talvez": "não é uma das opções"} {
		id, v, _ := strings.Cut(in, "=")
		if err := Answer(root, "r-1", map[string]string{id: v}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", in, err)
		}
	}
	if err := Answer(root, "r-1", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if err := Answer(root, "r-1", map[string]string{}); err == nil || !strings.Contains(err.Error(), "já foi respondida") {
		t.Errorf("answered twice: %v", err)
	}
}

// A run blocked at its gate is answered from another terminal and carries
// on to delivery; every agent it ran is on disk with its state (0002, D22).
func TestABlockedRunIsAnsweredFromElsewhereAndGoesOn(t *testing.T) {
	w := newWorld(t)
	w.runner.Gate = &FileGate{Root: w.root, Run: "r-1", Timeout: time.Minute, Poll: 5 * time.Millisecond}
	w.agent.steps["implement"] = []step{block(w.root), nil}
	w.agent.steps["audit"] = []step{audit(w.root, "aprovado")}
	go func() {
		waitOpen(t, w.root, "r-1")
		agents, _ := Agents(w.root, "r-1")
		if len(agents) != 1 || agents[0].Status != AgentBlocked {
			t.Errorf("while waiting: %+v", agents)
		}
		_ = Answer(w.root, "r-1", map[string]string{"q1": "não"})
	}()
	s := w.run(t)
	if s.Outcome != "accepted" || !strings.Contains(w.agent.turns[1].Prompt, "Arredonda? → não") {
		t.Fatalf("outcome %s, resumed turn:\n%s", s.Outcome, w.agent.turns[1].Prompt)
	}
	agents, _ := Agents(w.root, "r-1")
	var got []string
	for _, a := range agents {
		got = append(got, a.ID+":"+a.Status)
	}
	if strings.Join(got, " ") != "implement-1:concluido implement-2:concluido audit-3:concluido" {
		t.Errorf("agents = %v", got)
	}
	var evs []string
	all, _ := Events(w.root, "r-1")
	for _, e := range all {
		if strings.HasPrefix(e.Ev, "gate.") {
			evs = append(evs, e.Ev)
		}
	}
	if strings.Join(evs, " ") != "gate.open gate.answered" {
		t.Errorf("gate events = %v", evs)
	}
}

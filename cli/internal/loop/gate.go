package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
)

// GateSchema names the format of an open gate.
const GateSchema = "gofi-gate/v1"

// The gate is two files in the run's folder (0002, D22): the supervisor
// writes the question to GateFile and waits; whoever answers — gofi answer,
// gofi watch, the supervisor's own terminal — writes AnswerFile. The question
// leaves the supervisor's terminal, so a run is unblocked from any other.
const (
	GateFile   = "gate.json"
	AnswerFile = "answer.json"
)

// Question is what the gate asks: the decisions an agent stopped on.
type Question struct {
	Schema string `json:"schema"`
	Run    string `json:"run"`
	Node   string `json:"node"`
	// Agent is the agent that stopped, <node>-<iter>.
	Agent     string               `json:"agent"`
	Questions []intake.ElicitQuest `json:"questions"`
	Opened    time.Time            `json:"opened"`
	// Deadline is when the gate gives up and the run goes on unanswered;
	// zero waits for as long as the run lives.
	Deadline time.Time `json:"deadline,omitzero"`
}

// Elicitation is the question in the shape a conductor asks it.
func (q *Question) Elicitation() *intake.Elicitation {
	return &intake.Elicitation{Schema: intake.ElicitSchema, Questions: q.Questions}
}

// FileGate is the gate of one run: it opens GateFile and waits for an answer
// until Timeout. Local, when set, asks the person at the supervisor's
// terminal at the same time; the first answer wins and the other is
// cancelled.
type FileGate struct {
	Root, Run string
	// Timeout is how long the gate waits; zero does not wait — nobody is
	// expected, and the run goes on unanswered.
	Timeout time.Duration
	// Poll is how often the answer file is looked for.
	Poll  time.Duration
	Local func(ctx context.Context, e *intake.Elicitation) (map[string]string, bool)
	Now   func() time.Time
}

func (g *FileGate) Ask(ctx context.Context, q Question) (map[string]string, bool) {
	if g.Timeout <= 0 && g.Local == nil {
		return nil, false
	}
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	q.Schema, q.Run, q.Opened = GateSchema, g.Run, now().UTC()
	if g.Timeout > 0 {
		q.Deadline = q.Opened.Add(g.Timeout)
	}
	dir := Dir(g.Root, g.Run)
	_ = os.Remove(filepath.Join(dir, AnswerFile))
	if err := writeJSON(filepath.Join(dir, GateFile), q); err != nil {
		return nil, false
	}
	defer func() {
		_ = os.Remove(filepath.Join(dir, GateFile))
		_ = os.Remove(filepath.Join(dir, AnswerFile))
	}()

	wait, cancel := context.WithCancel(ctx)
	defer cancel()
	if g.Timeout > 0 {
		wait, cancel = context.WithTimeout(wait, g.Timeout)
		defer cancel()
	}
	type reply struct {
		answers map[string]string
		ok      bool
	}
	local := make(chan reply, 1)
	if g.Local != nil {
		go func() {
			a, ok := g.Local(wait, q.Elicitation())
			local <- reply{a, ok}
		}()
	}
	poll := g.Poll
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	tick := time.NewTicker(poll)
	defer tick.Stop()
	for {
		if a, ok := readAnswer(g.Root, g.Run); ok {
			return a, true
		}
		select {
		case r := <-local:
			// Declined at the terminal, the gate still waits for another
			// answerer while it has time; with none, nobody answers.
			if r.ok || g.Timeout <= 0 {
				return r.answers, r.ok
			}
			local = nil
		case <-wait.Done():
			return nil, false
		case <-tick.C:
		}
	}
}

// OpenGate reads the question a run is waiting on, when it is.
func OpenGate(root, run string) (*Question, bool) {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), GateFile))
	if err != nil {
		return nil, false
	}
	var q Question
	if json.Unmarshal(data, &q) != nil || q.Schema != GateSchema {
		return nil, false
	}
	return &q, true
}

// Answer answers the gate a run is waiting on. Every id must be one the gate
// asked, and a choice one of its options; a question left out is taken as
// its recommendation, and written down as assumed.
func Answer(root, run string, answers map[string]string) error {
	q, ok := OpenGate(root, run)
	if !ok {
		return fmt.Errorf("a execução %s não espera resposta", run)
	}
	if _, answered := readAnswer(root, run); answered {
		return fmt.Errorf("a execução %s já foi respondida", run)
	}
	for id, v := range answers {
		i := slices.IndexFunc(q.Questions, func(x intake.ElicitQuest) bool { return x.ID == id })
		if i < 0 {
			return fmt.Errorf("a execução %s não perguntou %q", run, id)
		}
		if opts := q.Questions[i].Options; len(opts) > 0 && !slices.Contains(opts, v) {
			return fmt.Errorf("%s: %q não é uma das opções (%v)", id, v, opts)
		}
	}
	return writeJSON(filepath.Join(Dir(root, run), AnswerFile), answers)
}

func readAnswer(root, run string) (map[string]string, bool) {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), AnswerFile))
	if err != nil {
		return nil, false
	}
	var a map[string]string
	if json.Unmarshal(data, &a) != nil {
		return nil, false
	}
	return a, true
}

// writeJSON writes v whole or not at all: a reader in another process never
// sees half a file.
func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}

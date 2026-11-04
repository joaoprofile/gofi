package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// hangingSession is an agent whose turn ends only when it is cancelled.
type hangingSession struct{ fakeSession }

func (h hangingSession) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	*h.prompts = append(*h.prompts, t.Prompt)
	ch := make(chan engine.Event, 1)
	go func() {
		<-ctx.Done()
		ch <- engine.Done{Cancelled: true}
		close(ch)
	}()
	return ch, nil
}

// The run r-20261104-181655 sat in one turn with no end in sight. A turn now
// ends at loop.turn_timeout: the run stops, resumable, and says why.
func TestATurnThatHangsIsCutByTheTurnTimeout(t *testing.T) {
	root := committedChangeProject(t, "true")
	writeFile(t, root+"/.gofi.yaml", strings.Replace(readFile(t, root+"/.gofi.yaml"), "loop:\n", "loop:\n  turn_timeout: 50ms\n", 1))
	gitIn(t, root, "commit", "--quiet", "-am", "turn timeout")
	var prompts []string
	runPorts.open = func(dir string) engine.Session { return hangingSession{fakeSession{root: dir, prompts: &prompts}} }

	out, err := execCmd(newRunCmd, "criar-pedidos")
	if err == nil || !strings.Contains(err.Error(), "50ms") {
		t.Fatalf("a hanging turn was not cut by the timeout: %v\n%s", err, out)
	}
}

// Each turn is given what is left of the run's budget, or the per-turn limit
// when lower, for the engine to stop itself there.
func TestEachTurnIsGivenItsBudget(t *testing.T) {
	root := committedChangeProject(t, "true")
	writeFile(t, root+"/.gofi.yaml", strings.Replace(readFile(t, root+"/.gofi.yaml"), "loop:\n", "loop:\n  budget_usd: 5\n  turn_budget_usd: 0.5\n", 1))
	gitIn(t, root, "commit", "--quiet", "-am", "budgets")
	var prompts []string
	var budgets []float64
	runPorts.open = func(dir string) engine.Session { return fakeSession{root: dir, prompts: &prompts} }
	runPorts.budget = func(usd float64) { budgets = append(budgets, usd) }
	t.Cleanup(func() { runPorts.budget = nil })

	if out, err := execCmd(newRunCmd, "criar-pedidos"); err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if len(budgets) == 0 {
		t.Fatal("no turn was given a budget")
	}
	for _, b := range budgets {
		if b <= 0 || b > 0.5 {
			t.Errorf("a turn was given %v, want in (0, 0.5]", b)
		}
	}
}

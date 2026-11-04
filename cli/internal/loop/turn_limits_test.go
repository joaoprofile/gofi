package loop

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A turn could spend without bound: the budget was checked only before a
// node started. Now each turn is given what is left, or the per-turn limit
// when lower, shared by the turns that run at once.
func TestATurnMaySpendOnlyWhatIsLeft(t *testing.T) {
	for _, c := range []struct {
		name         string
		budget, turn float64
		spent        float64
		n            int
		want         float64
	}{
		{"no budget, no limit", 0, 0, 3, 1, 0},
		{"what is left", 5, 0, 3, 1, 2},
		{"the per-turn limit, lower", 5, 0.5, 3, 1, 0.5},
		{"what is left, lower than the limit", 5, 4, 3, 1, 2},
		{"shared by lenses", 5, 0, 3, 4, 0.5},
		{"overspent", 5, 0, 6, 1, 0},
		{"per-turn limit alone", 0, 0.8, 9, 1, 0.8},
	} {
		got := Policy{BudgetUSD: c.budget, TurnBudgetUSD: c.turn}.turnBudget(c.spent, c.n)
		if got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// A turn that does not end is cut at the policy's timeout and says so; the
// caller's own cancel is not called a timeout.
func TestATurnPastItsTimeoutIsCutAndSaysWhy(t *testing.T) {
	p := Policy{TurnTimeout: 20 * time.Millisecond}
	tctx, why, cancel := p.turnContext(context.Background(), "gofi-ui")
	defer cancel()
	<-tctx.Done()
	if err := why(context.Canceled); !errors.Is(err, ErrTurnTimeout) {
		t.Fatalf("a turn past its timeout: %v", err)
	}

	parent, stop := context.WithCancel(context.Background())
	tctx, why, cancel = p.turnContext(parent, "gofi-ui")
	defer cancel()
	stop()
	<-tctx.Done()
	if err := why(context.Canceled); errors.Is(err, ErrTurnTimeout) {
		t.Errorf("a cancel by the caller was called a timeout: %v", err)
	}
	if _, why, _ := (Policy{}).turnContext(context.Background(), "x"); why(nil) != nil {
		t.Error("no timeout must change nothing")
	}
}

package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// The report said what a run cost and how many turns, not where: a run
// twice as dear could not be told from another by more than the price. It
// adds up the tokens, the tool calls and the time, and says them by role.
func TestTheReportSaysWhatEachRoleSpent(t *testing.T) {
	agents := []loop.AgentState{
		{Role: "gofi-eng", CostUSD: 1.5, Seconds: 300, Usage: &loop.Usage{Input: 1000, CacheRead: 120000, Output: 8000, Tools: 40}},
		{Role: "gofi-qa", CostUSD: 0.4, Seconds: 90, Usage: &loop.Usage{Input: 500, CacheRead: 30000, Output: 2000, Tools: 12}},
		{Role: "gofi-eng", CostUSD: 0.6, Seconds: 60, Usage: &loop.Usage{Input: 200, CacheRead: 20000, Output: 1000, Tools: 9}},
	}
	got := deliveryReport(&loop.State{Change: "c", Context: "x", Run: "r-1", Outcome: "accepted", CostUSD: 2.5}, agents, nil)
	for _, want := range []string{
		"| Uso | 171.700 tokens de entrada (170.000 do cache) · 11.000 de saída · 61 chamadas de ferramenta · 7m30s de agentes |",
		"| gofi-eng | 2 | 49 | 141.200 | 9.000 | 2.10 | 6m0s |",
		"| gofi-qa | 1 | 12 | 30.500 | 2.000 | 0.40 | 1m30s |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report lacks %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "| gofi-eng |") > strings.Index(got, "| gofi-qa |") {
		t.Error("the dearest role must come first")
	}
}

// usageSession is an agent that calls two tools and says what it spent.
type usageSession struct{ fakeSession }

func (u usageSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	*u.prompts = append(*u.prompts, t.Prompt)
	ch := make(chan engine.Event, 4)
	ch <- engine.Message{Blocks: []engine.Block{{Kind: engine.BlockToolUse, Name: "Read"}, {Kind: engine.BlockToolUse, Name: "Edit"}}}
	ch <- engine.Usage{Total: true, Input: 10, Output: 20, CacheRead: 300}
	ch <- engine.Done{CostUSD: 0.01, Duration: time.Second}
	close(ch)
	return ch, nil
}

func TestATurnRecordsItsTokensAndToolCalls(t *testing.T) {
	committedChangeProject(t, "true")
	var prompts []string
	runPorts.open = func(dir string) engine.Session { return usageSession{fakeSession{root: dir, prompts: &prompts}} }
	a := &engineAgent{root: t.TempDir(), run: "r-1", open: func(dir string, _ host.Tier, _ []string, _ float64) engine.Session { return runPorts.open(dir) }, out: &strings.Builder{}, quiet: true}
	res, err := a.Run(context.Background(), loop.Turn{Node: "implement", Role: "gofi-eng", Iter: 1, Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Usage == nil || *res.Usage != (loop.Usage{Input: 10, Output: 20, CacheRead: 300, Tools: 2}) {
		t.Errorf("usage = %+v", res.Usage)
	}
}

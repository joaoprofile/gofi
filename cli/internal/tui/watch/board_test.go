package watch

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// The board is the live runs alone: who works, on what, and what waits on
// the person — at a width, in a few lines.
func TestTheBoardShowsWhoIsAtWork(t *testing.T) {
	root := t.TempDir()
	release, err := loop.Acquire(root, "r-live")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	started := time.Now().Add(-75 * time.Second)
	dir := loop.Dir(root, "r-live")
	write(t, filepath.Join(dir, "state.json"), loop.State{Schema: loop.StateSchema, Run: "r-live", Flow: "delivery", Change: "criar-pedidos",
		Context: "pedidos", Node: "implement", Iter: 1, Taken: map[string]int{}, CostUSD: 1.5, Updated: time.Now()})
	write(t, filepath.Join(dir, "agents", "implement-1.json"), loop.AgentState{ID: "implement-1", Node: "implement", Role: "gofi-eng",
		Tier: "standard", Iter: 1, Status: loop.AgentWorking, Started: started})
	write(t, filepath.Join(dir, "agents", "implement-1.ndjson"),
		`{"kind":"prompt","text":"/gofi-eng"}`+"\n"+`{"kind":"tool","text":"Edit {\"file_path\":\"/tmp/wt/r-live/internal/pedido.go\"}"}`+"\n")
	// A run that is over is not on the board.
	run(t, root, "r-old", "estoque", "", "accepted", 3, agent("implement-1", "implement", "gofi-eng", 1, loop.AgentDone, 3, time.Now()))

	b := Live(root)
	if !b.Active() || b.Running != 1 || b.Working != 1 || b.Waiting != 0 || b.Cost != 1.5 {
		t.Fatalf("board = %+v", b)
	}
	lines := b.Lines(100, 8)
	text := strings.Join(lines, "\n")
	for _, want := range []string{"agentes", "pedidos", "#1 gofi-eng", "standard", "1m15s", "Edit internal/pedido.go", "US$ 1,50"} {
		if !strings.Contains(text, want) {
			t.Errorf("the board misses %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "estoque") {
		t.Errorf("a run that ended is on the board:\n%s", text)
	}
	if !strings.HasPrefix(lines[len(lines)-1], "─") {
		t.Errorf("the board should end on a rule, got %q", lines[len(lines)-1])
	}
	for _, l := range b.Lines(40, 8) {
		if n := len([]rune(l)); n > 40 {
			t.Errorf("line wider than 40 (%d): %q", n, l)
		}
	}
	if got := b.Lines(100, 3); len(got) != 3 {
		t.Errorf("limited to 3 lines, got %d: %q", len(got), got)
	}
}

func TestTheBoardIsEmptyWhenNothingRuns(t *testing.T) {
	root := t.TempDir()
	run(t, root, "r-old", "estoque", "", "accepted", 3)
	if b := Live(root); b.Active() || b.Lines(100, 8) != nil {
		t.Errorf("nothing runs, the board should be empty: %+v", b)
	}
}

func TestABlockedRunSaysItWaits(t *testing.T) {
	root, _ := blockedRun(t)
	b := Live(root)
	if b.Waiting != 1 {
		t.Fatalf("board = %+v", b)
	}
	if text := strings.Join(b.Lines(100, 8), "\n"); !strings.Contains(text, "esperando você") || !strings.Contains(text, "/watch") {
		t.Errorf("a blocked run should say it waits and where to answer:\n%s", text)
	}
}

package watch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

func write(t *testing.T, path string, v any) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.Marshal(v)
	if s, ok := v.(string); ok {
		data = []byte(s)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// blockedRun is a run whose supervisor is this test, waiting at its gate
// after one agent stopped on a decision.
func blockedRun(t *testing.T) (root string, answered <-chan map[string]string) {
	root = t.TempDir()
	release, err := loop.Acquire(root, "r-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	dir := loop.Dir(root, "r-1")
	write(t, filepath.Join(dir, "state.json"), loop.State{Schema: loop.StateSchema, Run: "r-1", Flow: "delivery", Change: "criar-pedidos",
		Context: "pedidos", Node: "decide", Iter: 1, Taken: map[string]int{}, Updated: time.Now()})
	write(t, filepath.Join(dir, "agents", "implement-1.json"), loop.AgentState{ID: "implement-1", Node: "implement", Role: "gofi-eng",
		Iter: 1, Status: loop.AgentBlocked, Started: time.Now()})
	write(t, filepath.Join(dir, "agents", "implement-1.ndjson"),
		`{"kind":"prompt","text":"/gofi-eng a\nb"}`+"\n"+`{"kind":"tool","text":"Edit pedido.go"}`+"\n"+`{"kind":"text","text":"Parei: falta decidir o arredondamento."}`+"\n")

	ch := make(chan map[string]string, 1)
	g := &loop.FileGate{Root: root, Run: "r-1", Timeout: time.Minute, Poll: 5 * time.Millisecond}
	go func() {
		a, _ := g.Ask(context.Background(), loop.Question{Node: "decide", Agent: "implement-1", Questions: []intake.ElicitQuest{
			{ID: "q1", Text: "Arredonda?", Options: []string{"sim", "não"}, Recommended: "não"}}})
		ch <- a
	}()
	for range 200 {
		if _, ok := loop.OpenGate(root, "r-1"); ok {
			return root, ch
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the gate never opened")
	return
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(key(k))
	}
	return m
}

// The tree shows the run as a tab, blocked, with its agent as a pane; the
// run in focus shows its question, the agent in focus its transcript.
func TestTheScreenShowsRunsAgentsAndWhatTheyDo(t *testing.T) {
	root, _ := blockedRun(t)
	m := New(Options{Root: root, Project: "loja"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	v := m.View()
	for _, want := range []string{"gofi watch", "r-1", "criar-pedidos", loop.Blocked, "implement-1", loop.AgentBlocked, "Arredonda?"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	v = press(m, "down").View()
	for _, want := range []string{"Edit pedido.go", "Parei: falta decidir"} {
		if !strings.Contains(v, want) {
			t.Errorf("the agent's pane lacks %q:\n%s", want, v)
		}
	}
}

// a walks the questions of the gate, recommendation first; the answer is
// the run's to carry on with.
func TestAnsweringFromTheScreenUnblocksTheRun(t *testing.T) {
	root, answered := blockedRun(t)
	m := press(New(Options{Root: root}), "a")
	if v := m.View(); !strings.Contains(v, "Arredonda?") || !strings.Contains(v, "▸ não") {
		t.Fatalf("the question is not asked, recommendation first:\n%s", v)
	}
	press(m, "down", "enter")
	select {
	case a := <-answered:
		if a["q1"] != "sim" {
			t.Errorf("the gate got %v", a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the gate never got the answer")
	}
}

// n lists the sealed changes and starts the one chosen in the background.
func TestStartingAChangeFromTheScreen(t *testing.T) {
	var started []string
	m := New(Options{Root: t.TempDir(),
		Changes: func() ([]string, error) { return []string{"a", "b"}, nil },
		Start:   func(args ...string) (string, error) { started = append(started, args...); return "r-9", nil }})
	m = press(m, "n", "down", "enter")
	if strings.Join(started, " ") != "b" || !strings.Contains(m.View(), "r-9") {
		t.Errorf("started %v\n%s", started, m.View())
	}
}

package watch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// The screen is read in Portuguese here: its sentences are what is tested.
func TestMain(m *testing.M) {
	i18n.SetLanguage(i18n.LangPT)
	os.Exit(m.Run())
}

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
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(key(k))
	}
	return m
}

// The context says how it stands and asks its question; going down, its
// phase shows what the agent did.
func TestTheScreenShowsContextsPhasesAndWhatTheyDo(t *testing.T) {
	root, _ := blockedRun(t)
	m := New(Options{Root: root, Project: "loja"})
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	v := m.View()
	for _, want := range []string{"gofi watch", "r-1", "criar-pedidos", "pedidos", "espera sua resposta", "implementação", loop.AgentBlocked, "Arredonda?"} {
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

func agent(id, node, role string, iter int, status string, cost float64, at time.Time) loop.AgentState {
	return loop.AgentState{ID: id, Node: node, Role: role, Iter: iter, Status: status, CostUSD: cost, Started: at}
}

// run writes a run nobody carries — its agents as given — into the project.
func run(t *testing.T, root, id, context, node, outcome string, cost float64, agents ...loop.AgentState) {
	t.Helper()
	dir := loop.Dir(root, id)
	write(t, filepath.Join(dir, "state.json"), loop.State{Schema: loop.StateSchema, Run: id, Flow: "delivery", Change: context,
		Context: context, Node: node, Outcome: outcome, CostUSD: cost, Taken: map[string]int{}, Updated: time.Now()})
	write(t, filepath.Join(dir, "events.ndjson"), "")
	for _, a := range agents {
		write(t, filepath.Join(dir, "agents", a.ID+".json"), a)
	}
}

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	at := time.Now()
	run(t, root, "r-1", "tenant", "", "accepted", 6.42,
		agent("implement-1", "implement", "gofi-eng", 1, loop.AgentDone, 2.5, at),
		agent("audit-2-a", "audit", "gofi-audit", 2, loop.AgentFailed, 0.1, at.Add(time.Second)),
		agent("audit-2-b", "audit", "gofi-audit-b", 2, loop.AgentFailed, 0.1, at.Add(2*time.Second)),
		agent("audit-3-a", "audit", "gofi-audit", 3, loop.AgentDone, 0.2, at.Add(3*time.Second)),
		agent("audit-3-b", "audit", "gofi-audit-b", 3, loop.AgentDone, 0.2, at.Add(4*time.Second)),
		agent("deliver-4", "deliver", "gofi-deliver", 4, loop.AgentDone, 0.01, at.Add(5*time.Second)),
		agent("deliver-5", "deliver", "gofi-doc", 5, loop.AgentDone, 0.01, at.Add(6*time.Second)))
	release, err := loop.Acquire(root, "r-2")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	run(t, root, "r-2", "user", "test", "", 5.12,
		agent("implement-1", "implement", "gofi-eng", 1, loop.AgentDone, 5.12, at),
		agent("test-2", "test", "gofi-test", 2, loop.AgentWorking, 0, at.Add(time.Second)))
	write(t, loop.Transcript(root, "r-2", "test-2"), `{"kind":"tool","text":"Write foo_test.go"}`+"\n")
	return root
}

func sized(m tea.Model) tea.Model {
	m, _ = m.Update(tea.WindowSizeMsg{Width: 170, Height: 30})
	return m
}

// What works comes first and says so in a sentence, the screen on the agent
// at work; what is over folds, and its audit is judged by its last round.
func TestContextsSayHowTheyStandAndFollowTheWork(t *testing.T) {
	v := sized(New(Options{Root: project(t), Project: "erp"})).View()
	for _, want := range []string{"segue na etapa de testes, com US$ 5,12 gastos até agora.", "foi aceita", "Write foo_test.go", "seguindo"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "auditoria ×2") {
		t.Errorf("a context that is over should be folded:\n%s", v)
	}
	if strings.Index(v, "user") > strings.Index(v, "tenant") {
		t.Errorf("the context at work should come first:\n%s", v)
	}
}

// tab goes from context to context; unfolded, a phase of several agents is
// one line, and its last round is what it says.
func TestNavigatingContextsAndPhases(t *testing.T) {
	m := press(sized(New(Options{Root: project(t)})), "tab")
	v := m.View()
	if strings.Contains(v, "seguindo") {
		t.Errorf("moving should stop following:\n%s", v)
	}
	if !strings.Contains(v, "r-1") {
		t.Fatalf("tab should reach the other context:\n%s", v)
	}
	m = press(m, "right")
	v = m.View()
	if !strings.Contains(v, "✓ auditoria ×2") {
		t.Errorf("the audit should be one phase, passed on its last round:\n%s", v)
	}
	if strings.Contains(v, "#2 gofi-audit") {
		t.Errorf("the phase's agents should stay folded:\n%s", v)
	}
	if strings.Contains(v, "entrega ×") {
		t.Errorf("two roles taking turns are one round:\n%s", v)
	}
	m = press(m, "]", "]", "right")
	if v = m.View(); !strings.Contains(v, "#2 gofi-audit") {
		t.Errorf("unfolding the phase should show its agents:\n%s", v)
	}
	m = press(m, "f")
	if v = m.View(); !strings.Contains(v, "Write foo_test.go") {
		t.Errorf("f should go back to the agent at work:\n%s", v)
	}
}

// A frame is the screen in plain text, with no keys to press.
func TestAFrameIsTheScreenInPlainText(t *testing.T) {
	f := Frame(Options{Root: project(t), Project: "erp"}, 70, 24)
	for _, want := range []string{"gofi watch", "testes", "Write foo_test.go"} {
		if !strings.Contains(f, want) {
			t.Errorf("frame lacks %q:\n%s", want, f)
		}
	}
	if strings.Contains(f, "q sair") || strings.Contains(f, "\x1b") {
		t.Errorf("a frame has no keys and no colors:\n%q", f)
	}
}

// The pane reads back through what scrolled by, and holds its place.
func TestThePaneScrolls(t *testing.T) {
	root := project(t)
	var lines strings.Builder
	for i := range 100 {
		lines.WriteString(`{"kind":"tool","text":"passo-` + strconv.Itoa(i) + `-fim"}` + "\n")
	}
	write(t, loop.Transcript(root, "r-2", "test-2"), lines.String())
	m := sized(New(Options{Root: root}))
	if v := m.View(); !strings.Contains(v, "passo-99-fim") {
		t.Fatalf("the pane should end on the last line:\n%s", v)
	}
	v := press(m, "pgup").View()
	if strings.Contains(v, "passo-99-fim") || !strings.Contains(v, "abaixo") {
		t.Errorf("pgup should read back:\n%s", v)
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

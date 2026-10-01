package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/approval"
	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// fakeSession records what reached the engine and replays a scripted turn.
type fakeSession struct {
	sent      []string
	resumed   string
	model     string
	cancelled int
	closed    bool
	script    []engine.Event
	onSend    func(prompt string)
}

func (f *fakeSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	f.sent = append(f.sent, t.Prompt)
	if f.onSend != nil {
		f.onSend(t.Prompt)
	}
	ch := make(chan engine.Event, len(f.script))
	for _, e := range f.script {
		ch <- e
	}
	close(ch)
	return ch, nil
}
func (f *fakeSession) SetModel(m string) { f.model = m }
func (f *fakeSession) Cancel()           { f.cancelled++ }
func (f *fakeSession) Close() error      { f.closed = true; return nil }

func newTestModel(t *testing.T) (model, *[]*fakeSession) {
	t.Helper()
	var sessions []*fakeSession
	m := newModel(Options{
		Root:   t.TempDir(),
		Models: []string{"model-a", "model-b"},
		Sessions: func() []engine.Saved {
			return []engine.Saved{{ID: "old-1", Title: "Payment refactor", Updated: time.Now().Add(-time.Hour)}}
		},
		NewSession: func(resumeID string) engine.Session {
			s := &fakeSession{resumed: resumeID, script: []engine.Event{
				engine.Meta{SessionID: "s1", Model: "fake-model"},
				engine.Delta{Kind: engine.DeltaText, Text: "Reading"},
				engine.Message{Blocks: []engine.Block{
					{Kind: engine.BlockText, Text: "Reading the file."},
					{Kind: engine.BlockToolUse, ID: "t1", Name: "Read", Input: json.RawMessage(`{"file_path":"main.go"}`)},
				}},
				engine.ToolResult{ToolUseID: "t1", Text: "package main\n\nfunc main() {}\n"},
				engine.Done{Duration: time.Second},
			}}
			sessions = append(sessions, s)
			return s
		},
	}, false)
	return m, &sessions
}

func typeText(m model, text string) model {
	m.input.SetValue(text)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(model)
}

// drain runs a turn to its end the way the program would: feeding every event
// and running the commands they return.
func drain(t *testing.T, m model) model {
	t.Helper()
	for i := 0; m.running && i < 50; i++ {
		msg := waitEvent(m.events)()
		next, _ := m.Update(msg)
		m = next.(model)
	}
	if m.running {
		t.Fatal("turn never ended")
	}
	return m
}

func TestTextAndSkillsReachTheEngine(t *testing.T) {
	m, sessions := newTestModel(t)
	m = drain(t, typeText(m, "explain main.go"))
	m = drain(t, typeText(m, "/gofi-status"))
	s := (*sessions)[0]
	if strings.Join(s.sent, "|") != "explain main.go|/gofi-status" {
		t.Errorf("engine got %q", s.sent)
	}
	if m.modelN != "fake-model" {
		t.Errorf("model = %q, want the one the engine reported", m.modelN)
	}
	if m.tools["t1"] != "Read" {
		t.Errorf("tool call not tracked: %v", m.tools)
	}
}

func TestOwnCommandsStayLocal(t *testing.T) {
	m, sessions := newTestModel(t)
	m = typeText(m, "/help")
	m = typeText(m, "/clear")
	if len(*sessions) != 2 || !(*sessions)[0].closed {
		t.Errorf("/clear should close the conversation and open a new one")
	}
	for _, s := range *sessions {
		if len(s.sent) != 0 {
			t.Errorf("a chat command reached the engine: %q", s.sent)
		}
	}
	m = typeText(m, "!echo hi")
	if !m.running {
		t.Fatal("a shell command should run")
	}
	if len((*sessions)[1].sent) != 0 {
		t.Error("the shell command reached the engine")
	}
}

func TestEscInterruptsAndCtrlCExitsOnlyTwice(t *testing.T) {
	m, sessions := newTestModel(t)
	m.running = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if (*sessions)[0].cancelled != 1 {
		t.Error("esc should interrupt the turn")
	}
	m.running = false

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(model)
	if cmd != nil {
		t.Error("the first ctrl+c must not exit")
	}
	if m.notice == "" {
		t.Error("the first ctrl+c should say how to exit")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("the second ctrl+c should exit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("the second ctrl+c should quit")
	}
}

func TestHistoryRecallsPreviousPrompts(t *testing.T) {
	m, _ := newTestModel(t)
	m = drain(t, typeText(m, "first"))
	m = drain(t, typeText(m, "second"))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(model)
	if m.input.Value() != "second" {
		t.Errorf("up = %q", m.input.Value())
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(model)
	if m.input.Value() != "first" {
		t.Errorf("up up = %q", m.input.Value())
	}
}

func TestToolSummary(t *testing.T) {
	cases := map[string]string{
		`{"file_path":"/p/internal/a.go"}`:           "internal/a.go",
		`{"command":"go test ./...","timeout":1000}`: "go test ./...",
		`{"pattern":"TODO","path":"/p"}`:             "TODO",
		`{"other":"value"}`:                          "value",
	}
	for in, want := range cases {
		if got := toolSummary(json.RawMessage(in), "/p", 60); got != want {
			t.Errorf("%s → %q, want %q", in, got, want)
		}
	}
}

func TestViewFitsNarrowTerminals(t *testing.T) {
	m, _ := newTestModel(t)
	m.width = 30
	m.running = true
	m.live = strings.Repeat("word ", 40)
	for _, l := range strings.Split(m.View(), "\n") {
		if w := len([]rune(l)); w > 30 {
			t.Errorf("line wider than the terminal (%d): %q", w, l)
		}
	}
}

// pendingRequest builds a request the way the approval server would, with a
// reply the test can read.
func pendingRequest(t *testing.T, s *approval.Server, tool string) (*approval.Request, <-chan approval.Decision) {
	t.Helper()
	got := make(chan *approval.Request, 1)
	s.OnRequest(func(r *approval.Request) { got <- r })
	out := make(chan approval.Decision, 1)
	go func() {
		out <- approval.Ask([]byte(`{"tool_name":"`+tool+`","tool_input":{"command":"make build"}}`), s.Addr())
	}()
	select {
	case r := <-got:
		return r, out
	case <-time.After(5 * time.Second):
		t.Fatal("request never arrived")
	}
	return nil, nil
}

func TestApprovalAnswersReachTheHook(t *testing.T) {
	s, err := approval.Listen()
	if err != nil {
		t.Skip("no local sockets:", err)
	}
	defer s.Close()
	m, _ := newTestModel(t)
	m.opts.Approvals = s

	press := func(key string) {
		t.Helper()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "esc" {
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		}
		next, _ := m.Update(msg)
		m = next.(model)
	}
	ask := func(tool string) <-chan approval.Decision {
		t.Helper()
		r, out := pendingRequest(t, s, tool)
		next, _ := m.Update(approvalMsg{r})
		m = next.(model)
		return out
	}

	out := ask("Bash")
	if len(m.asking) != 1 || !strings.Contains(m.View(), "Bash") {
		t.Fatal("the request should be shown in place of the prompt")
	}
	press("1")
	if d := <-out; !d.Allow {
		t.Error("1 should approve")
	}

	out = ask("Write")
	press("esc")
	if d := <-out; d.Allow || d.Reason == "" {
		t.Errorf("esc should deny with a reason, got %+v", d)
	}

	out = ask("Bash")
	press("2")
	if d := <-out; !d.Allow {
		t.Error("2 should approve")
	}
	out = ask("Bash")
	if len(m.asking) != 0 {
		t.Error("a tool approved for the conversation must not ask again")
	}
	if d := <-out; !d.Allow {
		t.Error("the remembered tool should be approved automatically")
	}

	out = ask("Edit")
	m.running = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(model)
	if d := <-out; d.Allow {
		t.Error("interrupting must deny what was waiting")
	}
	if len(m.asking) != 0 {
		t.Error("interrupting must clear the queue")
	}
}

func TestModelAndResumeStayLocalAndCarryOver(t *testing.T) {
	m, sessions := newTestModel(t)
	m = typeText(m, "/model")
	m = typeText(m, "/model 2")
	if (*sessions)[0].model != "model-b" || m.modelN != "model-b" {
		t.Fatalf("/model 2 should switch to model-b, session has %q", (*sessions)[0].model)
	}
	m = typeText(m, "/resume")
	if len(m.listed) != 1 {
		t.Fatalf("/resume should list the saved conversations, got %v", m.listed)
	}
	m = typeText(m, "/resume 1")
	if len(*sessions) != 2 || (*sessions)[1].resumed != "old-1" {
		t.Fatalf("/resume 1 should reopen old-1, sessions: %+v", *sessions)
	}
	if (*sessions)[1].model != "model-b" {
		t.Error("the model picked with /model should carry over to the resumed conversation")
	}
	for _, s := range *sessions {
		if len(s.sent) != 0 {
			t.Errorf("a chat command reached the engine: %q", s.sent)
		}
	}
}

func TestMarkdownRendersTheCommonConstructs(t *testing.T) {
	k := newLook(false)
	out := k.markdown("# Plan\n\nUse **gofi** and `go test`.\n\n- first item that is long enough to wrap around the line\n- second\n\n```go\nfunc main() {}\n```\n\n> a note\n\nSee [the docs](https://example.com).", 40)
	for _, want := range []string{"● Plan", "Use gofi and go test.", "• first item", "│ func main() {}", "│ a note", "See the docs."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, l := range strings.Split(out, "\n") {
		if w := len([]rune(l)); w > 40 {
			t.Errorf("line wider than 40 (%d): %q", w, l)
		}
		if strings.Contains(l, "**") || strings.Contains(l, "```") {
			t.Errorf("markup left in the output: %q", l)
		}
	}
	// the wrapped continuation of a list item sits under its text
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		if strings.Contains(l, "• first") && i+1 < len(lines) && !strings.HasPrefix(lines[i+1], "    ") {
			t.Errorf("continuation not indented under the item: %q", lines[i+1])
		}
	}
}

func TestTabCompletes(t *testing.T) {
	m, _ := newTestModel(t)
	m.skills = []string{"gofi-pd", "gofi-eng", "gofi-spec"}
	root := m.opts.Root
	for _, p := range []string{"internal/cli/chat.go", "internal/config/config.go", "README.md", ".hidden"} {
		full := filepath.Join(root, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		_ = os.WriteFile(full, nil, 0o644)
	}
	cases := []struct {
		in, want  string
		ambiguous bool
	}{
		{"/gofi-e", "/gofi-eng ", false},
		{"/gofi-", "/gofi-", true},
		{"/cl", "/clear ", false},
		{"explain @int", "explain @internal/", false},
		{"explain @internal/c", "explain @internal/c", true},
		{"explain @internal/cli/ch", "explain @internal/cli/chat.go ", false},
		{"@.hi", "@.hidden ", false},
		{"plain text", "plain text", false},
	}
	for _, c := range cases {
		got, options := m.complete(c.in)
		if got != c.want || (len(options) > 0) != c.ambiguous {
			t.Errorf("complete(%q) = %q %v, want %q ambiguous=%v", c.in, got, options, c.want, c.ambiguous)
		}
	}
	if _, options := m.complete("@"); strings.Contains(strings.Join(options, " "), ".hidden") {
		t.Error("hidden entries should only be offered when asked for")
	}
}

package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/approval"
	"github.com/joaoprofile/gofi/cli/internal/clipboard"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/tui/watch"
)

// press sends keys to the chat the way the program would.
func press(m model, keys ...string) model {
	for _, k := range keys {
		next, _ := m.Update(keyOf(k))
		m = next.(model)
	}
	return m
}

func menuValues(m model) []string {
	if m.menu == nil {
		return nil
	}
	out := make([]string, 0, len(m.menu.items))
	for _, it := range m.menu.items {
		out = append(out, it.value)
	}
	return out
}

func TestEnterSendsAndTheOtherEntersBreakTheLine(t *testing.T) {
	m, sessions := newTestModel(t)
	m = press(m, "/send one", "alt+enter", "two \\", "enter", "three")
	if got := m.input.Value(); got != "/send one\ntwo \nthree" {
		t.Fatalf("prompt = %q", got)
	}
	if len((*sessions)[0].sent) != 0 {
		t.Fatal("a line break must not send")
	}
	m = drain(t, press(m, "enter"))
	if got := (*sessions)[0].sent; len(got) != 1 || got[0] != "one\ntwo \nthree" {
		t.Errorf("engine got %q, want the three lines as one message", got)
	}
	if !m.input.Empty() {
		t.Error("the prompt should be empty after sending")
	}
}

// The menu lists the chat's commands, the project's skills with what they
// do, and what the engine said it runs — a name the chat answers shown once.
func TestTheSlashMenuListsEveryCommand(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".claude", "skills", "gofi-eng")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: gofi-eng\ndescription: Builds a context. From its spec.\n---\n# x\n"), 0o644)
	m, sessions := newTestModel(t)
	m.opts.Root = root
	m.onDisk = readProjectSkills(root)

	m = press(m, "/")
	got := strings.Join(menuValues(m), " ")
	for _, want := range []string{"/help", "/clear", "/model", "/cost", "/watch", "/find", "/gofi-eng"} {
		if !strings.Contains(got, want) {
			t.Errorf("the menu misses %s: %s", want, got)
		}
	}
	if view := strings.Join(m.menu.view(m.look, 100), "\n"); !strings.Contains(view, "/help") || !strings.Contains(m.View(), "/help") {
		t.Errorf("the menu should show under the prompt:\n%s", view)
	}

	// The engine says what it runs: its commands join the list, described.
	next, _ := m.Update(described{engine.Meta{Commands: []engine.Command{
		{Name: "compact", Description: "Free up context. By summarizing.", Hint: "<instructions>", Builtin: true},
		{Name: "clear", Description: "the engine's own clear", Builtin: true},
		{Name: "gofi-eng", Description: "Builds a context"},
		{Name: "color", Description: "terminal only", Builtin: true},
	}}})
	m = next.(model)
	got = strings.Join(menuValues(m), " ")
	if strings.Count(got, "/clear") != 1 || strings.Count(got, "/gofi-eng") != 1 || strings.Contains(got, "/color") {
		t.Errorf("a name is listed once and what means nothing here is left out: %s", got)
	}
	m = press(m, "comp")
	if v := menuValues(m); len(v) != 1 || v[0] != "/compact" || m.menu.items[0].desc != "Free up context" {
		t.Fatalf("/comp should leave /compact with its first sentence: %+v", m.menu)
	}
	// Tab only completes; enter runs it — what the engine shows after the
	// name is optional.
	m = press(m, "tab")
	if m.input.Value() != "/compact " || m.menu != nil || len((*sessions)[0].sent) != 0 {
		t.Errorf("tab completes and sends nothing: %q", m.input.Value())
	}
	m.input.Reset()
	m = drain(t, press(m, "/comp", "enter"))
	if got := (*sessions)[0].sent; len(got) != 1 || got[0] != "/compact" {
		t.Errorf("enter on an engine command runs it: %q", got)
	}
	// One of the chat's own that cannot run without its argument waits for it.
	m = press(m, "/sen", "enter")
	if m.input.Value() != "/send " || len((*sessions)[0].sent) != 1 {
		t.Errorf("enter on /send waits for the text: %q", m.input.Value())
	}

	// What starts with the word comes before what only holds it.
	m.input.Reset()
	m = press(m, "/el")
	if v := menuValues(m); len(v) < 2 || v[0] != "/help" && !strings.Contains(strings.Join(v, " "), "/help") {
		t.Errorf("/el should find /help and /model by what they hold: %v", v)
	}
	m.input.Reset()
	m = press(m, "/he", "enter")
	if len((*sessions)[0].sent) != 1 || !m.input.Empty() {
		t.Error("enter on /he should run /help here, and send nothing")
	}
	m = press(m, "/he", "esc")
	if m.menu != nil || m.input.Value() != "/he" {
		t.Error("esc closes the menu and keeps what was typed")
	}
	m = press(m, "l")
	if m.menu == nil {
		t.Error("typing again opens the menu again")
	}
}

func TestTheAtMenuCompletesPaths(t *testing.T) {
	m, _ := newTestModel(t)
	for _, p := range []string{"internal/cli/chat.go", "internal/config/config.go"} {
		full := filepath.Join(m.opts.Root, p)
		_ = os.MkdirAll(filepath.Dir(full), 0o755)
		_ = os.WriteFile(full, nil, 0o644)
	}
	m = press(m, "read", " ", "@int")
	if v := menuValues(m); len(v) != 1 || v[0] != "@internal/" {
		t.Fatalf("menu = %v", v)
	}
	m = press(m, "tab")
	if m.input.Value() != "read @internal/" || len(menuValues(m)) != 2 {
		t.Fatalf("tab goes into the folder and lists it: %q %v", m.input.Value(), menuValues(m))
	}
	m = press(m, "down", "enter")
	if m.input.Value() != "read @internal/config/" {
		t.Errorf("enter takes the path and does not send: %q", m.input.Value())
	}
}

func TestWhatAlwaysCovers(t *testing.T) {
	prefixes := map[string]string{
		"go test ./...":              "go test",
		"git push origin main":       "git push",
		"make build":                 "make build",
		"ls -la":                     "ls",
		"./run.sh --fast":            "./run.sh",
		"go test ./... && rm -rf x":  "",
		"cat a | sh":                 "",
		"echo $(whoami)":             "",
		"FOO=1 make":                 "",
		"go test ./... > out.txt":    "",
		"":                           "",
		"git -C /tmp status":         "git",
		"npm run build; echo done":   "",
		"python3 script.py `id -u`":  "",
		"docker compose up --detach": "docker compose",
	}
	for in, want := range prefixes {
		if got := commandPrefix(in); got != want {
			t.Errorf("commandPrefix(%q) = %q, want %q", in, got, want)
		}
	}

	m, _ := newTestModel(t)
	bash := func(cmd string) *approval.Request {
		in, _ := json.Marshal(map[string]string{"command": cmd})
		return &approval.Request{Tool: "Bash", Input: in}
	}
	m.always[scopeOf(bash("go test ./a")).key] = true
	for cmd, want := range map[string]bool{
		"go test ./b -run X":      true,
		"go build ./...":          false,
		"go test ./b && rm -rf /": false,
		"go testing":              false,
	} {
		if got := m.remembered(bash(cmd)); got != want {
			t.Errorf("after allowing `go test`, %q remembered = %v, want %v", cmd, got, want)
		}
	}
	if len(answers(bash("a && b"))) != 2 {
		t.Error("a line of several commands cannot be allowed for the conversation")
	}
	// Allowing edits covers every tool that edits, and no shell command.
	m.always[scopeOf(&approval.Request{Tool: "Edit"}).key] = true
	if !m.remembered(&approval.Request{Tool: "Write"}) || m.remembered(bash("rm x")) {
		t.Error("allowing edits covers Write and nothing else")
	}
}

// The dialog shows the call as it will run and is answered with the arrows;
// a "no" can carry what the agent should do instead.
func TestTheDialogShowsTheChangeAndTakesANote(t *testing.T) {
	s, err := approval.Listen()
	if err != nil {
		t.Skip("no local sockets:", err)
	}
	defer s.Close()
	m, _ := newTestModel(t)
	m.opts.Approvals = s
	got := make(chan *approval.Request, 1)
	s.OnRequest(func(r *approval.Request) { got <- r })
	out := make(chan approval.Decision, 1)
	call := `{"tool_name":"Edit","tool_input":{"file_path":"` + filepath.Join(m.opts.Root, "a.go") + `","old_string":"return 1","new_string":"return 2\n// done"}}`
	go func() { out <- approval.Ask([]byte(call), s.Addr()) }()
	next, _ := m.Update(approvalMsg{<-got})
	m = next.(model)

	view := m.View()
	for _, want := range []string{"a.go", "- return 1", "+ return 2", "+ // done", "1.", "2.", "3."} {
		if !strings.Contains(view, want) {
			t.Errorf("the dialog misses %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, m.opts.Root) {
		t.Error("the file should show relative to the project")
	}
	m.width = 34
	for _, l := range strings.Split(m.View(), "\n") {
		if n := len([]rune(l)); n > 34 {
			t.Errorf("line wider than the terminal (%d): %q", n, l)
		}
	}

	m = press(m, "down", "down", "enter")
	if !m.ask.typing || len(m.asking) != 1 {
		t.Fatal("choosing no asks what to do instead before answering")
	}
	m = press(m, "use", " ", "a", " ", "constant", "enter")
	d := <-out
	if d.Allow || !strings.Contains(d.Reason, "use a constant") {
		t.Errorf("the note should reach the agent: %+v", d)
	}
	if len(m.asking) != 0 || m.ask.typing {
		t.Error("the dialog should close once answered")
	}
}

// ctrl+v attaches the clipboard's picture: it stands in the text and goes
// with the message, as it is — no plan is made of a picture.
func TestAPastedPictureGoesWithTheMessage(t *testing.T) {
	m, sessions := newTestModel(t)
	m.opts.Clipboard = func(context.Context) (clipboard.Picture, error) {
		return clipboard.Picture{MediaType: "image/png", Data: []byte("png"), Width: 300, Height: 200}, nil
	}
	m.opts.Intake = func(string, map[string]string, bool) (*intake.Result, error) {
		t.Error("a message with a picture must not be planned")
		return nil, nil
	}
	paste := func(m model, msg tea.KeyMsg) model {
		t.Helper()
		next, cmd := m.Update(msg)
		if cmd == nil {
			t.Fatal("pasting should read the clipboard off the update loop")
		}
		next, _ = next.(model).Update(cmd())
		return next.(model)
	}
	m = press(m, "what", " ", "is", " ")
	m = paste(m, keyOf("ctrl+v"))
	m = press(m, "and", " ")
	// An empty paste is what a terminal sends for a clipboard with no text.
	m = paste(m, tea.KeyMsg{Type: tea.KeyRunes, Paste: true})
	if got := m.input.Value(); got != "what is [Image #1] and [Image #2] " || len(m.images) != 2 {
		t.Fatalf("prompt = %q, %d images", got, len(m.images))
	}
	// A picture whose mark was deleted stays behind.
	for range len("[Image #2] ") {
		m = press(m, "backspace")
	}
	m = drain(t, press(m, "enter"))
	s := (*sessions)[0]
	if len(s.sent) != 1 || s.sent[0] != "what is [Image #1] and" || s.images[0] != 1 {
		t.Errorf("engine got %q with %v pictures", s.sent, s.images)
	}
	if len(m.images) != 0 {
		t.Error("the pictures are spent once sent")
	}

	m.opts.Clipboard = func(context.Context) (clipboard.Picture, error) { return clipboard.Picture{}, clipboard.ErrNoImage }
	m = paste(m, keyOf("ctrl+v"))
	if m.notice == "" || !m.input.Empty() {
		t.Error("ctrl+v on a clipboard with no picture should say so")
	}
	m = paste(m, tea.KeyMsg{Type: tea.KeyRunes, Paste: true})
	if m.notice != "" {
		t.Error("an empty paste with no picture says nothing: nobody asked for one")
	}
}

func TestTheDroppedPathOfAnImageIsRead(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "my shot.png")
	_ = os.WriteFile(shot, []byte("x"), 0o644)
	for _, in := range []string{shot, "'" + shot + "'", strings.ReplaceAll(shot, " ", `\ `), "file://" + shot + "\n"} {
		if got, ok := pastedPath(in); !ok || got != shot {
			t.Errorf("pastedPath(%q) = %q %v", in, got, ok)
		}
	}
	for _, in := range []string{filepath.Join(dir, "gone.png"), dir, "see " + shot + "\nand more", "notes.txt"} {
		if _, ok := pastedPath(in); ok {
			t.Errorf("pastedPath(%q) should not be an image to read", in)
		}
	}
}

// The engine reports its running total: a turn cost what the total grew by,
// and the conversation what its turns did.
func TestCostAndContextOfTheConversation(t *testing.T) {
	m, _ := newTestModel(t)
	turn := func(total float64) float64 { return m.account(engine.Done{CostUSD: total, ContextWindow: 1_000_000}) }
	if a, b := turn(0.50), turn(0.80); a != 0.50 || b < 0.299 || b > 0.301 {
		t.Errorf("turns cost %.2f and %.2f, want 0.50 and 0.30", a, b)
	}
	// An engine that started again counts from zero.
	if c := turn(0.10); c != 0.10 || m.spent < 0.899 || m.spent > 0.901 {
		t.Errorf("after a restart the turn cost %.2f and the conversation %.2f", c, m.spent)
	}

	m.events = make(chan engine.Event)
	next, _ := m.onEvent(engine.Usage{Input: 10, CacheRead: 120_000, CacheWrite: 2_000, Output: 500})
	m = next.(model)
	next, _ = m.onEvent(engine.Usage{Sub: true, Input: 5, CacheRead: 900_000})
	m = next.(model)
	next, _ = m.onEvent(engine.Usage{Total: true, Input: 999_999})
	m = next.(model)
	if m.ctxTokens != 122_510 {
		t.Errorf("context = %d: a subagent's and the turn's total are not what the conversation holds", m.ctxTokens)
	}
	if got := m.meter(80); !strings.Contains(got, "ctx 123k (12%)") || !strings.Contains(got, "$0.90") || money(0.0031) != "$0.003" {
		t.Errorf("meter = %q", got)
	}
	if got := m.meter(12); got != "" && strings.Contains(got, "ctx") {
		t.Errorf("a short line keeps the model, not the rest: %q", got)
	}
	if m.contextTip() == "" {
		t.Error("past 100k the person is told what the conversation carries")
	}
	if m.contextTip() != "" {
		t.Error("and told once, not on every turn")
	}
	m.replaceSession("")
	if m.ctxTokens != 0 || m.lastCost != 0 || m.spent < 0.899 {
		t.Errorf("a new conversation holds nothing, and what was spent stays: %d %.2f %.2f", m.ctxTokens, m.lastCost, m.spent)
	}
}

// liveRun writes a run this test carries, one agent at work in it.
func liveRun(t *testing.T, root string) {
	t.Helper()
	release, err := loop.Acquire(root, "r-live")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	dir := loop.Dir(root, "r-live")
	put := func(name string, v any) {
		path := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		data, _ := json.Marshal(v)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	put("state.json", loop.State{Schema: loop.StateSchema, Run: "r-live", Flow: "delivery", Change: "criar-pedidos",
		Context: "pedidos", Node: "implement", Iter: 1, Taken: map[string]int{}, Updated: time.Now()})
	put("agents/implement-1.json", loop.AgentState{ID: "implement-1", Node: "implement", Role: "gofi-eng", Iter: 1,
		Status: loop.AgentWorking, Started: time.Now()})
}

// What is said reaches the terminal in the order it was said, one batch at a
// time. On a short terminal that is all there is.
func TestWhatIsSaidKeepsItsOrder(t *testing.T) {
	m, _ := newTestModel(t)
	update := func(msg tea.Msg) tea.Cmd {
		next, cmd := m.Update(msg)
		m = next.(model)
		return cmd
	}
	update(tea.WindowSizeMsg{Width: 80, Height: 12})
	if m.pinned || !m.greeted || !m.flushing {
		t.Fatalf("a short terminal pins nothing, and the banner goes to the scrollback: pinned %v greeted %v flushing %v", m.pinned, m.greeted, m.flushing)
	}
	update(flushed{})

	if cmd := update(sayMsg{"one\ntwo"}); cmd == nil || !m.flushing || len(m.queue) != 0 {
		t.Fatalf("the first lines should go at once: flushing %v, queue %v", m.flushing, m.queue)
	}
	update(sayMsg{"three"})
	if len(m.queue) != 1 {
		t.Fatalf("lines said while a batch is out wait their turn: %v", m.queue)
	}
	if cmd := update(flushed{}); cmd == nil || len(m.queue) != 0 || !m.flushing {
		t.Fatal("the next batch goes once the terminal took the last")
	}
	update(flushed{})
	if n := len(strings.Split(m.View(), "\n")); n > 6 {
		t.Fatalf("with nothing at work a short terminal redraws the prompt alone (%d lines):\n%s", n, m.View())
	}
	// Something at work shows over the prompt, with no room to pin it.
	liveRun(t, m.opts.Root)
	update(boardMsg{watch.Live(m.opts.Root)})
	if m.pinned || !strings.Contains(m.View(), "pedidos") {
		t.Errorf("a short terminal shows the panel without pinning it:\n%s", m.View())
	}
}

// On a terminal with room, the panel is pinned to the top and the chat draws
// the whole screen: the newest lines of the conversation are held under the
// panel and the older ones let go, in order, as they are pushed out.
func TestThePanelIsPinnedOverTheConversation(t *testing.T) {
	m, _ := newTestModel(t)
	update := func(msg tea.Msg) tea.Cmd {
		next, cmd := m.Update(msg)
		m = next.(model)
		return cmd
	}
	update(tea.WindowSizeMsg{Width: 80, Height: 30})
	if !m.pinned || m.flushing || len(m.window) == 0 {
		t.Fatalf("with room the panel is pinned and the banner is held under it: pinned %v, window %d", m.pinned, len(m.window))
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 30 || !strings.Contains(lines[0], "activity") || !strings.Contains(lines[0], "nothing running") {
		t.Fatalf("the view is the screen, the panel on top (%d lines):\n%s", len(lines), m.View())
	}
	if !strings.Contains(m.View(), "At the prompt") || !strings.Contains(lines[len(lines)-1], "enter") {
		t.Errorf("the banner shows under the panel and the prompt stays at the bottom:\n%s", m.View())
	}

	// A run starts: its agents join the panel.
	liveRun(t, m.opts.Root)
	update(boardMsg{watch.Live(m.opts.Root)})
	if view := m.View(); !strings.Contains(view, "pedidos") || !strings.Contains(view, "gofi-eng") || len(strings.Split(view, "\n")) != 30 {
		t.Fatalf("a live run shows on the panel:\n%s", view)
	}

	// The window holds what fits; what is pushed out goes to the terminal in
	// the order it was said, and nothing goes twice.
	var said, gone []string
	for i := range 40 {
		line := fmt.Sprintf("line %02d", i)
		said = append(said, line)
		update(sayMsg{line})
		gone = append(gone, m.queue...)
		if m.flushing {
			update(flushed{})
		}
	}
	room := 30 - len(m.panel(m.panelLimit())) - len(m.promptLines())
	if len(m.window) != room || m.window[room-1] != "line 39" {
		t.Fatalf("the window holds the newest %d lines, got %d ending in %q", room, len(m.window), m.window[len(m.window)-1])
	}
	view := m.View()
	if !strings.Contains(view, "line 39") || strings.Contains(view, "line 00") || len(strings.Split(view, "\n")) != 30 {
		t.Errorf("the screen shows the newest lines under the panel:\n%s", view)
	}

	// Leaving lets go of what is held, and only then quits.
	held := append([]string{}, m.window...)
	m = typeText(m, "/exit")
	if !m.leaving || m.pinned || len(m.window) != 0 {
		t.Fatalf("leaving unpins and lets the window go: leaving %v pinned %v window %d", m.leaving, m.pinned, len(m.window))
	}
	if n := len(strings.Split(m.View(), "\n")); n > 8 {
		t.Errorf("what stays on screen after leaving is the prompt, not the whole frame (%d lines)", n)
	}
	if got := strings.Join(held[len(held)-3:], "|"); got != "line 37|line 38|line 39" {
		t.Errorf("the lines held were not the newest in order: %s", got)
	}
}

// The panel follows the conversation's own turn: how long, how many tools,
// the last one, the subagents it started and what each is doing — and a
// subagent's steps stay on the panel, out of the conversation.
func TestThePanelFollowsTheTurnAndItsSubagents(t *testing.T) {
	m, _ := newTestModel(t)
	m.width, m.height = 100, 30
	if head := m.panel(8); len(head) != 2 || !strings.Contains(head[0], "nothing running") {
		t.Fatalf("with nothing at work the panel is its head and a rule: %q", head)
	}
	m.running, m.started, m.modelN = true, time.Now().Add(-75*time.Second), "model-a"
	m.events = make(chan engine.Event)
	feed := func(e engine.Event) {
		next, _ := m.onEvent(e)
		m = next.(model)
	}
	feed(engine.Message{Blocks: []engine.Block{
		{Kind: engine.BlockToolUse, ID: "t1", Name: "Read", Input: json.RawMessage(`{"file_path":"main.go"}`)},
		{Kind: engine.BlockToolUse, ID: "a1", Name: "Task", Input: json.RawMessage(`{"subagent_type":"Explore","description":"map the handlers","prompt":"…"}`)},
	}})
	feed(engine.Message{Parent: "a1", Blocks: []engine.Block{
		{Kind: engine.BlockText, Text: "looking"},
		{Kind: engine.BlockToolUse, ID: "s1", Name: "Grep", Input: json.RawMessage(`{"pattern":"func New"}`)},
	}})
	feed(engine.ToolResult{ToolUseID: "s1", Parent: "a1", Text: "a.go:1"})
	feed(engine.Delta{Sub: true, Text: "the subagent's answer"})
	if m.live != "" {
		t.Errorf("a subagent's text is not the conversation's: %q", m.live)
	}
	if m.act.tools != 2 || len(m.act.subs) != 1 || m.act.subs[0].steps != 1 {
		t.Fatalf("activity = %+v", m.act)
	}
	text := strings.Join(m.panel(8), "\n")
	for _, want := range []string{"1m15s", "model-a", "2 tools", "Explore · map the handlers", "1 steps", "Grep func New"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel misses %q:\n%s", want, text)
		}
	}
	// Its result ends it: one less at work, one more done.
	feed(engine.ToolResult{ToolUseID: "a1", Text: "found 3 handlers"})
	if text = strings.Join(m.panel(8), "\n"); strings.Contains(text, "Explore · map the handlers") || !strings.Contains(text, "1 subagent(s) done") {
		t.Errorf("a subagent that finished leaves the panel:\n%s", text)
	}
	for _, l := range m.panel(8) {
		if n := len([]rune(l)); n > 99 {
			t.Errorf("panel line wider than the terminal (%d): %q", n, l)
		}
	}
	if got := m.panel(3); len(got) != 3 {
		t.Errorf("limited to 3 lines, got %d: %q", len(got), got)
	}
}

// steerSession is a session that takes messages during a turn, and whose
// turn stays open until the test ends it.
type steerSession struct {
	fakeSession
	steered []string
	events  chan engine.Event
}

func (s *steerSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	s.sent = append(s.sent, t.Prompt)
	s.events = make(chan engine.Event, 8)
	return s.events, nil
}

func (s *steerSession) Steer(t engine.Turn) (string, bool) {
	if s.events == nil {
		return "", false
	}
	s.steered = append(s.steered, t.Prompt)
	return fmt.Sprintf("id-%d", len(s.steered)), true
}

// While a turn runs, a message typed goes to the engine for the model's next
// step: it waits over the prompt and joins the conversation when the model
// takes it. What only reads the chat is answered at once, and a command
// waits for the turn.
func TestAMessageTypedWhileTheTurnRuns(t *testing.T) {
	s := &steerSession{}
	m := newModel(Options{Root: t.TempDir(), NewSession: func(string) engine.Session { return s }}, false)
	m.width, m.height = 90, 30
	m = typeText(m, "/send refactor the parser")
	if !m.running || len(s.sent) != 1 {
		t.Fatal("the turn should be running")
	}

	m = typeText(m, "keep the public names")
	if len(s.steered) != 1 || s.steered[0] != "keep the public names" || len(s.sent) != 1 {
		t.Fatalf("typed during a turn, a message goes to the turn: steered %q sent %q", s.steered, s.sent)
	}
	if len(m.steered) != 1 || !m.input.Empty() || !strings.Contains(m.View(), "keep the public names") {
		t.Fatalf("it waits over the prompt until the model takes it:\n%s", m.View())
	}

	// A command waits for the turn and stays in the prompt; /cost answers.
	m = typeText(m, "/clear")
	if len(s.steered) != 1 || m.input.Value() != "/clear" || m.notice == "" || s.closed {
		t.Errorf("a command typed during a turn waits for it: prompt %q, notice %q", m.input.Value(), m.notice)
	}
	m.input.Reset()
	m = typeText(m, "/cost")
	if !m.running || !m.input.Empty() || len(s.steered) != 1 {
		t.Error("/cost is answered at once and the turn goes on")
	}

	// The model takes it: it leaves the prompt's side for the conversation.
	next, _ := m.onEvent(engine.Delivered{ID: "id-1"})
	m = next.(model)
	if len(m.steered) != 0 || strings.Contains(strings.Join(m.promptLines(), "\n"), "keep the public names") {
		t.Error("once delivered, the message no longer waits")
	}

	// One the engine never took comes back to the prompt.
	m = typeText(m, "and add tests")
	close(s.events)
	next, _ = m.Update(turnEnded{})
	m = next.(model)
	if m.running || len(m.steered) != 0 || m.input.Value() != "and add tests" || m.notice == "" {
		t.Errorf("an undelivered message goes back to the prompt: %q, notice %q", m.input.Value(), m.notice)
	}
}

// A dialog that comes up while the person types must not take their keys for
// an answer: in its first moment only a no counts, and with a message half
// written a letter never does.
func TestADialogDoesNotTakeWhatWasTypedForThePrompt(t *testing.T) {
	approvalGuard = time.Hour
	defer func() { approvalGuard = 0 }()
	m, _ := newTestModel(t)
	answered := make(chan approval.Decision, 4)
	request := func() *approval.Request {
		s, err := approval.Listen()
		if err != nil {
			t.Skip("no local sockets:", err)
		}
		t.Cleanup(func() { s.Close() })
		got := make(chan *approval.Request, 1)
		s.OnRequest(func(r *approval.Request) { got <- r })
		go func() {
			answered <- approval.Ask([]byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf build"}}`), s.Addr())
		}()
		return <-got
	}
	m = press(m, "s", "i", "m")
	next, _ := m.Update(approvalMsg{request()})
	m = next.(model)
	m = press(m, "s", "y", "1", "enter", "2", "a")
	if len(m.asking) != 1 {
		t.Fatal("a dialog that just came up must not be answered by keys on their way to the prompt")
	}
	select {
	case d := <-answered:
		t.Fatalf("the call was answered: %+v", d)
	default:
	}

	// Past its first moment, a letter is still the message's, a digit is an answer.
	m.ask.since = time.Now().Add(-2 * time.Hour)
	m = press(m, "s", "y")
	if len(m.asking) != 1 {
		t.Fatal("with a message half written, a letter does not answer")
	}
	m = press(m, "1")
	if d := <-answered; !d.Allow || len(m.asking) != 0 {
		t.Errorf("a digit answers: %+v", d)
	}

	// A no always counts, at once.
	next, _ = m.Update(approvalMsg{request()})
	m = next.(model)
	m = press(m, "esc")
	if d := <-answered; d.Allow || len(m.asking) != 0 {
		t.Errorf("esc refuses even in the dialog's first moment: %+v", d)
	}
}

// A plan being conducted shows on the panel as it stands: what ran, the
// phase at work, what is still to come.
func TestThePanelShowsThePlanAsItStands(t *testing.T) {
	m, _, _ := planModel(t, nil, nil)
	m.width, m.height = 100, 30
	m.plan = &plan{res: &intake.Result{Plan: []intake.Phase{{Role: "gofi-pd"}, {Role: "gofi-spec"}, {Role: "gofi-eng"}}}, next: 2}
	m.conducting, m.running, m.started = true, true, time.Now()
	m.events = make(chan engine.Event)
	text := strings.Join(m.panel(8), "\n")
	for _, want := range []string{"plan 2/3", "✓ gofi-pd", "● gofi-spec", "○ gofi-eng", "/gofi-spec"} {
		if !strings.Contains(text, want) {
			t.Errorf("the panel misses %q:\n%s", want, text)
		}
	}
	// Stopped for review between two phases: the last one ran, none is at work.
	m.conducting, m.running, m.events = false, false, nil
	if text = strings.Join(m.panel(8), "\n"); !strings.Contains(text, "✓ gofi-spec") || strings.Contains(text, "● gofi-spec") {
		t.Errorf("a phase that ended is done, not at work:\n%s", text)
	}
}

// The banner leaves the screen whole — a card cut in half is a broken card —
// and the rows a terminal draws over at the top stay empty, so the panel is
// never under them. A new size never clears the screen: a terminal that
// keeps what is cleared would file every frame away in its scrollback.
func TestTheBannerLeavesWholeAndTheTopRowsStayFree(t *testing.T) {
	m, _ := newTestModel(t)
	m.opts.Inset = 1
	update := func(msg tea.Msg) tea.Cmd {
		next, cmd := m.Update(msg)
		m = next.(model)
		return cmd
	}
	update(tea.WindowSizeMsg{Width: 100, Height: 30})
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 30 || lines[0] != "" || !strings.Contains(lines[1], "activity") {
		t.Fatalf("the first row is the terminal's, the panel comes under it:\n%s", m.View())
	}
	banner := m.head
	if banner < 10 || banner != len(m.window) {
		t.Fatalf("the banner is %d of the window's %d lines", banner, len(m.window))
	}

	// Lines fill the room under the banner; it stays whole while they fit.
	room := 30 - 1 - len(m.panel(m.panelLimit())) - len(m.promptLines())
	for i := 0; len(m.window) < room; i++ {
		update(sayMsg{fmt.Sprintf("line %02d", i)})
	}
	if m.head != banner || len(m.queue) != 0 || m.flushing {
		t.Fatalf("while everything fits nothing leaves: head %d, queue %d", m.head, len(m.queue))
	}
	// One line too many: the banner goes, all of it, and nothing else.
	update(sayMsg{"one more"})
	if m.head != 0 || len(m.window) != room-banner+1 || m.window[0] != "line 00" || strings.Contains(m.View(), "At the prompt") {
		t.Fatalf("the banner should leave whole: head %d, window %d starting at %q", m.head, len(m.window), m.window[0])
	}
	if view := strings.Split(m.View(), "\n"); len(view) != 30 || view[0] != "" {
		t.Errorf("the screen keeps its height and its free row (%d lines)", len(view))
	}

	if _, cmd := m.update(tea.WindowSizeMsg{Width: 90, Height: 28}); cmd != nil {
		t.Error("a new size is only drawn over the old frame: it asks nothing of the terminal")
	}
	update(tea.WindowSizeMsg{Width: 90, Height: 28})
	if n := len(strings.Split(m.View(), "\n")); n != 28 {
		t.Errorf("after a resize the view is the new screen, got %d lines", n)
	}
}

// /audit reads what the conversation read: the tokens by kind, the calls and
// what they brought, what the numbers say — and the documents that cannot be
// read cheaply, each with a fix that /audit <n> hands the agent. Listing
// costs nothing; only the fix is a request.
func TestAuditListsWhatWasReadAndAsksForAFix(t *testing.T) {
	m, sessions := newTestModel(t)
	root := m.opts.Root
	spec := filepath.Join(root, "specs", "billing", "spec.md")
	_ = os.MkdirAll(filepath.Dir(spec), 0o755)
	_ = os.WriteFile(spec, []byte("# Billing\n"+strings.Repeat("a line\n", 200)), 0o644)

	m.events = make(chan engine.Event)
	feed := func(e engine.Event) {
		next, _ := m.onEvent(e)
		m = next.(model)
	}
	feed(engine.Usage{Input: 120, Output: 80, CacheRead: 9000, CacheWrite: 800})
	feed(engine.Usage{Total: true, Input: 999999})
	feed(engine.Message{Blocks: []engine.Block{{Kind: engine.BlockToolUse, ID: "r1", Name: "Read", Input: json.RawMessage(`{"file_path":"specs/billing/spec.md"}`)}}})
	feed(engine.ToolResult{ToolUseID: "r1", Text: strings.Repeat("x", 12000)})
	feed(engine.Message{Parent: "a1", Blocks: []engine.Block{{Kind: engine.BlockToolUse, ID: "g1", Name: "Grep", Input: json.RawMessage(`{"pattern":"Invoice","path":"internal"}`)}}})
	feed(engine.ToolResult{ToolUseID: "g1", Parent: "a1", Text: strings.Repeat("y", 400)})
	feed(engine.Done{})
	m.events = nil

	block, fixes := m.auditBlock()
	for _, want := range []string{
		"tokens: 120 new · 9k read from the cache · 800 written to the cache · 80 of output",
		"reads: 2 calls, ~3.1k tokens — 0 through the graph · 1 in the code · 1 in documents",
		"Read specs/billing/spec.md ~3k",
		"specs/billing/spec.md read whole",
		"91% of the input came from the cache",
		"1. specs/billing/spec.md (201 lines) has no front matter",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("/audit misses %q in:\n%s", want, block)
		}
	}
	if len(fixes) < 1 {
		t.Fatalf("a document with no front matter is a fix to offer: %d", len(fixes))
	}

	m = typeText(m, "/audit")
	if len((*sessions)[0].sent) != 0 || len(m.fixes) != len(fixes) {
		t.Fatal("listing asks nothing of the agent")
	}
	m = typeText(m, "/audit 9")
	if len((*sessions)[0].sent) != 0 {
		t.Error("a fix that is not in the list is not sent")
	}
	m = drain(t, typeText(m, "/audit 1"))
	sent := (*sessions)[0].sent
	if len(sent) != 1 || !strings.Contains(sent[0], "`specs/billing/spec.md`") || !strings.Contains(sent[0], "Do not change the document's content") {
		t.Errorf("/audit 1 should hand the agent the fix: %q", sent)
	}

	// A new conversation starts a new account.
	m = typeText(m, "/clear")
	if block, _ := m.auditBlock(); !strings.Contains(block, "no read recorded") || len(m.fixes) != 0 {
		t.Errorf("after /clear the account is empty:\n%s", block)
	}
}

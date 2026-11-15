package claude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

func TestTranslate(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		terminal bool
		check    func(t *testing.T, events []engine.Event)
	}{
		{
			name: "init carries the session and the skills",
			line: `{"type":"system","subtype":"init","session_id":"abc","model":"claude-opus","slash_commands":["gofi-pd","gofi-eng"]}`,
			check: func(t *testing.T, ev []engine.Event) {
				m := ev[0].(engine.Meta)
				if m.SessionID != "abc" || m.Model != "claude-opus" || len(m.SlashCommands) != 2 {
					t.Errorf("meta = %+v", m)
				}
			},
		},
		{
			name: "text delta streams",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hel"}}}`,
			check: func(t *testing.T, ev []engine.Event) {
				if d := ev[0].(engine.Delta); d.Kind != engine.DeltaText || d.Text != "Hel" {
					t.Errorf("delta = %+v", d)
				}
			},
		},
		{
			name: "assistant message keeps text and tool calls, then usage",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"ok"},{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"a.go"}}],"usage":{"input_tokens":3,"output_tokens":5}}}`,
			check: func(t *testing.T, ev []engine.Event) {
				m := ev[0].(engine.Message)
				if len(m.Blocks) != 2 || m.Blocks[1].Name != "Read" || !strings.Contains(string(m.Blocks[1].Input), "a.go") {
					t.Errorf("message = %+v", m)
				}
				if u := ev[1].(engine.Usage); u.Input != 3 || u.Output != 5 || u.Total {
					t.Errorf("usage = %+v", u)
				}
			},
		},
		{
			name: "tool result as a list of blocks",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","is_error":true,"content":[{"type":"text","text":"no such file"}]}]}}`,
			check: func(t *testing.T, ev []engine.Event) {
				if r := ev[0].(engine.ToolResult); r.ToolUseID != "t1" || !r.IsError || r.Text != "no such file" {
					t.Errorf("result = %+v", r)
				}
			},
		},
		{
			name:     "result ends the turn",
			line:     `{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01,"duration_ms":1500,"usage":{"output_tokens":9}}`,
			terminal: true,
			check: func(t *testing.T, ev []engine.Event) {
				d := ev[len(ev)-1].(engine.Done)
				if d.IsError || d.Duration != 1500*time.Millisecond || d.CostUSD != 0.01 {
					t.Errorf("done = %+v", d)
				}
			},
		},
		{
			name:  "unknown lines are dropped",
			line:  `{"type":"rate_limit_event"}`,
			check: func(t *testing.T, ev []engine.Event) {},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, terminal := translate([]byte(c.line))
			if terminal != c.terminal {
				t.Fatalf("terminal = %v", terminal)
			}
			c.check(t, ev)
		})
	}
}

// fakeClaude writes a stand-in for the claude binary: it records its
// arguments, and answers every stdin line the way the real one does — or,
// with slow, never answers, so a turn can be cancelled.
func fakeClaude(t *testing.T, slow bool) (exe, argsFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake engine is a shell script")
	}
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	answer := `echo '{"type":"system","subtype":"init","session_id":"sess-1","model":"fake"}'
  echo '{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"pong"}}}'
  echo '{"type":"assistant","message":{"content":[{"type":"text","text":"pong"}]}}'
  echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}'`
	if slow {
		answer = `sleep 30`
	}
	script := "#!/bin/sh\necho \"$@\" >> " + argsFile + "\nwhile read line; do\n  " + answer + "\ndone\n"
	exe = filepath.Join(dir, "claude")
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, argsFile
}

func collect(t *testing.T, ch <-chan engine.Event) []engine.Event {
	t.Helper()
	var out []engine.Event
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, e)
		case <-timeout:
			t.Fatal("turn never ended")
		}
	}
}

func TestSessionRunsTurnsOnOneProcess(t *testing.T) {
	exe, argsFile := fakeClaude(t, false)
	s := New(Options{Executable: exe, Dir: t.TempDir(), Model: "m1", PermissionMode: "acceptEdits"})
	defer s.Close()

	for i := 0; i < 2; i++ {
		ch, err := s.Send(context.Background(), engine.Turn{Prompt: "ping"})
		if err != nil {
			t.Fatal(err)
		}
		ev := collect(t, ch)
		if _, ok := ev[len(ev)-1].(engine.Done); !ok {
			t.Fatalf("turn %d ended with %#v", i, ev[len(ev)-1])
		}
	}
	if s.SessionID() != "sess-1" {
		t.Errorf("session id = %q", s.SessionID())
	}
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--print"); n != 1 {
		t.Errorf("engine started %d times for two turns, want once", n)
	}
	for _, want := range []string{"--model m1", "--permission-mode acceptEdits", "--disallowed-tools AskUserQuestion"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args miss %q: %s", want, args)
		}
	}
}

// A cancelled turn ends as cancelled, not as a crash, and the next turn
// continues the same conversation on a fresh process.
func TestCancelThenResume(t *testing.T) {
	exe, argsFile := fakeClaude(t, true)
	s := New(Options{Executable: exe, Dir: t.TempDir(), ResumeID: "sess-9"})
	defer s.Close()

	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "long"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	s.Cancel()
	ev := collect(t, ch)
	if d, ok := ev[len(ev)-1].(engine.Done); !ok || !d.Cancelled {
		t.Fatalf("cancelled turn ended with %#v", ev[len(ev)-1])
	}

	ch, err = s.Send(context.Background(), engine.Turn{Prompt: "again"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	s.Cancel()
	collect(t, ch)
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--resume sess-9"); n != 2 {
		t.Errorf("both processes should resume sess-9, got %d:\n%s", n, args)
	}
}

func TestMissingEngineIsAFailureWithAHint(t *testing.T) {
	s := New(Options{Executable: "gofi-no-such-engine", Dir: t.TempDir()})
	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	ev := collect(t, ch)
	f, ok := ev[0].(engine.Failure)
	if !ok || f.Hint == "" {
		t.Fatalf("got %#v", ev)
	}
}

func TestListSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := "/work/my.project"
	store := filepath.Join(home, ".claude", "projects", "-work-my-project")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(id, body string, age time.Duration) {
		p := filepath.Join(store, id+".jsonl")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		_ = os.Chtimes(p, when, when)
	}
	write("aaaaaaaa-1111", `{"type":"user","message":{"content":[{"type":"text","text":"<ide_opened_file>x</ide_opened_file>"},{"type":"text","text":"fix the login bug\nplease"}]}}`+"\n", time.Hour)
	write("bbbbbbbb-2222", `{"type":"user","message":{"content":"first"}}`+"\n"+`{"type":"ai-title","aiTitle":"Payment refactor"}`+"\n", time.Minute)
	write("not-a-session!", `{"type":"user"}`, 0)
	write("cccccccc-3333", `{"type":"queue-operation"}`+"\n", 0)

	got := ListSessions(project, 10)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].ID != "bbbbbbbb-2222" || got[0].Title != "Payment refactor" {
		t.Errorf("newest = %+v, want the titled one", got[0])
	}
	if got[1].Title != "fix the login bug" {
		t.Errorf("untitled one should use its opening message, got %q", got[1].Title)
	}
}

func TestSetModelRestartsOnTheNewModel(t *testing.T) {
	exe, argsFile := fakeClaude(t, false)
	s := New(Options{Executable: exe, Dir: t.TempDir(), Model: "m1"})
	defer s.Close()
	for _, model := range []string{"", "m2"} {
		if model != "" {
			s.SetModel(model)
		}
		ch, err := s.Send(context.Background(), engine.Turn{Prompt: "ping"})
		if err != nil {
			t.Fatal(err)
		}
		collect(t, ch)
	}
	args, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(args), "--model m2 --resume sess-1") {
		t.Errorf("the second process should resume on m2:\n%s", args)
	}
}

// A model the engine does not know is priced at its list price; one it knows
// keeps the engine's figure. Measured: Sonnet 5.5 on Claude Code 2.1.283 was
// reported at Opus prices, 0.8366 for a run whose list price is 0.4517.
func TestResultCostOfAModelTheEngineDoesNotKnow(t *testing.T) {
	raw := `{"type":"result","subtype":"success","total_cost_usd":0.8384406,"duration_ms":64006,
		"usage":{"input_tokens":12,"output_tokens":8006,"cache_read_input_tokens":333498,"cache_creation_input_tokens":76215,
			"cache_creation":{"ephemeral_1h_input_tokens":76215,"ephemeral_5m_input_tokens":0}},
		"modelUsage":{
			"claude-haiku-4-5-20251001":{"inputTokens":1763,"outputTokens":18,"costUSD":0.001853,"costBasis":"list"},
			"claude-sonnet-5-5":{"inputTokens":12,"outputTokens":8006,"cacheReadInputTokens":333498,"cacheCreationInputTokens":76215,"costUSD":0.8365876,"costBasis":"unknown"}}}`
	events, _ := translate([]byte(strings.Join(strings.Fields(raw), " ")))
	var done engine.Done
	for _, e := range events {
		if d, ok := e.(engine.Done); ok {
			done = d
		}
	}
	if want := 0.4517 + 0.001853; done.CostUSD < want-0.001 || done.CostUSD > want+0.001 {
		t.Errorf("cost = %.4f, want %.4f", done.CostUSD, want)
	}
}

// A warmed engine starts before the first turn and writes nothing to it; what
// it says before the turn — its init — reaches that turn, and the turn runs on
// the same process.
func TestWarmStartsTheEngineAheadOfTheFirstTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake engine is a shell script")
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$@\" >> " + argsFile + "\n" +
		`echo '{"type":"system","subtype":"init","session_id":"sess-w","model":"fake-warm"}'` + "\n" +
		"while read line; do\n" +
		`  echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}'` + "\n" +
		"done\n"
	exe := filepath.Join(dir, "claude")
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s := New(Options{Executable: exe, Dir: t.TempDir()})
	defer s.Close()
	s.Warm()
	for deadline := time.Now().Add(5 * time.Second); s.SessionID() == ""; {
		if time.Now().After(deadline) {
			t.Fatal("the warmed engine never said its init")
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.Warm() // idempotent: the engine is up
	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "oi"})
	if err != nil {
		t.Fatal(err)
	}
	ev := collect(t, ch)
	if m, ok := ev[0].(engine.Meta); !ok || m.Model != "fake-warm" {
		t.Errorf("first event = %#v, want the init", ev[0])
	}
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--print"); n != 1 {
		t.Errorf("engine started %d times, want once", n)
	}
}

// A turn of the loop runs on the project alone, with the MCP servers it is
// given and its own system prompt; the chat's session keeps the person's.
func TestAnIsolatedSessionRunsOnTheProjectAlone(t *testing.T) {
	loop := strings.Join((&Session{opts: Options{Isolated: true, SystemPrompt: "turn of the loop", MCPConfig: `{"mcpServers":{}}`}}).args(), " ")
	for _, want := range []string{"--setting-sources project", "--strict-mcp-config", "--exclude-dynamic-system-prompt-sections", `--mcp-config {"mcpServers":{}}`, "--append-system-prompt turn of the loop"} {
		if !strings.Contains(loop, want) {
			t.Errorf("isolated args lack %q: %s", want, loop)
		}
	}
	chat := strings.Join((&Session{}).args(), " ")
	if strings.Contains(chat, "--setting-sources") || strings.Contains(chat, "--strict-mcp-config") || !strings.Contains(chat, systemPrompt) {
		t.Errorf("chat args: %s", chat)
	}
}

// A session given a budget passes it to the engine, which stops itself there;
// one without a budget passes nothing.
func TestABudgetIsPassedAsMaxBudgetUSD(t *testing.T) {
	if a := strings.Join((&Session{opts: Options{MaxBudgetUSD: 1.5}}).args(), " "); !strings.Contains(a, "--max-budget-usd 1.50") {
		t.Errorf("args = %s", a)
	}
	if a := strings.Join((&Session{}).args(), " "); strings.Contains(a, "--max-budget-usd") {
		t.Errorf("args without a budget = %s", a)
	}
}

// Tools narrows what the session has at all; left nil, the engine's defaults
// stand, and an empty list is no tools rather than all of them.
func TestToolsArePassedOnlyWhenNarrowed(t *testing.T) {
	if a := strings.Join((&Session{opts: Options{Tools: []string{"Read", "Edit"}}}).args(), " "); !strings.Contains(a, "--tools Read,Edit") {
		t.Errorf("args = %s", a)
	}
	if a := (&Session{}).args(); slices.Contains(a, "--tools") {
		t.Errorf("args without a narrowing = %v", a)
	}
	a := (&Session{opts: Options{Tools: []string{}}}).args()
	if i := slices.Index(a, "--tools"); i < 0 || a[i+1] != "" {
		t.Errorf("an empty list must pass no tools: %v", a)
	}
}

func TestACacheLifetimeReachesTheEngine(t *testing.T) {
	if e := (&Session{}).env(); e != nil {
		t.Errorf("no lifetime asked, env = %d entries", len(e))
	}
	if e := (&Session{opts: Options{CacheTTL: "5m"}}).env(); !slices.Contains(e, "CLAUDE_CODE_PROMPT_CACHE_TTL=5m") {
		t.Errorf("env lacks the lifetime")
	}
}

// What a run's sessions open with goes at the end of the system prompt; a
// text that would not fit an argument is refused, for the prompt to carry.
func TestSharedWordsGoToTheSystemPrompt(t *testing.T) {
	s := &Session{opts: Options{SystemPrompt: "turn of the loop"}}
	if !s.Share("  the change  ") {
		t.Fatal("refused")
	}
	a := s.args()
	if i := slices.Index(a, "--append-system-prompt"); i < 0 || a[i+1] != "turn of the loop\n\nthe change" {
		t.Errorf("args = %v", a)
	}
	if (&Session{}).Share(strings.Repeat("x", maxShared)) {
		t.Error("a text past the argument's limit must be refused")
	}
}

// protocolClaude is a stand-in that speaks the control protocol: it says what
// it offers when asked, holds a turn that says "long" until it is told to
// stop, and answers every other one.
func protocolClaude(t *testing.T) (exe, argsFile, stdinFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake engine is a shell script")
	}
	dir := t.TempDir()
	argsFile, stdinFile = filepath.Join(dir, "args"), filepath.Join(dir, "stdin")
	waiting := filepath.Join(dir, "waiting")
	script := `#!/bin/sh
echo "$@" >> ` + argsFile + `
while read line; do
  echo "$line" >> ` + stdinFile + `
  case "$line" in
    *initialize*) echo '{"type":"control_response","response":{"subtype":"success","request_id":"describe","response":{"commands":[{"name":"compact","description":"Free up context","argumentHint":"<instructions>","builtin":true},{"name":"gofi-eng","description":"Builds a context","aliases":["eng"]}]}}}' ;;
    *interrupt*) echo '{"type":"result","terminal_reason":"aborted_streaming","is_error":true,"duration_ms":5}'
       if [ -f ` + waiting + ` ]; then
         echo '{"type":"command_lifecycle","command_uuid":"'"$(cat ` + waiting + `)"'","state":"started"}'
         sleep 1
         echo '{"type":"assistant","message":{"content":[{"type":"text","text":"took it after the stop"}]}}'
         echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}'
       fi ;;
    *steer-now*) id=$(echo "$line" | sed -n 's/.*"uuid":"\([^"]*\)".*/\1/p')
       echo '{"type":"command_lifecycle","command_uuid":"'"$id"'","state":"queued"}'
       echo '{"type":"command_lifecycle","command_uuid":"'"$id"'","state":"started"}'
       echo '{"type":"assistant","message":{"content":[{"type":"text","text":"took it in the turn"}]}}'
       echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}' ;;
    *steer-late*) id=$(echo "$line" | sed -n 's/.*"uuid":"\([^"]*\)".*/\1/p')
       echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}'
       echo '{"type":"command_lifecycle","command_uuid":"'"$id"'","state":"started"}'
       echo '{"type":"assistant","message":{"content":[{"type":"text","text":"took it as a turn"}]}}'
       echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10}' ;;
    *steer-wait*) echo "$line" | sed -n 's/.*"uuid":"\([^"]*\)".*/\1/p' > ` + waiting + ` ;;
    *long*) echo '{"type":"system","subtype":"init","session_id":"sess-1","model":"fake"}' ;;
    *) echo '{"type":"assistant","message":{"content":[{"type":"text","text":"pong"}]}}'
       echo '{"type":"result","subtype":"success","is_error":false,"duration_ms":10,"total_cost_usd":0.5,"modelUsage":{"big":{"inputTokens":900,"contextWindow":1000000,"costUSD":0.4},"small":{"inputTokens":10,"contextWindow":200000,"costUSD":0.1}}}' ;;
  esac
done
`
	exe = filepath.Join(dir, "claude")
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, argsFile, stdinFile
}

// With Interrupt, a cancelled turn ends as cancelled and the engine stays:
// the next turn runs on the same process, with nothing to resume.
func TestInterruptKeepsTheEngine(t *testing.T) {
	exe, argsFile, _ := protocolClaude(t)
	s := New(Options{Executable: exe, Dir: t.TempDir(), Interrupt: true})
	defer s.Close()

	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "long"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	s.Cancel()
	ev := collect(t, ch)
	if d, ok := ev[len(ev)-1].(engine.Done); !ok || !d.Cancelled || d.IsError {
		t.Fatalf("interrupted turn ended with %#v", ev[len(ev)-1])
	}

	ch, err = s.Send(context.Background(), engine.Turn{Prompt: "ping"})
	if err != nil {
		t.Fatal(err)
	}
	ev = collect(t, ch)
	d, ok := ev[len(ev)-1].(engine.Done)
	if !ok || d.Cancelled || d.IsError {
		t.Fatalf("the turn after an interrupt ended with %#v", ev[len(ev)-1])
	}
	if d.ContextWindow != 1000000 {
		t.Errorf("context window = %d, want the one of the model that read the most", d.ContextWindow)
	}
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--print"); n != 1 {
		t.Errorf("engine started %d times, want once: an interrupt must not end it", n)
	}
}

// The engine says what it offers before any turn, and no turn is sent to ask.
func TestDescribeListsCommandsWithoutATurn(t *testing.T) {
	exe, _, stdinFile := protocolClaude(t)
	s := New(Options{Executable: exe, Dir: t.TempDir(), StablePrompt: true})
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	meta, ok := s.Describe(ctx)
	if !ok || len(meta.Commands) != 2 {
		t.Fatalf("describe = %+v, %v", meta, ok)
	}
	if c := meta.Commands[0]; c.Name != "compact" || c.Hint != "<instructions>" || !c.Builtin {
		t.Errorf("first command = %+v", c)
	}
	if c := meta.Commands[1]; c.Builtin || len(c.Aliases) != 1 {
		t.Errorf("second command = %+v", c)
	}
	// Asked again, it answers from what it heard.
	if meta, ok = s.Describe(ctx); !ok || len(meta.Commands) != 2 {
		t.Errorf("second describe = %+v, %v", meta, ok)
	}
	sent, _ := os.ReadFile(stdinFile)
	if strings.Count(string(sent), "initialize") != 1 || strings.Contains(string(sent), `"type":"user"`) {
		t.Errorf("describing must ask the engine once and send no turn:\n%s", sent)
	}
	if got := strings.Join(s.args(), " "); !strings.Contains(got, "--exclude-dynamic-system-prompt-sections") || strings.Contains(got, "--setting-sources") {
		t.Errorf("a stable prompt keeps the person's settings: %s", got)
	}
}

// A turn's images go ahead of its text, as the engine reads them.
func TestImagesGoAheadOfTheText(t *testing.T) {
	exe, _, stdinFile := protocolClaude(t)
	s := New(Options{Executable: exe, Dir: t.TempDir()})
	defer s.Close()
	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "what is this", Images: []engine.Image{{MediaType: "image/png", Data: []byte("png")}}})
	if err != nil {
		t.Fatal(err)
	}
	collect(t, ch)
	sent, _ := os.ReadFile(stdinFile)
	var msg struct {
		Message struct {
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				Source struct {
					MediaType string `json:"media_type"`
					Data      string `json:"data"`
				} `json:"source"`
			} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(sent))), &msg); err != nil {
		t.Fatalf("%v: %s", err, sent)
	}
	c := msg.Message.Content
	if len(c) != 2 || c[0].Type != "image" || c[0].Source.MediaType != "image/png" || c[0].Source.Data != "cG5n" || c[1].Text != "what is this" {
		t.Errorf("content = %+v", c)
	}
}

// kinds names the events of a turn, in order, for a test to read.
func kinds(events []engine.Event) string {
	var out []string
	for _, e := range events {
		switch v := e.(type) {
		case engine.Meta:
			out = append(out, "meta")
		case engine.Delivered:
			out = append(out, "delivered")
		case engine.Message:
			out = append(out, "message")
		case engine.Done:
			if v.Cancelled {
				out = append(out, "cancelled")
			} else {
				out = append(out, "done")
			}
		case engine.Failure:
			out = append(out, "failure")
		}
	}
	return strings.Join(out, " ")
}

// A message sent during a turn is the engine's to place: handed to the model
// at its next step, the turn goes on with it and ends once; with no step
// left, the engine runs it as a turn of its own and this one ends after it.
func TestAMessageSentDuringATurn(t *testing.T) {
	exe, argsFile, stdinFile := protocolClaude(t)
	s := New(Options{Executable: exe, Dir: t.TempDir(), Interrupt: true})
	defer s.Close()

	if _, ok := s.Steer(engine.Turn{Prompt: "steer-now"}); ok {
		t.Fatal("with no turn in flight there is nothing to steer")
	}

	for _, c := range []struct{ text, want string }{
		{"steer-now", "meta delivered message done"},
		{"steer-late", "meta done delivered message done"},
	} {
		ch, err := s.Send(context.Background(), engine.Turn{Prompt: "long"})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(150 * time.Millisecond)
		id, ok := s.Steer(engine.Turn{Prompt: c.text})
		if !ok || len(id) != 36 {
			t.Fatalf("steer(%s) = %q, %v", c.text, id, ok)
		}
		ev := collect(t, ch)
		if got := kinds(ev); got != c.want {
			t.Errorf("%s: events %q, want %q", c.text, got, c.want)
		}
		for _, e := range ev {
			if d, ok := e.(engine.Delivered); ok && d.ID != id {
				t.Errorf("delivered %q, want the id Steer gave: %q", d.ID, id)
			}
		}
	}
	sent, _ := os.ReadFile(stdinFile)
	if n := strings.Count(string(sent), `"uuid"`); n != 2 {
		t.Errorf("only the messages sent during a turn carry an id, got %d:\n%s", n, sent)
	}
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--print"); n != 1 {
		t.Errorf("engine started %d times, want once", n)
	}
}

// Stopping a turn with a message waiting stops that turn only: the engine
// goes on to the message, and the wait for a stop that did not come must not
// end the engine under it.
func TestStoppingATurnLeavesTheMessageThatWaited(t *testing.T) {
	exe, argsFile, _ := protocolClaude(t)
	grace := interruptGrace
	interruptGrace = 200 * time.Millisecond
	defer func() { interruptGrace = grace }()
	s := New(Options{Executable: exe, Dir: t.TempDir(), Interrupt: true})
	defer s.Close()

	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "long"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if _, ok := s.Steer(engine.Turn{Prompt: "steer-wait"}); !ok {
		t.Fatal("steer refused")
	}
	time.Sleep(150 * time.Millisecond)
	s.Cancel()
	if got := kinds(collect(t, ch)); got != "meta cancelled delivered message done" {
		t.Errorf("events %q", got)
	}
	args, _ := os.ReadFile(argsFile)
	if n := strings.Count(string(args), "--print"); n != 1 {
		t.Errorf("engine started %d times: the message that waited ran on an engine that was ended", n)
	}
}

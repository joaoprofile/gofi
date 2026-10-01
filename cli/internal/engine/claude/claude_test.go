package claude

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofi-labs/gofi/cli/internal/engine"
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

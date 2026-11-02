//go:build !windows

package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// fake is a codex that records its arguments and prints a turn's events.
func fake(t *testing.T, events string) (exe, argsFile string) {
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	exe = filepath.Join(dir, "codex")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + argsFile + "\ncat <<'EOF'\n" + events + "\nEOF\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, argsFile
}

func turn(t *testing.T, s engine.Session, prompt string) []engine.Event {
	ch, err := s.Send(context.Background(), engine.Turn{Prompt: prompt})
	if err != nil {
		t.Fatal(err)
	}
	var out []engine.Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

const completed = `{"type":"thread.started","thread_id":"th-1"}
{"type":"turn.started"}
{"type":"item.started","item":{"id":"i1","type":"command_execution","command":"bash -lc 'go test ./...'","status":"in_progress"}}
{"type":"item.completed","item":{"id":"i2","type":"file_change","changes":[{"path":"pedido.go","kind":"update"}],"status":"completed"}}
{"type":"item.completed","item":{"id":"i3","type":"agent_message","text":"feito"}}
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":4,"output_tokens":3}}`

// A turn's events become the engine's: the commands and edits as tool
// calls, the answer as text, the usage; the next turn resumes the thread.
func TestATurnIsReadAndTheNextResumesItsThread(t *testing.T) {
	exe, argsFile := fake(t, completed)
	s := New(Options{Executable: exe, Dir: t.TempDir(), Model: "gpt-5.3-codex"})
	evs := turn(t, s, "implemente")
	var tools, text []string
	var done *engine.Done
	for _, e := range evs {
		switch e := e.(type) {
		case engine.Message:
			for _, b := range e.Blocks {
				if b.Kind == engine.BlockToolUse {
					tools = append(tools, b.Name)
				} else if b.Kind == engine.BlockText {
					text = append(text, b.Text)
				}
			}
		case engine.Done:
			done = &e
		}
	}
	if strings.Join(tools, " ") != "shell file_change" || strings.Join(text, "") != "feito" || done == nil || done.IsError {
		t.Fatalf("tools %v, text %v, done %+v", tools, text, done)
	}
	turn(t, s, "de novo")
	args, _ := os.ReadFile(argsFile)
	lines := strings.Split(strings.TrimSpace(string(args)), "\n")
	if !strings.HasPrefix(lines[0], "exec --json --sandbox workspace-write --skip-git-repo-check --model gpt-5.3-codex implemente") {
		t.Errorf("first turn: %s", lines[0])
	}
	if !strings.Contains(lines[1], "resume th-1 de novo") {
		t.Errorf("second turn does not resume: %s", lines[1])
	}
}

// A failed turn ends as an error with the engine's reason.
func TestAFailedTurnSaysWhy(t *testing.T) {
	exe, _ := fake(t, `{"type":"thread.started","thread_id":"th-1"}
{"type":"turn.failed","error":{"message":"quota exceeded"}}`)
	evs := turn(t, New(Options{Executable: exe}), "x")
	d, ok := evs[len(evs)-1].(engine.Done)
	if !ok || !d.IsError || d.Err != "quota exceeded" {
		t.Errorf("last event %+v", evs[len(evs)-1])
	}
}

// Not installed: a failure that says how to install it.
func TestAMissingCodexSaysHowToInstallIt(t *testing.T) {
	evs := turn(t, New(Options{Executable: "codex-que-nao-existe"}), "x")
	f, ok := evs[0].(engine.Failure)
	if !ok || !strings.Contains(f.Hint, "Codex CLI") {
		t.Errorf("events %+v", evs)
	}
}

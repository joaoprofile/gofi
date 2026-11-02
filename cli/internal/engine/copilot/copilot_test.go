//go:build !windows

package copilot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

func fake(t *testing.T, body string) (exe, argsFile string) {
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	exe = filepath.Join(dir, "copilot")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + argsFile + "\n" + body + "\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, argsFile
}

func turn(t *testing.T, s engine.Session) []engine.Event {
	ch, err := s.Send(context.Background(), engine.Turn{Prompt: "implemente"})
	if err != nil {
		t.Fatal(err)
	}
	var out []engine.Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

// The response is the turn's text; the CLI runs headless, every tool
// allowed, never asking.
func TestTheResponseIsTheTurnsText(t *testing.T) {
	exe, argsFile := fake(t, "echo 'linha 1'; echo 'linha 2'")
	evs := turn(t, New(Options{Executable: exe, Model: "gpt-5.3-codex"}))
	m, ok := evs[0].(engine.Message)
	if !ok || m.Blocks[0].Text != "linha 1\nlinha 2" {
		t.Fatalf("events %+v", evs)
	}
	if d, ok := evs[1].(engine.Done); !ok || d.IsError {
		t.Errorf("done %+v", evs[1])
	}
	args, _ := os.ReadFile(argsFile)
	if strings.TrimSpace(string(args)) != "-p implemente --allow-all-tools --no-ask-user --silent --model gpt-5.3-codex" {
		t.Errorf("args: %s", args)
	}
}

// A CLI that exits with an error ends the turn as one, with its stderr.
func TestAFailingCLIEndsTheTurnAsAnError(t *testing.T) {
	exe, _ := fake(t, "echo 'not signed in' >&2; exit 1")
	evs := turn(t, New(Options{Executable: exe}))
	d, ok := evs[len(evs)-1].(engine.Done)
	if !ok || !d.IsError || d.Err != "not signed in" {
		t.Errorf("events %+v", evs)
	}
}

// Cancel ends the process and the turn is reported cancelled.
func TestCancelEndsTheTurn(t *testing.T) {
	exe, _ := fake(t, "sleep 30")
	s := New(Options{Executable: exe})
	ch, _ := s.Send(context.Background(), engine.Turn{Prompt: "x"})
	time.AfterFunc(50*time.Millisecond, s.Cancel)
	var last engine.Event
	done := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				if d, ok := last.(engine.Done); !ok || !d.Cancelled {
					t.Errorf("last %+v", last)
				}
				return
			}
			last = e
		case <-done:
			t.Fatal("the turn never ended")
		}
	}
}

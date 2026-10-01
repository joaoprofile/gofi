package claude

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// TestLive talks to the real Claude Code. It costs a request, so it runs only
// when asked: GOFI_LIVE=1 go test ./internal/engine/claude -run Live -v
func TestLive(t *testing.T) {
	if os.Getenv("GOFI_LIVE") == "" {
		t.Skip("set GOFI_LIVE=1 to run against the real engine")
	}
	s := New(Options{Dir: t.TempDir(), Model: "haiku"})
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ch, err := s.Send(ctx, engine.Turn{Prompt: "Reply with exactly one word: pong"})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	var sawMeta, sawDelta bool
	var last engine.Event
	for e := range ch {
		switch v := e.(type) {
		case engine.Meta:
			sawMeta = v.SessionID != ""
		case engine.Delta:
			sawDelta = true
			text.WriteString(v.Text)
		}
		last = e
	}
	t.Logf("answer %q, last event %#v", text.String(), last)
	if d, ok := last.(engine.Done); !ok || d.IsError {
		t.Fatalf("turn ended with %#v", last)
	}
	if !sawMeta || !sawDelta || !strings.Contains(strings.ToLower(text.String()), "pong") {
		t.Errorf("meta=%v delta=%v text=%q", sawMeta, sawDelta, text.String())
	}
}

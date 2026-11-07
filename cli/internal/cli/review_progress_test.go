package cli

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// slowSession reads for a while before it ends: no tool, no text.
type slowSession struct {
	fakeSession
	wait time.Duration
}

func (s slowSession) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	*s.prompts = append(*s.prompts, t.Prompt)
	ch := make(chan engine.Event, 1)
	go func() {
		defer close(ch)
		select {
		case <-time.After(s.wait):
			ch <- engine.Done{CostUSD: 0.01, Duration: s.wait}
		case <-ctx.Done():
			ch <- engine.Done{Cancelled: true}
		}
	}()
	return ch, nil
}

func stillLine() string {
	return strings.TrimSpace(strings.TrimSuffix(i18n.T("turn.still", ""), "%!s(MISSING)"))
}

// A turn that reads at length says it is still going; one that is quiet
// (events mode) or has no heartbeat says nothing of its own.
func TestATurnWithoutEventsSaysItIsStillGoing(t *testing.T) {
	var prompts []string
	open := func(string, host.Tier, []string, float64) engine.Session {
		return slowSession{fakeSession{prompts: &prompts}, 120 * time.Millisecond}
	}
	var out strings.Builder
	a := &engineAgent{root: t.TempDir(), run: "r-1", open: open, out: &out, heartbeat: 20 * time.Millisecond}
	if _, err := a.Run(context.Background(), loop.Turn{Node: "review", Iter: 1, Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), stillLine()) {
		t.Errorf("no heartbeat:\n%s", out.String())
	}
	out.Reset()
	a.heartbeat = 0
	if _, err := a.Run(context.Background(), loop.Turn{Node: "review", Iter: 1, Prompt: "p"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), stillLine()) {
		t.Errorf("a heartbeat with none asked for:\n%s", out.String())
	}
}

// A review is bounded by loop.turn_timeout: a reviewer that does not end is
// cut, and the error says where what it did is.
func TestAReviewThatDoesNotEndIsCut(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	installRoles(t, root)
	change(t, root, "a", "")
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  turn_timeout: 50ms\n")
	var prompts []string
	runPorts.open = func(dir string) engine.Session {
		return slowSession{fakeSession{root: dir, prompts: &prompts}, time.Minute}
	}
	t.Cleanup(func() { runPorts.open = nil })
	done := make(chan error, 1)
	go func() {
		_, err := execCmd(newReviewCmd, "a")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "turn_timeout") || !strings.Contains(err.Error(), ".ndjson") {
			t.Errorf("err = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the review was not cut")
	}
}

// The chat's review hands on what the review does as it happens, and keeps
// only the findings for the stop.
func TestTheChatReviewShowsItsProgressAsItGoes(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	installRoles(t, root)
	change(t, root, "a", "")
	var prompts []string
	runPorts.open = func(dir string) engine.Session { return fakeSession{root: dir, prompts: &prompts} }
	t.Cleanup(func() { runPorts.open = nil })
	cfg, _, err := loadProjectConfig()
	if err != nil {
		t.Fatal(err)
	}
	var progress []string
	lines, blocking, err := chatReview(root, cfg)(context.Background(), "a", func(l string) { progress = append(progress, l) })
	if err != nil || blocking != 0 {
		t.Fatalf("blocking %d, err %v", blocking, err)
	}
	started := strings.TrimSuffix(i18n.T("review.started", "a", "gofi-spec-review"), "…")
	if len(progress) == 0 || !strings.Contains(strings.Join(progress, "\n"), started) {
		t.Errorf("progress = %q", progress)
	}
	if strings.Contains(strings.Join(lines, "\n"), started) {
		t.Errorf("the stop repeats the progress: %q", lines)
	}
}

func TestALineWriterHandsOnWholeLines(t *testing.T) {
	var got []string
	w := &lineWriter{emit: func(l string) { got = append(got, l) }}
	_, _ = w.Write([]byte("  um\n  do"))
	_, _ = w.Write([]byte("is\n\ntrês"))
	w.Flush()
	if strings.Join(got, "|") != "um|dois|três" {
		t.Errorf("got %q", got)
	}
}

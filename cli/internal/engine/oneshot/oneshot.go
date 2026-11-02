// Package oneshot runs an engine whose CLI takes one prompt per process —
// Codex's exec, Copilot's -p — as an engine.Session. Each turn is a process;
// the conversation goes on by the id the engine reported, when it reports
// one. An engine package says only how to call its CLI and how to read what
// it prints.
package oneshot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// Dialect is how one engine's CLI is called and read.
type Dialect interface {
	// Args are the arguments of a turn; resume is the conversation to go on
	// with, empty for the first turn.
	Args(prompt, resume string) []string
	// Decoder reads one turn's output.
	Decoder() Decoder
	// Missing is the hint when the executable is not on PATH.
	Missing() string
}

// Decoder turns a turn's output, line by line, into events.
type Decoder interface {
	// Line reads one line of stdout.
	Line(line []byte, emit func(engine.Event))
	// End is called once stdout closed and the process exited: it emits the
	// turn's terminal event, Done or Failure. failed is the exit error, and
	// stderr the end of what the process wrote there.
	End(emit func(engine.Event), failed error, stderr string, took time.Duration)
	// Resume is the conversation id the turn reported, if any.
	Resume() string
}

// Session is one conversation over a one-prompt-per-process CLI.
type Session struct {
	Executable, Dir string
	Dialect         Dialect

	mu        sync.Mutex
	resume    string
	cmd       *exec.Cmd
	cancelled bool
	closed    bool
}

// Send runs one turn as a process and streams its events.
func (s *Session) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("session closed")
	}
	if s.cmd != nil {
		s.mu.Unlock()
		return nil, errors.New("a turn is already in flight")
	}
	cmd := exec.Command(s.Executable, s.Dialect.Args(t.Prompt, s.resume)...)
	cmd.Dir = s.Dir
	ownGroup(cmd)
	stderr := &tail{max: 8 * 1024}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	events := make(chan engine.Event, 64)
	if err := cmd.Start(); err != nil {
		s.mu.Unlock()
		events <- s.startFailure(err)
		close(events)
		return events, nil
	}
	s.cmd, s.cancelled = cmd, false
	s.mu.Unlock()

	stop := context.AfterFunc(ctx, s.Cancel)
	go func() {
		defer close(events)
		defer stop()
		start := time.Now()
		dec := s.Dialect.Decoder()
		emit := func(e engine.Event) { events <- e }
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			dec.Line(sc.Bytes(), emit)
		}
		failed := cmd.Wait()
		s.mu.Lock()
		cancelled := s.cancelled
		s.cmd = nil
		if id := dec.Resume(); id != "" {
			s.resume = id
		}
		s.mu.Unlock()
		if cancelled {
			emit(engine.Done{Cancelled: true, Duration: time.Since(start)})
			return
		}
		dec.End(emit, failed, strings.TrimSpace(stderr.String()), time.Since(start))
	}()
	return events, nil
}

// Cancel ends the turn in flight and what it started; the next turn goes on
// with the conversation. Idempotent.
func (s *Session) Cancel() {
	s.mu.Lock()
	cmd := s.cmd
	if cmd != nil {
		s.cancelled = true
	}
	s.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = signalGroup(cmd, syscall.SIGTERM)
	time.AfterFunc(2*time.Second, func() { _ = signalGroup(cmd, syscall.SIGKILL) })
}

// Close ends the session, and the turn in flight with it.
func (s *Session) Close() error {
	s.Cancel()
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *Session) startFailure(err error) engine.Failure {
	if errors.Is(err, exec.ErrNotFound) {
		return engine.Failure{Message: fmt.Sprintf("`%s` was not found on PATH", s.Executable), Hint: s.Dialect.Missing()}
	}
	return engine.Failure{Message: fmt.Sprintf("could not start `%s`: %v", s.Executable, err)}
}

// tail keeps the last max bytes written to it: all that is ever shown of a
// failing engine's stderr is its end.
type tail struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, b...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	return len(b), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

var _ engine.Session = (*Session)(nil)

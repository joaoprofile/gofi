// Package claude runs the conversation through the Claude Code CLI.
//
// Going through the CLI rather than the raw API is what makes this a gofi chat
// instead of a generic one: the process runs in the project root and inherits
// its .claude/, so every skill the project installed — /gofi-pd, /gofi-eng,
// /gofi-qa — is a slash command the user can type, and the agent reads the
// project's memory, knowledge and specs with no wiring on our side.
//
// The process is kept alive across turns and fed over stdin. Starting one per
// turn is simpler but measurably worse: the CLI spends seconds on startup and
// discovery before the model sees a byte, and a resumed session replays its
// history on top of that.
package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

// DefaultExecutable is the Claude Code binary looked up on PATH.
const DefaultExecutable = "claude"

// Tools the engine must not use from here: they need a terminal of their own,
// and in a headless session they error mid-turn instead of asking.
var interactiveTools = []string{"AskUserQuestion"}

// systemPrompt tells the engine what it cannot infer: it is driven by a chat,
// so a question for the user is plain text that ends the turn.
const systemPrompt = "You are answering inside the gofi interactive terminal, not in an interactive Claude Code session. " +
	"When you need a decision from the user, write the question as text and end your turn — " +
	"the answer arrives in the next message. Never try to open an interactive prompt."

// Options configure a session.
type Options struct {
	Executable     string // DefaultExecutable when empty
	Dir            string // the project root the engine runs in
	Model          string // passed as --model when set
	ResumeID       string // a conversation to continue instead of starting one
	PermissionMode string // passed as --permission-mode unless empty or "default"
	Settings       string // extra settings JSON, passed as --settings (e.g. an approval hook)
	// AllowedTools are permission rules the session runs with, passed as
	// --allowedTools: what a phase gofi conducts may do without asking.
	AllowedTools []string
}

// Session is a conversation held by one long-lived claude process, restarted
// with --resume when a cancel ends it.
type Session struct {
	opts Options

	mu        sync.Mutex
	proc      *process
	sessionID string
	turn      *turn
	early     []engine.Event // what a warmed engine said before the first turn
	closed    bool
}

// process is one running engine. Only its reader goroutine delivers events
// and closes turn channels, so a cancel racing an answer cannot double-close.
type process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *tail
}

type turn struct {
	events    chan engine.Event
	finished  chan struct{}
	cancelled bool
}

// New prepares a session. The engine starts on the first Send.
func New(opts Options) *Session {
	if opts.Executable == "" {
		opts.Executable = DefaultExecutable
	}
	return &Session{opts: opts, sessionID: opts.ResumeID}
}

// SessionID is the engine's id for this conversation, known once it started.
func (s *Session) SessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionID
}

// ErrBusy is returned by Send while another turn is in flight.
var ErrBusy = errors.New("a turn is already running")

// Send implements engine.Session.
func (s *Session) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, errors.New("session closed")
	}
	if s.turn != nil {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	tr := &turn{events: make(chan engine.Event, 64), finished: make(chan struct{})}

	if s.proc == nil {
		p, err := s.start()
		if err != nil {
			s.mu.Unlock()
			go func() {
				tr.events <- startFailure(s.opts.Executable, err)
				close(tr.events)
			}()
			return tr.events, nil
		}
		s.proc = p
	}
	s.turn = tr
	p := s.proc
	for _, e := range s.early {
		tr.events <- e
	}
	s.early = nil
	s.mu.Unlock()

	msg, _ := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": t.Prompt}}},
	})
	if _, err := p.stdin.Write(append(msg, '\n')); err != nil {
		// The process died between turns; its reader reports the exit on this
		// turn's channel.
		s.kill(p)
	}

	go func() {
		select {
		case <-ctx.Done():
			s.Cancel()
		case <-tr.finished:
		}
	}()
	return tr.events, nil
}

// Cancel implements engine.Session. The engine has no interrupt on this
// protocol, so the process goes, and the next turn resumes the conversation
// by id — a cold start, but only for the turn after a cancel.
func (s *Session) Cancel() {
	s.mu.Lock()
	tr, p := s.turn, s.proc
	if tr == nil || p == nil {
		s.mu.Unlock()
		return
	}
	tr.cancelled = true
	s.mu.Unlock()
	s.kill(p)
}

// Close implements engine.Session.
func (s *Session) Close() error {
	s.mu.Lock()
	s.closed = true
	p := s.proc
	s.mu.Unlock()
	if p != nil {
		s.kill(p)
	}
	return nil
}

// args are the flags of a persistent, streaming session.
func (s *Session) args() []string {
	a := []string{
		"--print",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--append-system-prompt", systemPrompt,
	}
	if mode := strings.TrimSpace(s.opts.PermissionMode); mode != "" && mode != "default" {
		a = append(a, "--permission-mode", mode)
	}
	if len(s.opts.AllowedTools) > 0 {
		a = append(a, "--allowedTools", strings.Join(s.opts.AllowedTools, ","))
	}
	if st := strings.TrimSpace(s.opts.Settings); st != "" {
		a = append(a, "--settings", st)
	}
	if m := strings.TrimSpace(s.opts.Model); m != "" {
		a = append(a, "--model", m)
	}
	// After a cancel the process that held the thread is gone; resuming by id
	// is how the next turn continues it rather than starting another one.
	if s.sessionID != "" {
		a = append(a, "--resume", s.sessionID)
	}
	// Last, because the flag takes every following argument as a tool name.
	return append(append(a, "--disallowed-tools"), interactiveTools...)
}

// start spawns the engine. Called with s.mu held.
func (s *Session) start() (*process, error) {
	cmd := exec.Command(s.opts.Executable, s.args()...)
	cmd.Dir = s.opts.Dir
	ownGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p := &process{cmd: cmd, stdin: stdin, stderr: &tail{max: 4000}}
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go s.read(p, stdout)
	return p, nil
}

// read turns the engine's output into events for whichever turn is running,
// and reports the process's exit on the turn it cut short.
func (s *Session) read(p *process, stdout io.Reader) {
	r := bufio.NewReaderSize(stdout, 1<<16)
	for {
		raw, err := r.ReadBytes('\n')
		if len(strings.TrimSpace(string(raw))) > 0 {
			s.deliver(p, raw)
		}
		if err != nil {
			break
		}
	}
	waitErr := p.cmd.Wait()

	s.mu.Lock()
	var tr *turn
	// Only the session's current process reports on the turn: one replaced
	// on purpose (a model switch) dies quietly, and never on its successor's turn.
	if s.proc == p {
		s.proc = nil
		tr, s.turn = s.turn, nil
	}
	s.mu.Unlock()
	if tr == nil {
		return // idle between turns, or replaced: nothing to report
	}
	switch {
	case tr.cancelled:
		tr.events <- engine.Done{Cancelled: true}
	default:
		msg := strings.TrimSpace(p.stderr.String())
		if msg == "" {
			msg = fmt.Sprintf("the engine exited (%v) without an answer", waitErr)
		}
		tr.events <- engine.Failure{Message: msg}
	}
	close(tr.finished)
	close(tr.events)
}

// Warm starts the engine before the first turn, while the chat plans the
// request: the seconds of startup overlap the planning instead of following
// it. Nothing is written to the engine. What it says before a turn — its
// init — is kept for the turn that follows.
func (s *Session) Warm() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.proc != nil {
		return
	}
	if p, err := s.start(); err == nil {
		s.proc = p
	}
}

// deliver translates one line and hands its events to the running turn.
func (s *Session) deliver(p *process, raw []byte) {
	events, terminal := translate(raw)
	if len(events) == 0 {
		return
	}
	s.mu.Lock()
	for _, e := range events {
		if m, ok := e.(engine.Meta); ok && m.SessionID != "" {
			s.sessionID = m.SessionID
		}
	}
	tr := s.turn
	if s.proc != p {
		tr = nil // a replaced process: its turn was already reported
	} else if tr == nil {
		// Said before any turn — a warmed engine's init: kept for the next one.
		for _, e := range events {
			if _, ok := e.(engine.Meta); ok {
				s.early = append(s.early, e)
			}
		}
	}
	if terminal && tr != nil {
		s.turn = nil
	}
	s.mu.Unlock()
	if tr == nil {
		return
	}
	for _, e := range events {
		tr.events <- e
	}
	if terminal {
		close(tr.finished)
		close(tr.events)
	}
}

// kill ends a process and whatever it started: a polite signal first, then
// force.
func (s *Session) kill(p *process) {
	_ = p.stdin.Close()
	if p.cmd.Process == nil {
		return
	}
	_ = signalGroup(p.cmd, syscall.SIGTERM)
	time.AfterFunc(2*time.Second, func() { _ = signalGroup(p.cmd, syscall.SIGKILL) })
}

func startFailure(exe string, err error) engine.Failure {
	if errors.Is(err, exec.ErrNotFound) {
		return engine.Failure{
			Message: fmt.Sprintf("`%s` was not found on PATH", exe),
			Hint:    "Install Claude Code (https://claude.com/claude-code), then run `gofi doctor`.",
		}
	}
	return engine.Failure{Message: fmt.Sprintf("could not start `%s`: %v", exe, err)}
}

// tail keeps the last max bytes written to it: a wedged engine can print
// megabytes, and all that is ever shown is the end, as a failure reason.
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

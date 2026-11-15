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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
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
	// DisallowedTools are refused for the session whatever the person's own
	// settings allow, passed with --disallowed-tools: what keeps a role to
	// the tools its contract grants.
	DisallowedTools []string
	// SystemPrompt replaces the chat's appended system prompt: a turn of the
	// loop has no one to answer it in text.
	SystemPrompt string
	// Isolated runs the session on the project alone — its settings and
	// CLAUDE.md, not the person's own —, with only the MCP servers of
	// MCPConfig and the dynamic sections of the system prompt left out, so
	// turns side by side share a cached prefix.
	Isolated  bool
	MCPConfig string // JSON or a file, passed as --mcp-config
	// CacheTTL is how long the session's prompt cache lives, "5m" or "1h";
	// empty leaves it to the engine. A write kept 5 minutes costs 1.25 times
	// the input price, one kept an hour twice.
	CacheTTL string
	// Tools, when not nil, are the engine's own tools the session has at all,
	// passed as --tools. AllowedTools only spares a prompt: a tool left out
	// here is not even described to the model, which is what a turn pays for.
	Tools []string
	// MaxBudgetUSD is what the session may spend, passed as --max-budget-usd
	// when positive: the engine stops itself there.
	MaxBudgetUSD float64
	// StablePrompt leaves what changes from one session to the next — the
	// folder, the git status — out of the system prompt, as Isolated does,
	// with the person's own settings kept: the sessions a conversation opens
	// one after another then read that prefix from the cache.
	StablePrompt bool
	// Interrupt has Cancel ask the engine to stop the turn where it is,
	// rather than end its process: the next turn goes on in the same engine,
	// with no start and no conversation to load again. An engine that does
	// not stop is ended, as without it.
	Interrupt bool
}

// interruptGrace is how long an engine has to end a turn it was asked to
// stop, before its process is ended instead.
var interruptGrace = 3 * time.Second

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

	described chan struct{}    // closed once the engine said what it offers
	commands  []engine.Command // what it said
}

// process is one running engine. Only its reader goroutine delivers events
// and closes turn channels, so a cancel racing an answer cannot double-close.
type process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *tail
	wmu    sync.Mutex // a turn and a control request may be written at once
}

// write sends one line to the engine.
func (p *process) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_, err = p.stdin.Write(append(b, '\n'))
	return err
}

// control is a request of the engine's control protocol: it is answered by
// the engine itself, and no model is called.
func control(id, subtype string) map[string]any {
	return map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{"subtype": subtype}}
}

type turn struct {
	events    chan engine.Event
	finished  chan struct{}
	cancelled bool
	// queued are the messages sent during the turn that the engine has not
	// handed the model yet: while there is one, the engine's end of a turn
	// is not the end of this one — it goes on to run it.
	queued map[string]bool
	// round counts the turns the engine ended within this one.
	round int
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

	if err := p.write(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": content(t)},
	}); err != nil {
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

// content is a turn as the engine reads it: its images, then its text.
func content(t engine.Turn) []map[string]any {
	out := make([]map[string]any, 0, len(t.Images)+1)
	for _, img := range t.Images {
		out = append(out, map[string]any{
			"type":   "image",
			"source": map[string]any{"type": "base64", "media_type": img.MediaType, "data": base64.StdEncoding.EncodeToString(img.Data)},
		})
	}
	return append(out, map[string]any{"type": "text", "text": t.Prompt})
}

// Cancel implements engine.Session. Without Options.Interrupt the process
// goes, and the next turn resumes the conversation by id — a cold start, but
// only for the turn after a cancel. With it the engine is asked to stop the
// turn and stays up; one that does not stop within interruptGrace, or a
// second Cancel, ends the process as before.
func (s *Session) Cancel() {
	s.mu.Lock()
	tr, p := s.turn, s.proc
	if tr == nil || p == nil {
		s.mu.Unlock()
		return
	}
	again, round := tr.cancelled, tr.round
	tr.cancelled = true
	s.mu.Unlock()
	if !s.opts.Interrupt || again || p.write(control("interrupt", "interrupt")) != nil {
		s.kill(p)
		return
	}
	time.AfterFunc(interruptGrace, func() {
		s.mu.Lock()
		stuck := s.turn == tr && s.proc == p && tr.round == round
		s.mu.Unlock()
		if stuck {
			s.kill(p)
		}
	})
}

// Steer implements engine.Steerer. The message goes to the engine as any
// other, with an id: the engine queues it and reports, by that id, when the
// model takes it.
func (s *Session) Steer(t engine.Turn) (string, bool) {
	s.mu.Lock()
	tr, p := s.turn, s.proc
	if tr == nil || p == nil || s.closed {
		s.mu.Unlock()
		return "", false
	}
	id := newID()
	if tr.queued == nil {
		tr.queued = map[string]bool{}
	}
	tr.queued[id] = true
	s.mu.Unlock()
	err := p.write(map[string]any{
		"type":    "user",
		"uuid":    id,
		"message": map[string]any{"role": "user", "content": content(t)},
	})
	if err != nil {
		s.mu.Lock()
		delete(tr.queued, id)
		s.mu.Unlock()
		return "", false
	}
	return id, true
}

// newID is a random UUID, the form the engine wants a message's id in.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Describe implements engine.Describer: it starts the engine if it must and
// asks what it offers. The answer comes from the engine, not from a model.
func (s *Session) Describe(ctx context.Context) (engine.Meta, bool) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return engine.Meta{}, false
	}
	if s.proc == nil {
		p, err := s.start()
		if err != nil {
			s.mu.Unlock()
			return engine.Meta{}, false
		}
		s.proc = p
	}
	p, ask := s.proc, s.described == nil
	if ask {
		s.described = make(chan struct{})
	}
	wait := s.described
	s.mu.Unlock()
	if ask && p.write(control("describe", "initialize")) != nil {
		return engine.Meta{}, false
	}
	select {
	case <-wait:
	case <-ctx.Done():
		return engine.Meta{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return engine.Meta{SessionID: s.sessionID, Commands: s.commands}, len(s.commands) > 0
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
	}
	prompt := systemPrompt
	if s.opts.SystemPrompt != "" {
		prompt = s.opts.SystemPrompt
	}
	a = append(a, "--append-system-prompt", prompt)
	if s.opts.Isolated {
		a = append(a, "--setting-sources", "project", "--strict-mcp-config", "--exclude-dynamic-system-prompt-sections")
	} else if s.opts.StablePrompt {
		a = append(a, "--exclude-dynamic-system-prompt-sections")
	}
	if c := strings.TrimSpace(s.opts.MCPConfig); c != "" {
		a = append(a, "--mcp-config", c)
	}
	if mode := strings.TrimSpace(s.opts.PermissionMode); mode != "" && mode != "default" {
		a = append(a, "--permission-mode", mode)
	}
	if s.opts.Tools != nil {
		a = append(a, "--tools", strings.Join(s.opts.Tools, ","))
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
	if s.opts.MaxBudgetUSD > 0 {
		a = append(a, "--max-budget-usd", strconv.FormatFloat(s.opts.MaxBudgetUSD, 'f', 2, 64))
	}
	// After a cancel the process that held the thread is gone; resuming by id
	// is how the next turn continues it rather than starting another one.
	if s.sessionID != "" {
		a = append(a, "--resume", s.sessionID)
	}
	// Last, because the flag takes every following argument as a tool name.
	return append(append(append(a, "--disallowed-tools"), interactiveTools...), s.opts.DisallowedTools...)
}

// maxShared bounds the system prompt a session is started with: it goes as
// one argument, and the system refuses one past 128 KiB.
const maxShared = 100_000

// Share appends to the system prompt what every session of a run opens with.
// Sessions share a cache up to the system prompt and no further: the same
// words in the first message are written to it again by each. False when the
// session has started or the text does not fit.
func (s *Session) Share(text string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	base := s.opts.SystemPrompt
	if base == "" {
		base = systemPrompt
	}
	if s.proc != nil || s.closed || len(base)+len(text) > maxShared {
		return false
	}
	s.opts.SystemPrompt = base + "\n\n" + strings.TrimSpace(text)
	return true
}

// env is the engine's environment; nil, the process's own.
func (s *Session) env() []string {
	if s.opts.CacheTTL == "" {
		return nil
	}
	return append(os.Environ(), "CLAUDE_CODE_PROMPT_CACHE_TTL="+s.opts.CacheTTL)
}

// start spawns the engine. Called with s.mu held.
func (s *Session) start() (*process, error) {
	cmd := exec.Command(s.opts.Executable, s.args()...)
	cmd.Dir = s.opts.Dir
	cmd.Env = s.env()
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
	tr := s.turn
	if s.proc != p {
		tr = nil // a replaced process: its turn was already reported
	}
	kept := events[:0]
	for _, e := range events {
		switch v := e.(type) {
		case engine.Meta:
			if v.SessionID != "" {
				s.sessionID = v.SessionID
			}
			if len(v.Commands) > 0 {
				s.commands = v.Commands
				if s.described == nil {
					s.described = make(chan struct{})
				}
				select {
				case <-s.described:
				default:
					close(s.described)
				}
			}
			// Said before any turn — a warmed engine's init: kept for the
			// next one.
			if tr == nil && s.proc == p {
				s.early = append(s.early, e)
			}
		case engine.Delivered:
			// Only a message this turn sent is this turn's to report.
			if tr == nil || !tr.queued[v.ID] {
				continue
			}
			delete(tr.queued, v.ID)
		case engine.Done:
			// A turn the person stopped ended as they asked, whatever the
			// engine calls it.
			if tr != nil && tr.cancelled {
				v.Cancelled, v.IsError, v.Err = true, false, ""
				e = v
			}
		}
		kept = append(kept, e)
	}
	events = kept
	if terminal && tr != nil {
		if len(tr.queued) > 0 {
			// The engine ended a turn with a message still waiting: it runs
			// it next, and this turn goes on with it.
			terminal = false
			tr.round++
			tr.cancelled = false
		} else {
			s.turn = nil
		}
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

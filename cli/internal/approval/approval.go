// Package approval lets the chat decide, call by call, whether the engine may
// run a tool that changes something.
//
// The engine asks through its PreToolUse hook, and the hook is gofi itself
// (`gofi hook pretool`): a short-lived process the engine starts before every
// matching tool call. It carries the call over a local socket to the chat —
// where the user is — and waits for the answer.
//
// Failing closed is the whole design. A chat that is gone, a socket that does
// not answer, a request nobody decides on in time — all deny. A gate whose
// failure mode is "allow" is not a gate.
//
// The hook answers with an explicit permission decision, not an exit code:
// "allow" settles the call — the engine does not ask again — and "deny" hands
// the reason to the model, which can then adapt instead of retrying.
package approval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gofi-labs/gofi/cli/internal/guard"
)

// Matcher selects the tools that ask first: the ones that can change
// something. Reads are never worth a prompt, and asking about them would
// train the user to approve without reading.
const Matcher = "Edit|MultiEdit|Write|NotebookEdit|Bash"

// Timeout is how long a request waits for the user before it is denied.
const Timeout = 5 * time.Minute

// Request is one tool call waiting for a decision.
type Request struct {
	Tool  string          `json:"tool_name"`
	Input json.RawMessage `json:"tool_input"`

	reply chan Decision
}

// Decision is the answer to a Request. Reason goes back to the model when the
// call is denied, so it can adapt instead of retrying.
type Decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason,omitempty"`
}

// Answer decides the request. Only the first answer counts.
func (r *Request) Answer(d Decision) {
	select {
	case r.reply <- d:
	default:
	}
}

// Server listens for the hook on a local socket.
type Server struct {
	ln   net.Listener
	dir  string
	addr string

	mu        sync.Mutex
	onRequest func(*Request)
	pending   map[*Request]struct{}
	closed    bool
}

// Listen opens the socket. Requests that arrive before OnRequest is set are
// denied.
func Listen() (*Server, error) {
	dir, err := os.MkdirTemp("", "gofi-approve-")
	if err != nil {
		return nil, err
	}
	addr := filepath.Join(dir, "s")
	ln, err := net.Listen("unix", addr)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	s := &Server{ln: ln, dir: dir, addr: addr, pending: map[*Request]struct{}{}}
	go s.serve()
	return s, nil
}

// Addr is what the hook dials.
func (s *Server) Addr() string { return s.addr }

// OnRequest sets who decides. It is called on its own goroutine, once per
// request, and must eventually Answer it.
func (s *Server) OnRequest(f func(*Request)) {
	s.mu.Lock()
	s.onRequest = f
	s.mu.Unlock()
}

// DenyAll answers every request still waiting — used when a turn is
// interrupted, so no hook is left blocking the engine.
func (s *Server) DenyAll(reason string) {
	s.mu.Lock()
	var reqs []*Request
	for r := range s.pending {
		reqs = append(reqs, r)
	}
	s.mu.Unlock()
	for _, r := range reqs {
		r.Answer(Decision{Reason: reason})
	}
}

// Close stops listening and denies whatever is still waiting.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.DenyAll("The gofi chat closed before an answer.")
	err := s.ln.Close()
	_ = os.RemoveAll(s.dir)
	return err
}

func (s *Server) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return
	}
	req := &Request{reply: make(chan Decision, 1)}
	if json.Unmarshal(line, req) != nil {
		writeDecision(conn, Decision{Reason: "The approval request was malformed."})
		return
	}

	s.mu.Lock()
	decide := s.onRequest
	if s.closed || decide == nil {
		s.mu.Unlock()
		writeDecision(conn, Decision{Reason: "Nobody is there to approve this call."})
		return
	}
	s.pending[req] = struct{}{}
	s.mu.Unlock()

	go decide(req)
	var d Decision
	select {
	case d = <-req.reply:
	case <-time.After(Timeout):
		d = Decision{Reason: "Nobody answered the approval request in time."}
	}

	s.mu.Lock()
	delete(s.pending, req)
	s.mu.Unlock()
	writeDecision(conn, d)
}

func writeDecision(conn net.Conn, d Decision) {
	b, _ := json.Marshal(d)
	_, _ = conn.Write(append(b, '\n'))
}

// HookSettings is the --settings payload that installs gofi as the engine's
// PreToolUse hook for the tools in Matcher.
func HookSettings(gofiExe, addr string) (string, error) {
	command := quote(gofiExe) + " hook pretool --addr " + quote(addr)
	b, err := json.Marshal(map[string]any{
		// The project's gofi MCP server, trusted without the first-use prompt,
		// and its read-only tools allowed like the read-only commands.
		"enabledMcpjsonServers": []string{"gofi"},
		"permissions":           map[string]any{"allow": guard.ReadOnlyRules},
		"hooks": map[string]any{
			"PreToolUse": []any{map[string]any{
				"matcher": Matcher,
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": command,
					// A little past our own timeout, so the answer is always ours.
					"timeout": int(Timeout.Seconds()) + 30,
				}},
			}},
		},
	})
	return string(b), err
}

// readOnlyQuery reports a Bash call that only queries the gofi index.
func readOnlyQuery(stdin []byte) bool {
	var call struct {
		Tool  string `json:"tool_name"`
		Input struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	return json.Unmarshal(stdin, &call) == nil && call.Tool == "Bash" && guard.ReadOnlyCommand(call.Input.Command)
}

// quote protects a path in the shell the engine runs hook commands in.
func quote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Ask is `gofi hook pretool`: it carries the call the engine is about to make
// (the hook's stdin) to the chat at addr and returns the chat's decision.
// Every way of not getting an answer is a denial with a reason.
func Ask(stdin []byte, addr string) Decision {
	deny := func(reason string) Decision { return Decision{Reason: reason} }
	// A query of the index only reads. Asking the user before each one would
	// make the index the expensive path and grep the cheap one — the opposite
	// of what the agents are told.
	if readOnlyQuery(stdin) {
		return Decision{Allow: true}
	}
	if strings.TrimSpace(addr) == "" {
		return deny("gofi: no approval channel; the call was blocked.")
	}
	conn, err := net.DialTimeout("unix", addr, 5*time.Second)
	if err != nil {
		return deny("gofi: the chat is not answering approvals; the call was blocked.")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(Timeout + 15*time.Second))

	if _, err := conn.Write(append(compact(stdin), '\n')); err != nil {
		return deny("gofi: could not send the approval request; the call was blocked.")
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return deny("gofi: no answer to the approval request; the call was blocked.")
	}
	var d Decision
	if json.Unmarshal(line, &d) != nil {
		return deny("gofi: unreadable approval answer; the call was blocked.")
	}
	if !d.Allow && d.Reason == "" {
		d.Reason = "The user did not approve this call."
	}
	return d
}

// HookResponse is what the hook prints for the engine. The decision is
// explicit on purpose: exiting 0 only means "carry on", and the engine then
// runs its own permission check and may still refuse what the user approved.
// "allow" settles it; "deny" hands the reason to the model as it is.
func HookResponse(d Decision) []byte {
	decision := "deny"
	if d.Allow {
		decision = "allow"
	}
	reason := d.Reason
	if d.Allow && reason == "" {
		reason = "Approved in the gofi chat."
	}
	b, _ := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       decision,
			"permissionDecisionReason": reason,
		},
	})
	return b
}

// compact keeps the request on one line, which is how the server reads it.
func compact(raw []byte) []byte {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		b, _ := json.Marshal(map[string]string{"tool_name": "unknown"})
		return b
	}
	b, _ := json.Marshal(v)
	return b
}

// String describes a request in one line, for logs and tests.
func (r *Request) String() string { return fmt.Sprintf("%s %s", r.Tool, r.Input) }

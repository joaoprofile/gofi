package approval

import (
	"encoding/json"
	"strings"
	"testing"
)

func hookCall(t *testing.T, addr string) Decision {
	t.Helper()
	return Ask([]byte(`{"tool_name":"Bash","tool_input":{"command":"rm -rf build"},"session_id":"x"}`), addr)
}

func TestAllowAndDenyReachTheHook(t *testing.T) {
	s, err := Listen()
	if err != nil {
		t.Skip("no local sockets here:", err)
	}
	defer s.Close()

	var seen *Request
	s.OnRequest(func(r *Request) {
		seen = r
		r.Answer(Decision{Allow: r.Tool == "Bash" && strings.Contains(string(r.Input), "rm -rf build")})
	})
	if d := hookCall(t, s.Addr()); !d.Allow {
		t.Fatalf("allowed call came back %+v", d)
	}
	if seen == nil || seen.Tool != "Bash" {
		t.Fatalf("request = %v", seen)
	}

	s.OnRequest(func(r *Request) { r.Answer(Decision{Reason: "not now"}) })
	if d := hookCall(t, s.Addr()); d.Allow || d.Reason != "not now" {
		t.Fatalf("denied call came back %+v", d)
	}
}

// Every way the chat can be missing ends in a denial, never an allow.
func TestFailsClosed(t *testing.T) {
	if hookCall(t, "").Allow {
		t.Error("no address must deny")
	}
	if hookCall(t, "/nonexistent/gofi.sock").Allow {
		t.Error("an unreachable chat must deny")
	}

	s, err := Listen()
	if err != nil {
		t.Skip("no local sockets here:", err)
	}
	if hookCall(t, s.Addr()).Allow {
		t.Error("a chat with nobody deciding must deny")
	}

	block := make(chan struct{})
	s.OnRequest(func(r *Request) { <-block })
	done := make(chan Decision)
	go func() { done <- hookCall(t, s.Addr()) }()
	for {
		s.mu.Lock()
		n := len(s.pending)
		s.mu.Unlock()
		if n > 0 {
			break
		}
	}
	s.Close()
	if d := <-done; d.Allow {
		t.Error("closing the chat must deny what was waiting")
	}
	close(block)
}

func TestHookSettingsInstallGofi(t *testing.T) {
	raw, err := HookSettings("/usr/local/bin/gofi", "/tmp/gofi-approve-1/s")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Hooks struct {
			PreToolUse []struct {
				Matcher string
				Hooks   []struct {
					Type, Command string
				}
			}
		}
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	h := v.Hooks.PreToolUse[0]
	if h.Matcher != Matcher || !strings.Contains(h.Hooks[0].Command, "hook pretool --addr") {
		t.Errorf("settings = %s", raw)
	}
}

// The engine reads the decision from stdout; an exit code alone would leave it
// to run its own permission check on a call the user already approved.
func TestHookResponseIsAnExplicitDecision(t *testing.T) {
	var v struct {
		HookSpecificOutput struct {
			HookEventName, PermissionDecision, PermissionDecisionReason string
		}
	}
	for _, c := range []struct {
		d    Decision
		want string
	}{{Decision{Allow: true}, "allow"}, {Decision{Reason: "no"}, "deny"}} {
		if err := json.Unmarshal(HookResponse(c.d), &v); err != nil {
			t.Fatal(err)
		}
		o := v.HookSpecificOutput
		if o.HookEventName != "PreToolUse" || o.PermissionDecision != c.want || o.PermissionDecisionReason == "" {
			t.Errorf("%+v → %+v", c.d, o)
		}
	}
}

// Reading the index needs no approval: the query answers without a channel,
// while anything chained to it still goes to the user.
func TestReadOnlyQueriesSkipTheUser(t *testing.T) {
	if d := Ask([]byte(`{"tool_name":"Bash","tool_input":{"command":"gofi find \"pedido\""}}`), ""); !d.Allow {
		t.Errorf("a read-only query was sent for approval: %+v", d)
	}
	if d := Ask([]byte(`{"tool_name":"Bash","tool_input":{"command":"gofi find x && rm -rf y"}}`), ""); d.Allow {
		t.Error("a chained command rode on the query's pass")
	}
	raw, _ := HookSettings("/usr/local/bin/gofi", "/tmp/s")
	for _, want := range []string{`"enabledMcpjsonServers":["gofi"]`, `"mcp__gofi__find"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("settings lack %s: %s", want, raw)
		}
	}
}

package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// A role gets exactly the tools its contract grants: on Claude Code the
// gated ones outside the grant are refused, so no role runs a shell and the
// audit edits nothing (0002, D15). Searching is the graph's: search grants
// gofi's find, show and path, and Grep and Glob are refused to every role —
// a turn that swept the tree read what the index already knew.
func TestARoleGetsExactlyItsTools(t *testing.T) {
	allowed, denied := claudeTools([]string{"read", "search", "gofi", "verify", "verdict", "memory"})
	for _, want := range []string{"Read", "mcp__gofi__find", "mcp__gofi__show", "mcp__gofi__path", "mcp__gofi__verify", "mcp__gofi__verdict_write"} {
		if !slices.Contains(allowed, want) {
			t.Errorf("the audit lacks %s: %v", want, allowed)
		}
	}
	if slices.Contains(allowed, "Grep") || slices.Contains(allowed, "Glob") {
		t.Errorf("a role may sweep the tree: %v", allowed)
	}
	if n := len(slices.Compact(slices.Sorted(slices.Values(allowed)))); n != len(allowed) {
		t.Errorf("a tool is granted twice: %v", allowed)
	}
	for _, want := range []string{"Bash", "Edit", "Write", "Grep", "Glob"} {
		if !slices.Contains(denied, want) {
			t.Errorf("the audit may %s: refused %v", want, denied)
		}
	}
	_, denied = claudeTools([]string{"read", "edit", "write", "search", "gofi", "verify"})
	if !slices.Equal(denied, []string{"Bash", "BashOutput", "KillShell", "Task", "Agent", "WebFetch", "WebSearch", "Grep", "Glob"}) {
		t.Errorf("the implementation is refused %v, want the shell, the subagents, the web and the raw search", denied)
	}
}

// Every internal role gofi ships grants its tools, and none a shell.
func TestNoShippedRoleGetsAShell(t *testing.T) {
	roles, problems := role.Load(shippedSkills)
	for _, p := range problems {
		t.Error(p)
	}
	for _, r := range roles {
		if !r.Contract.Internal() {
			continue
		}
		if len(r.Contract.Tools) == 0 {
			t.Errorf("%s grants no tools: it would run with the defaults", r.Name)
		}
		if slices.Contains(r.Contract.Tools, "shell") {
			t.Errorf("%s is granted a shell", r.Name)
		}
	}
}

// A loop turn is told only of what its role uses: a tool outside the grant
// costs tokens to describe on every step, and is refused besides.
func TestALoopTurnIsToldOnlyOfItsTools(t *testing.T) {
	grant := []string{"read", "search", "gofi", "verify", "verdict"}
	if got := strings.Join(claudeBuiltins(grant), ","); got != "Read" {
		t.Errorf("an auditor's own tools = %q", got)
	}
	if got := claudeBuiltins([]string{"gofi"}); got == nil || len(got) != 0 {
		t.Errorf("a role granted none of the engine's has none, got %v", got)
	}
	denied := strings.Join(ungrantedGofi(grant), ",")
	for _, want := range []string{"approve", "intake", "memory_write"} {
		if !strings.Contains(denied, want) {
			t.Errorf("%s should be refused to an auditor: %s", want, denied)
		}
	}
	if strings.Contains(denied, "verdict_write") {
		t.Errorf("a tool the grant names was refused: %s", denied)
	}
}

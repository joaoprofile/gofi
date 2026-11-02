package cli

import (
	"slices"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// A role gets exactly the tools its contract grants: on Claude Code the
// gated ones outside the grant are refused, so no role runs a shell and the
// audit edits nothing (0002, D15).
func TestARoleGetsExactlyItsTools(t *testing.T) {
	allowed, denied := claudeTools([]string{"read", "search", "gofi", "verify", "verdict", "memory"})
	for _, want := range []string{"Read", "Grep", "mcp__gofi__verify", "mcp__gofi__verdict_write"} {
		if !slices.Contains(allowed, want) {
			t.Errorf("the audit lacks %s: %v", want, allowed)
		}
	}
	for _, want := range []string{"Bash", "Edit", "Write"} {
		if !slices.Contains(denied, want) {
			t.Errorf("the audit may %s: refused %v", want, denied)
		}
	}
	_, denied = claudeTools([]string{"read", "edit", "write", "search", "gofi", "verify"})
	if !slices.Equal(denied, []string{"Bash"}) {
		t.Errorf("the implementation is refused %v, want only the shell", denied)
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

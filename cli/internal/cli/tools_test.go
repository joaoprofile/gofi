package cli

import (
	"context"
	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/host"
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

// A model chosen for a role wins over its tier; a role not chosen, or one
// the loop raised (which opens with no role), keeps the tier's.
func TestAModelCanBeChosenForOneRole(t *testing.T) {
	h := host.Host{SkillModel: true, Models: map[host.Tier]string{host.Standard: "sonnet"}}
	ai := config.AI{Roles: map[string]string{"gofi-qa": "haiku"}}
	for _, c := range []struct{ role, want string }{{"gofi-qa", "haiku"}, {"gofi-eng", "sonnet"}, {"", "sonnet"}} {
		if got := turnModel(h, ai, c.role, host.Standard); got != c.want {
			t.Errorf("role %q: %s, want %s", c.role, got, c.want)
		}
	}
}

// A role that only reads keeps its cache 5 minutes; one that writes is left
// to the engine, for a turn that can go quiet for longer.
func TestOnlyAReadingRoleShortensItsCache(t *testing.T) {
	if got := turnCacheTTL([]string{"read", "search", "gofi", "verdict"}); got != "5m" {
		t.Errorf("an auditor: %q", got)
	}
	if got := turnCacheTTL([]string{"read", "edit", "write"}); got != "" {
		t.Errorf("an implementer: %q", got)
	}
}

type sharingSession struct {
	shared, sent *string
}

func (s sharingSession) Share(text string) bool { *s.shared = text; return true }
func (s sharingSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	*s.sent = t.Prompt
	ch := make(chan engine.Event, 2)
	ch <- engine.Delta{Kind: engine.DeltaText, Text: "ok"}
	ch <- engine.Done{}
	close(ch)
	return ch, nil
}
func (sharingSession) Cancel()      {}
func (sharingSession) Close() error { return nil }

// An engine that shares gets the change apart and the rest as the prompt;
// the transcript keeps the turn whole, and the first answer warms the lenses.
func TestATurnGivesTheSharedWordsToTheEngineApart(t *testing.T) {
	var shared, sent string
	root := t.TempDir()
	a := &engineAgent{root: root, run: "r-1", out: io.Discard, quiet: true,
		open: func(_, _ string, _ host.Tier, _ []string, _ float64) engine.Session {
			return sharingSession{&shared, &sent}
		}}
	warmed := false
	_, err := a.Run(context.Background(), loop.Turn{Node: "audit", Iter: 1, Prompt: "Trabalhe.\n\nA MUDANÇA\n\nO papel.", Shared: "A MUDANÇA\n\n", Warmed: func() { warmed = true }})
	if err != nil {
		t.Fatal(err)
	}
	if shared != "A MUDANÇA\n\n" || sent != "Trabalhe.\n\nO papel." || !warmed {
		t.Errorf("shared %q, sent %q, warmed %v", shared, sent, warmed)
	}
	if b, _ := os.ReadFile(loop.Transcript(root, "r-1", "audit-1")); !strings.Contains(string(b), "A MUDANÇA") {
		t.Errorf("the transcript lost the change: %s", b)
	}
}

package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

func pre(tool string, input map[string]any) Input {
	return Input{SessionID: "s1", Event: EventPreTool, Tool: tool, ToolInput: input}
}

func TestClassify(t *testing.T) {
	root := t.TempDir()
	big := filepath.Join(root, "big.go")
	small := filepath.Join(root, "small.go")
	if err := os.WriteFile(big, []byte(strings.Repeat("x\n", SmallFile+5)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(small, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		in   Input
		want Kind
	}{
		{"prompt", Input{Event: EventPrompt}, NewPrompt},
		{"mcp find", pre("mcp__gofi__find", nil), Query},
		{"mcp index is no query", pre("mcp__gofi__index", nil), Other},
		{"shell find", pre("Bash", map[string]any{"command": "cd x && gofi find \"pedido\""}), Query},
		{"shell show by path", pre("Bash", map[string]any{"command": "./bin/gofi show Bill"}), Query},
		{"grep tool", pre("Grep", map[string]any{"pattern": "Bill"}), Search},
		{"grep outside", pre("Grep", map[string]any{"pattern": "x", "path": "/usr/share"}), Other},
		{"glob", pre("Glob", map[string]any{"pattern": "**/*.go"}), Search},
		{"shell grep in a chain", pre("Bash", map[string]any{"command": "ls && grep -rn Bill ."}), Search},
		{"shell rg piped", pre("Bash", map[string]any{"command": "cat x | rg Bill"}), Search},
		{"git grep", pre("Bash", map[string]any{"command": "git grep Bill"}), Search},
		{"shell find files", pre("Bash", map[string]any{"command": "find . -name '*.go'"}), Search},
		{"go test", pre("Bash", map[string]any{"command": "go test ./..."}), Other},
		{"whole big file", pre("Read", map[string]any{"file_path": big}), Search},
		{"ranged read", pre("Read", map[string]any{"file_path": big, "offset": 10, "limit": 20}), Other},
		{"small file", pre("Read", map[string]any{"file_path": small}), Other},
		{"read outside", pre("Read", map[string]any{"file_path": "/etc/hosts"}), Other},
		{"edit", pre("Edit", map[string]any{"file_path": big}), Other},
	} {
		if got, _ := Classify(c.in, root); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func decision(t *testing.T, out []byte) map[string]any {
	t.Helper()
	if out == nil {
		return nil
	}
	var v struct {
		H map[string]any `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	return v.H
}

func msg(search string, deny bool) string {
	if deny {
		return "deny " + search
	}
	return "warn " + search
}

// The session's life: a prompt starts over, a search before a query is
// flagged, a query clears the way, and the next prompt starts over again.
func TestDecideFollowsTheQuestion(t *testing.T) {
	root := t.TempDir()
	store := Store{Dir: t.TempDir()}
	grep := pre("Grep", map[string]any{"pattern": "Bill"})
	prompt := Input{SessionID: "s1", Event: EventPrompt}

	Decide(prompt, root, config.GuardWarn, store, msg)
	d := decision(t, Decide(grep, root, config.GuardWarn, store, msg))
	if d == nil || d["additionalContext"] != "warn Grep Bill" || d["permissionDecision"] != nil {
		t.Fatalf("warn: got %v", d)
	}
	if d := Decide(grep, root, config.GuardWarn, store, msg); d != nil {
		t.Errorf("warned twice for one question: %s", d)
	}

	Decide(pre("mcp__gofi__find", nil), root, config.GuardWarn, store, msg)
	Decide(prompt, root, config.GuardEnforce, store, msg)
	d = decision(t, Decide(grep, root, config.GuardEnforce, store, msg))
	if d == nil || d["permissionDecision"] != "deny" || d["permissionDecisionReason"] != "deny Grep Bill" {
		t.Fatalf("enforce after a new prompt: got %v", d)
	}
	Decide(pre("Bash", map[string]any{"command": "gofi show Bill"}), root, config.GuardEnforce, store, msg)
	if d := Decide(grep, root, config.GuardEnforce, store, msg); d != nil {
		t.Errorf("a search after a query was refused: %s", d)
	}
}

func TestDecideOffAndOtherSessions(t *testing.T) {
	root := t.TempDir()
	store := Store{Dir: t.TempDir()}
	grep := pre("Grep", map[string]any{"pattern": "Bill"})
	if d := Decide(grep, root, config.GuardOff, store, msg); d != nil {
		t.Errorf("off mode spoke: %s", d)
	}
	Decide(pre("mcp__gofi__show", nil), root, config.GuardEnforce, store, msg)
	other := grep
	other.SessionID = "s2"
	if d := Decide(other, root, config.GuardEnforce, store, msg); d == nil {
		t.Error("a query in one session cleared another")
	}
}

func TestReadOnlyCommand(t *testing.T) {
	for cmd, want := range map[string]bool{
		`gofi find "faturar pedido"`:           true,
		`cd backend && gofi show service.Bill`: true,
		`/usr/local/bin/gofi path A B`:         true,
		`gofi index status --json`:             true,
		`gofi index code`:                      false,
		`gofi find x && rm -rf y`:              false,
		`gofi find x > out.txt`:                false,
		`gofi find $(cat q)`:                   false,
		`gofi find x | head`:                   false,
		`cd x`:                                 false,
		`ls`:                                   false,
	} {
		if got := ReadOnlyCommand(cmd); got != want {
			t.Errorf("%q: got %v, want %v", cmd, got, want)
		}
	}
}

package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetAcceptsOldSpellings(t *testing.T) {
	if h, ok := Get("claude-vscode"); !ok || h.ID != ClaudeCode.ID {
		t.Errorf("claude-vscode → %+v %v", h, ok)
	}
	if _, ok := Get("cursor"); ok {
		t.Error("an unsupported host was accepted")
	}
	if !IsClaude("claude-code") || IsClaude("codex") {
		t.Error("IsClaude")
	}
}

func TestModelPrefersTheProjectsChoice(t *testing.T) {
	if m := ClaudeCode.Model(Deep, nil); m != "opus" {
		t.Errorf("default deep = %q", m)
	}
	if m := ClaudeCode.Model(Deep, map[string]string{"deep": "opusplan"}); m != "opusplan" {
		t.Errorf("override = %q", m)
	}
	// gofi presumes no model name it has not verified.
	if m := Codex.Model(Light, nil); m != "" {
		t.Errorf("codex light = %q, want the session's model", m)
	}
}

// Each host's file, merged with what the team has, and idempotent.
func TestRegisterMCPEveryFormat(t *testing.T) {
	for _, c := range []struct {
		h        Host
		existing string
		keep     string
	}{
		{ClaudeCode, `{"mcpServers":{"db":{"command":"db-mcp"}}}`, `"db"`},
		{Copilot, `{"inputs":[],"servers":{"db":{"command":"db-mcp"}}}`, `"inputs"`},
		{Codex, "model = \"x\"\n\n[mcp_servers.db]\ncommand = \"db-mcp\"\n", "[mcp_servers.db]"},
	} {
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(c.h.MCP.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(c.existing), 0o644); err != nil {
			t.Fatal(err)
		}
		if changed, err := c.h.RegisterMCP(root); err != nil || !changed {
			t.Fatalf("%s: changed=%v err=%v", c.h.ID, changed, err)
		}
		if changed, err := c.h.RegisterMCP(root); err != nil || changed {
			t.Errorf("%s: second run changed=%v err=%v", c.h.ID, changed, err)
		}
		b, _ := os.ReadFile(path)
		if !strings.Contains(string(b), c.keep) || !c.h.Registered(root) {
			t.Errorf("%s: %s", c.h.ID, b)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ClaudeCode.RegisterMCP(root); err == nil {
		t.Error("a broken file was rewritten")
	}
}

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/guard"
)

// An agent writes a context's memory through gofi, which checks it: the file
// names the context and carries versao, status and keywords.
func TestMemoryWrite(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	write := func(name, body string) error {
		cmd := NewRoot()
		cmd.SetIn(strings.NewReader(body))
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"memory", "write", name})
		return cmd.Execute()
	}
	good := "---\nformato: memoria\ncontexto: catalog\nversao: \"1.0\"\nstatus: spec\nkeywords: [catalogo]\n---\n# catalog\n"
	if err := write("catalog", good); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".claude/memory/contexts/catalog.md"))
	if err != nil || string(b) != good {
		t.Fatalf("written = %q, %v", b, err)
	}
	for name, body := range map[string]string{
		"catalog":   "# no frontmatter\n",
		"order":     good, // names another context
		"../escape": good,
	} {
		if err := write(name, body); err == nil {
			t.Errorf("%s: %q was written", name, body)
		}
	}
	if err := write("catalog", strings.Replace(good, "status: spec\n", "", 1)); err == nil {
		t.Error("a memory without status was written")
	}
	if err := write("project", "---\nformato: memoria\n---\n# projeto\n"); err != nil {
		t.Errorf("project memory: %v", err)
	}
}

// The project allows the memory write ahead of time, beside the read-only
// queries: Claude Code would stop the agent at .claude/ otherwise.
func TestAgentSettingsAllowTheMemoryWrite(t *testing.T) {
	root := t.TempDir()
	if _, err := installAgentSettings(root, config.AI{}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(root, claudeSettingsFile))
	for _, rule := range append([]string{MemoryWriteRule}, guard.ReadOnlyRules...) {
		if !strings.Contains(string(b), rule) {
			t.Errorf("settings lack %q", rule)
		}
	}
}

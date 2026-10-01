package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
)

// runInstall runs `gofi install <args>` in the current directory.
func runInstall(t *testing.T, args ...string) (string, error) {
	t.Helper()
	restoreGlobals(t)
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"install"}, args...))
	err := root.Execute()
	return out.String(), err
}

// line is the report line of one item.
func line(t *testing.T, out, item string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[1] == item {
			return l
		}
	}
	t.Fatalf("no line for %q in:\n%s", item, out)
	return ""
}

// Without a target, install puts back everything init wires — and a second
// run changes nothing.
func TestInstallAllRepairsWhatInitWired(t *testing.T) {
	root := setupProject(t)
	for _, f := range []string{".mcp.json", claudeSettingsFile} {
		if err := os.Remove(filepath.Join(root, f)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runInstall(t)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, item := range []string{"mcp", "guard"} {
		if l := line(t, out, localized(item)); !strings.HasPrefix(strings.TrimSpace(l), "✓") {
			t.Errorf("%s not reinstalled: %q", item, l)
		}
	}
	for _, f := range []string{".mcp.json", claudeSettingsFile} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("%s not written: %v", f, err)
		}
	}
	out, err = runInstall(t)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	for _, item := range []string{"mcp", "guard", "hooks"} {
		if l := line(t, out, localized(item)); !strings.HasPrefix(strings.TrimSpace(l), "=") {
			t.Errorf("%s changed on a second run: %q", item, l)
		}
	}
}

// The host decides what applies: Codex gets its MCP file, and neither the
// panel (it drives Claude Code) nor the guard (no hooks to use).
func TestInstallAllFollowsTheHost(t *testing.T) {
	root := setupProject(t)
	path := filepath.Join(root, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AI.Host = host.Codex.ID
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := runInstall(t)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	for _, item := range []string{"extension", "guard"} {
		if l := line(t, out, localized(item)); !strings.HasPrefix(strings.TrimSpace(l), "–") {
			t.Errorf("%s should be skipped on Codex: %q", item, l)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".codex", "config.toml")); err != nil {
		t.Errorf("Codex MCP config not written: %v", err)
	}
}

// Outside a project only the extension can be installed; the rest says why.
func TestInstallAllOutsideAProject(t *testing.T) {
	t.Chdir(t.TempDir())
	out, err := runInstall(t)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if l := line(t, out, localized("extension")); !strings.HasPrefix(strings.TrimSpace(l), "✓") {
		t.Errorf("extension should still install: %q", l)
	}
	for _, item := range []string{"mcp", "guard", "hooks"} {
		if l := line(t, out, localized(item)); !strings.HasPrefix(strings.TrimSpace(l), "–") {
			t.Errorf("%s should be skipped outside a project: %q", item, l)
		}
	}
}

// Every target answers in the singular too, the way it is typed.
func TestInstallTargetsAndAliases(t *testing.T) {
	root := NewRoot()
	for _, args := range [][]string{
		{"install", "extensions"}, {"install", "extension"},
		{"install", "hooks"}, {"install", "hook"},
		{"install", "mcp"}, {"install", "guard"},
	} {
		c, _, err := root.Find(args)
		if err != nil || c.Name() == "install" {
			t.Errorf("%v does not resolve to a target: %v", args, err)
		}
	}
}

func localized(item string) string {
	return map[string]string{
		"extension": i18n.T("install.item.extension"),
		"mcp":       i18n.T("install.item.mcp"),
		"guard":     i18n.T("install.item.guard"),
		"hooks":     i18n.T("install.item.hooks"),
	}[item]
}

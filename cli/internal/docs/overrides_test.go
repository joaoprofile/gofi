package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A correction pointing at a rule that moved is an error, named with where the
// sections are now; one that points right is silent; outside knowledge/ the
// field is not a correction at all.
func TestCheckOverrides(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude/expertise/p/rules.md", "# Rules\n\n## Retry\n\nx\n")
	write(".claude/knowledge/shared/ok.md", "---\noverrides: [expertise/p/rules.md#Retry]\n---\n# Ok\n\nx\n")
	write(".claude/knowledge/shared/renamed.md", "---\noverrides: [expertise/p/rules.md#Retentativa]\n---\n# R\n\nx\n")
	write(".claude/knowledge/shared/gone.md", "---\noverrides: [.claude/expertise/p/old.md]\n---\n# G\n\nx\n")
	write(".claude/expertise/p/sneaky.md", "---\noverrides: [expertise/p/nothing.md]\n---\n# S\n\nx\n")
	if _, _, _, err := (&Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range CheckOverrides(root) {
		if !f.Error {
			t.Errorf("a broken correction must be an error: %+v", f)
		}
		got[f.Path] = f.Message
	}
	if len(got) != 2 {
		t.Fatalf("findings = %v", got)
	}
	if m := got[".claude/knowledge/shared/renamed.md"]; !strings.Contains(m, "no such section") || !strings.Contains(m, "Retry") {
		t.Errorf("renamed: %q", m)
	}
	if m := got[".claude/knowledge/shared/gone.md"]; !strings.Contains(m, "no such document") {
		t.Errorf("gone: %q", m)
	}
}

package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// findingsFor keeps the findings about one document.
func findingsFor(fs []Finding, path string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Path == path {
			out = append(out, f)
		}
	}
	return out
}

// The QA writes ressalvas next to status; a memory carrying it is the format,
// not drift.
func TestValidateAcceptsTheAuditCaveats(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, ".claude/memory/contexts/churn.md", `---
formato: 2
contexto: churn
versao: "2.0"
status: aprovado
ressalvas: 0
spec: specs/churn/sdd-churn.md
---
# churn
`)
	if got := findingsFor(Validate(root), ".claude/memory/contexts/churn.md"); len(got) > 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestValidateRejectsAnUnknownField(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, ".claude/memory/contexts/churn.md", `---
formato: 2
contexto: churn
versao: "2.0"
status: aprovado
ressalva: 0
---
# churn
`)
	got := findingsFor(Validate(root), ".claude/memory/contexts/churn.md")
	if len(got) != 1 || !got[0].Error || !strings.Contains(got[0].Message, `"ressalva"`) {
		t.Errorf("findings = %+v, want one error naming the misspelt field", got)
	}
}

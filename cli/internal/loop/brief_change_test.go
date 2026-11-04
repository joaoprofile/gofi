package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeChange(t *testing.T, work string, design string) {
	t.Helper()
	for rel, body := range map[string]string{
		"specs/pricing/changes/alinhar/proposal.md":       "---\ntipo: mudanca\n---\n# Alinhar\n\n## Por quê\n\nA tela mente.\n",
		"specs/pricing/changes/alinhar/deltas/pricing.md": "---\ntipo: delta\n---\n## MODIFICADO\n\n### Requisito: REQ-07 — Tela\nA tela DEVE dizer o alvo.\n",
		"specs/pricing/changes/alinhar/deltas/INDEX.md":   "# derived\n",
		"specs/pricing/changes/alinhar/design.md":         design,
	} {
		p := filepath.Join(work, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Every turn was told only where the change was, and spent its first calls
// finding and reading it — 20 index queries before the first edit in a real
// run. The brief carries the change inline, after its first line, the same
// words for every role of the run.
func TestTheBriefCarriesTheChangeInline(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "---\ntipo: design\n---\n## Abordagem\n\nUm componente.\n")
	s := &State{Run: "r-1", Change: "alinhar", Context: "pricing", ChangeDir: "specs/pricing/changes/alinhar", Iter: 1}
	got := withChange("Trabalhe na mudança.\n\nResto do papel.\n", work, s)

	for _, want := range []string{"A tela mente.", "A tela DEVE dizer o alvo.", "Um componente.", "O que ler neste turno"} {
		if !strings.Contains(got, want) {
			t.Errorf("the brief lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "tipo: delta") || strings.Contains(got, "# derived") {
		t.Errorf("frontmatter or a derived INDEX.md came in:\n%s", got)
	}
	if !strings.HasPrefix(got, "Trabalhe na mudança.\n\n**A mudança, na íntegra**") || !strings.HasSuffix(got, "Resto do papel.\n") {
		t.Errorf("the change must follow the first paragraph, before the role's part:\n%s", got)
	}
}

func TestADesignTooBigIsNamedNotCarried(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "## Abordagem\n\n"+strings.Repeat("x", maxChangeText+1))
	s := &State{ChangeDir: "specs/pricing/changes/alinhar"}
	got := withChange("Trabalhe.\n\nResto.\n", work, s)
	if !strings.Contains(got, "A tela DEVE dizer o alvo.") {
		t.Error("the deltas must always come")
	}
	if strings.Contains(got, strings.Repeat("x", 100)) || !strings.Contains(got, "specs/pricing/changes/alinhar/design.md") {
		t.Errorf("a design too big must be named, not carried")
	}
	if withChange("Trabalhe.\n\nResto.\n", work, &State{}) != "Trabalhe.\n\nResto.\n" {
		t.Error("a run with no change folder must keep its brief")
	}
}

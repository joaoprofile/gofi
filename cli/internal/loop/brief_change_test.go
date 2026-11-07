package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/role"
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
	got := withChange("Trabalhe na mudança.\n\nResto do papel.\n", work, s, role.Role{})

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

// The iteration sat in the brief's first paragraph, before the change: every
// turn after the first diverged there, and wrote the whole change to the
// cache again instead of reading it. Two turns of one run share every word
// up to the end of the change.
func TestTurnsOfARunShareThePrefixThroughTheChange(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "## Abordagem\n\nUm componente.\n")
	rl := shippedRoles()[0] // gofi-eng
	prompt := func(iter int) string {
		s := &State{Run: "r-1", Change: "alinhar", Context: "pricing", ChangeDir: "specs/pricing/changes/alinhar", Iter: iter}
		return withChange(DefaultBrief(rl, s, ""), work, s, rl)
	}
	first, second := prompt(1), prompt(2)
	end := strings.Index(first, "O que ler neste turno")
	if end < 0 || !strings.Contains(first, "Um componente.") {
		t.Fatalf("the brief must carry the change:\n%s", first)
	}
	if first[:end] != second[:end] {
		t.Errorf("two iterations diverge before the end of the change:\n%s\n---\n%s", first[:end], second[:end])
	}
	if !strings.Contains(second, "Iteração 2 da execução r-1.") {
		t.Errorf("the turn must still be told its iteration:\n%s", second)
	}
}

func TestADesignTooBigIsNamedNotCarried(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "## Abordagem\n\n"+strings.Repeat("x", maxChangeText+1))
	s := &State{ChangeDir: "specs/pricing/changes/alinhar"}
	got := withChange("Trabalhe.\n\nResto.\n", work, s, role.Role{})
	if !strings.Contains(got, "A tela DEVE dizer o alvo.") {
		t.Error("the deltas must always come")
	}
	if strings.Contains(got, strings.Repeat("x", 100)) || !strings.Contains(got, "specs/pricing/changes/alinhar/design.md") {
		t.Errorf("a design too big must be named, not carried")
	}
	if withChange("Trabalhe.\n\nResto.\n", work, &State{}, role.Role{}) != "Trabalhe.\n\nResto.\n" {
		t.Error("a run with no change folder must keep its brief")
	}
}

// An internal role was named by the path of its skill: a fresh session each
// turn spent its first call reading it, and the guard refused a whole-file
// Read made that early. The skill comes inline, after the change and before
// what changes from turn to turn.
func TestTheRolesSkillComesInlineBeforeTheIteration(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "## Abordagem\n\nUm componente.\n")
	rl := shippedRoles()[0] // gofi-eng
	skill := filepath.Join(work, ".claude", "gofi", "roles", "gofi-eng", "SKILL.md")
	_ = os.MkdirAll(filepath.Dir(skill), 0o755)
	if err := os.WriteFile(skill, []byte("---\nname: gofi-eng\n---\n# gofi-eng\n\nImplemente o domínio.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &State{Run: "r-1", Change: "alinhar", Context: "pricing", ChangeDir: "specs/pricing/changes/alinhar", Iter: 2}
	got := withChange(DefaultBrief(rl, s, ""), work, s, rl)
	role, iter := strings.Index(got, "Implemente o domínio."), strings.Index(got, "Iteração 2")
	if role < 0 || strings.Contains(got, "name: gofi-eng") || strings.Contains(got, "Siga o papel") {
		t.Fatalf("the skill must come inline, without its frontmatter:\n%s", got)
	}
	if change := strings.Index(got, "Um componente."); change > role || role > iter {
		t.Errorf("the skill must sit between the change and the iteration:\n%s", got)
	}
}

// The delivery and the learner got the whole change, up to 40KB — a haiku
// turn writing a commit message, in a cache no other turn of theirs reads.
// They get the proposal, and the rest by name.
func TestTheDeliveryGetsTheProposalAndTheRestByName(t *testing.T) {
	work := t.TempDir()
	writeChange(t, work, "## Abordagem\n\nUm componente.\n")
	s := &State{Run: "r-1", Change: "alinhar", Context: "pricing", ChangeDir: "specs/pricing/changes/alinhar", Iter: 5}
	for _, stage := range []string{"deliver", "learn"} {
		rl := role.Role{Name: "gofi-x", Contract: role.Contract{Stage: stage}}
		got := withChange("Trabalhe.\n\nResto.\n", work, s, rl)
		if !strings.Contains(got, "A tela mente.") {
			t.Errorf("%s: the proposal must come:\n%s", stage, got)
		}
		if strings.Contains(got, "A tela DEVE dizer o alvo.") || strings.Contains(got, "Um componente.") || strings.Contains(got, "na íntegra") {
			t.Errorf("%s: the deltas and the design must be named, not carried:\n%s", stage, got)
		}
		if !strings.Contains(got, "specs/pricing/changes/alinhar/deltas/pricing.md") || !strings.Contains(got, "specs/pricing/changes/alinhar/design.md") {
			t.Errorf("%s: the rest must be named:\n%s", stage, got)
		}
	}
	impl := role.Role{Name: "gofi-eng", Contract: role.Contract{Stage: "implement"}}
	if got := withChange("Trabalhe.\n\nResto.\n", work, s, impl); !strings.Contains(got, "A tela DEVE dizer o alvo.") {
		t.Errorf("an implementing turn must still carry the whole change:\n%s", got)
	}
}

// A turn sent back to work is a fresh session, and spent its first calls
// finding again the code the run had written. It is told what the graph says
// was written; a first turn, with nothing to fix, is not.
func TestAReworkTurnIsToldWhatTheRunAlreadyWrote(t *testing.T) {
	rl := shippedRoles()[0] // gofi-eng
	diff := &GraphDiff{Touched: []Touch{{Symbol: "pricing.Service.Quote", File: "backend/pricing/service.go"}}}
	first := &State{Run: "r-1", Change: "alinhar", Context: "pricing", Iter: 1, Diff: diff}
	if got := DefaultBrief(rl, first, ""); strings.Contains(got, "já escreveu") {
		t.Errorf("a turn with nothing to fix must not get the map:\n%s", got)
	}
	again := &State{Run: "r-1", Change: "alinhar", Context: "pricing", Iter: 3, Diff: diff,
		Open: []Finding{{Severity: "major", Rule: "erro", Message: "falta o erro"}}}
	if got := DefaultBrief(rl, again, ""); !strings.Contains(got, "já escreveu") || !strings.Contains(got, "pricing.Service.Quote") {
		t.Errorf("a rework turn must be told what the run already wrote:\n%s", got)
	}
}

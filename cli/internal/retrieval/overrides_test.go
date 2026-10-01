package retrieval

import (
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/docs"
)

// A pack with two rules, and a team correction of one of them that shares no
// word with the question: the search reaches the rule, never the correction.
func correctionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, ".claude/expertise/event-driven/executor-pattern.md", `# Event-driven executor

## Retry transient vs permanent

Erro temporário deve ser tentado de novo com backoff exponencial, até cinco vezes.

## Contrato do evento

O envelope carrega type, id e payload.
`)
	write(t, root, ".claude/knowledge/shared/nossa-politica.md", `---
overrides: [expertise/event-driven/executor-pattern.md#Retry transient vs permanent]
keywords: [politica]
---
# Nossa política

Aqui só três vezes, e o timeout do broker é curto.
`)
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	return root
}

const rule = ".claude/expertise/event-driven/executor-pattern.md"
const learning = ".claude/knowledge/shared/nossa-politica.md"

// The acceptance of A5: the team's correction wins over the pack — shown right
// above the rule it corrects, with the reason on both.
func TestCorrectionIsShownAboveTheRule(t *testing.T) {
	e, err := Open(correctionFixture(t), "")
	if err != nil {
		t.Fatal(err)
	}
	hits := e.Find(Query{Text: "erro temporário deve ser tentado de novo com backoff", Limit: 5})
	i, j := indexOf(hits, learning), indexOf(hits, rule)
	if i < 0 || j < 0 || i != j-1 {
		t.Fatalf("the correction must sit right above the rule: %s", paths(hits))
	}
	if hits[i].Overrides != rule+"#Retry transient vs permanent" {
		t.Errorf("the correction does not say what it corrects: %+v", hits[i])
	}
	if hits[j].OverriddenBy != learning {
		t.Errorf("the rule does not say who corrects it: %+v", hits[j])
	}
	if hits[i].Score < hits[j].Score {
		t.Errorf("the correction ranks below its rule: %.2f < %.2f", hits[i].Score, hits[j].Score)
	}
}

// Narrowing the search to the packs does not hide the team's correction.
func TestCorrectionSurvivesTheAreaFilter(t *testing.T) {
	e, _ := Open(correctionFixture(t), "")
	hits := e.Find(Query{Text: "erro temporário tentado de novo", Areas: []string{docs.AreaExpertise}, Limit: 5})
	if indexOf(hits, learning) < 0 {
		t.Fatalf("the correction was filtered out: %s", paths(hits))
	}
}

// A correction of one section says nothing about the others.
func TestCorrectionIsScopedToItsSection(t *testing.T) {
	e, _ := Open(correctionFixture(t), "")
	hits := e.Find(Query{Text: "envelope do evento carrega type id payload", Limit: 5})
	j := indexOf(hits, rule)
	if j < 0 || hits[j].Title != "Contrato do evento" {
		t.Fatalf("expected the other section: %s", paths(hits))
	}
	if hits[j].OverriddenBy != "" || indexOf(hits, learning) >= 0 {
		t.Errorf("an uncorrected section was flagged: %+v", hits)
	}
}

// show names the correction on the rule's document and section, and the rule
// on the correction.
func TestShowNamesTheCorrection(t *testing.T) {
	e, _ := Open(correctionFixture(t), "")
	v, err := e.Show(rule+"#Retry transient vs permanent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if d := v.Data.(SectionData); len(d.CorrectedBy) != 1 || d.CorrectedBy[0] != learning || !strings.Contains(v.Text, learning) {
		t.Errorf("section view lacks the correction: %+v\n%s", d, v.Text)
	}
	v, _ = e.Show(rule+"#Contrato do evento", 10)
	if d := v.Data.(SectionData); len(d.CorrectedBy) != 0 {
		t.Errorf("an uncorrected section names a correction: %+v", d)
	}
	v, _ = e.Show(learning, 10)
	if d := v.Data.(DocumentData); len(d.Overrides) != 1 || !strings.Contains(v.Text, "corrects") {
		t.Errorf("the correction does not say what it corrects: %+v", d)
	}
}

func indexOf(hits []Hit, p string) int {
	for i, h := range hits {
		if h.Path == p {
			return i
		}
	}
	return -1
}

func paths(hits []Hit) string {
	var b strings.Builder
	for _, h := range hits {
		b.WriteString("\n  " + h.Path + " § " + h.Title)
	}
	return b.String()
}

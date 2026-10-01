package intake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/host"
)

// A role whose contract elicits runs a cheaper phase first: it writes the open
// decisions to a file, and the role is told to write at once.
func TestARoleThatElicitsGetsACheaperPhaseFirst(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	el, spec := r.Plan[0], r.Plan[1]
	if !el.Elicit || el.Role != "gofi-spec" || el.Tier != host.Standard || el.Base != host.Deep || el.Produces != "" || el.ReviewAfter {
		t.Fatalf("elicitation = %+v", el)
	}
	if el.ElicitFile != ".gofi/elicit/pricing-gofi-spec.json" || el.Turn != el.RoleTurn {
		t.Errorf("file %q, turns differ: %v", el.ElicitFile, el.Turn != el.RoleTurn)
	}
	for _, want := range []string{"modo elicitação", ".claude/skills/gofi-spec/reference/elicitation.md", ElicitSchema, "# Pedido montado"} {
		if !strings.Contains(el.Turn, want) {
			t.Errorf("elicitation turn lacks %q", want)
		}
	}
	if strings.HasPrefix(el.Turn, "/gofi-spec") {
		t.Error("the elicitation invokes the skill, which runs on the role's tier")
	}
	if !strings.HasPrefix(spec.Turn, "/gofi-spec ") || !strings.Contains(spec.Turn, "Decisões confirmadas") || !strings.Contains(spec.Turn, "sem entrevista") || !strings.Contains(spec.Turn, "status: proposto") {
		t.Errorf("spec turn = %q", spec.Turn)
	}
	// A wide reach, or a tier asked, does not raise what only reads and asks.
	asked := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{Tier: host.Deep})
	if asked.Plan[0].Tier != host.Standard {
		t.Errorf("an asked tier raised the elicitation: %s", asked.Plan[0].Tier)
	}
}

func TestDecisionsReachTheRole(t *testing.T) {
	root := t.TempDir()
	rel := ElicitDir + "/pricing-gofi-spec.json"
	if e, err := ReadElicitation(root, rel); e != nil || err != nil {
		t.Fatalf("nothing written: %v, %v", e, err)
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(`{"schema":"gofi.elicit/v1","questions":[
		{"text":"Listagem paginada?","options":["simples","paginada"],"recommended":"paginada","why":"lista sem limite"},
		{"id":"cache","text":"Cache nas leituras?","options":["não","sim"],"recommended":"não","why":"dado muda a cada envio"}],
		"derived":["banco: PostgreSQL (.gofi.yaml)"]}`), 0o644)
	e, err := ReadElicitation(root, rel)
	if err != nil || len(e.Questions) != 2 || e.Questions[0].ID != "q1" {
		t.Fatalf("elicitation = %+v, %v", e, err)
	}
	qs := e.AsQuestions()
	if qs[0].Options[0] != "paginada" || !strings.Contains(qs[0].Text, "recomendado: paginada") {
		t.Errorf("the recommendation is not the default: %+v", qs[0])
	}
	turn := WithDecisions("/gofi-spec # Pedido montado\n", e, map[string]string{"q1": "simples"})
	for _, want := range []string{"Listagem paginada? → simples", "Cache nas leituras? → não (assumido", "banco: PostgreSQL (.gofi.yaml)"} {
		if !strings.Contains(turn, want) {
			t.Errorf("turn lacks %q:\n%s", want, turn)
		}
	}
	ClearElicitation(root, rel)
	if e, _ := ReadElicitation(root, rel); e != nil {
		t.Error("cleared elicitation still read")
	}
	_ = os.WriteFile(path, []byte(`{"schema":"other"}`), 0o644)
	if _, err := ReadElicitation(root, rel); err == nil {
		t.Error("an unknown schema was read")
	}
}

// An implementation may stop on a decision the spec does not cover: it is
// told where to write it, and the plan carries what follows — the spec
// records the answer on a cheaper tier, and the implementation runs again.
func TestABlockedImplementationRecordsTheAnswerInTheSpec(t *testing.T) {
	r := runIntake(t, project(t), "corrija o bug no cancelamento de pedidos", Options{})
	var eng Phase
	for _, ph := range r.Plan {
		if ph.Role == "gofi-eng" {
			eng = ph
		}
	}
	if eng.ElicitFile == "" || len(eng.OnBlock) != 2 {
		t.Fatalf("eng = %+v", eng)
	}
	turn := eng.Turn
	if eng.RoleTurn != "" {
		turn = eng.RoleTurn
	}
	if !strings.Contains(turn, eng.ElicitFile) || !strings.Contains(turn, "Não suponha nem pergunte") {
		t.Errorf("eng is not told how to stop: %q", turn)
	}
	rec, again := eng.OnBlock[0], eng.OnBlock[1]
	if rec.Role != "gofi-spec" || rec.Tier != host.Standard || rec.Base != host.Deep || !strings.Contains(rec.Turn, "modo registro") || !strings.Contains(rec.Turn, "sem subir a versão") {
		t.Errorf("record = %+v", rec)
	}
	if again.Role != "gofi-eng" || len(again.OnBlock) != 0 || strings.Contains(again.Turn, "Não suponha nem pergunte") {
		t.Errorf("again = %+v", again)
	}
	var qa Phase
	for _, ph := range r.Plan {
		if ph.Role == "gofi-qa" {
			qa = ph
		}
	}
	if qa.ElicitFile != "" || qa.OnBlock != nil {
		t.Errorf("the audit, which writes nothing, got a block: %+v", qa)
	}
}

// The documentation phase stops on what the code does not say, and runs
// again with the answers — no spec record in between.
func TestTheDocPhaseAsksAndRunsAgain(t *testing.T) {
	r := runIntake(t, project(t), "documente o pricing", Options{})
	var doc Phase
	for _, ph := range r.Plan {
		if ph.Role == "gofi-doc" {
			doc = ph
		}
	}
	if doc.ElicitFile == "" || len(doc.OnBlock) != 1 || doc.OnBlock[0].Role != "gofi-doc" || len(doc.OnBlock[0].OnBlock) != 0 {
		t.Fatalf("doc = %+v", doc)
	}
	if !strings.Contains(doc.Turn, "Não suponha nem pergunte") || !strings.Contains(doc.OnBlock[0].Turn, "Decisões confirmadas") {
		t.Errorf("turns: %q / %q", doc.Turn[:120], doc.OnBlock[0].Turn[:120])
	}
}

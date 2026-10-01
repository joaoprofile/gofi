package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/intake"
)

func TestARunIsRecordedStepByStep(t *testing.T) {
	root := t.TempDir()
	res := &intake.Result{Request: "Altere o fluxo de pricing!", Intent: "change", Artifact: "code",
		Context: &intake.Context{Name: "pricing"}, Signals: []string{"intenção change"},
		Spent: &intake.Spent{USD: 0.005}}
	run, err := Start(root, "ask", res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(run.Path(), "-altere-o-fluxo-de-pricing.json") || !strings.Contains(run.Path(), filepath.FromSlash(Dir)) {
		t.Errorf("path = %s", run.Path())
	}
	// On record before any phase: a plan cut short still leaves its trace.
	if r, err := Load(run.Path()); err != nil || r.Outcome != OutcomeRunning || r.IntakeUSD != 0.005 {
		t.Fatalf("started run = %+v, %v", r, err)
	}
	_ = run.Phase(Phase{Role: "gofi-spec", Tier: "deep", CostUSD: 0.10, OK: true})
	_ = run.Phase(Phase{Role: "gofi-eng", Tier: "standard", CostUSD: 0.05, OK: true})
	if err := run.Finish(OutcomeDone, "aprovado"); err != nil {
		t.Fatal(err)
	}
	r, err := Load(run.Path())
	if err != nil {
		t.Fatal(err)
	}
	if r.Schema != Schema || len(r.Phases) != 2 || r.Outcome != OutcomeDone || r.Verdict != "aprovado" || r.CostUSD < 0.149 || r.Context != "pricing" {
		t.Errorf("run = %+v", r)
	}
	if _, err := os.Stat(run.Path() + ".tmp"); !os.IsNotExist(err) {
		t.Error("a temporary file was left behind")
	}
}

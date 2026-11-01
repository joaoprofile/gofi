package intake

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/host"
)

// A failed audit sends the implementation back one tier up, then audits again
// — once, decided with the plan.
func TestAFailedAuditIsRetriedOneTierUp(t *testing.T) {
	root := project(t)
	r := runIntake(t, root, "altere o fluxo de pricing, adicionando um rate limit", Options{})
	qa := r.Plan[len(r.Plan)-1]
	if qa.Role != "gofi-qa" || len(qa.OnReject) != 2 {
		t.Fatalf("qa = %+v", qa)
	}
	fix, again := qa.OnReject[0], qa.OnReject[1]
	if fix.Role != "gofi-eng" || fix.Tier != host.Deep || fix.Base != host.Standard || !strings.HasPrefix(fix.RoleTurn, "Siga o papel") || !strings.Contains(fix.RoleTurn, "reprovou") {
		t.Errorf("fix = %+v", fix)
	}
	if again.Role != "gofi-qa" || len(again.OnReject) != 0 || !strings.HasPrefix(again.Turn, "/gofi-qa ") {
		t.Errorf("audit again = %+v", again)
	}
	if r.VerdictFile != ".claude/memory/contexts/pricing.md" {
		t.Errorf("verdict file = %q", r.VerdictFile)
	}

	before := Verdict(root, r)
	if Rejected(root, r, before) {
		t.Error("an audit that wrote nothing rejected")
	}
	path := filepath.Join(root, filepath.FromSlash(r.VerdictFile))
	_ = os.WriteFile(path, []byte("---\ncontexto: pricing\nstatus: reprovado\n---\n# pricing\n"), 0o644)
	if !Rejected(root, r, before) {
		t.Error("a reprovado verdict did not reject")
	}
	// The same verdict, left by an earlier run, is not this audit's.
	if Rejected(root, r, Verdict(root, r)) {
		t.Error("an unchanged verdict rejected")
	}
	_ = os.WriteFile(path, []byte("---\nstatus: aprovado\n---\n"), 0o644)
	if Rejected(root, r, before) {
		t.Error("an aprovado verdict rejected")
	}
}

// Only a clean audit is accepted: aprovado with majors or minors still open
// reopens the implementation, as /gofi-full does.
func TestCaveatsReopenTheDelivery(t *testing.T) {
	root := project(t)
	r := runIntake(t, root, "altere o fluxo de pricing, adicionando um rate limit", Options{})
	before := Verdict(root, r)
	path := filepath.Join(root, filepath.FromSlash(r.VerdictFile))
	for _, tc := range []struct {
		memory string
		want   bool
	}{
		{"---\nstatus: aprovado\nressalvas: 2\n---\n", true},
		{"---\nstatus: aprovado\nressalvas: \"0\"\n---\n", false},
		{"---\nstatus: aprovado\nressalvas: 0\n---\n", false},
		// Unreadable is not clean.
		{"---\nstatus: aprovado\nressalvas: alguns\n---\n", true},
		// A memory written before the field existed keeps meaning what it meant.
		{"---\nstatus: aprovado\n---\n", false},
		// The count belongs to an approval; it does not turn another status into one.
		{"---\nstatus: implementado\nressalvas: 3\n---\n", false},
	} {
		_ = os.WriteFile(path, []byte(tc.memory), 0o644)
		if got := Rejected(root, r, before); got != tc.want {
			t.Errorf("%q: rejected = %v, want %v", tc.memory, got, tc.want)
		}
	}
}

// An audit alone, with no implementation before it, has nothing to retry.
func TestAnAuditAloneHasNoRetry(t *testing.T) {
	r := runIntake(t, project(t), "audite o contexto de pricing", Options{})
	if len(r.Plan) != 1 || r.Plan[0].OnReject != nil || r.VerdictFile != "" {
		t.Errorf("plan = %+v, verdict file %q", r.Plan, r.VerdictFile)
	}
}

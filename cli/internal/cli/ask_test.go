package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/runs"
)

// askSession records the turns and models it got, and ends each turn with
// the next scripted outcome.
type askSession struct {
	sent   []string
	opened []string // the model of each session opened, one a phase
	fail   int      // the turn, 1-based, that fails; 0 for none
	onSend func(prompt string)
}

func (s *askSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	s.sent = append(s.sent, t.Prompt)
	if s.onSend != nil {
		s.onSend(t.Prompt)
	}
	ch := make(chan engine.Event, 2)
	ch <- engine.Message{Blocks: []engine.Block{{Kind: engine.BlockText, Text: "feito"}}}
	if len(s.sent) == s.fail {
		ch <- engine.Done{IsError: true, Err: "boom"}
	} else {
		ch <- engine.Done{Duration: time.Second, CostUSD: 0.01}
	}
	close(ch)
	return ch, nil
}

// open is the conductor's session factory: every session is this recorder.
func (s *askSession) open(model string) engine.Session {
	s.opened = append(s.opened, model)
	return s
}
func (s *askSession) Cancel()      {}
func (s *askSession) Close() error { return nil }

func askPlan(raiseEng bool) *intake.Result {
	eng := intake.Phase{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard}
	if raiseEng {
		eng.Tier, eng.TierWhy = host.Deep, "acoplado"
	}
	return &intake.Result{Request: "altere o pricing", Brief: "# Pedido montado\n",
		Plan: []intake.Phase{
			{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep},
			eng,
			{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard},
		}}
}

func tiers(t host.Tier) string { return "m-" + string(t) }

func TestConductRunsEveryPhase(t *testing.T) {
	s := &askSession{}
	var out bytes.Buffer
	if err := conduct(&out, askPlan(false), s.open, "session", tiers, func(string, string) bool { return true }, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 3 || !strings.HasPrefix(s.sent[0], "/gofi-spec ") || !strings.HasPrefix(s.sent[2], "/gofi-qa ") {
		t.Errorf("sent = %q", s.sent)
	}
	if strings.Join(s.opened, ",") != "session,session,session" {
		t.Errorf("each phase opens its own session on the session's model: %v", s.opened)
	}
}

// A review stop that is not approved ends the run after the spec.
func TestConductStopsForReview(t *testing.T) {
	s := &askSession{}
	var stops []string
	err := conduct(&bytes.Buffer{}, askPlan(false), s.open, "session", tiers, func(done, next string) bool {
		stops = append(stops, done+"→"+next)
		return false
	}, nil, "", nil)
	if err != nil || len(s.sent) != 1 || len(stops) != 1 || stops[0] != "spec→gofi-eng" {
		t.Errorf("err=%v sent=%d stops=%v", err, len(s.sent), stops)
	}
}

// A raised phase runs in a session on its tier's model, as the role; the
// next phase is back on the session's model.
func TestConductRaisesThenRestores(t *testing.T) {
	s := &askSession{}
	if err := conduct(&bytes.Buffer{}, askPlan(true), s.open, "session", tiers, func(string, string) bool { return true }, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s.sent[1], "Siga o papel descrito em .claude/skills/gofi-eng/SKILL.md") {
		t.Errorf("raised phase = %q", s.sent[1])
	}
	if want := []string{"session", "m-deep", "session"}; strings.Join(s.opened, ",") != strings.Join(want, ",") {
		t.Errorf("sessions = %v, want %v", s.opened, want)
	}
}

func TestConductStopsOnAFailedPhase(t *testing.T) {
	s := &askSession{fail: 2}
	err := conduct(&bytes.Buffer{}, askPlan(false), s.open, "session", tiers, func(string, string) bool { return true }, nil, "", nil)
	if err == nil || !strings.Contains(err.Error(), "gofi-eng") || len(s.sent) != 2 {
		t.Errorf("err=%v sent=%d", err, len(s.sent))
	}
}

func TestConductSendsARequestWithNoPlanAsItIs(t *testing.T) {
	s := &askSession{}
	if err := conduct(&bytes.Buffer{}, &intake.Result{Request: "oi"}, s.open, "session", nil, nil, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || s.sent[0] != "oi" {
		t.Errorf("sent = %q", s.sent)
	}
}

// retryPlan is a fix with its audit's retry, as the intake fills it.
func retryPlan() *intake.Result {
	r := &intake.Result{Request: "corrija o pricing", Brief: "# Pedido montado\n", VerdictFile: "mem/pricing.md",
		Plan: []intake.Phase{
			{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-eng corrija"},
			{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-qa audite"},
		}}
	r.Plan[1].OnReject = []intake.Phase{
		{Role: "gofi-eng", Produces: "code", Tier: host.Deep, Base: host.Standard, Turn: "/gofi-eng refaça", RoleTurn: "Siga o papel: refaça"},
		{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-qa de novo"},
	}
	return r
}

// verdicts has each audit write the next verdict into the context's memory.
func verdicts(t *testing.T, root string, v ...string) func(string) {
	path := filepath.Join(root, "mem", "pricing.md")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	n := 0
	return func(prompt string) {
		if strings.HasPrefix(prompt, "/gofi-qa") && n < len(v) {
			_ = os.WriteFile(path, []byte(fmt.Sprintf("---\nstatus: %s\nrodada: %d\n---\n", v[n], n)), 0o644)
			n++
		}
	}
}

// A failed audit runs the implementation again one tier up, then audits again;
// the run records every phase and how it ended.
func TestConductRetriesAFailedAuditOneTierUp(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: verdicts(t, root, "reprovado", "aprovado")}
	res := retryPlan()
	rec, err := runs.Start(root, "ask", res)
	if err != nil {
		t.Fatal(err)
	}
	if err := conduct(&bytes.Buffer{}, res, s.open, "session", tiers, func(string, string) bool { return true }, nil, root, rec); err != nil {
		t.Fatal(err)
	}
	want := []string{"/gofi-eng corrija", "/gofi-qa audite", "Siga o papel: refaça", "/gofi-qa de novo"}
	if strings.Join(s.sent, "|") != strings.Join(want, "|") {
		t.Errorf("sent = %q", s.sent)
	}
	if strings.Join(s.opened, ",") != "session,session,m-deep,session" {
		t.Errorf("sessions = %v", s.opened)
	}
	if len(res.Plan) != 2 {
		t.Error("the retry changed the plan the intake returned")
	}
	got, _ := runs.Load(rec.Path())
	if got.Outcome != runs.OutcomeDone || got.Verdict != "aprovado" || len(got.Phases) != 4 || got.Phases[2].Model != "m-deep" || got.CostUSD < 0.039 {
		t.Errorf("run = %+v", got)
	}
}

// Failed again after the retry: the plan stops, escalated to the person.
func TestConductStopsWhenTheRetryFailsToo(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: verdicts(t, root, "reprovado", "reprovado")}
	res := retryPlan()
	rec, _ := runs.Start(root, "ask", res)
	if err := conduct(&bytes.Buffer{}, res, s.open, "session", tiers, func(string, string) bool { return true }, nil, root, rec); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 4 {
		t.Errorf("sent = %q", s.sent)
	}
	if got, _ := runs.Load(rec.Path()); got.Outcome != runs.OutcomeEscalated || got.Verdict != "reprovado" {
		t.Errorf("run = %+v", got)
	}
}

// An audit that approves runs no retry.
func TestConductApprovedAuditEndsThePlan(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: verdicts(t, root, "aprovado")}
	if err := conduct(&bytes.Buffer{}, retryPlan(), s.open, "session", tiers, func(string, string) bool { return true }, nil, root, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 2 {
		t.Errorf("sent = %q", s.sent)
	}
}

// elicitPlan is a spec prepared by its elicitation, as the intake fills it.
func elicitPlan() *intake.Result {
	return &intake.Result{Request: "especifique o catálogo", Brief: "# Pedido montado\n",
		Plan: []intake.Phase{
			{Role: "gofi-spec", Tier: host.Standard, Base: host.Deep, Elicit: true, ElicitFile: ".gofi/elicit/catalog-gofi-spec.json",
				Turn: "elicite", RoleTurn: "elicite"},
			{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep, Turn: "/gofi-spec escreva"},
		}}
}

// writesElicitation has the elicitation turn leave two open decisions.
func writesElicitation(root string) func(string) {
	return func(prompt string) {
		if prompt == "elicite" {
			path := filepath.Join(root, ".gofi", "elicit", "catalog-gofi-spec.json")
			_ = os.MkdirAll(filepath.Dir(path), 0o755)
			_ = os.WriteFile(path, []byte(`{"schema":"gofi.elicit/v1","questions":[
				{"id":"list","text":"Listagem paginada?","options":["simples","paginada"],"recommended":"paginada","why":"sem limite"},
				{"id":"cache","text":"Cache?","options":["não","sim"],"recommended":"não","why":"muda sempre"}],"derived":["banco: PostgreSQL (.gofi.yaml)"]}`), 0o644)
		}
	}
}

// The elicitation runs on its cheaper tier; its decisions are asked, and the
// role gets them at the end of its turn — answered or assumed.
func TestConductAsksWhatTheElicitationLeft(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: writesElicitation(root)}
	var asked []intake.Question
	elicit := func(qs []intake.Question) (map[string]string, bool) {
		asked = qs
		return map[string]string{"list": "simples"}, true
	}
	res := elicitPlan()
	rec, _ := runs.Start(root, "ask", res)
	if err := conduct(&bytes.Buffer{}, res, s.open, "session", tiers, func(string, string) bool { return true }, elicit, root, rec); err != nil {
		t.Fatal(err)
	}
	if strings.Join(s.opened, ",") != "m-standard,session" {
		t.Errorf("sessions = %v", s.opened)
	}
	if len(asked) != 2 || asked[0].Options[0] != "paginada" {
		t.Errorf("asked = %+v", asked)
	}
	spec := s.sent[1]
	for _, want := range []string{"/gofi-spec escreva", "Listagem paginada? → simples", "Cache? → não (assumido", "banco: PostgreSQL"} {
		if !strings.Contains(spec, want) {
			t.Errorf("spec turn lacks %q:\n%s", want, spec)
		}
	}
	got, _ := runs.Load(rec.Path())
	if got.Phases[0].Asked != 2 || got.Phases[0].Assumed != 1 {
		t.Errorf("run phase = %+v", got.Phases[0])
	}
}

// Unanswered in a script, the plan stops before the expensive role.
func TestConductStopsForTheElicitationsAnswers(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: writesElicitation(root)}
	stop := func([]intake.Question) (map[string]string, bool) { return nil, false }
	if err := conduct(&bytes.Buffer{}, elicitPlan(), s.open, "session", tiers, func(string, string) bool { return true }, stop, root, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Errorf("the role ran without the answers: %q", s.sent)
	}
}

// blockPlan is a fix whose implementation may stop on a decision.
func blockPlan() *intake.Result {
	eng := intake.Phase{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-eng corrija",
		ElicitFile: ".gofi/elicit/order-gofi-eng.json"}
	again := eng
	again.Turn = "/gofi-eng retome"
	eng.OnBlock = []intake.Phase{
		{Role: "gofi-spec", Tier: host.Standard, Base: host.Deep, Turn: "registre", RoleTurn: "registre"},
		again,
	}
	return &intake.Result{Request: "corrija o cancelamento", Brief: "# Pedido montado\n", Plan: []intake.Phase{eng}}
}

// blocks has each turn in stops write a question to the implementation's file.
func blocks(root string, stops ...string) func(string) {
	return func(prompt string) {
		for _, s := range stops {
			if prompt == s {
				path := filepath.Join(root, ".gofi", "elicit", "order-gofi-eng.json")
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, []byte(`{"schema":"gofi.elicit/v1","questions":[{"id":"idx","text":"Perfil de acesso?","options":["append-only","hot UPDATE"],"recommended":"hot UPDATE","why":"cancelamento atualiza"}]}`), 0o644)
			}
		}
	}
}

// A stopped implementation's question is asked; the spec records the answer
// on a cheaper tier, and the implementation runs again.
func TestConductRecordsABlockedDecisionInTheSpec(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: blocks(root, "/gofi-eng corrija")}
	answer := func([]intake.Question) (map[string]string, bool) {
		return map[string]string{"idx": "append-only"}, true
	}
	if err := conduct(&bytes.Buffer{}, blockPlan(), s.open, "session", tiers, func(string, string) bool { return true }, answer, root, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 3 || !strings.HasPrefix(s.sent[1], "registre") || !strings.Contains(s.sent[1], "Perfil de acesso? → append-only") || s.sent[2] != "/gofi-eng retome" {
		t.Errorf("sent = %q", s.sent)
	}
	if strings.Join(s.opened, ",") != "session,m-standard,session" {
		t.Errorf("sessions = %v", s.opened)
	}
}

// Stopped again after the spec was completed, the plan stops.
func TestConductStopsWhenBlockedAgain(t *testing.T) {
	root := t.TempDir()
	s := &askSession{onSend: blocks(root, "/gofi-eng corrija", "/gofi-eng retome")}
	answer := func([]intake.Question) (map[string]string, bool) {
		return map[string]string{"idx": "append-only"}, true
	}
	var out bytes.Buffer
	if err := conduct(&out, blockPlan(), s.open, "session", tiers, func(string, string) bool { return true }, answer, root, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 3 || strings.Contains(out.String(), "plano concluído") || strings.Contains(out.String(), "plan done") {
		t.Errorf("sent = %q\n%s", s.sent, out.String())
	}
}

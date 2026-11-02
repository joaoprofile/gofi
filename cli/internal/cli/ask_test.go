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

// A review stop that is not approved ends the run after the spec.
func TestConductStopsForReview(t *testing.T) {
	s := &askSession{}
	var stops []string
	err := conduct(&bytes.Buffer{}, askPlan(false), s.open, "session", tiers, func(done, next string) bool {
		stops = append(stops, done+"→"+next)
		return false
	}, nil, nil, "", nil)
	if err != nil || len(s.sent) != 1 || len(stops) != 1 || stops[0] != "spec→gofi-eng" {
		t.Errorf("err=%v sent=%d stops=%v", err, len(s.sent), stops)
	}
}

func TestConductStopsOnAFailedPhase(t *testing.T) {
	s := &askSession{fail: 1}
	err := conduct(&bytes.Buffer{}, askPlan(false), s.open, "session", tiers, func(string, string) bool { return true }, nil, nil, "", nil)
	if err == nil || !strings.Contains(err.Error(), "gofi-spec") || len(s.sent) != 1 {
		t.Errorf("err=%v sent=%d", err, len(s.sent))
	}
}

func TestConductSendsARequestWithNoPlanAsItIs(t *testing.T) {
	s := &askSession{}
	if err := conduct(&bytes.Buffer{}, &intake.Result{Request: "oi"}, s.open, "session", nil, nil, nil, nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 || s.sent[0] != "oi" {
		t.Errorf("sent = %q", s.sent)
	}
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
	if err := conduct(&bytes.Buffer{}, res, s.open, "session", tiers, func(string, string) bool { return true }, elicit, nil, root, rec); err != nil {
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
	if err := conduct(&bytes.Buffer{}, elicitPlan(), s.open, "session", tiers, func(string, string) bool { return true }, stop, nil, root, nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Errorf("the role ran without the answers: %q", s.sent)
	}
}

// Past the contract, what the plan builds is the loop's: the conductor hands
// it to the executor once, and the audit the plan listed is the flow's — no
// turn runs for either.
func TestConductHandsTheDeliveryToTheLoop(t *testing.T) {
	s := &askSession{}
	plan := askPlan(false)
	plan.Plan = append(plan.Plan, intake.Phase{Role: "gofi-doc", Produces: "docs", Tier: host.Light, Base: host.Light})
	var flows []string
	handoff := func(flow string) (string, error) { flows = append(flows, flow); return "accepted", nil }
	if err := conduct(&bytes.Buffer{}, plan, s.open, "session", tiers, func(string, string) bool { return true }, nil, handoff, "", nil); err != nil {
		t.Fatal(err)
	}
	if len(flows) != 1 || flows[0] != "delivery" {
		t.Errorf("handoffs = %v", flows)
	}
	var roles []string
	for _, p := range s.sent {
		roles = append(roles, strings.Fields(p)[0])
	}
	if strings.Join(roles, " ") != "/gofi-spec /gofi-doc" {
		t.Errorf("turns = %v: the spec and the docs run as turns, the delivery does not", roles)
	}

	// A delivery that is not accepted ends the plan there.
	s = &askSession{}
	refused := func(string) (string, error) { return "impasse", nil }
	if err := conduct(&bytes.Buffer{}, plan, s.open, "session", tiers, func(string, string) bool { return true }, nil, refused, "", nil); err != nil {
		t.Fatal(err)
	}
	if len(s.sent) != 1 {
		t.Errorf("turns after an impasse: %d", len(s.sent))
	}
}

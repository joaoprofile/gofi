package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// intakeCall is one call the chat made to the intake.
type intakeCall struct {
	text    string
	answers map[string]string
	proceed bool
}

// planModel is a chat whose intake answers from a script, one result a call.
func planModel(t *testing.T, script []*intake.Result, done engine.Event) (model, *[]*fakeSession, *[]intakeCall) {
	t.Helper()
	var sessions []*fakeSession
	var calls []intakeCall
	if done == nil {
		done = engine.Done{Duration: time.Second}
	}
	m := newModel(Options{
		Root:  t.TempDir(),
		Model: "session-model",
		NewSession: func(string) engine.Session {
			s := &fakeSession{script: []engine.Event{engine.Meta{SessionID: "s1"}, done}}
			sessions = append(sessions, s)
			return s
		},
		Intake: func(text string, answers map[string]string, proceed bool) (*intake.Result, error) {
			calls = append(calls, intakeCall{text, answers, proceed})
			r := script[0]
			if len(script) > 1 {
				script = script[1:]
			}
			return r, nil
		},
		TierModel: func(t host.Tier) string { return "model-for-" + string(t) },
		Deliver:   fakeDeliver(&lastDeliveries, "accepted"),
	}, false)
	lastDeliveries = nil
	return m, &sessions, &calls
}

// lastDeliveries records what the plan of the last planModel handed to the
// loop, as "flow context".
var lastDeliveries []string

// fakeDeliver stands in for gofi run --context: it records the handoff and
// ends with the outcome given.
func fakeDeliver(record *[]string, outcome string) func(flow, ctx string) Delivery {
	return func(flow, ctx string) Delivery {
		*record = append(*record, flow+" "+ctx)
		ch := make(chan loop.Event, 3)
		ch <- loop.Event{Ev: "node.start", Node: "implement", Run: "r-1", CostUSD: 0.4}
		ch <- loop.Event{Ev: "run.result", Node: "r-1", Why: outcome, Run: "r-1", CostUSD: 1.25}
		close(ch)
		return Delivery{Events: ch, Wait: func() error { return nil }, Stop: func() {}}
	}
}

// pump feeds the messages a command produces back into the chat until none
// is left that belongs to a delivery, as the program would.
func pump(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	for i := 0; cmd != nil && i < 50; i++ {
		msg := findMsg(cmd, func(msg tea.Msg) bool {
			switch msg.(type) {
			case deliveryEvent, deliveryDone:
				return true
			}
			return false
		})
		if msg == nil {
			return m
		}
		next, c := m.Update(msg)
		m, cmd = next.(model), c
	}
	return m
}

// ask types text and delivers the intake's answer, as the program would.
func ask(t *testing.T, m model, text string) model {
	t.Helper()
	m.input.SetValue(text)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.planning {
		return m
	}
	return deliverIntake(t, m, cmd)
}

// deliverIntake runs the intake command the chat issued and feeds its answer.
func deliverIntake(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	msg := findMsg(cmd, func(msg tea.Msg) bool { _, ok := msg.(intakeDone); return ok })
	if msg == nil {
		t.Fatal("the chat did not run the intake")
	}
	next, _ := m.Update(msg)
	return next.(model)
}

// findMsg runs a command tree and returns the first message that matches.
func findMsg(cmd tea.Cmd, match func(tea.Msg) bool) tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch batch := msg.(type) {
	case tea.BatchMsg:
		for _, c := range batch {
			if found := findMsg(c, match); found != nil {
				return found
			}
		}
		return nil
	}
	// tea.Sequence wraps its commands in an unexported slice type.
	if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		for i := 0; i < v.Len(); i++ {
			if found := findMsg(v.Index(i).Interface().(tea.Cmd), match); found != nil {
				return found
			}
		}
		return nil
	}
	if match(msg) {
		return msg
	}
	return nil
}

// phases is what the plan's phases sent, in order: each phase runs in a
// session of its own, after the chat's (sessions[0]).
func phases(sessions []*fakeSession) (sent, models []string) {
	for _, s := range sessions[1:] {
		sent = append(sent, s.sent...)
		models = append(models, s.model)
	}
	return sent, models
}

// enterCmd presses enter and returns what the chat issued with it.
func enterCmd(m model) (model, tea.Cmd) {
	m.input.SetValue("")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(model), cmd
}

func enter(m model) model {
	m.input.SetValue("")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(model)
}

func threePhases() *intake.Result {
	return &intake.Result{
		Request: "altere o fluxo de pricing", Intent: "change", Artifact: "code",
		Context: &intake.Context{Name: "pricing", Has: []string{"spec", "code"}},
		Plan: []intake.Phase{
			{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"},
			{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, TierWhy: "contrato"},
			{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, TierWhy: "contrato"},
		},
		Brief: "# Pedido montado\n\n**Objetivo:** altere o fluxo de pricing\n",
	}
}

// Free text is planned, then conducted: each phase one turn with its skill,
// a stop after the spec for review, and the next phases on their own.
func TestFreeTextIsPlannedAndConducted(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{threePhases()}, nil)
	m = ask(t, m, "altere o fluxo de pricing")
	if len(*calls) != 1 {
		t.Fatalf("intake calls = %d", len(*calls))
	}
	sent, _ := phases(*sessions)
	if len(sent) != 1 || !strings.HasPrefix(sent[0], "/gofi-spec ") || !strings.Contains(sent[0], "Pedido montado") {
		t.Fatalf("first phase = %q", sent)
	}
	m = drain(t, m)
	if sent, _ = phases(*sessions); m.wait != waitContinue || len(sent) != 1 {
		t.Fatalf("after the spec the chat must wait for review: wait=%q sent=%d", m.wait, len(sent))
	}
	// Past the review, what the plan builds is the loop's: one handoff, and
	// no turn runs for the implementation or its audit.
	m, c := enterCmd(m)
	m = pump(t, m, c)
	sent, _ = phases(*sessions)
	if len(sent) != 1 || len(lastDeliveries) != 1 || lastDeliveries[0] != "delivery pricing" {
		t.Fatalf("phases sent = %q, deliveries = %v", sent, lastDeliveries)
	}
	if len((*sessions)[0].sent) != 0 || !(*sessions)[1].closed {
		t.Errorf("the spec runs in a session of its own and is closed: chat sent %q", (*sessions)[0].sent)
	}
	if m.plan != nil || m.wait != waitNone {
		t.Errorf("the plan should be over: %+v wait=%q", m.plan, m.wait)
	}
}

// A question is answered by its option's number; the answer goes to the
// intake's last round, and the plan waits for confirmation before it runs.
func TestAnsweredQuestionThenConfirm(t *testing.T) {
	first := &intake.Result{Request: "altere as regras", Intent: "change", Artifact: "code", Ask: true,
		Questions: []intake.Question{{ID: "context", Text: "Em qual contexto?", Options: []string{"invoice", "shipping"}}}}
	second := &intake.Result{Request: "altere as regras", Intent: "change", Artifact: "code",
		Context: &intake.Context{Name: "shipping"},
		Plan:    []intake.Phase{{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard}}}
	m, sessions, calls := planModel(t, []*intake.Result{first, second}, nil)
	m = ask(t, m, "altere as regras")
	if sent, _ := phases(*sessions); m.wait != waitAnswer || len(sent) != 0 {
		t.Fatalf("the chat must ask before sending: wait=%q", m.wait)
	}
	m = ask(t, m, "2")
	if len(*calls) != 2 || (*calls)[1].answers["context"] != "shipping" {
		t.Fatalf("the answer did not reach the intake: %+v", *calls)
	}
	if m.wait != waitConfirm {
		t.Fatalf("an answered plan waits for confirmation, wait=%q", m.wait)
	}
	m, c := enterCmd(m)
	m = pump(t, m, c)
	if sent, _ := phases(*sessions); len(sent) != 0 || len(lastDeliveries) != 1 || lastDeliveries[0] != "delivery shipping" {
		t.Errorf("sent = %q, deliveries = %v", sent, lastDeliveries)
	}
}

// A phase raised above its contract runs in a session on the raised tier's
// model, as the role — not the skill, whose own model would win — and the
// chat's session keeps its model.
func TestRaisedPhaseRunsOnItsTier(t *testing.T) {
	r := &intake.Result{Request: "documente o cancelamento", Intent: "document", Artifact: "docs",
		Plan: []intake.Phase{{Role: "gofi-doc", Produces: "docs", Tier: host.Standard, Base: host.Light, TierWhy: "acoplado a 3 contextos"}}}
	m, sessions, _ := planModel(t, []*intake.Result{r}, nil)
	m = ask(t, m, "documente o cancelamento")
	sent, models := phases(*sessions)
	if len(models) != 1 || models[0] != "model-for-standard" || !strings.HasPrefix(sent[0], "Siga o papel descrito em .claude/skills/gofi-doc/SKILL.md") {
		t.Fatalf("models=%q sent=%q", models, sent)
	}
	m = drain(t, m)
	if (*sessions)[0].model != "" || m.plan != nil || m.modelN != "session-model" {
		t.Errorf("the chat's session changed: model %q, shown %q", (*sessions)[0].model, m.modelN)
	}
}

// A skill invoked outright and /send never go through the intake.
func TestSkillsAndSendSkipTheIntake(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{threePhases()}, nil)
	m = drain(t, ask(t, m, "/gofi-eng implemente o cancelamento"))
	m = drain(t, ask(t, m, "/send altere o fluxo, sem plano"))
	if len(*calls) != 0 {
		t.Errorf("the intake ran: %+v", *calls)
	}
	s := (*sessions)[0]
	if len(s.sent) != 2 || s.sent[1] != "altere o fluxo, sem plano" {
		t.Errorf("sent = %q", s.sent)
	}
	_ = m
}

// A phase that fails stops the plan: the next one does not start on a broken
// state.
func TestFailedPhaseStopsThePlan(t *testing.T) {
	m, sessions, _ := planModel(t, []*intake.Result{threePhases()}, engine.Done{IsError: true, Err: "boom"})
	m = drain(t, ask(t, m, "altere o fluxo de pricing"))
	if sent, _ := phases(*sessions); len(sent) != 1 || m.plan != nil {
		t.Errorf("sent=%d plan=%+v", len(sent), m.plan)
	}
}

// The elicitation runs first; its decisions are asked in the chat, one at a
// time, and reach the role at the end of its turn.
func TestElicitedDecisionsAreAskedBeforeTheRole(t *testing.T) {
	r := &intake.Result{Request: "especifique o catálogo", Intent: "create", Artifact: "spec",
		Context: &intake.Context{Name: "catalog", Has: []string{"prd"}},
		Plan: []intake.Phase{
			{Role: "gofi-spec", Tier: host.Standard, Base: host.Deep, Elicit: true, ElicitFile: ".gofi/elicit/catalog-gofi-spec.json", Turn: "elicite", RoleTurn: "elicite"},
			{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep, Turn: "/gofi-spec escreva"},
		}}
	m, sessions, _ := planModel(t, []*intake.Result{r}, nil)
	root := m.opts.Root
	next := m.opts.NewSession
	m.opts.NewSession = func(id string) engine.Session {
		s := next(id).(*fakeSession)
		s.onSend = func(p string) {
			if p == "elicite" {
				path := filepath.Join(root, ".gofi", "elicit", "catalog-gofi-spec.json")
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, []byte(`{"schema":"gofi.elicit/v1","questions":[
					{"id":"list","text":"Listagem paginada?","options":["simples","paginada"],"recommended":"paginada","why":"sem limite"},
					{"id":"cache","text":"Cache?","options":["não","sim"],"recommended":"não","why":"muda sempre"}],"derived":[]}`), 0o644)
			}
		}
		return s
	}
	m = drain(t, ask(t, m, "especifique o catálogo"))
	if m.wait != waitElicit {
		t.Fatalf("the chat must ask the decisions: wait=%q", m.wait)
	}
	_, models := phases(*sessions)
	if models[0] != "model-for-standard" {
		t.Errorf("the elicitation ran on %q", models[0])
	}
	m = drain(t, ask(t, m, "2")) // simples — the second option, after the recommendation
	if m.wait != waitElicit {
		t.Fatalf("the second decision was not asked: wait=%q", m.wait)
	}
	m = drain(t, ask(t, m, "seguir"))
	sent, _ := phases(*sessions)
	if len(sent) != 2 {
		t.Fatalf("sent = %q", sent)
	}
	for _, want := range []string{"/gofi-spec escreva", "Listagem paginada? → simples", "Cache? → não (assumido"} {
		if !strings.Contains(sent[1], want) {
			t.Errorf("role turn lacks %q:\n%s", want, sent[1])
		}
	}
}

// A message that is no task goes to the engine as typed, with no plan block.
func TestChatIsSentAsTyped(t *testing.T) {
	m, sessions, _ := planModel(t, []*intake.Result{{Request: "oi", Chat: true}}, nil)
	m = drain(t, ask(t, m, "oi"))
	if s := (*sessions)[0]; len(s.sent) != 1 || s.sent[0] != "oi" || m.plan != nil {
		t.Errorf("sent = %q, plan = %+v", s.sent, m.plan)
	}
}

// A question about the project goes with the sections that answer it.
func TestAQuestionGoesWithItsPointers(t *testing.T) {
	m, sessions, _ := planModel(t, []*intake.Result{{Request: "explique o pricing", Intent: "explain", Turn: "explique o pricing\n\n- specs/pricing/sdd-pricing.md L1-9"}}, nil)
	m = drain(t, ask(t, m, "explique o pricing"))
	if s := (*sessions)[0]; len(s.sent) != 1 || !strings.Contains(s.sent[0], "specs/pricing/sdd-pricing.md") {
		t.Errorf("sent = %q", s.sent)
	}
}

// A spec is reviewed at its review stop: what the reviewer found comes
// before the stop, and the approval stays the person's.
func TestASpecIsReviewedBeforeTheApproval(t *testing.T) {
	m, _, _ := planModel(t, []*intake.Result{threePhases()}, nil)
	var asked []string
	m.opts.Review = func(ctx string) ([]string, int, error) {
		asked = append(asked, ctx)
		return []string{"[major] ambiguidade — deltas/pricing.md#REQ-01.2"}, 1, nil
	}
	m = ask(t, m, "altere o fluxo de pricing")
	// The spec's turn, to its end, keeping what its end issues.
	var last tea.Cmd
	for i := 0; m.running && i < 50; i++ {
		next, c := m.Update(waitEvent(m.events)())
		m, last = next.(model), c
	}
	msg := findMsg(last, func(msg tea.Msg) bool { _, ok := msg.(reviewed); return ok })
	if msg == nil || strings.Join(asked, ",") != "pricing" {
		t.Fatalf("the spec was not reviewed: %v", asked)
	}
	if m.wait == waitContinue {
		t.Error("the stop came before the review")
	}
	next, cmd := m.Update(msg)
	if m = next.(model); m.wait != waitContinue || cmd == nil {
		t.Errorf("after the review the chat must stop for the person: wait=%q", m.wait)
	}
}

// A delivery is stopped from the chat — once, at its next checkpoint —, a
// gate it opens is asked here and answered into the run, and what each run
// spent is said.
func TestADeliveryIsStoppedAndItsGateAnsweredFromTheChat(t *testing.T) {
	m, _, _ := planModel(t, nil, nil)
	stops := 0
	ch := make(chan loop.Event, 4)
	m.plan = &plan{res: &intake.Result{}, delivery: &delivery{events: ch, wait: func() error { return nil }, stop: func() { stops++ }, spent: map[string]float64{}}}

	run := "r-1"
	q := loop.Question{Schema: loop.GateSchema, Run: run, Node: "implement", Questions: []intake.ElicitQuest{
		{ID: "q1", Text: "Paginar?", Options: []string{"sim", "não"}, Recommended: "sim"},
		{ID: "q2", Text: "Ordem?", Options: []string{"data", "valor"}, Recommended: "data"}}}
	data, _ := json.Marshal(q)
	gate := filepath.Join(loop.Dir(m.opts.Root, run), loop.GateFile)
	_ = os.MkdirAll(filepath.Dir(gate), 0o755)
	_ = os.WriteFile(gate, data, 0o644)

	next, _ := m.onDeliveryEvent(loop.Event{Ev: "gate.open", Node: "implement", Run: run, CostUSD: 0.7})
	m = next.(model)
	if m.wait != waitGate || len(m.plan.asking) != 2 {
		t.Fatalf("wait %q, asking %d", m.wait, len(m.plan.asking))
	}
	next, _ = m.onPlanReply("2", nil)
	m = next.(model)
	next, _ = m.onPlanReply("seguir", nil)
	m = next.(model)
	raw, err := os.ReadFile(filepath.Join(loop.Dir(m.opts.Root, run), loop.AnswerFile))
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	answers, _ := got["answers"].(map[string]any)
	if answers == nil {
		answers = got
	}
	if err != nil || answers["q1"] != "não" || answers["q2"] != "data" || m.wait != waitNone {
		t.Errorf("answer %s (%v), wait %q", raw, err, m.wait)
	}
	if m.plan.delivery.spent[run] != 0.7 {
		t.Errorf("spent %v", m.plan.delivery.spent)
	}

	m.stopDelivery()
	m.stopDelivery()
	if stops != 1 || m.notice == "" {
		t.Errorf("stops %d, notice %q", stops, m.notice)
	}
}

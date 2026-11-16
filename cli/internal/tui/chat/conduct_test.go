package chat

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/approval"

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
	m.opts.Review = func(_ context.Context, ctx string, _ func(string)) ([]string, int, error) {
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

// What the review does while it reads is shown as it comes, before the
// review itself: a review of minutes is seen working.
func TestTheReviewShowsItsProgressBeforeItsFindings(t *testing.T) {
	m, _, _ := planModel(t, []*intake.Result{threePhases()}, nil)
	m.opts.Review = func(_ context.Context, _ string, progress func(string)) ([]string, int, error) {
		progress("revisando a (gofi-spec-review)…")
		progress("· Read")
		return nil, 0, nil
	}
	m = ask(t, m, "altere o fluxo de pricing")
	var last tea.Cmd
	for i := 0; m.running && i < 50; i++ {
		next, c := m.Update(waitEvent(m.events)())
		m, last = next.(model), c
	}
	var seen []string
	msg := findMsg(last, func(msg tea.Msg) bool { _, ok := msg.(reviewProgress); return ok })
	for i := 0; i < 5 && msg != nil; i++ {
		p, ok := msg.(reviewProgress)
		if !ok {
			break
		}
		seen = append(seen, p.line)
		msg = readReview(p.ch)()
	}
	if strings.Join(seen, "|") != "revisando a (gofi-spec-review)…|· Read" {
		t.Errorf("progress = %q", seen)
	}
	if _, ok := msg.(reviewed); !ok {
		t.Errorf("after the progress came %T, not the review", msg)
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

// Approving the contract is asked every time: the chat offers no "always"
// for it, and a 2 approves nothing.
func TestApprovingTheContractIsNeverAlways(t *testing.T) {
	m, _, _ := planModel(t, nil, nil)
	req := &approval.Request{Tool: approval.ApproveTool, Input: json.RawMessage(`{"ref":"adicionar-filtro"}`)}
	m.asking = []*approval.Request{req}
	view := strings.Join(m.approvalView(req, 30), "\n")
	if strings.Contains(view, "2.") || !strings.Contains(view, "adicionar-filtro") {
		t.Errorf("view:\n%s", view)
	}
	next, _ := m.onApprovalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = next.(model)
	if len(m.asking) != 1 || m.always[approval.ApproveTool] {
		t.Errorf("a 2 settled the approval: asking %d, always %v", len(m.asking), m.always)
	}
}

// says is the script of a turn whose answer is text: what the role says, and
// the turn's end.
func says(text string) []engine.Event {
	return []engine.Event{
		engine.Meta{SessionID: "s1"},
		engine.Message{Blocks: []engine.Block{{Kind: engine.BlockText, Text: text}}},
		engine.Done{Duration: time.Second},
	}
}

// An answer is not a request. A discovery opened with its role stays with
// it: what the person answers — with a verb in it or not — goes to the role
// as typed, and nothing of it is planned. Planned, "pode adicionar…" became a
// plan of specification, code and audit in the middle of the discovery.
func TestAnAnswerToARoleIsNotPlanned(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{threePhases()}, nil)
	m.opts.Interviews = map[string]string{"gofi-pd": "prd", "gofi-spec": "spec"}
	m = drain(t, ask(t, m, "/gofi-pd contexto de tenant"))
	if m.talking != "gofi-pd" {
		t.Fatalf("the discovery's conversation should be open: talking=%q", m.talking)
	}
	m = drain(t, ask(t, m, "aceito a recomendação, pode adicionar um PF por sócio da empresa"))
	m = drain(t, ask(t, m, "altere também o limite de sócios"))
	chat := (*sessions)[0]
	if len(*calls) != 0 || m.plan != nil || len(chat.sent) != 3 || chat.sent[1] != "aceito a recomendação, pode adicionar um PF por sócio da empresa" {
		t.Fatalf("the answers were planned: intake calls %d, plan %v, sent %q", len(*calls), m.plan != nil, chat.sent)
	}
	if view := m.View(); !strings.Contains(view, "/gofi-pd") || !strings.Contains(view, "/clear") {
		t.Errorf("the prompt should say whose conversation it is and how it is left:\n%s", view)
	}

	// Another role invoked takes the conversation; a command of the engine
	// does not end it.
	m = drain(t, ask(t, m, "/compact"))
	if m.talking != "gofi-pd" {
		t.Errorf("a command that is no role closed the conversation: %q", m.talking)
	}
	m = drain(t, ask(t, m, "/gofi-spec tenant"))
	if m.talking != "gofi-spec" {
		t.Errorf("the specification's conversation should be open: %q", m.talking)
	}

	// /clear closes it: what comes next is a request again, and is planned.
	m = drain(t, ask(t, m, "/clear"))
	m = ask(t, m, "altere o fluxo de pricing")
	if m.talking != "" || len(*calls) != 1 || m.plan == nil {
		t.Errorf("after /clear a request is planned: talking=%q intake calls=%d plan=%v", m.talking, len(*calls), m.plan != nil)
	}
}

// With no role's conversation open, what follows a question of the chat is
// its answer too — and what follows an answer that asked nothing is a
// request, planned as before.
func TestWhatFollowsAQuestionAnswersIt(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{{Request: "oi", Chat: true}, threePhases()}, nil)
	chat := (*sessions)[0]
	chat.script = says("Posso registrar assim.\n\nAceita, ou quer simplificar (uma empresa por proprietário)?")
	m = drain(t, ask(t, m, "oi"))
	if !m.asked || len(*calls) != 1 {
		t.Fatalf("the chat asked and should wait for the answer: asked=%v calls=%d", m.asked, len(*calls))
	}
	chat.script = says("Registrado.")
	m = drain(t, ask(t, m, "aceito, pode adicionar os sócios"))
	if len(*calls) != 1 || len(chat.sent) != 2 || chat.sent[1] != "aceito, pode adicionar os sócios" || m.asked {
		t.Fatalf("the answer was planned: calls=%d sent=%q asked=%v", len(*calls), chat.sent, m.asked)
	}
	m = ask(t, m, "altere o fluxo de pricing")
	if len(*calls) != 2 || m.plan == nil {
		t.Errorf("with nothing asked, a request is planned: calls=%d", len(*calls))
	}
}

func TestEndsAsking(t *testing.T) {
	for text, want := range map[string]bool{
		"Aceita, ou quer simplificar (por exemplo, um único proprietário no MVP)?":  true,
		"Qual modelo prefere?\n\n1. Um tenant por pessoa\n2. Um tenant por empresa": true,
		"**Posso seguir assim?**":                                true,
		"PRD gravado em prd/tenant/prd-tenant.md.":               false,
		"Por que isso importa? Porque o sócio decide.\n\nFeito.": false, // a question answered in its own line is not left open
		"": false,
		"Perguntei antes: quem aprova?\n\na\nb\nc\nd\ne\nf\ng\nh\n\nConcluído.": false,
	} {
		if got := endsAsking(text); got != want {
			t.Errorf("endsAsking(%q) = %v, want %v", text, got, want)
		}
	}
}

// At the stop at the end of a leg, what is written out is an adjustment to
// the document: it goes to the role that wrote it, in its session, and the
// stop comes back — the reviewer does not read it a second time on its own.
// A word or two is a slip of the yes or no, and sends nothing.
func TestAnAdjustmentAtTheReviewStopGoesToTheRole(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{threePhases()}, nil)
	reviews := 0
	m.opts.Review = func(context.Context, string, func(string)) ([]string, int, error) {
		reviews++
		return nil, 0, nil
	}
	toStop := func(m model) model {
		t.Helper()
		var last tea.Cmd
		for i := 0; m.running && i < 50; i++ {
			next, c := m.Update(waitEvent(m.events)())
			m, last = next.(model), c
		}
		if msg := findMsg(last, func(msg tea.Msg) bool { _, ok := msg.(reviewed); return ok }); msg != nil {
			next, _ := m.Update(msg)
			m = next.(model)
		}
		return m
	}
	m = toStop(ask(t, m, "altere o fluxo de pricing"))
	if m.wait != waitContinue || reviews != 1 {
		t.Fatalf("the spec stops for review, read once: wait=%q reviews=%d", m.wait, reviews)
	}
	if said := m.reviewPrompt(); !strings.Contains(said, "/gofi-spec") || !strings.Contains(said, "loop") {
		t.Errorf("the stop names who refines and what going on does: %q", said)
	}

	m = ask(t, m, "pode seguir")
	if sent, _ := phases(*sessions); len(sent) != 1 || m.wait != waitContinue {
		t.Fatalf("two words are no adjustment: sent=%d wait=%q", len(sent), m.wait)
	}

	m = toStop(ask(t, m, "troque o prazo do requisito 2 para trinta dias"))
	sent, _ := phases(*sessions)
	if len(sent) != 2 || sent[1] != "troque o prazo do requisito 2 para trinta dias" || len(*sessions) != 2 {
		t.Fatalf("the adjustment goes to the spec's own session: sent=%q sessions=%d", sent, len(*sessions))
	}
	if len(*calls) != 1 || m.wait != waitContinue || reviews != 1 || len(lastDeliveries) != 0 {
		t.Fatalf("after it the stop is back, nothing planned, nothing built, not reviewed again: calls=%d wait=%q reviews=%d deliveries=%v", len(*calls), m.wait, reviews, lastDeliveries)
	}

	// Enter still goes on to the loop.
	m, c := enterCmd(m)
	m = pump(t, m, c)
	if len(lastDeliveries) != 1 || m.plan != nil {
		t.Errorf("enter should hand the change to the loop: %v", lastDeliveries)
	}
}

// A role that ends its phase on a question is answered: what the person types
// — a plain "sim" too — goes to it, and the plan goes on once it asks nothing
// more, or on an empty Enter.
func TestARoleThatAsksIsAnswered(t *testing.T) {
	prd := &intake.Result{
		Request: "descoberta do tenant", Intent: "create", Artifact: "prd",
		Context: &intake.Context{Name: "tenant", New: true},
		Plan:    []intake.Phase{{Role: "gofi-pd", Produces: "prd", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"}},
		Brief:   "# Pedido montado\n",
	}
	m, sessions, calls := planModel(t, []*intake.Result{prd}, nil)
	m.opts.NewSession = func(string) engine.Session {
		s := &fakeSession{script: says("Recomendo um tenant pessoal por CPF.\n\nAceita, ou quer simplificar?")}
		*sessions = append(*sessions, s)
		return s
	}
	m = drain(t, ask(t, m, "descoberta do tenant"))
	if m.wait != waitReply || m.plan == nil {
		t.Fatalf("the role asked: the plan waits for the answer, it does not end: wait=%q plan=%v", m.wait, m.plan != nil)
	}
	role := (*sessions)[1]
	role.script = says("Registrado. PRD gravado em prd/tenant/prd-tenant.md.")
	m = drain(t, ask(t, m, "sim"))
	if len(role.sent) != 2 || role.sent[1] != "sim" || len(*calls) != 1 {
		t.Fatalf("the answer goes to the role, as typed: sent=%q intake calls=%d", role.sent, len(*calls))
	}
	if m.plan != nil || m.wait != waitNone || !role.closed {
		t.Errorf("with nothing more asked the plan ends: plan=%v wait=%q", m.plan != nil, m.wait)
	}

	// Left unanswered, an empty Enter goes on.
	m2, sessions2, _ := planModel(t, []*intake.Result{prd}, nil)
	m2.opts.NewSession = func(string) engine.Session {
		s := &fakeSession{script: says("Feito. Quer revisar algo?")}
		*sessions2 = append(*sessions2, s)
		return s
	}
	m2 = drain(t, ask(t, m2, "descoberta do tenant"))
	if m2.wait != waitReply {
		t.Fatalf("wait=%q", m2.wait)
	}
	if m2 = enter(m2); m2.plan != nil || m2.wait != waitNone {
		t.Errorf("an empty Enter goes on, here to the end of the plan: plan=%v wait=%q", m2.plan != nil, m2.wait)
	}
}

// The plan is read by its legs — discovery, specification, delivery —, each
// with its specialist, and what is built is one line: the loop's. A plan of
// one leg reads as it always did.
func TestThePlanIsReadByItsLegs(t *testing.T) {
	m, _, _ := planModel(t, nil, nil)
	whole := &intake.Result{
		Intent: "create", Artifact: "code", Context: &intake.Context{Name: "tenant", New: true},
		Plan: []intake.Phase{
			{Role: "gofi-pd", Tier: host.Standard, Base: host.Deep, Elicit: true, TierWhy: "elicitação"},
			{Role: "gofi-pd", Produces: "prd", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"},
			{Role: "gofi-spec", Tier: host.Standard, Base: host.Deep, Elicit: true, TierWhy: "elicitação"},
			{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"},
			{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, TierWhy: "contrato"},
			{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, TierWhy: "contrato"},
		},
	}
	block := m.planBlock(whole)
	at := func(s string) int { return strings.Index(block, s) }
	if !(at("Discovery") > 0 && at("Discovery") < at("1. /gofi-pd") && at("2. /gofi-pd") < at("Specification") &&
		at("Specification") < at("3. /gofi-spec") && at("4. /gofi-spec") < at("Delivery loop") && at("Delivery loop") < at("5. gofi run")) {
		t.Errorf("the plan should be read leg by leg, in order:\n%s", block)
	}
	if strings.Contains(block, "/gofi-eng") || strings.Contains(block, "/gofi-qa") {
		t.Errorf("what is built is the loop's, not a role run here:\n%s", block)
	}
	for i, want := range []string{legDiscovery, legDiscovery, legSpec, legSpec, legDelivery, legDelivery} {
		if got := legAt(whole.Plan, i); got != want {
			t.Errorf("leg of phase %d = %q, want %q", i+1, got, want)
		}
	}

	one := threePhases()
	one.Plan = one.Plan[:1]
	if b := m.planBlock(one); strings.Contains(b, "Specification") || !strings.Contains(b, "1. /gofi-spec") {
		t.Errorf("a plan of one leg has no heading:\n%s", b)
	}
}

// saidBy runs a command tree and joins every text it says, for what a step
// put on screen.
func saidBy(cmd tea.Cmd) string {
	var out []string
	findMsg(cmd, func(msg tea.Msg) bool {
		if s, ok := msg.(sayMsg); ok {
			out = append(out, s.text)
		}
		return false
	})
	return strings.Join(out, "\n")
}

// toEnd runs the turn under way to its end and returns what its end said.
func toEnd(t *testing.T, m model) (model, string) {
	t.Helper()
	var last tea.Cmd
	for i := 0; m.running && i < 50; i++ {
		next, c := m.Update(waitEvent(m.events)())
		m, last = next.(model), c
	}
	return m, saidBy(last)
}

func discovery(then ...intake.Phase) *intake.Result {
	return &intake.Result{
		Request: "descoberta do tenant", Intent: "create", Artifact: "prd",
		Context: &intake.Context{Name: "tenant", New: true},
		Plan:    append([]intake.Phase{{Role: "gofi-pd", Produces: "prd", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"}}, then...),
		Brief:   "# Pedido montado\n",
	}
}

// The discovery has a sensor: the PRD just written is read with no model
// asked, and what still asks for a correction is said before the stop. Going
// on stays the person's to decide — the sensor informs, it does not lock.
func TestThePRDIsSensedAtTheEndOfTheDiscovery(t *testing.T) {
	spec := intake.Phase{Role: "gofi-spec", Produces: "spec", Tier: host.Deep, Base: host.Deep, TierWhy: "contrato"}
	m, sessions, _ := planModel(t, []*intake.Result{discovery(spec)}, nil)
	read := 0
	m.opts.PRD = func(ctx string) ([]string, int, int, bool) {
		read++
		return []string{"✗ §2 sem métrica com hoje, meta e como medir", "· §6 sem restrição, premissa ou risco descrito"}, 1, 1, ctx == "tenant"
	}
	m, said := toEnd(t, ask(t, m, "descoberta do tenant"))
	if m.wait != waitContinue || read != 1 {
		t.Fatalf("the discovery stops for the person, its PRD read once: wait=%q read=%d", m.wait, read)
	}
	for _, want := range []string{"tenant: 1 to correct, 1 warning(s)", "§2 sem métrica", "the sensor found 1 point(s) to correct", "/gofi-pd refines it here", "opens the specification with /gofi-spec"} {
		if !strings.Contains(said, want) {
			t.Errorf("the stop should say %q:\n%s", want, said)
		}
	}
	// Enter goes on to the specification all the same.
	m, _ = enterCmd(m)
	if sent, _ := phases(*sessions); len(sent) != 2 || !strings.HasPrefix(sent[1], "/gofi-spec ") {
		t.Errorf("the specification should start: %q", sent)
	}
}

// A plan that ends at the PRD does not end on one that still asks for
// corrections: the role is a line away. An empty Enter takes the PRD as it
// is, and a PRD with nothing to correct ends the plan at once.
func TestADiscoveryDoesNotEndOnAPRDToCorrect(t *testing.T) {
	m, sessions, calls := planModel(t, []*intake.Result{discovery()}, nil)
	errs := 2
	m.opts.PRD = func(string) ([]string, int, int, bool) {
		if errs == 0 {
			return nil, 0, 0, true
		}
		return []string{"✗ §4 sem nada em \"Fora\"", "✗ §5 sem jornada com passos"}, errs, 0, true
	}
	m, said := toEnd(t, ask(t, m, "descoberta do tenant"))
	if m.wait != waitReply || m.plan == nil || !strings.Contains(said, "it goes to /gofi-pd") || !strings.Contains(said, "§5 sem jornada") {
		t.Fatalf("the plan waits with the role at hand: wait=%q\n%s", m.wait, said)
	}
	// The adjustment goes to the role; corrected, the plan ends.
	errs = 0
	m, said = toEnd(t, ask(t, m, "acrescente o que fica fora e a jornada do convite"))
	role := (*sessions)[1]
	if len(role.sent) != 2 || len(*calls) != 1 || m.plan != nil || !strings.Contains(said, "ready for the specification") {
		t.Fatalf("after the correction the plan ends, the PRD ready: sent=%d calls=%d plan=%v\n%s", len(role.sent), len(*calls), m.plan != nil, said)
	}

	// Taken as it is: an empty Enter ends the plan, and the sensor does not
	// hold it a second time.
	m2, _, _ := planModel(t, []*intake.Result{discovery()}, nil)
	m2.opts.PRD = func(string) ([]string, int, int, bool) { return []string{"✗ §1 sem o problema"}, 1, 0, true }
	m2, _ = toEnd(t, ask(t, m2, "descoberta do tenant"))
	if m2 = enter(m2); m2.plan != nil || m2.wait != waitNone {
		t.Errorf("an empty Enter takes the PRD as it is: plan=%v wait=%q", m2.plan != nil, m2.wait)
	}

	// A role's question left unanswered is not the PRD taken as it is: the
	// sensor still has its say before the plan ends.
	m3, sessions3, _ := planModel(t, []*intake.Result{discovery()}, nil)
	m3.opts.PRD = func(string) ([]string, int, int, bool) { return []string{"✗ §1 sem o problema"}, 1, 0, true }
	m3.opts.NewSession = func(string) engine.Session {
		s := &fakeSession{script: says("Rascunho gravado. Quer revisar algo?")}
		*sessions3 = append(*sessions3, s)
		return s
	}
	m3, _ = toEnd(t, ask(t, m3, "descoberta do tenant"))
	if m3.wait != waitReply || m3.plan.fixing {
		t.Fatalf("first the role's question: wait=%q fixing=%v", m3.wait, m3.plan.fixing)
	}
	m3, c := enterCmd(m3)
	if said := saidBy(c); m3.plan == nil || m3.wait != waitReply || !m3.plan.fixing || !strings.Contains(said, "§1 sem o problema") {
		t.Fatalf("past the question the sensor stops the plan: wait=%q\n%s", m3.wait, said)
	}
	if m3 = enter(m3); m3.plan != nil {
		t.Errorf("past the sensor the plan ends")
	}
}

// In a discovery held with the role itself, the turn that writes the PRD is
// read by the sensor too — only that turn, and only a discovery's.
func TestADiscoverysTurnThatWritesThePRDIsSensed(t *testing.T) {
	m, sessions, _ := planModel(t, nil, nil)
	m.opts.Interviews = map[string]string{"gofi-pd": "prd", "gofi-spec": "spec"}
	wrote := []string{}
	m.opts.PRDsSince = func(time.Time) []string { return wrote }
	m.opts.PRD = func(ctx string) ([]string, int, int, bool) { return []string{"✗ §2 sem métrica"}, 1, 0, true }
	(*sessions)[0].script = says("Qual é a métrica de sucesso?")
	m, said := toEnd(t, ask(t, m, "/gofi-pd tenant"))
	if strings.Contains(said, "PRD sensor") {
		t.Errorf("no PRD was written in this turn:\n%s", said)
	}
	wrote = []string{"tenant"}
	(*sessions)[0].script = says("PRD gravado em prd/tenant/prd-tenant.md.")
	m, said = toEnd(t, ask(t, m, "80% dos sócios conciliados"))
	if !strings.Contains(said, "PRD sensor for tenant: 1 to correct") || !strings.Contains(said, "§2 sem métrica") {
		t.Errorf("the turn that wrote the PRD should be sensed:\n%s", said)
	}
	// A specification's turn is not a discovery's.
	m = drain(t, ask(t, m, "/gofi-spec tenant"))
	if m, said = toEnd(t, ask(t, m, "siga")); strings.Contains(said, "PRD sensor") {
		t.Errorf("the sensor is the discovery's:\n%s", said)
	}
	_ = m
}

// /scan and /prd read the disk through gofi and call no model: the first
// needs no argument and works in a folder that is no gofi project, the
// second is the discovery's sensor on demand.
func TestScanAndPRDAreAskedOfGofi(t *testing.T) {
	for text, want := range map[string]string{
		"/scan":       "scan",
		"/prd":        "prd check",
		"/prd tenant": "prd check tenant",
	} {
		args, ok := gofiArgs(text)
		if !ok || strings.Join(args, " ") != want {
			t.Errorf("%s → %v (%v), want %q", text, args, ok, want)
		}
	}
	m, sessions, calls := planModel(t, nil, nil)
	var ran [][]string
	m.opts.Gofi = func(args ...string) *exec.Cmd {
		ran = append(ran, args)
		return exec.Command("true")
	}
	m.input.SetValue("/scan")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if len(ran) != 1 || strings.Join(ran[0], " ") != "scan" || !m.running {
		t.Fatalf("/scan should run gofi scan with no argument asked: %v", ran)
	}
	if len(*calls) != 0 || len((*sessions)[0].sent) != 0 {
		t.Errorf("nothing of it goes to a model: intake %d, engine %q", len(*calls), (*sessions)[0].sent)
	}
}

// Outside a gofi project the chat says so where it is seen first, with the
// way in; inside one, the plan of a context being adopted says what the
// repository already holds of it.
func TestAdoptionIsSaidWhereItMatters(t *testing.T) {
	m, _, _ := planModel(t, nil, nil)
	m.opts.Gofi = func(...string) *exec.Cmd { return exec.Command("true") }
	m.width = 120
	if b := m.banner(); strings.Contains(b, "/scan") {
		t.Errorf("a gofi project needs no way in:\n%s", b)
	}
	m.opts.Intake = nil
	if b := m.banner(); !strings.Contains(b, "/scan") || !strings.Contains(b, "not a gofi project") {
		t.Errorf("outside a project the banner should point at /scan:\n%s", b)
	}

	r := discovery()
	r.Found = &intake.Found{Dir: "domain/tenant/", Sources: 12, Tests: 3, Tables: []string{"tenant"}}
	if block := m.planBlock(r); !strings.Contains(block, "already in the repository: código em `domain/tenant/` (12 fontes, 3 testes), 1 tabela(s)") {
		t.Errorf("the plan should say what the repository holds:\n%s", block)
	}
}

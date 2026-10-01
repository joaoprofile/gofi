package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gofi-labs/gofi/cli/internal/engine"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/intake"
	"github.com/gofi-labs/gofi/cli/internal/runs"
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
	}, false)
	return m, &sessions, &calls
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
	m = drain(t, enter(m))
	sent, _ = phases(*sessions)
	if len(sent) != 3 || !strings.HasPrefix(sent[1], "/gofi-eng ") || !strings.HasPrefix(sent[2], "/gofi-qa ") {
		t.Fatalf("phases sent = %q", sent)
	}
	if len(*sessions) != 4 || len((*sessions)[0].sent) != 0 {
		t.Errorf("each phase runs in a session of its own, apart from the chat's: %d sessions, chat sent %q", len(*sessions), (*sessions)[0].sent)
	}
	for _, s := range (*sessions)[1:3] {
		if !s.closed {
			t.Error("a finished phase's session was left open")
		}
	}
	if !strings.Contains(sent[1], "Fase 2 de 3") || !strings.Contains(sent[1], "/gofi-spec") {
		t.Errorf("the next phase is not told what came before: %q", sent[1])
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
	m = drain(t, enter(m))
	if sent, _ := phases(*sessions); len(sent) != 1 || !strings.HasPrefix(sent[0], "/gofi-eng ") {
		t.Errorf("sent = %q", sent)
	}
}

// A phase raised above its contract runs in a session on the raised tier's
// model, as the role — not the skill, whose own model would win — and the
// chat's session keeps its model.
func TestRaisedPhaseRunsOnItsTier(t *testing.T) {
	r := &intake.Result{Request: "corrija o cancelamento", Intent: "fix", Artifact: "code",
		Plan: []intake.Phase{{Role: "gofi-eng", Produces: "code", Tier: host.Deep, Base: host.Standard, TierWhy: "acoplado a 3 contextos"}}}
	m, sessions, _ := planModel(t, []*intake.Result{r}, nil)
	m = ask(t, m, "corrija o cancelamento")
	sent, models := phases(*sessions)
	if len(models) != 1 || models[0] != "model-for-deep" || !strings.HasPrefix(sent[0], "Siga o papel descrito em .claude/skills/gofi-eng/SKILL.md") {
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

// A failed audit runs the retry the plan carries — the implementation one
// tier up, then the audit — and the run is recorded under .gofi/runs.
func TestFailedAuditIsRetriedOneTierUp(t *testing.T) {
	r := &intake.Result{
		Request: "corrija o pricing", Intent: "fix", Artifact: "code", VerdictFile: "mem/pricing.md",
		Context: &intake.Context{Name: "pricing", Has: []string{"spec", "code"}},
		Plan: []intake.Phase{
			{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-eng corrija"},
			{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-qa audite"},
		},
	}
	r.Plan[1].OnReject = []intake.Phase{
		{Role: "gofi-eng", Produces: "code", Tier: host.Deep, Base: host.Standard, Turn: "/gofi-eng refaça", RoleTurn: "Siga o papel: refaça"},
		{Role: "gofi-qa", Produces: "audit", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-qa de novo"},
	}
	m, sessions, _ := planModel(t, []*intake.Result{r}, nil)
	root := m.opts.Root
	verdict := filepath.Join(root, "mem", "pricing.md")
	_ = os.MkdirAll(filepath.Dir(verdict), 0o755)
	audits := []string{"reprovado", "aprovado"}
	m.session = nil
	m.opts.NewSession = func(string) engine.Session {
		s := &fakeSession{script: []engine.Event{engine.Meta{SessionID: "s1"}, engine.Done{Duration: time.Second, CostUSD: 0.01}}}
		s.onSend = func(p string) {
			if strings.HasPrefix(p, "/gofi-qa") && len(audits) > 0 {
				_ = os.WriteFile(verdict, []byte("---\nstatus: "+audits[0]+"\n---\n"), 0o644)
				audits = audits[1:]
			}
		}
		*sessions = append(*sessions, s)
		return s
	}
	m.session = m.opts.NewSession("")
	m = drain(t, ask(t, m, "corrija o pricing"))
	sent, _ := phases(*sessions)
	want := []string{"/gofi-eng corrija", "/gofi-qa audite", "Siga o papel: refaça", "/gofi-qa de novo"}
	if strings.Join(sent, "|") != strings.Join(want, "|") {
		t.Fatalf("sent = %q", sent)
	}
	if m.plan != nil {
		t.Errorf("the plan should be over")
	}
	files, _ := filepath.Glob(filepath.Join(root, ".gofi", "runs", "*.json"))
	if len(files) != 1 {
		t.Fatalf("runs = %v", files)
	}
	run, _ := runs.Load(files[0])
	if run.Surface != "chat" || run.Outcome != runs.OutcomeDone || run.Verdict != "aprovado" || len(run.Phases) != 4 || run.Phases[2].Model != "model-for-deep" {
		t.Errorf("run = %+v", run)
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

// An implementation that stops on a decision gets it asked in the chat; the
// spec records the answer and the implementation runs again.
func TestABlockedPhaseIsAskedAndRecordedInTheSpec(t *testing.T) {
	eng := intake.Phase{Role: "gofi-eng", Produces: "code", Tier: host.Standard, Base: host.Standard, Turn: "/gofi-eng corrija",
		ElicitFile: ".gofi/elicit/order-gofi-eng.json"}
	again := eng
	again.Turn = "/gofi-eng retome"
	eng.OnBlock = []intake.Phase{{Role: "gofi-spec", Tier: host.Standard, Base: host.Deep, Turn: "registre", RoleTurn: "registre"}, again}
	r := &intake.Result{Request: "corrija o cancelamento", Intent: "fix", Artifact: "code",
		Context: &intake.Context{Name: "order", Has: []string{"spec", "code"}}, Plan: []intake.Phase{eng}}
	m, sessions, _ := planModel(t, []*intake.Result{r}, nil)
	root := m.opts.Root
	next := m.opts.NewSession
	m.opts.NewSession = func(id string) engine.Session {
		s := next(id).(*fakeSession)
		s.onSend = func(p string) {
			if p == "/gofi-eng corrija" {
				path := filepath.Join(root, ".gofi", "elicit", "order-gofi-eng.json")
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, []byte(`{"schema":"gofi.elicit/v1","questions":[{"id":"idx","text":"Perfil de acesso?","options":["append-only","hot UPDATE"],"recommended":"hot UPDATE","why":"atualiza"}]}`), 0o644)
			}
		}
		return s
	}
	m = drain(t, ask(t, m, "corrija o cancelamento"))
	if m.wait != waitElicit {
		t.Fatalf("the block was not asked: wait=%q", m.wait)
	}
	m = drain(t, ask(t, m, "2"))
	sent, models := phases(*sessions)
	if len(sent) != 3 || !strings.HasPrefix(sent[1], "registre") || !strings.Contains(sent[1], "Perfil de acesso? → append-only") || sent[2] != "/gofi-eng retome" {
		t.Fatalf("sent = %q", sent)
	}
	if models[1] != "model-for-standard" || m.plan != nil {
		t.Errorf("models = %v, plan over = %v", models, m.plan == nil)
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

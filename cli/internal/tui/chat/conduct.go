package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gofi-labs/gofi/cli/internal/engine"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/intake"
	"github.com/gofi-labs/gofi/cli/internal/runs"
)

// What the chat waits for while a plan is open.
const (
	waitNone     = ""
	waitAnswer   = "answer"   // a question of the intake
	waitConfirm  = "confirm"  // the plan, before it starts
	waitContinue = "continue" // the next phase, after a PRD or a spec
	waitElicit   = "elicit"   // a decision an elicitation left
)

// intakeDone carries the intake's answer back into the update loop.
type intakeDone struct {
	res *intake.Result
	err error
}

// plan is a request the chat conducts: its intake result and where it is.
type plan struct {
	res     *intake.Result
	next    int               // the phase to run next
	answers map[string]string // what the person answered, for the last round
	phase   engine.Session    // the running phase's own session

	rec     *runs.Run // the run's record, from the first phase on
	model   string    // the model a raised phase set, for the record
	before  []byte    // the context's memory before the running phase
	retried bool      // a failed audit's retry was spliced in
	outcome string    // how the plan ended, for the record
	verdict string    // the QA's last verdict, for the record

	// decisions are what each role's elicitation left and what the person
	// answered; they end the role's turn (intake.WithDecisions).
	decisions map[string]*decided
	asking    []intake.Question // the elicitation's questions still to answer
	askingFor string            // the role they are for
}

// decided is an elicitation and its answers.
type decided struct {
	e       *intake.Elicitation
	answers map[string]string
}

// runIntake plans a request off the update loop: the intake may consult the
// light tier, which takes seconds.
func (m model) runIntake(text string, answers map[string]string, proceed bool) tea.Cmd {
	in := m.opts.Intake
	return func() tea.Msg {
		res, err := in(text, answers, proceed)
		return intakeDone{res, err}
	}
}

// startIntake begins planning a request typed in free text.
func (m model) startIntake(text string, echo tea.Cmd) (tea.Model, tea.Cmd) {
	// The engine starts while the request is planned: most requests go to it
	// as typed, and its startup then overlaps the planning.
	if w, ok := m.session.(engine.Warmer); ok {
		w.Warm()
	}
	m.plan = &plan{answers: map[string]string{}}
	m.running, m.planning, m.started, m.frame = true, true, time.Now(), 0
	return m, tea.Batch(echo, m.runIntake(text, nil, false), tickCmd())
}

// onIntake shows what the intake decided and takes the next step: ask,
// confirm, conduct, or send the request as it is when there is nothing to plan.
func (m model) onIntake(msg intakeDone) (tea.Model, tea.Cmd) {
	m.running, m.planning = false, false
	k := m.look
	if msg.err != nil {
		m.plan = nil
		return m, tea.Println("\n" + k.bullet(k.bad, msg.err.Error(), k.text, m.width) + "\n" + k.nested([]string{i18n.T("chat.plan.send_hint")}, k.dim))
	}
	r := msg.res
	m.plan.res = r
	if r.Direct != "" || r.Chat {
		m.plan = nil
		return m.sendToEngine(r.Request, nil)
	}
	shown := tea.Println(m.planBlock(r))
	switch {
	case r.Ask:
		m.wait = waitAnswer
		return m, tea.Sequence(shown, tea.Println(m.questionsBlock(r)))
	case r.Answer != "":
		m.plan, m.wait = nil, waitNone
		return m, tea.Sequence(shown, tea.Println(k.nested([]string{i18n.T("chat.plan.answer", r.Answer)}, k.text)))
	case len(r.Plan) == 0:
		m.plan = nil
		if r.Turn != "" {
			// A question about the project: it goes with the sections that
			// answer it, not as a plan.
			return m.sendToEngine(r.Turn, shown)
		}
		return m.sendToEngine(r.Request, shown)
	case len(r.Assumptions) > 0 || len(m.plan.answers) > 0:
		// A question was answered or a default assumed: the plan is shown for
		// review before anything runs (0001, Q1).
		m.wait = waitConfirm
		return m, tea.Sequence(shown, tea.Println(k.nested([]string{i18n.T("chat.plan.confirm")}, k.accent)))
	}
	return m.startPhase(shown)
}

// onPlanReply takes what was typed while a plan waits.
func (m model) onPlanReply(text string, echo tea.Cmd) (tea.Model, tea.Cmd) {
	k := m.look
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "cancelar" || t == "/cancelar" {
		m.stopPlan()
		return m, tea.Sequence(echo, tea.Println(k.nested([]string{i18n.T("chat.plan.cancelled")}, k.dim)))
	}
	switch m.wait {
	case waitAnswer:
		r := m.plan.res
		if t == "seguir" || t == "/seguir" {
			m.wait = waitNone
			m.running, m.planning, m.started = true, true, time.Now()
			return m, tea.Batch(echo, m.runIntake(r.Request, m.plan.answers, true), tickCmd())
		}
		q := r.Questions[0]
		answer := strings.TrimSpace(text)
		if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= len(q.Options) {
			answer = q.Options[n-1]
		}
		m.plan.answers[q.ID] = answer
		if len(r.Questions) > 1 {
			// One question answered, another left: it is asked next, from the
			// same round.
			r.Questions = r.Questions[1:]
			return m, tea.Sequence(echo, tea.Println(m.questionsBlock(r)))
		}
		m.wait = waitNone
		m.running, m.planning, m.started = true, true, time.Now()
		return m, tea.Batch(echo, m.runIntake(r.Request, m.plan.answers, false), tickCmd())
	case waitElicit:
		p := m.plan
		d := p.decisions[p.askingFor]
		if t == "seguir" || t == "/seguir" {
			// The rest go on their recommendation, written down as assumed.
			p.asking = nil
		} else {
			q := p.asking[0]
			answer := strings.TrimSpace(text)
			if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= len(q.Options) {
				answer = q.Options[n-1]
			}
			d.answers[q.ID] = answer
			p.asking = p.asking[1:]
		}
		if len(p.asking) > 0 {
			return m, tea.Sequence(echo, tea.Println(m.questionBlock(p.asking[0])))
		}
		m.wait = waitNone
		return m.startPhase(echo)
	case waitConfirm, waitContinue:
		if yes(t) {
			m.wait = waitNone
			return m.startPhase(echo)
		}
		if no(t) {
			m.stopPlan()
			return m, tea.Sequence(echo, tea.Println(k.nested([]string{i18n.T("chat.plan.stopped")}, k.dim)))
		}
		return m, tea.Sequence(echo, tea.Println(k.nested([]string{i18n.T("chat.plan.yes_no")}, k.warn)))
	}
	return m, echo
}

func yes(t string) bool {
	switch t {
	case "", "s", "sim", "y", "yes", "1", "seguir", "conduzir", "ok":
		return true
	}
	return false
}

func no(t string) bool {
	switch t {
	case "n", "nao", "não", "no", "2", "parar":
		return true
	}
	return false
}

// startPhase runs the plan's next phase as one turn, in a session of its own:
// the phase reads what the ones before it left on disk — the spec, the
// context's memory, the code — never their conversation, nor the chat's. A
// phase at its role's contract tier invokes the skill, which names that
// tier's model; a phase the router raised runs its session on the tier's
// model and has the agent follow the role's SKILL.md instead — invoking the
// skill would run it on the contract's model.
func (m model) startPhase(before tea.Cmd) (tea.Model, tea.Cmd) {
	p := m.plan
	r := p.res
	ph := r.Plan[p.next]
	k := m.look
	header := i18n.T("chat.plan.phase", p.next+1, len(r.Plan), ph.Role, string(ph.Tier))
	if p.rec == nil && p.next == 0 {
		// The record is for calibration; the plan does not wait on it.
		p.rec, _ = runs.Start(m.opts.Root, "chat", r)
	}
	p.closePhase()
	p.phase = m.opts.NewSession("")
	sw, canSwitch := p.phase.(engine.ModelSwitcher)
	text := intake.TurnFor(r, p.next, false)
	p.model = ""
	if ph.Tier != ph.Base && m.opts.TierModel != nil && canSwitch {
		if model := m.opts.TierModel(ph.Tier); model != "" {
			sw.SetModel(model)
			p.model = model
			text = intake.TurnFor(r, p.next, true)
		}
	}
	if p.model == "" && canSwitch && m.chosenModel != "" {
		sw.SetModel(m.chosenModel)
	}
	if ph.ElicitFile != "" {
		intake.ClearElicitation(m.opts.Root, ph.ElicitFile)
	}
	if d := p.decisions[ph.Role]; d != nil && !ph.Elicit {
		text = intake.WithDecisions(text, d.e, d.answers)
		delete(p.decisions, ph.Role)
	}
	p.before = intake.Verdict(m.opts.Root, r)
	p.next++
	m.conducting = true
	line := tea.Println("\n" + k.bullet(k.accent, header, k.bold, m.width))
	if before != nil {
		line = tea.Sequence(before, line)
	}
	return m.sendToEngine(text, line)
}

// afterPhase decides what follows a finished phase: stop on a failed turn,
// wait for approval after a PRD or a spec, run the next phase, or end.
func (m model) afterPhase() (tea.Model, tea.Cmd) {
	k := m.look
	p := m.plan
	m.conducting = false
	r := p.res
	done := r.Plan[p.next-1]
	var el *intake.Elicitation
	var elErr error
	if done.ElicitFile != "" && !m.lastFailed {
		el, elErr = intake.ReadElicitation(m.opts.Root, done.ElicitFile)
	}
	asked := 0
	if el != nil {
		asked = len(el.Questions)
	}
	p.record(done, m.lastDone, m.lastFailed, asked)
	if m.lastFailed || elErr != nil {
		p.outcome = runs.OutcomeFailed
		m.stopPlan()
		lines := []string{i18n.T("chat.plan.halted")}
		if elErr != nil {
			lines = append([]string{elErr.Error()}, lines...)
		}
		return m, tea.Println(k.nested(lines, k.warn))
	}
	if done.Elicit {
		if el == nil {
			return m.startPhase(tea.Println(k.nested([]string{i18n.T("ask.elicit.none", done.Role)}, k.dim)))
		}
		if p.decisions == nil {
			p.decisions = map[string]*decided{}
		}
		p.decisions[done.Role] = &decided{e: el, answers: map[string]string{}}
		if qs := el.AsQuestions(); len(qs) > 0 {
			// Asked here, where answering costs nothing, before the role
			// runs on its own tier.
			p.asking, p.askingFor = qs, done.Role
			m.wait = waitElicit
			head := k.nested([]string{i18n.T("ask.elicit.questions", len(qs), done.Role)}, k.accent)
			return m, tea.Sequence(tea.Println("\n"+head), tea.Println(m.questionBlock(qs[0])))
		}
		return m.startPhase(nil)
	}
	if el != nil && len(el.Questions) > 0 {
		if len(done.OnBlock) == 0 {
			// Stopped again after the spec was completed: the person's to look at.
			m.stopPlan()
			return m, tea.Println("\n" + k.bullet(k.warn, i18n.T("ask.blocked_again", done.Role, done.ElicitFile), k.text, m.width))
		}
		// The implementation stopped on decisions the spec does not cover:
		// asked here, recorded in the spec, and the implementation runs again.
		recorder := done.OnBlock[0].Role
		if p.decisions == nil {
			p.decisions = map[string]*decided{}
		}
		p.decisions[recorder] = &decided{e: el, answers: map[string]string{}}
		r.Plan = append(r.Plan[:p.next], append(append([]intake.Phase{}, done.OnBlock...), r.Plan[p.next:]...)...)
		qs := el.AsQuestions()
		p.asking, p.askingFor = qs, recorder
		m.wait = waitElicit
		msg := i18n.T("ask.blocked", done.Role, recorder)
		if recorder == done.Role {
			msg = i18n.T("ask.blocked_rerun", done.Role)
		}
		head := k.nested([]string{msg}, k.warn)
		return m, tea.Sequence(tea.Println("\n"+head), tea.Println(m.questionBlock(qs[0])))
	}
	if done.Produces == "audit" {
		p.verdict = intake.Status(intake.Verdict(m.opts.Root, r))
		if intake.Rejected(m.opts.Root, r, p.before) {
			if len(done.OnReject) > 0 {
				// The retry the plan carries: the implementation one tier up,
				// then the audit again.
				r.Plan = append(r.Plan[:p.next], append(append([]intake.Phase{}, done.OnReject...), r.Plan[p.next:]...)...)
				p.retried = true
				return m.startPhase(tea.Println("\n" + k.nested([]string{i18n.T("chat.plan.rejected", done.OnReject[0].Role, string(done.OnReject[0].Tier))}, k.warn)))
			}
			if p.retried {
				p.outcome = runs.OutcomeEscalated
				m.stopPlan()
				return m, tea.Println("\n" + k.bullet(k.warn, i18n.T("chat.plan.still_rejected"), k.text, m.width))
			}
		}
	}
	if p.next >= len(r.Plan) {
		p.outcome = runs.OutcomeDone
		msg := i18n.T("chat.plan.done")
		if r.Next != "" {
			msg += " " + i18n.T("chat.plan.next", r.Next)
		}
		m.stopPlan()
		return m, tea.Println("\n" + k.bullet(k.good, msg, k.text, m.width))
	}
	if done.ReviewAfter || intake.ApprovalAfter(r, p.next-1) {
		// An approval point: what the next phases build rests on this document.
		m.wait = waitContinue
		return m, tea.Println("\n" + k.nested([]string{i18n.T("chat.plan.review", done.Produces, r.Plan[p.next].Role)}, k.accent))
	}
	return m.startPhase(nil)
}

// record writes a finished phase into the run's record.
func (p *plan) record(ph intake.Phase, d engine.Done, failed bool, asked int) {
	if p.rec == nil {
		return
	}
	rp := runs.Phase{Role: ph.Role, Tier: string(ph.Tier), Model: p.model, Why: ph.Why, TierWhy: ph.TierWhy,
		Seconds: d.Duration.Seconds(), CostUSD: d.CostUSD, OK: !failed, Asked: asked}
	if failed {
		rp.Error = d.Err
	}
	_ = p.rec.Phase(rp)
}

// stopPlan ends the plan, closes its record and puts the session's model back
// if a phase set it.
func (m *model) stopPlan() {
	if p := m.plan; p != nil && p.rec != nil {
		outcome := p.outcome
		if outcome == "" {
			outcome = runs.OutcomeStopped
		}
		_ = p.rec.Finish(outcome, p.verdict)
		p.rec = nil
	}
	if m.plan != nil && m.plan.phase != nil {
		m.plan.closePhase()
		// The status line showed the phase's model; the chat is back on its own.
		m.modelN = m.chosenModel
		if m.modelN == "" {
			m.modelN = m.opts.Model
		}
	}
	m.plan, m.wait, m.conducting = nil, waitNone, false
}

// closePhase ends the running phase's session.
func (p *plan) closePhase() {
	if p.phase != nil {
		_ = p.phase.Close()
		p.phase = nil
	}
}

// engine is the session a turn goes to: the running phase's, while a plan
// conducts one, or the chat's own.
func (m model) engine() engine.Session {
	if m.conducting && m.plan != nil && m.plan.phase != nil {
		return m.plan.phase
	}
	return m.session
}

// sendToEngine runs one turn with the text as typed.
func (m model) sendToEngine(text string, before tea.Cmd) (tea.Model, tea.Cmd) {
	events, err := m.engine().Send(context.Background(), engine.Turn{Prompt: text})
	if err != nil {
		m.stopPlan()
		return m, tea.Sequence(before, tea.Println(m.look.bullet(m.look.bad, err.Error(), m.look.text, m.width)))
	}
	m.running, m.events, m.started, m.frame = true, events, time.Now(), 0
	m.live, m.lastFailed = "", false
	cmds := []tea.Cmd{waitEvent(events), tickCmd()}
	if before != nil {
		cmds = append([]tea.Cmd{before}, cmds...)
	}
	return m, tea.Batch(cmds...)
}

// planBlock shows what the intake understood and the plan, with the reasons.
func (m model) planBlock(r *intake.Result) string {
	k := m.look
	head := fmt.Sprintf("%s · %s", r.Intent, r.Artifact)
	if r.Context != nil {
		state := i18n.T("chat.plan.new")
		if !r.Context.New {
			state = strings.Join(r.Context.Has, ", ")
		}
		head += " · " + r.Context.Name + " (" + state + ")"
	}
	var lines []string
	for i, ph := range r.Plan {
		tier := string(ph.Tier)
		switch {
		case ph.Elicit:
			tier += " · " + i18n.T("chat.plan.elicit")
		case ph.Tier != ph.Base:
			tier += " ↑"
		}
		lines = append(lines, fmt.Sprintf("%d. /%s  %s — %s", i+1, ph.Role, tier, ph.TierWhy))
	}
	for _, p := range r.Packs {
		lines = append(lines, k.dim.Render("+ "+p.Pack+" — "+p.Why))
	}
	for _, a := range r.Assumptions {
		lines = append(lines, k.warn.Render(i18n.T("chat.plan.assumed", a)))
	}
	if r.Spent != nil {
		lines = append(lines, k.dim.Render(r.Spent.Note))
	}
	out := "\n" + k.bullet(k.accent, i18n.T("chat.plan.title", head), k.bold, m.width)
	if len(lines) > 0 {
		out += "\n" + k.nested(lines, k.text)
	}
	return out
}

// questionsBlock asks the first open question, its options numbered.
func (m model) questionsBlock(r *intake.Result) string {
	return m.questionBlock(r.Questions[0])
}

// questionBlock asks one question, its options numbered.
func (m model) questionBlock(q intake.Question) string {
	k := m.look
	lines := []string{q.Text}
	for i, o := range q.Options {
		lines = append(lines, k.accent.Render(fmt.Sprintf("%d.", i+1))+" "+o)
	}
	lines = append(lines, "", k.dim.Render(i18n.T("chat.plan.answer_hint")))
	return k.nested(lines, k.text)
}

package chat

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/runs"
)

// What the chat waits for while a plan is open.
const (
	waitNone     = ""
	waitAnswer   = "answer"   // a question of the intake
	waitConfirm  = "confirm"  // the plan, before it starts
	waitContinue = "continue" // the next phase, after a PRD or a spec
	waitElicit   = "elicit"   // a decision an elicitation left
	waitGate     = "gate"     // a decision a delivery's run waits on
	waitReply    = "reply"    // an answer to what the phase's role asked
)

// The three legs of the work, each with its own specialist, one at a time:
// the problem becomes a PRD, the PRD a contract, the contract code. A plan may
// cross more than one; it stops at the end of each for the person.
const (
	legDiscovery = "discovery"
	legSpec      = "spec"
	legDelivery  = "delivery"
)

// legOf is the leg an artifact is produced in, or "" for one that is of none
// — documentation, a status.
func legOf(produces string) string {
	switch {
	case produces == "prd":
		return legDiscovery
	case produces == "spec":
		return legSpec
	case loop.Delivers(produces):
		return legDelivery
	}
	return ""
}

// legAt is the leg of phase i of a plan: an elicitation is of the leg of the
// role it prepares.
func legAt(plan []intake.Phase, i int) string {
	for j := i; j < len(plan) && plan[j].Role == plan[i].Role; j++ {
		if plan[j].Produces != "" {
			return legOf(plan[j].Produces)
		}
	}
	return ""
}

// legName is a leg as the person reads it.
func legName(leg string) string {
	if leg == "" {
		return i18n.T("chat.leg.other")
	}
	return i18n.T("chat.leg." + leg)
}

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
	outcome string    // how the plan ended, for the record
	verdict string    // the QA's last verdict, for the record

	// decisions are what each role's elicitation left and what the person
	// answered; they end the role's turn (intake.WithDecisions).
	decisions map[string]*decided
	asking    []intake.Question // the elicitation's questions still to answer
	askingFor string            // the role they are for

	delivery *delivery // the loop's delivery, while it runs

	// asks says the phase that just ran ended on a question to the person;
	// reviewed is the phase whose document the reviewer already read, so an
	// adjustment to it is not read again on its own.
	asks     bool
	reviewed int
	// fixing says the plan waits on what the discovery's sensor found;
	// accepted, that the person went on past it: it is not put in front of
	// them again until the PRD changes.
	fixing   bool
	accepted bool
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
		return m, say("\n" + k.bullet(k.bad, msg.err.Error(), k.text, m.width) + "\n" + k.nested([]string{i18n.T("chat.plan.send_hint")}, k.dim))
	}
	r := msg.res
	m.plan.res = r
	if r.Direct != "" || r.Chat {
		m.plan = nil
		return m.sendToEngine(r.Request, nil)
	}
	shown := say(m.planBlock(r))
	switch {
	case r.Blocked != "":
		m.plan, m.wait = nil, waitNone
		return m, tea.Sequence(shown, say(k.nested([]string{i18n.T("intake.blocked", r.Blocked)}, k.warn)))
	case r.Ask:
		m.wait = waitAnswer
		return m, tea.Sequence(shown, say(m.questionsBlock(r)))
	case r.Answer != "":
		m.plan, m.wait = nil, waitNone
		return m, tea.Sequence(shown, say(k.nested([]string{i18n.T("chat.plan.answer", r.Answer)}, k.text)))
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
		return m, tea.Sequence(shown, say(k.nested([]string{i18n.T("chat.plan.confirm")}, k.accent)))
	}
	return m.startPhase(shown)
}

// onPlanReply takes what was typed while a plan waits.
func (m model) onPlanReply(text string, echo tea.Cmd) (tea.Model, tea.Cmd) {
	k := m.look
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "cancelar" || t == "/cancelar" {
		m.stopPlan()
		return m, tea.Sequence(echo, say(k.nested([]string{i18n.T("chat.plan.cancelled")}, k.dim)))
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
			return m, tea.Sequence(echo, say(m.questionsBlock(r)))
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
			return m, tea.Sequence(echo, say(m.questionBlock(p.asking[0])))
		}
		m.wait = waitNone
		return m.startPhase(echo)
	case waitGate:
		d := m.plan.delivery
		p := m.plan
		if d == nil || d.gate == nil {
			m.wait = waitNone
			return m, echo
		}
		if t != "seguir" && t != "/seguir" {
			q := p.asking[0]
			answer := strings.TrimSpace(text)
			if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= len(q.Options) {
				answer = q.Options[n-1]
			}
			d.gateAnswers[q.ID] = answer
			p.asking = p.asking[1:]
		} else {
			p.asking = nil
		}
		if len(p.asking) > 0 {
			return m, tea.Sequence(echo, say(m.questionBlock(p.asking[0])))
		}
		// What was not answered takes its recommendation, as gofi answer
		// does.
		answers := d.gate.Elicitation().Defaults()
		maps.Copy(answers, d.gateAnswers)
		run := d.gate.Run
		d.gate, d.gateAnswers, m.wait = nil, nil, waitNone
		if err := loop.Answer(m.opts.Root, run, answers); err != nil {
			return m, tea.Sequence(echo, say(k.nested([]string{err.Error()}, k.warn)))
		}
		return m, echo
	case waitReply:
		if t == "" || t == "seguir" || t == "/seguir" {
			// Left unanswered: the plan goes on as if nothing had been asked.
			// Past the sensor's findings, the PRD is taken as it is; past a
			// question of the role, only the question is.
			p := m.plan
			m.wait, p.asks, p.accepted, p.fixing = waitNone, false, p.accepted || p.fixing, false
			next, cmd := m.afterWriting()
			return next, tea.Sequence(echo, cmd)
		}
		return m.sayToPhase(text, echo)
	case waitConfirm, waitContinue:
		if yes(t) {
			m.wait = waitNone
			// Past a review stop on the contract, the conductor seals what the
			// person approved; one that does not pass stops the plan.
			if p := m.plan; p != nil && p.next > 0 {
				sealed, err := intake.SealContract(m.opts.Root, p.res, p.next-1)
				if err != nil {
					m.stopPlan()
					return m, tea.Sequence(echo, say(k.nested([]string{err.Error()}, k.warn)))
				}
				for _, s := range sealed {
					echo = tea.Sequence(echo, say(k.nested([]string{i18n.T("ask.sealed", s)}, k.dim)))
				}
			}
			return m.startPhase(echo)
		}
		if no(t) {
			m.stopPlan()
			return m, tea.Sequence(echo, say(k.nested([]string{i18n.T("chat.plan.stopped")}, k.dim)))
		}
		// At a review stop, what is written out is an adjustment to the
		// document: it goes to the role that wrote it, in the same session,
		// and the stop comes back. A word or two is a slip of the yes or no.
		if m.wait == waitContinue && m.plan != nil && m.plan.phase != nil && len(strings.Fields(text)) >= 3 {
			return m.sayToPhase(text, echo)
		}
		return m, tea.Sequence(echo, say(k.nested([]string{i18n.T("chat.plan.yes_no")}, k.warn)))
	}
	return m, echo
}

// sayToPhase says something to the role of the phase that just ran — an answer
// to what it asked, an adjustment to what it wrote — as one more turn of its
// session. What follows that turn is what follows the phase.
func (m model) sayToPhase(text string, echo tea.Cmd) (tea.Model, tea.Cmd) {
	m.wait = waitNone
	m.conducting = true
	m.plan.accepted, m.plan.fixing = false, false
	return m.sendToEngine(text, echo)
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

// delivery is a delivery the loop is carrying out for the plan.
type delivery struct {
	events   <-chan loop.Event
	wait     func() error
	stop     func()
	stopping bool
	outcome  string
	// spent is what each run of it has spent, as its events say.
	spent map[string]float64
	// gate is the run whose gate is being answered here, and its answers.
	gate        *loop.Question
	gateAnswers map[string]string
}

// total is what the delivery has spent, all its runs together.
func (d *delivery) total() float64 {
	t := 0.0
	for _, usd := range d.spent {
		t += usd
	}
	return t
}

// stopDelivery stops the delivery under way, once: the run checkpoints and
// can go on with gofi run --resume. It reports whether there was one.
func (m *model) stopDelivery() bool {
	p := m.plan
	if p == nil || p.delivery == nil {
		return false
	}
	if d := p.delivery; !d.stopping && d.stop != nil {
		d.stopping = true
		d.stop()
		m.notice = i18n.T("chat.delivery.stopping")
	}
	return true
}

// startDelivery hands what the plan builds to the loop — the sealed changes
// of the context through the flow of the artifact — and skips the phases the
// flow holds: the build and its audit are the loop's now.
func (m model) startDelivery(flow string, before tea.Cmd) (tea.Model, tea.Cmd) {
	p := m.plan
	r := p.res
	k := m.look
	if m.opts.Deliver == nil || r.Context == nil {
		m.stopPlan()
		return m, tea.Sequence(before, say(k.nested([]string{i18n.T("chat.plan.no_delivery")}, k.warn)))
	}
	p.closePhase()
	d := m.opts.Deliver(flow, r.Context.Name)
	events := d.Events
	p.delivery = &delivery{events: d.Events, wait: d.Wait, stop: d.Stop, spent: map[string]float64{}}
	for p.next++; p.next < len(r.Plan) && loop.Delivers(r.Plan[p.next].Produces) && !r.Plan[p.next].Elicit; p.next++ {
	}
	m.conducting = true
	line := say("\n" + k.bullet(k.accent, i18n.T("ask.handoff", flow), k.bold, m.width))
	if before != nil {
		line = tea.Sequence(before, line)
	}
	return m, tea.Sequence(line, readDelivery(events))
}

func readDelivery(ch <-chan loop.Event) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return deliveryDone{}
		}
		return deliveryEvent{e}
	}
}

// onDeliveryEvent shows the delivery as it goes and reads the next event.
func (m model) onDeliveryEvent(e loop.Event) (tea.Model, tea.Cmd) {
	p := m.plan
	if p == nil || p.delivery == nil {
		return m, nil
	}
	k := m.look
	d := p.delivery
	if e.Run != "" && e.CostUSD > d.spent[e.Run] {
		d.spent[e.Run] = e.CostUSD
	}
	var line string
	switch e.Ev {
	case "node.start":
		line = k.nested([]string{"● " + e.Node}, k.text)
	case "edge":
		line = k.nested([]string{"→ " + e.To + "  (" + e.Why + ")"}, k.dim)
	case "node.failed", "run.cancelled", "land.conflict":
		line = k.nested([]string{e.Ev + " " + e.Why}, k.warn)
	case "dispatch", "escalate", "plan", "skip":
		line = k.nested([]string{e.Ev + " " + strings.TrimSpace(e.To+" "+e.Why)}, k.dim)
	case "gate.open":
		// The run waits on decisions the change does not cover: asked here,
		// answered into the run's gate — any terminal could answer it too.
		if q, ok := loop.OpenGate(m.opts.Root, e.Run); ok && m.wait == waitNone {
			if qs := q.Elicitation().AsQuestions(); len(qs) > 0 {
				d.gate, d.gateAnswers = q, map[string]string{}
				p.asking, m.wait = qs, waitGate
				head := k.nested([]string{i18n.T("chat.delivery.gate_ask", len(qs), e.Run)}, k.accent)
				return m, tea.Sequence(say("\n"+head), say(m.questionBlock(qs[0])), readDelivery(d.events))
			}
		}
		line = k.nested([]string{i18n.T("chat.delivery.gate", e.Why)}, k.warn)
	case "gate.answered":
		line = k.nested([]string{i18n.T("chat.delivery.gate_answered")}, k.dim)
	case "run.result":
		d.outcome = e.Why
		line = k.nested([]string{i18n.T("chat.delivery.result", e.Node, e.Why, d.spent[e.Node])}, k.text)
	}
	next := readDelivery(d.events)
	if line == "" {
		return m, next
	}
	return m, tea.Sequence(say(line), next)
}

// onDeliveryDone goes on with the plan when every change was accepted, and
// stops it with the outcome otherwise.
func (m model) onDeliveryDone() (tea.Model, tea.Cmd) {
	p := m.plan
	if p == nil || p.delivery == nil {
		return m, nil
	}
	k := m.look
	d := p.delivery
	p.delivery = nil
	err := d.wait()
	if d.total() > 0 {
		m.notice = i18n.T("chat.delivery.spent", d.total())
	}
	if err != nil || d.outcome != "accepted" {
		why := d.outcome
		if why == "" && err != nil {
			why = err.Error()
		}
		m.stopPlan()
		return m, say(k.nested([]string{i18n.T("run.not_accepted", why)}, k.warn))
	}
	if p.next >= len(p.res.Plan) {
		m.stopPlan()
		return m, say("\n" + k.bullet(k.good, i18n.T("chat.plan.done"), k.text, m.width))
	}
	return m.startPhase(nil)
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
	if flow, build := loop.FlowFor(ph.Produces); build && !ph.Elicit {
		return m.startDelivery(flow, before)
	}
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
	if ph.AsRole() && m.opts.TierModel != nil && canSwitch {
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
	p.next++
	m.conducting = true
	line := say("\n" + k.bullet(k.accent, header, k.bold, m.width))
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
	if done.Elicit && done.ElicitFile != "" && !m.lastFailed {
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
		return m, say(k.nested(lines, k.warn))
	}
	if done.Elicit {
		if el == nil {
			return m.startPhase(say(k.nested([]string{i18n.T("ask.elicit.none", done.Role)}, k.dim)))
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
			return m, tea.Sequence(say("\n"+head), say(m.questionBlock(qs[0])))
		}
		return m.startPhase(nil)
	}
	return m.afterWriting()
}

// afterWriting is what follows a phase that wrote: the role's question, if it
// left one; the end of the plan; the stop for the person at the end of a leg;
// or the next phase.
func (m model) afterWriting() (tea.Model, tea.Cmd) {
	k := m.look
	p := m.plan
	r := p.res
	done := r.Plan[p.next-1]
	if p.asks && p.phase != nil {
		// The role asked the person something: what is typed now is its
		// answer, in its session — never a request to plan.
		m.wait = waitReply
		return m, say("\n" + k.nested([]string{i18n.T("chat.plan.asked", done.Role)}, k.accent))
	}
	// The discovery's sensor: the PRD just written is read, with no model
	// asked, before anything is built on it.
	var sensed tea.Cmd
	sensedErrs := 0
	if done.Produces == "prd" && r.Context != nil {
		if block, errs := m.prdSensed(r.Context.Name); block != "" {
			sensed, sensedErrs = say(block), errs
		}
	}
	if p.next >= len(r.Plan) {
		if sensedErrs > 0 && p.phase != nil && !p.accepted {
			// The plan ends here, but the PRD still asks for corrections:
			// the role is one line away, in its own session.
			m.wait, p.fixing = waitReply, true
			return m, tea.Sequence(sensed, say("\n"+k.nested([]string{i18n.T("chat.plan.prd_fix", done.Role)}, k.accent)))
		}
		p.outcome = runs.OutcomeDone
		msg := i18n.T("chat.plan.done")
		if r.Next != "" {
			msg += " " + i18n.T("chat.plan.next", r.Next)
		}
		m.stopPlan()
		return m, tea.Sequence(sensed, say("\n"+k.bullet(k.good, msg, k.text, m.width)))
	}
	if done.ReviewAfter || intake.ApprovalAfter(r, p.next-1) {
		if done.Produces == "prd" {
			// The end of the discovery: what the sensor found comes before
			// the stop, and going on stays the person's to decide.
			m.wait = waitContinue
			prompt := m.reviewPrompt()
			if sensedErrs > 0 {
				prompt = i18n.T("chat.plan.prd_blocking", sensedErrs) + " " + prompt
			}
			return m, tea.Sequence(sensed, say("\n"+k.nested([]string{prompt}, k.accent)))
		}
		// A spec is read by the reviewer first: the person approves knowing
		// what it found. It runs aside — a turn of its own — and once: an
		// adjustment after it is checked by the seal, not read again.
		if done.Produces == "spec" && m.opts.Review != nil && r.Context != nil && p.reviewed != p.next {
			p.reviewed = p.next
			review, ctx := m.opts.Review, r.Context.Name
			ch := make(chan tea.Msg, 64)
			go func() {
				defer close(ch)
				lines, blocking, err := review(context.Background(), ctx, func(line string) { ch <- reviewProgress{line: line, ch: ch} })
				ch <- reviewed{lines: lines, blocking: blocking, err: err}
			}()
			return m, tea.Batch(say(k.nested([]string{i18n.T("chat.plan.reviewing", ctx)}, k.dim)), readReview(ch))
		}
		// An approval point: what the next phases build rests on this document.
		m.wait = waitContinue
		return m, say("\n" + k.nested([]string{m.reviewPrompt()}, k.accent))
	}
	return m.startPhase(nil)
}

// prdSensed is what the discovery's sensor says of a context's PRD, as a
// block to show, and how much of it asks for a correction. Empty when there
// is no sensor here or no PRD of the current format to read.
func (m model) prdSensed(ctx string) (string, int) {
	if m.opts.PRD == nil {
		return "", 0
	}
	lines, errs, warns, ok := m.opts.PRD(ctx)
	if !ok {
		return "", 0
	}
	k := m.look
	if len(lines) == 0 {
		return k.nested([]string{i18n.T("chat.prd.ready", ctx)}, k.dim), 0
	}
	style := k.dim
	if errs > 0 {
		style = k.warn
	}
	return k.nested(append([]string{i18n.T("chat.prd.sensor", ctx, errs, warns)}, lines...), style), errs
}

// reviewPrompt is what the stop at the end of a leg says: what was written,
// that an adjustment is written here to the role that wrote it, and what
// going on means — the next leg, by name.
func (m model) reviewPrompt() string {
	p := m.plan
	r := p.res
	done, next := r.Plan[p.next-1], r.Plan[p.next]
	switch {
	case done.Produces == "prd":
		return i18n.T("chat.plan.review.discovery", done.Role, next.Role)
	case done.Produces == "spec" && legAt(r.Plan, p.next) == legDelivery && m.opts.Deliver != nil:
		return i18n.T("chat.plan.review.spec", done.Role)
	}
	return i18n.T("chat.plan.review", done.Produces, next.Role)
}

// reviewed is the review of a spec at its review stop.
type reviewed struct {
	lines    []string
	blocking int
	err      error
}

// reviewProgress is a line of what the review does while it reads; ch is
// where the next one comes from.
type reviewProgress struct {
	line string
	ch   <-chan tea.Msg
}

// readReview reads the next message of a review running aside: a line of
// progress, or the review itself.
func readReview(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// onReviewed shows what the review found and stops for the person: the
// approval stays theirs, the blocking findings said before it.
func (m model) onReviewed(msg reviewed) (tea.Model, tea.Cmd) {
	k := m.look
	p := m.plan
	if p == nil {
		return m, nil
	}
	out := []tea.Cmd{}
	if msg.err != nil {
		out = append(out, say(k.nested([]string{msg.err.Error()}, k.warn)))
	} else if len(msg.lines) > 0 {
		style := k.dim
		if msg.blocking > 0 {
			style = k.warn
		}
		out = append(out, say(k.nested(msg.lines, style)))
	}
	m.wait = waitContinue
	prompt := m.reviewPrompt()
	if msg.blocking > 0 {
		prompt = i18n.T("chat.plan.review_blocking", msg.blocking) + " " + prompt
	}
	out = append(out, say("\n"+k.nested([]string{prompt}, k.accent)))
	return m, tea.Sequence(out...)
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
	m.stopDelivery()
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
	turn := engine.Turn{Prompt: text, Images: m.outgoing}
	m.outgoing = nil
	m.said, m.asked = "", false
	events, err := m.engine().Send(context.Background(), turn)
	if err != nil {
		m.stopPlan()
		return m, tea.Sequence(before, say(m.look.bullet(m.look.bad, err.Error(), m.look.text, m.width)))
	}
	m.running, m.events, m.started, m.frame = true, events, time.Now(), 0
	m.live, m.lastFailed = "", false
	m.act = activity{}
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
	// The plan is read by its legs — discovery, specification, delivery —,
	// each with its own specialist: it is where the plan stops for review.
	legs := 0
	for i := range r.Plan {
		if i == 0 || legAt(r.Plan, i) != legAt(r.Plan, i-1) {
			legs++
		}
	}
	for i := 0; i < len(r.Plan); i++ {
		ph := r.Plan[i]
		leg := legAt(r.Plan, i)
		if legs > 1 && (i == 0 || leg != legAt(r.Plan, i-1)) {
			lines = append(lines, k.bold.Render(legName(leg)))
		}
		if _, build := loop.FlowFor(ph.Produces); build && !ph.Elicit && m.opts.Deliver != nil && r.Context != nil {
			// What is built is the loop's: its roles are dispatched by the
			// change, not run here one by one.
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, i18n.T("chat.leg.loop")))
			for i+1 < len(r.Plan) && loop.Delivers(r.Plan[i+1].Produces) && !r.Plan[i+1].Elicit {
				i++
			}
			continue
		}
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
	if r.Found != nil {
		// A context being adopted: what the repository already holds of it
		// is where its documents start from.
		lines = append(lines, k.dim.Render(i18n.T("chat.plan.found", r.Found.Summary())))
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

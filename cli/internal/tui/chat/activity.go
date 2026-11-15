package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

// The panel over the conversation says what is at work, as it happens: the
// plan being conducted and where it is, this conversation's own turn — how
// long, how many tools, the last one —, the subagents the turn started, and
// the runs of the loop with their agents. All of it is read from what the
// engine already streams and from the runs on disk: nothing is asked of a
// model to draw it.

// activity is what the conversation's turn has done so far.
type activity struct {
	tools int         // tool calls of the turn
	doing string      // the last one, in a few words
	subs  []*subagent // the subagents it started, in order
}

// subagent is an agent the turn started with a tool call, followed by the
// steps the engine reports under that call.
type subagent struct {
	id      string // the tool call that started it
	kind    string // what agent it is: Explore, general-purpose…
	what    string // what it was asked, in a few words
	started time.Time
	steps   int
	doing   string
	done    bool
}

// subagentTools are the engine's tools that start a subagent.
var subagentTools = map[string]bool{"Task": true, "Agent": true}

// noteCall takes a tool call into the activity: the turn's own, or — with
// parent — a step of the subagent that call started.
func (m *model) noteCall(b engine.Block, parent string) {
	doing := strings.TrimSpace(b.Name + " " + toolSummary(b.Input, m.opts.Root, 60))
	if parent != "" {
		for _, s := range m.act.subs {
			if s.id == parent {
				s.steps++
				s.doing = doing
			}
		}
		return
	}
	m.act.tools++
	m.act.doing = doing
	if subagentTools[b.Name] {
		var in struct {
			Kind string `json:"subagent_type"`
			What string `json:"description"`
		}
		_ = json.Unmarshal(b.Input, &in)
		if in.Kind == "" {
			in.Kind = b.Name
		}
		m.act.subs = append(m.act.subs, &subagent{id: b.ID, kind: in.Kind, what: in.What, started: time.Now()})
	}
}

// noteResult marks the subagent a tool call started as finished.
func (m *model) noteResult(id string) {
	for _, s := range m.act.subs {
		if s.id == id {
			s.done = true
		}
	}
}

// turnAtWork reports a turn of the conversation — or the planning of one —
// under way.
func (m model) turnAtWork() bool { return m.running && (m.events != nil || m.planning) }

// busy reports whether the panel has anything to show but its head.
func (m model) busy() bool {
	return m.turnAtWork() || m.board.Active() || (m.plan != nil && m.plan.res != nil && len(m.plan.res.Plan) > 0)
}

// panelLimit is how many lines the panel may take of a screen: a third of
// it, so the conversation always has the larger part.
func (m model) panelLimit() int { return min(max(m.height/3, 4), 12) }

// panel draws what is at work in at most limit lines, the last one a rule.
// With nothing at work it is its head alone, saying so.
func (m model) panel(limit int) []string {
	k := m.look
	w := max(m.width-1, 20)
	sep := k.dim.Render(" · ")

	var facts, rows []string
	if p := m.plan; p != nil && p.res != nil && len(p.res.Plan) > 0 {
		facts = append(facts, i18n.T("panel.plan", min(max(p.next, 1), len(p.res.Plan)), len(p.res.Plan)))
		rows = append(rows, ansi.Truncate(" "+k.dim.Render(i18n.T("panel.plan_row"))+"  "+m.planStrip(), w, "…"))
	}
	if m.turnAtWork() {
		secs := elapsed(int(time.Since(m.started).Seconds()))
		facts = append(facts, i18n.T("panel.turn", secs))
		who := i18n.T("panel.conversation")
		switch {
		case m.planning:
			who = i18n.T("panel.planning")
		case m.conducting && m.plan != nil && m.plan.next > 0 && m.plan.next <= len(m.plan.res.Plan):
			who = "/" + m.plan.res.Plan[m.plan.next-1].Role
		}
		line := " " + k.accent.Render("●") + " " + k.bold.Render(who)
		about := []string{secs}
		if m.modelN != "" && !m.planning {
			about = append([]string{m.modelN}, about...)
		}
		if m.act.tools > 0 {
			about = append(about, i18n.T("panel.tools", m.act.tools))
		}
		line += k.dim.Render(" · " + strings.Join(about, " · "))
		if m.act.doing != "" {
			line += "  " + k.dim.Render(m.act.doing)
		}
		rows = append(rows, ansi.Truncate(line, w, "…"))
		done := 0
		for _, s := range m.act.subs {
			if s.done {
				done++
				continue
			}
			name := s.kind
			if s.what != "" {
				name += " · " + s.what
			}
			line := "     " + k.accent.Render("●") + " " + name +
				k.dim.Render(" · "+elapsed(int(time.Since(s.started).Seconds()))+" · "+i18n.T("panel.steps", s.steps))
			if s.doing != "" {
				line += "  " + k.dim.Render(s.doing)
			}
			rows = append(rows, ansi.Truncate(line, w, "…"))
		}
		if done > 0 {
			rows = append(rows, "     "+k.good.Render("✓")+" "+k.dim.Render(i18n.T("panel.subs_done", done)))
		}
	}
	facts = append(facts, m.board.Facts()...)
	if len(facts) == 0 {
		facts = []string{k.dim.Render(i18n.T("panel.idle"))}
	}

	head := k.accent.Render("●") + " " + k.bold.Render(i18n.T("panel.title")) + sep + strings.Join(facts, sep)
	hint := k.dim.Render(i18n.T("board.hint"))
	gap := w - ansi.StringWidth(head) - ansi.StringWidth(hint)
	if gap < 2 {
		head = ansi.Truncate(head, w, "…")
	} else {
		head += strings.Repeat(" ", gap) + hint
	}

	// The conversation's rows come first; the runs take what is left.
	room := max(limit-2, 0)
	if len(rows) > room {
		hidden := len(rows) - room + 1
		rows = rows[:max(room-1, 0)]
		if room > 0 {
			rows = append(rows, k.dim.Render("     "+i18n.T("board.more", hidden)))
		}
	}
	rows = append(rows, m.board.Rows(w, room-len(rows))...)
	out := append([]string{head}, rows...)
	return append(out, k.border.Render(strings.Repeat("─", w)))
}

// planStrip is the plan on one line: what ran, the phase at work, what is
// still to come.
func (m model) planStrip() string {
	k := m.look
	p := m.plan
	at := p.next - 1 // the phase at work, or the last one that ran
	working := m.conducting || p.delivery != nil
	var parts []string
	for i, ph := range p.res.Plan {
		name := ph.Role
		switch {
		case i < at || (i == at && !working):
			parts = append(parts, k.good.Render("✓")+" "+name)
		case i == at:
			parts = append(parts, k.accent.Render("● "+name))
		default:
			parts = append(parts, k.dim.Render("○ "+name))
		}
	}
	return strings.Join(parts, k.dim.Render(" › "))
}

// steer is a message sent while the turn runs, waiting in the engine for
// the model's next step.
type steer struct {
	id, text string
}

// steerLines shows the messages that wait, over the prompt, until the model
// takes them and they join the conversation where it did.
func (m model) steerLines() []string {
	if len(m.steered) == 0 {
		return nil
	}
	k := m.look
	var out []string
	for _, s := range m.steered {
		text := strings.Join(strings.Fields(s.text), " ")
		out = append(out, k.accent.Render("  ⏵ ")+k.text.Render(ansi.Truncate(text, max(m.width-6, 10), "…")))
	}
	hint := i18n.T("chat.steer.hint")
	if n := len(m.steered); n > 1 {
		hint = fmt.Sprintf("%d · %s", n, hint)
	}
	return append(out, k.dim.Render(ansi.Truncate("    "+hint, max(m.width-1, 10), "…")))
}

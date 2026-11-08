package watch

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// group is a context and its runs, the one that matters most first.
type group struct {
	name string
	tabs []int
}

// phase is a node of a run and every agent that worked in it, however many
// times the run came back to it: what a person follows, where the agents are
// its detail.
type phase struct {
	node   string
	agents []int
	status string
	// rounds is how many times the phase was worked: the most turns any one
	// role had in it. Roles that take turns within a phase are one round.
	rounds int
	cost   float64
}

type rowKind int

const (
	rowContext rowKind = iota
	rowRun
	rowPhase
	rowAgent
)

// row is a line of the tree. A context with one run is that run — no line of
// its own for it — so tab is -1 only on a context of several. The key is
// "@context", "run", "run/node" or "run/node/agent": its prefixes are the
// row's parents, what the focus falls back to when a row goes away.
type row struct {
	kind              rowKind
	key               string
	group, tab, phase int
	agent, depth      int
	fold, open        bool
}

// The phases of a run, as they are worked and as they are still to come.
const (
	stepDone = iota
	stepNow
	stepWait
	stepBad
	stepNext
)

type step struct {
	node  string
	state int
}

// readFlow is the flow a run carries out, in the order it walks; nil when the
// run is older than flow.json.
func readFlow(path string) *loop.FlowView {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var v loop.FlowView
	if json.Unmarshal(data, &v) != nil || v.Schema != loop.FlowViewSchema {
		return nil
	}
	return &v
}

// walk is the flow's nodes breadth first from its start: the order a run
// goes through them when nothing sends it back.
func walk(v *loop.FlowView) []loop.ViewNode {
	byName := map[string]loop.ViewNode{}
	for _, n := range v.Nodes {
		byName[n.Name] = n
	}
	seen := map[string]bool{v.Start: true}
	names := []string{v.Start}
	for i := 0; i < len(names); i++ {
		for _, e := range v.Edges {
			if e.From == names[i] && !seen[e.To] {
				seen[e.To] = true
				names = append(names, e.To)
			}
		}
	}
	var out []loop.ViewNode
	for _, n := range names {
		if node, ok := byName[n]; ok {
			out = append(out, node)
		}
	}
	return out
}

// shape fills in a run's phases, in the flow's order, and the steps of its
// progress.
func (t *tab) shape(flow *loop.FlowView) {
	t.phases = nil
	at := map[string]int{}
	for i, a := range t.agents {
		j, seen := at[a.Node]
		if !seen {
			j = len(t.phases)
			at[a.Node] = j
			t.phases = append(t.phases, phase{node: a.Node})
		}
		t.phases[j].agents = append(t.phases[j].agents, i)
		t.phases[j].cost += a.CostUSD
	}
	for i := range t.phases {
		t.phases[i].settle(t.agents)
	}
	var nodes []loop.ViewNode
	if flow != nil {
		nodes = walk(flow)
	}
	order := func(node string) int {
		if i := slices.IndexFunc(nodes, func(n loop.ViewNode) bool { return n.Name == node }); i >= 0 {
			return i
		}
		return len(nodes)
	}
	sort.SliceStable(t.phases, func(i, j int) bool { return order(t.phases[i].node) < order(t.phases[j].node) })
	t.steps = t.stepsOf(nodes)
}

// settle is how a phase stands: an agent at work says so, and otherwise each
// role's last turn does — a lens that failed and was run again is not a
// failed audit.
func (p *phase) settle(agents []loop.AgentState) {
	last, turns := map[string]int{}, map[string]int{}
	for _, i := range p.agents {
		a := agents[i]
		last[a.Role] = max(last[a.Role], a.Iter)
		turns[a.Role]++
		p.rounds = max(p.rounds, turns[a.Role])
	}
	p.status = loop.AgentDone
	for _, i := range p.agents {
		switch a := agents[i]; {
		case a.Status == loop.AgentWorking:
			p.status = loop.AgentWorking
			return
		case a.Status == loop.AgentBlocked:
			p.status = loop.AgentBlocked
		case a.Status == loop.AgentFailed && a.Iter == last[a.Role] && p.status != loop.AgentBlocked:
			p.status = loop.AgentFailed
		}
	}
}

// stepsOf is the run's way through its flow: what ran, where it is, what is
// left. What it went past without running was optional and is not shown; a
// detour — repair, triage — only when the run took it.
func (t *tab) stepsOf(nodes []loop.ViewNode) []step {
	s, ran := t.info.State, map[string]string{}
	for _, p := range t.phases {
		ran[p.node] = p.status
	}
	if nodes == nil {
		out := make([]step, 0, len(t.phases))
		for _, p := range t.phases {
			out = append(out, step{p.node, stepState(p.status)})
		}
		return out
	}
	shown := slices.DeleteFunc(slices.Clone(nodes), func(n loop.ViewNode) bool { return n.Kind != "agent" && n.Kind != "sensor" })
	reached := -1
	for i, n := range shown {
		if _, ok := ran[n.Name]; ok || n.Name == s.Node {
			reached = i
		}
	}
	var out []step
	for i, n := range shown {
		status, did := ran[n.Name]
		switch {
		case n.Name == s.Node && (!did || status == loop.AgentDone):
			out = append(out, step{n.Name, stepNow})
		case did:
			out = append(out, step{n.Name, stepState(status)})
		case i < reached && n.Kind == "sensor":
			out = append(out, step{n.Name, stepDone})
		case i > reached && s.Node != "" && n.Stage != "repair" && n.Stage != "triage":
			out = append(out, step{n.Name, stepNext})
		}
	}
	return out
}

func stepState(status string) int {
	switch status {
	case loop.AgentWorking:
		return stepNow
	case loop.AgentBlocked:
		return stepWait
	case loop.AgentFailed:
		return stepBad
	}
	return stepDone
}

// rank is how much a run asks for the person's eye: what waits on them, what
// is working, what stopped, what is over.
func rank(l string) int {
	switch l {
	case loop.Blocked:
		return 0
	case loop.Running:
		return 1
	case loop.Ended:
		return 3
	}
	return 2
}

// groupTabs gathers the runs by context, the contexts and the runs in each
// by rank and otherwise as the runs are listed.
func groupTabs(tabs []tab) []group {
	var out []group
	at := map[string]int{}
	for i, t := range tabs {
		name := t.info.State.Context
		if name == "" {
			name = t.info.State.Change
		}
		j, seen := at[name]
		if !seen {
			j = len(out)
			at[name] = j
			out = append(out, group{name: name})
		}
		out[j].tabs = append(out[j].tabs, i)
	}
	level := func(i int) int { return rank(tabs[i].info.Liveness) }
	for _, g := range out {
		sort.SliceStable(g.tabs, func(a, b int) bool { return level(g.tabs[a]) < level(g.tabs[b]) })
	}
	sort.SliceStable(out, func(a, b int) bool { return level(out[a].tabs[0]) < level(out[b].tabs[0]) })
	return out
}

func live(t tab) bool { return t.info.Liveness != loop.Ended }

// isOpen is what the person chose for a row, or what suits it: what is alive
// shows, what is over folds.
func (m *model) isOpen(key string, auto bool) bool {
	if v, ok := m.open[key]; ok {
		return v
	}
	return auto
}

func (m *model) layoutRows() {
	m.rows = nil
	for gi, g := range m.groups {
		lead := m.tabs[g.tabs[0]]
		r := row{kind: rowContext, key: "@" + g.name, group: gi, tab: -1, agent: -1}
		r.open = m.isOpen(r.key, gi == 0 || live(lead))
		if len(g.tabs) == 1 {
			r.tab = g.tabs[0]
			r.fold = len(lead.phases) > 0
			m.rows = append(m.rows, r)
			if r.open {
				m.addPhases(gi, g.tabs[0], 1)
			}
			continue
		}
		r.fold = true
		m.rows = append(m.rows, r)
		if !r.open {
			continue
		}
		for _, ti := range g.tabs {
			t := m.tabs[ti]
			rr := row{kind: rowRun, key: t.info.State.Run, group: gi, tab: ti, agent: -1, depth: 1, fold: len(t.phases) > 0}
			rr.open = m.isOpen(rr.key, live(t))
			m.rows = append(m.rows, rr)
			if rr.open {
				m.addPhases(gi, ti, 2)
			}
		}
	}
}

// addPhases lays out a run's phases; a phase of one agent is that agent.
func (m *model) addPhases(gi, ti, depth int) {
	t := m.tabs[ti]
	run := t.info.State.Run
	for pi, p := range t.phases {
		r := row{kind: rowPhase, key: run + "/" + p.node, group: gi, tab: ti, phase: pi, agent: -1, depth: depth}
		if len(p.agents) == 1 {
			r.agent = p.agents[0]
			m.rows = append(m.rows, r)
			continue
		}
		r.fold = true
		r.open = m.isOpen(r.key, p.status == loop.AgentWorking || p.status == loop.AgentBlocked)
		m.rows = append(m.rows, r)
		if !r.open {
			continue
		}
		for _, ai := range p.agents {
			m.rows = append(m.rows, row{kind: rowAgent, key: r.key + "/" + t.agents[ai].ID, group: gi, tab: ti, phase: pi, agent: ai, depth: depth + 1})
		}
	}
}

// parents are the keys above a row's, nearest first.
func (m *model) parents(key string) []string {
	if strings.HasPrefix(key, "@") {
		return nil
	}
	parts := strings.Split(key, "/")
	var out []string
	for i := len(parts) - 1; i > 0; i-- {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	for _, g := range m.groups {
		for _, ti := range g.tabs {
			if m.tabs[ti].info.State.Run == parts[0] {
				out = append(out, "@"+g.name)
			}
		}
	}
	return out
}

// reveal opens everything above a row, and keeps it so: a run ending does not
// fold away what the person is reading.
func (m *model) reveal(key string) {
	for _, p := range m.parents(key) {
		m.open[p] = true
	}
}

// place finds the row in focus again, or the nearest one above it.
func (m *model) place() {
	m.cursor = 0
	for _, want := range append([]string{m.focus}, m.parents(m.focus)...) {
		if i := slices.IndexFunc(m.rows, func(r row) bool { return r.key == want }); i >= 0 {
			m.cursor = i
			break
		}
	}
	if len(m.rows) > 0 {
		m.focus = m.rows[m.cursor].key
	}
}

// activeKey is the row of what is happening now: a question waiting on the
// person, or the first agent at work.
func (m *model) activeKey() string {
	for _, g := range m.groups {
		for _, ti := range g.tabs {
			t := m.tabs[ti]
			run := t.info.State.Run
			if t.gate != nil {
				if len(g.tabs) == 1 {
					return "@" + g.name
				}
				return run
			}
			if t.info.Liveness != loop.Running {
				continue
			}
			for _, p := range t.phases {
				for _, ai := range p.agents {
					if t.agents[ai].Status != loop.AgentWorking {
						continue
					}
					if len(p.agents) == 1 {
						return run + "/" + p.node
					}
					return run + "/" + p.node + "/" + t.agents[ai].ID
				}
			}
		}
	}
	return ""
}

// phaseLabel is a node in the person's words; a project's own node, its name.
func phaseLabel(node string) string {
	key := "watch.phase." + node
	if v := i18n.T(key); v != key {
		return v
	}
	return node
}

// where is the phase a run is in: past a gate, the phase that asked.
func where(t tab) string {
	s := t.info.State
	if t.gate != nil && s.Agent != "" {
		return s.Agent
	}
	return s.Node
}

// say is how a run stands, in one sentence.
func say(t tab) string {
	s := t.info.State
	cost := money(s.CostUSD)
	switch t.info.Liveness {
	case loop.Running:
		return i18n.T("watch.say.running", phaseLabel(where(t)), cost)
	case loop.Blocked:
		return i18n.T("watch.say.blocked", phaseLabel(where(t)), cost)
	case loop.Interrupted:
		return i18n.T("watch.say.interrupted", phaseLabel(where(t)), cost)
	case loop.Paused:
		return i18n.T("watch.say.paused", phaseLabel(where(t)), cost)
	}
	if s.Outcome != "accepted" {
		return i18n.T("watch.say.ended", s.Outcome, cost)
	}
	switch s.Landing {
	case "integrada":
		return i18n.T("watch.say.landed", cost)
	case "aguardando":
		return i18n.T("watch.say.waiting", cost)
	case "conflito":
		return i18n.T("watch.say.conflict", cost)
	}
	return i18n.T("watch.say.accepted", cost)
}

// money is an amount as the person's language writes it.
func money(v float64) string {
	return "US$ " + strings.Replace(fmt.Sprintf("%.2f", v), ".", i18n.T("watch.decimal"), 1)
}

// count is a number of tokens at a glance.
func count(n int) string {
	var s string
	switch {
	case n >= 1_000_000:
		s = fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		s = fmt.Sprintf("%.0fk", float64(n)/1e3)
	default:
		return fmt.Sprint(n)
	}
	return strings.Replace(s, ".", i18n.T("watch.decimal"), 1)
}

// tokens is what the agents read and wrote, each kind apart: the kinds differ
// in price by far too much for their sum to mean anything. Empty while no
// turn has reported.
func tokens(agents []loop.AgentState, of []int) string {
	var u loop.Usage
	seen := false
	for _, i := range of {
		if a := agents[i].Usage; a != nil {
			seen = true
			u.Input += a.Input
			u.Output += a.Output
			u.CacheRead += a.CacheRead
			u.CacheWrite += a.CacheWrite
		}
	}
	if !seen {
		return ""
	}
	return i18n.T("watch.tokens", count(u.Input), count(u.Output), count(u.CacheRead), count(u.CacheWrite))
}

func everyAgent(t tab) []int {
	out := make([]int, len(t.agents))
	for i := range out {
		out[i] = i
	}
	return out
}

// mark is a state's sign, readable with color off.
func (k look) mark(state int) string {
	switch state {
	case stepDone:
		return k.good.Render("✓")
	case stepNow:
		return k.accent.Render("●")
	case stepWait:
		return k.warn.Render("!")
	case stepBad:
		return k.bad.Render("✗")
	}
	return k.dim.Render("○")
}

// runState is the sign of a run: working, waiting on the person, stopped, or
// how it ended.
func runState(i loop.Info) int {
	switch i.Liveness {
	case loop.Running:
		return stepNow
	case loop.Blocked:
		return stepWait
	case loop.Ended:
		switch {
		case i.State.Landing == "aguardando":
			return stepWait
		case i.State.Outcome == "accepted" && i.State.Landing != "conflito":
			return stepDone
		}
		return stepBad
	}
	return stepNext
}

// strip draws a run's progress on one line, the step it is on standing out.
func (k look) strip(t tab) string {
	parts := make([]string, 0, len(t.steps))
	for _, s := range t.steps {
		name := phaseLabel(s.node)
		switch s.state {
		case stepNow:
			name = k.accent.Render(name)
		case stepNext:
			name = k.dim.Render(name)
		}
		parts = append(parts, k.mark(s.state)+" "+name)
	}
	return strings.Join(parts, k.dim.Render(" › "))
}

// spread puts right at the end of a line of width w, left cut short if it
// must be.
func spread(left, right string, w int) string {
	room := w - ansi.StringWidth(right) - 1
	left = ansi.Truncate(left, max(room, 1), "…")
	return left + strings.Repeat(" ", max(w-ansi.StringWidth(left)-ansi.StringWidth(right), 1)) + right
}

// wrap breaks a line to the width of the pane.
func wrap(s string, w int) []string {
	return strings.Split(ansi.Wrap(s, max(w, 10), ""), "\n")
}

// Package watch is gofi watch: the runs of a project, by context (0002, D22).
// On the left, each context says in a sentence how it stands and folds out
// into its phases and their agents; on the right, the row in focus shows its
// progress and what it is doing. The screen only reads: every fact is on
// disk, written by the supervisors, so it shows the same whichever terminal
// started a run and keeps showing it when the screen is closed and opened
// again.
package watch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
)

// Options is the project the screen watches and what it acts with.
type Options struct {
	Root, Project string
	// Start starts gofi run in the background — a change, or --resume <run>
	// — and returns the run it carries out.
	Start func(args ...string) (string, error)
	// Changes are the sealed changes that can be started.
	Changes func() ([]string, error)
	// Every is how often the disk is read again.
	Every time.Duration
}

// Run opens the screen until the person quits.
func Run(o Options) error {
	_, err := tea.NewProgram(New(o), tea.WithAltScreen()).Run()
	return err
}

// Frame is the screen once, in plain text, at a width and a height: what a
// panel beside an editor draws, with no terminal to read keys from — so
// without the line that lists them.
func Frame(o Options, w, h int) string {
	m, _ := New(o).Update(tea.WindowSizeMsg{Width: w, Height: h})
	v := m.View()
	if i := strings.LastIndex(v, "\n"); i >= 0 {
		v = v[:i]
	}
	return ansi.Strip(v)
}

// tab is one run and its panes.
type tab struct {
	info   loop.Info
	agents []loop.AgentState
	gate   *loop.Question
	phases []phase
	steps  []step
}

const (
	browsing = iota
	answering
	starting
)

type model struct {
	o      Options
	tabs   []tab
	groups []group
	flows  map[string]*loop.FlowView
	rows   []row
	cursor int
	// focus names the row in focus, so it stays in focus when the tree is
	// read again and rows come and go.
	focus string
	// follow moves the focus to whatever is happening, until the person
	// moves it themselves.
	follow bool
	// open is what the person folded or unfolded; the rest folds by itself.
	open map[string]bool
	// head stays at the top of the pane, over the body that scrolls; scroll
	// is how many lines above the end the body is read, 0 following it.
	head, pane []string
	paneOf     string
	scroll     int
	mode       int
	ask        asking
	pick       picking
	note       string
	w, h       int
	look       look
}

// asking walks the questions of an open gate.
type asking struct {
	run     string
	qs      []intake.ElicitQuest
	i, opt  int
	text    string
	answers map[string]string
}

// picking chooses a sealed change to start.
type picking struct {
	changes []string
	i       int
}

type tick struct{}

// New is the screen's model, read once from disk.
func New(o Options) tea.Model {
	if o.Every <= 0 {
		o.Every = 500 * time.Millisecond
	}
	m := model{o: o, w: 100, h: 30, look: newLook(), follow: true, open: map[string]bool{}, flows: map[string]*loop.FlowView{}}
	m.reload()
	return m
}

func (m model) Init() tea.Cmd { return m.wait() }

func (m model) wait() tea.Cmd {
	return tea.Tick(m.o.Every, func(time.Time) tea.Msg { return tick{} })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.reload()
		return m, nil
	case tick:
		m.reload()
		return m, m.wait()
	case tea.KeyMsg:
		switch m.mode {
		case answering:
			return m.onAnswerKey(msg)
		case starting:
			return m.onPickKey(msg)
		}
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "right", "l":
		m.unfold()
	case "left", "h":
		m.foldOrUp()
	case "enter":
		if r, ok := m.row(); ok && r.fold {
			m.open[r.key], m.follow = !r.open, false
		}
	case "tab":
		m.jump(rowContext, 1)
	case "shift+tab":
		m.jump(rowContext, -1)
	case "]":
		m.jump(rowPhase, 1)
	case "[":
		m.jump(rowPhase, -1)
	case "f":
		m.follow, m.scroll = true, 0
	case "pgup":
		m.follow = false
		m.scrollBy(m.bodyHeight() - 1)
	case "pgdown":
		m.follow = false
		m.scrollBy(1 - m.bodyHeight())
	case "end":
		m.scroll = 0
	case "a":
		t, ok := m.current()
		if !ok {
			m.note = i18n.T("watch.pick_run")
			break
		}
		if t.gate == nil {
			m.note = i18n.T("watch.no_gate")
			break
		}
		m.mode, m.ask = answering, asking{run: t.info.State.Run, qs: t.gate.Questions, answers: map[string]string{}}
	case "c":
		t, ok := m.current()
		if !ok {
			m.note = i18n.T("watch.pick_run")
			break
		}
		if err := loop.Cancel(m.o.Root, t.info.State.Run); err != nil {
			m.note = err.Error()
		} else {
			m.note = i18n.T("watch.cancelled", t.info.State.Run)
		}
	case "r":
		t, ok := m.current()
		if !ok {
			m.note = i18n.T("watch.pick_run")
			break
		}
		if m.o.Start == nil {
			break
		}
		if l := t.info.Liveness; l != loop.Interrupted && l != loop.Paused {
			m.note = i18n.T("watch.not_resumable", t.info.State.Run)
			break
		}
		m.note = m.started(m.o.Start("--resume", t.info.State.Run))
	case "n":
		if m.o.Changes == nil || m.o.Start == nil {
			break
		}
		changes, err := m.o.Changes()
		if err != nil {
			m.note = err.Error()
			break
		}
		if len(changes) == 0 {
			m.note = i18n.T("watch.no_changes")
			break
		}
		m.mode, m.pick = starting, picking{changes: changes}
	}
	m.reload()
	return m, nil
}

func (m model) started(run string, err error) string {
	if err != nil {
		return err.Error()
	}
	return i18n.T("watch.started", run)
}

func (m model) onAnswerKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	a := &m.ask
	q := a.qs[a.i]
	switch k.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		m.mode, m.note = browsing, i18n.T("watch.answer_dropped")
		return m, nil
	case tea.KeyUp:
		a.opt = max(a.opt-1, 0)
		return m, nil
	case tea.KeyDown:
		a.opt = min(a.opt+1, max(len(options(q))-1, 0))
		return m, nil
	case tea.KeyBackspace:
		if r := []rune(a.text); len(r) > 0 {
			a.text = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyRunes, tea.KeySpace:
		if len(q.Options) == 0 {
			a.text += string(k.Runes)
			if k.Type == tea.KeySpace {
				a.text += " "
			}
		}
		return m, nil
	case tea.KeyEnter:
		if opts := options(q); len(opts) > 0 {
			a.answers[q.ID] = opts[a.opt]
		} else if v := strings.TrimSpace(a.text); v != "" {
			a.answers[q.ID] = v
		}
		a.i, a.opt, a.text = a.i+1, 0, ""
		if a.i < len(a.qs) {
			return m, nil
		}
		m.mode = browsing
		if err := loop.Answer(m.o.Root, a.run, a.answers); err != nil {
			m.note = err.Error()
		} else {
			m.note = i18n.T("answer.sent", a.run)
		}
		m.reload()
	}
	return m, nil
}

func (m model) onPickKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.pick
	switch k.String() {
	case "esc", "ctrl+c", "q":
		m.mode = browsing
	case "up", "k":
		p.i = max(p.i-1, 0)
	case "down", "j":
		p.i = min(p.i+1, len(p.changes)-1)
	case "enter":
		m.mode = browsing
		m.note = m.started(m.o.Start(p.changes[p.i]))
		m.reload()
	}
	return m, nil
}

// options are a question's choices, the recommended one first: enter on
// the first line takes the recommendation.
func options(q intake.ElicitQuest) []string {
	if len(q.Options) == 0 {
		return nil
	}
	out := []string{}
	if q.Recommended != "" {
		out = append(out, q.Recommended)
	}
	for _, o := range q.Options {
		if o != q.Recommended {
			out = append(out, o)
		}
	}
	return out
}

func (m *model) move(d int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor = min(max(m.cursor+d, 0), len(m.rows)-1)
	m.focusOn(m.rows[m.cursor].key)
}

// focusOn is the person choosing a row: it stops following what happens, and
// keeps the row where it is when the run around it ends.
func (m *model) focusOn(key string) {
	m.focus, m.follow = key, false
	m.reveal(key)
}

// unfold opens the row in focus; open already, it goes into it.
func (m *model) unfold() {
	r, ok := m.row()
	if !ok || !r.fold {
		return
	}
	if r.open {
		m.move(1)
		return
	}
	m.open[r.key], m.follow = true, false
}

// foldOrUp closes the row in focus; closed already, it goes to its parent.
func (m *model) foldOrUp() {
	r, ok := m.row()
	if !ok {
		return
	}
	if r.fold && r.open {
		m.open[r.key], m.follow = false, false
		return
	}
	for _, p := range m.parents(r.key) {
		if slices.ContainsFunc(m.rows, func(x row) bool { return x.key == p }) {
			m.focusOn(p)
			return
		}
	}
}

// jump goes to the next row of a kind, around the end of the tree.
func (m *model) jump(kind rowKind, d int) {
	for i, n := 1, len(m.rows); i <= n; i++ {
		if r := m.rows[((m.cursor+d*i)%n+n)%n]; r.kind == kind {
			m.focusOn(r.key)
			return
		}
	}
}

func (m *model) scrollBy(d int) {
	m.scroll = min(max(m.scroll+d, 0), max(len(m.pane)-m.bodyHeight(), 0))
}

func (m model) row() (row, bool) {
	if m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

// current is the run of the row in focus; a context of several has none.
func (m model) current() (tab, bool) {
	r, ok := m.row()
	if !ok || r.tab < 0 {
		return tab{}, false
	}
	return m.tabs[r.tab], true
}

// reload reads every run again, keeps the row in focus, and reads its pane.
func (m *model) reload() {
	infos, _ := loop.List(m.o.Root)
	m.tabs = nil
	for _, info := range infos {
		t := tab{info: info}
		t.agents, _ = loop.Agents(m.o.Root, info.State.Run)
		if q, ok := loop.OpenGate(m.o.Root, info.State.Run); ok {
			t.gate = q
		}
		t.shape(m.flow(info.State.Run))
		m.tabs = append(m.tabs, t)
	}
	m.groups = groupTabs(m.tabs)
	if m.follow {
		if k := m.activeKey(); k != "" {
			m.focus = k
			m.reveal(k)
		}
	}
	m.layoutRows()
	m.place()
	m.readPane()
}

// flow is what a run carries out; it does not change once it is written.
func (m *model) flow(run string) *loop.FlowView {
	if v, ok := m.flows[run]; ok {
		return v
	}
	v := readFlow(loop.FlowFile(m.o.Root, run))
	if v != nil {
		m.flows[run] = v
	}
	return v
}

// readPane reads what the row in focus shows, keeping the person's place in
// it as it grows.
func (m *model) readPane() {
	before, of := len(m.pane), m.paneOf
	m.head, m.pane = m.paneFor()
	m.paneOf = m.focus
	switch {
	case of != m.focus:
		m.scroll = 0
	case m.scroll > 0:
		m.scroll += len(m.pane) - before
	}
	m.scrollBy(0)
}

func (m model) paneFor() (head, body []string) {
	r, ok := m.row()
	if !ok {
		return nil, []string{m.look.dim.Render(i18n.T("watch.empty"))}
	}
	k, w := m.look, m.paneWidth()
	if r.tab < 0 {
		g := m.groups[r.group]
		return wrap(k.bold.Render(g.name)+" "+say(m.tabs[g.tabs[0]]), w), m.runsPane(g, w)
	}
	t := m.tabs[r.tab]
	run := t.info.State.Run
	switch {
	case r.kind == rowContext || r.kind == rowRun:
		name := t.info.State.Change
		if r.kind == rowContext {
			name = m.groups[r.group].name
		}
		head = wrap(k.bold.Render(name)+" "+say(t), w)
		head = append(head, wrap(k.strip(t), w)...)
		if u := tokens(t.agents, everyAgent(t)); u != "" {
			head = append(head, k.dim.Render(u))
		}
		return head, m.runPane(t, w)
	case r.agent >= 0:
		a := t.agents[r.agent]
		head = wrap(k.bold.Render(phaseLabel(a.Node))+" "+k.dim.Render(agentFacts(a)), w)
		head = append(head, wrap(k.strip(t), w)...)
		if u := tokens(t.agents, []int{r.agent}); u != "" {
			head = append(head, k.dim.Render(u))
		}
		return head, transcript(k, loop.Transcript(m.o.Root, run, a.ID), run, w)
	}
	p := t.phases[r.phase]
	head = wrap(k.bold.Render(phaseLabel(p.node))+" "+k.dim.Render(i18n.T("watch.phase_facts", len(p.agents), p.rounds, money(p.cost))), w)
	head = append(head, wrap(k.strip(t), w)...)
	if u := tokens(t.agents, p.agents); u != "" {
		head = append(head, k.dim.Render(u))
	}
	return head, m.phasePane(t, p, w)
}

// agentFacts is who an agent is and what its turn took.
func agentFacts(a loop.AgentState) string {
	out := fmt.Sprintf("%s · %s · %s", a.ID, a.Role, a.Tier)
	if a.Status != loop.AgentWorking {
		out += fmt.Sprintf(" · %s · %s", time.Duration(a.Seconds*float64(time.Second)).Round(time.Second), money(a.CostUSD))
	}
	return out
}

// runsPane is a context of several runs: each one, how it stands.
func (m model) runsPane(g group, w int) []string {
	var out []string
	for _, ti := range g.tabs {
		t, k := m.tabs[ti], m.look
		out = append(out, wrap(k.mark(runState(t.info))+" "+k.bold.Render(t.info.State.Change)+" "+say(t), w)...)
		out = append(out, wrap("  "+k.strip(t), w)...)
		out = append(out, k.dim.Render("  "+t.info.State.Run), "")
	}
	return out
}

// phasePane is a phase of several agents: each turn, and why it failed.
func (m model) phasePane(t tab, p phase, w int) []string {
	k := m.look
	var out []string
	for _, ai := range p.agents {
		a := t.agents[ai]
		out = append(out, fmt.Sprintf("%s #%d %-24s %-12s", k.mark(stepState(a.Status)), a.Iter, a.Role, a.Status)+k.dim.Render(costOf(a)))
		if a.Error != "" {
			out = append(out, wrap("    "+k.bad.Render(a.Error), w)...)
		}
	}
	return out
}

// costOf is what a turn cost; a turn at work has not said yet.
func costOf(a loop.AgentState) string {
	if a.Status == loop.AgentWorking {
		return "…"
	}
	return money(a.CostUSD)
}

// runPane is a run's facts and its last events, ending on its open gate: the
// end of the pane is what stays in view.
func (m model) runPane(t tab, w int) []string {
	s, k := t.info.State, m.look
	out := []string{k.dim.Render(fmt.Sprintf("%s · %s · %s · iter %d", s.Run, s.Change, s.Flow, s.Iter))}
	if s.Outcome != "" {
		out = append(out, k.dim.Render(i18n.T("watch.outcome"))+" "+s.Outcome)
	}
	if s.Branch != "" {
		out = append(out, k.dim.Render(i18n.T("watch.branch"))+" "+s.Branch+" → "+s.Base+k.dim.Render("  "+s.Landing))
	}
	if s.Verdict != "" {
		out = append(out, k.dim.Render(i18n.T("watch.verdict"))+" "+s.Verdict+k.dim.Render(fmt.Sprintf("  (%d)", len(s.Open))))
	}
	out = append(out, "")
	evs, _ := loop.Events(m.o.Root, s.Run)
	for _, e := range evs {
		line := k.dim.Render(e.T.Local().Format("15:04:05")) + "  " + e.Ev + " " + e.Node
		if e.To != "" {
			line += " → " + e.To
		}
		if e.Why != "" {
			line += k.dim.Render("  (" + strings.ReplaceAll(e.Why, "\n", " · ") + ")")
		}
		out = append(out, line)
	}
	if t.gate != nil {
		out = append(out, "", k.warn.Render(i18n.T("watch.gate", t.gate.Agent)))
		for _, q := range t.gate.Questions {
			line := "  " + q.ID + "  " + q.Text
			if len(q.Options) > 0 {
				line += k.dim.Render("  [" + strings.Join(options(q), " | ") + "]")
			}
			out = append(out, wrap(line, w)...)
		}
	}
	return out
}

// transcript renders an agent's turn as it is recorded: the prompt folded to
// one line, the text wrapped to the pane, each tool call on its line, how it
// ended. Paths lose the run's worktree, so what shows is the file.
func transcript(k look, path, run string, w int) []string {
	worktree := regexp.MustCompile(`"[^"]*?/` + regexp.QuoteMeta(run) + `/`)
	f, err := os.Open(path)
	if err != nil {
		return []string{k.dim.Render(i18n.T("watch.no_transcript"))}
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		var l struct{ Kind, Text string }
		if json.Unmarshal(sc.Bytes(), &l) != nil {
			continue
		}
		switch l.Kind {
		case "prompt":
			out = append(out, k.dim.Render(i18n.T("watch.prompt", strings.Count(l.Text, "\n")+1)))
		case "text":
			for _, line := range strings.Split(strings.TrimRight(l.Text, "\n"), "\n") {
				out = append(out, wrap(line, w)...)
			}
		case "tool":
			out = append(out, k.accent.Render("· ")+worktree.ReplaceAllString(strings.ReplaceAll(l.Text, "\n", " "), `"`))
		case "done":
			out = append(out, k.good.Render("✓ "+l.Text))
		case "failure":
			out = append(out, wrap(k.bad.Render("✗ "+l.Text), w)...)
		}
	}
	return out
}

// sideBySide is whether the tree and the pane fit next to each other.
func (m model) sideBySide() bool { return m.w >= 90 }

// treeWidth is the tree's share of the screen, enough for a context's
// sentence where there is room for it.
func (m model) treeWidth() int {
	if !m.sideBySide() {
		return m.w
	}
	return min(max(m.w*9/20, 44), 76)
}

func (m model) paneWidth() int {
	if !m.sideBySide() {
		return m.w
	}
	return m.w - m.treeWidth() - 3
}

// area is the lines between the title and the footer.
func (m model) area() int { return max(m.h-3, 4) }

// treeHeight is the tree's lines; stacked, up to half of the area.
func (m model) treeHeight() int {
	if m.sideBySide() {
		return m.area()
	}
	return min(max(len(m.rows), 1), max(m.area()/2, 3))
}

func (m model) paneHeight() int {
	if m.sideBySide() {
		return m.area()
	}
	return max(m.area()-m.treeHeight()-1, 3)
}

func (m model) bodyHeight() int { return max(m.paneHeight()-len(m.head)-1, 3) }

func (m model) View() string {
	k := m.look
	var b strings.Builder
	head := k.bold.Render("gofi watch") + k.dim.Render(" · "+m.o.Project+" · "+m.summary())
	b.WriteString(ansi.Truncate(head, m.w, "…") + "\n")
	rule := k.border.Render(strings.Repeat("─", max(m.w, 1)))
	b.WriteString(rule + "\n")

	tree, pane := m.treeLines(), m.paneLines()
	if m.sideBySide() {
		w := m.treeWidth()
		bar := k.border.Render("│")
		for i := range m.area() {
			left, right := "", ""
			if i < len(tree) {
				left = ansi.Truncate(tree[i], w, "…")
			}
			if i < len(pane) {
				right = pane[i]
			}
			line := left + strings.Repeat(" ", max(w-ansi.StringWidth(left), 0)) + " " + bar + " " + right
			b.WriteString(ansi.Truncate(line, m.w, "…") + "\n")
		}
	} else {
		for _, l := range tree {
			b.WriteString(ansi.Truncate(l, m.w, "…") + "\n")
		}
		b.WriteString(rule + "\n")
		for i := range m.paneHeight() {
			if i < len(pane) {
				b.WriteString(ansi.Truncate(pane[i], m.w, "…"))
			}
			b.WriteString("\n")
		}
	}
	foot := k.dim.Render(i18n.T("watch.keys"))
	if m.scroll > 0 {
		foot = k.accent.Render(i18n.T("watch.below", m.scroll)) + k.dim.Render("  · ") + foot
	}
	if m.follow {
		foot = k.accent.Render(i18n.T("watch.following")) + k.dim.Render("  · ") + foot
	}
	if m.note != "" {
		foot = k.warn.Render(m.note) + k.dim.Render("  · ") + foot
	}
	b.WriteString(ansi.Truncate(foot, m.w, "…"))
	return b.String()
}

// summary is the whole project at a glance.
func (m model) summary() string {
	n := map[string]int{}
	total := 0.0
	for _, t := range m.tabs {
		n[t.info.Liveness]++
		total += t.info.State.CostUSD
	}
	parts := []string{i18n.T("watch.count", len(m.tabs))}
	for _, c := range []struct {
		state, key string
	}{{loop.Blocked, "watch.n_blocked"}, {loop.Running, "watch.n_running"}, {loop.Ended, "watch.n_ended"}} {
		if n[c.state] > 0 {
			parts = append(parts, i18n.T(c.key, n[c.state]))
		}
	}
	if stopped := n[loop.Interrupted] + n[loop.Paused]; stopped > 0 {
		parts = append(parts, i18n.T("watch.n_stopped", stopped))
	}
	return strings.Join(append(parts, money(total)), " · ")
}

// treeLines is the part of the tree that fits, the focus in it.
func (m model) treeLines() []string {
	room := m.treeHeight()
	first := max(0, min(m.cursor-room/2, len(m.rows)-room))
	var out []string
	for i := first; i < len(m.rows) && i < first+room; i++ {
		out = append(out, m.treeLine(i))
	}
	return out
}

// paneLines is what the pane shows: the question or the choice being made,
// or the head of the row in focus over the part of its body being read.
func (m model) paneLines() []string {
	switch m.mode {
	case answering, starting:
		lines := m.pickView
		if m.mode == answering {
			lines = m.askView
		}
		var out []string
		for _, l := range lines() {
			out = append(out, wrap(l, m.paneWidth())...)
		}
		return out
	}
	body, h := m.pane, m.bodyHeight()
	end := max(len(body)-m.scroll, 0)
	body = body[max(end-h, 0):end]
	out := append([]string{}, m.head...)
	if len(m.head) > 0 {
		out = append(out, m.look.border.Render(strings.Repeat("─", max(m.paneWidth(), 1))))
	}
	return append(out, body...)
}

func (m model) treeLine(i int) string {
	k, r := m.look, m.rows[i]
	w := m.treeWidth()
	lead := "  "
	if i == m.cursor {
		lead = k.accent.Render("▌ ")
	}
	lead += strings.Repeat("  ", r.depth)
	switch {
	case r.fold && r.open:
		lead += k.dim.Render("▾ ")
	case r.fold:
		lead += k.dim.Render("▸ ")
	default:
		lead += "  "
	}
	switch r.kind {
	case rowContext:
		g := m.groups[r.group]
		t := m.tabs[g.tabs[0]]
		name := g.name
		if len(g.tabs) > 1 {
			name += fmt.Sprintf(" (%d)", len(g.tabs))
		}
		return lead + k.mark(runState(t.info)) + " " + k.bold.Render(name) + " " + say(t)
	case rowRun:
		t := m.tabs[r.tab]
		return lead + k.mark(runState(t.info)) + " " + t.info.State.Change + " " + k.dim.Render(say(t))
	case rowAgent:
		a := m.tabs[r.tab].agents[r.agent]
		left := fmt.Sprintf("%s #%d %s", k.mark(stepState(a.Status)), a.Iter, a.Role)
		if a.Status != loop.AgentDone {
			left += " " + k.dim.Render(a.Status)
		}
		return spread(lead+left, k.dim.Render(costOf(a)), w)
	}
	t := m.tabs[r.tab]
	p := t.phases[r.phase]
	left := k.mark(stepState(p.status)) + " " + phaseLabel(p.node)
	if p.rounds > 1 {
		left += k.dim.Render(fmt.Sprintf(" ×%d", p.rounds))
	}
	if p.status != loop.AgentDone {
		left += " " + k.dim.Render(p.status)
	}
	cost := money(p.cost)
	if p.status == loop.AgentWorking && p.cost == 0 {
		cost = "…"
	}
	return spread(lead+left, k.dim.Render(cost), w)
}

func (m model) askView() []string {
	k, a := m.look, m.ask
	q := a.qs[a.i]
	out := []string{
		k.warn.Render(i18n.T("watch.answering", a.run, a.i+1, len(a.qs))),
		"",
		k.bold.Render(q.Text),
	}
	if q.Why != "" {
		out = append(out, k.dim.Render(q.Why))
	}
	out = append(out, "")
	if opts := options(q); len(opts) > 0 {
		for i, o := range opts {
			line := "  " + o
			if i == a.opt {
				line = k.accent.Render("▸ " + o)
			}
			if o == q.Recommended {
				line += k.dim.Render("  (" + i18n.T("watch.recommended") + ")")
			}
			out = append(out, line)
		}
	} else {
		out = append(out, k.accent.Render("> ")+a.text+"▏")
	}
	return append(out, "", k.dim.Render(i18n.T("watch.answer_keys")))
}

func (m model) pickView() []string {
	k, p := m.look, m.pick
	out := []string{k.bold.Render(i18n.T("watch.pick")), ""}
	for i, c := range p.changes {
		line := "  " + c
		if i == p.i {
			line = k.accent.Render("▸ " + c)
		}
		out = append(out, line)
	}
	return out
}

// look is the screen's styles; plain text with color off.
type look struct {
	bold, dim, accent, good, bad, warn, border lipgloss.Style
}

func newLook() look {
	s := lipgloss.NewStyle
	if !styles.Enabled() {
		return look{bold: s(), dim: s(), accent: s(), good: s(), bad: s(), warn: s(), border: s()}
	}
	return look{bold: s().Bold(true).Foreground(styles.Text), dim: s().Foreground(styles.Dim),
		accent: s().Foreground(styles.Accent), good: s().Foreground(styles.Good), bad: s().Foreground(styles.Bad),
		warn: s().Foreground(styles.Amber), border: s().Foreground(styles.Border)}
}

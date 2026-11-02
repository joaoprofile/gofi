// Package watch is gofi watch: the runs of a project as a multiplexer shows
// its agents (0002, D22) — the project is the workspace, each run a tab, each
// agent of a run a pane with its state, and the pane in focus shows what that
// agent is doing. The screen only reads: every fact is on disk, written by
// the supervisors, so it shows the same whichever terminal started a run and
// keeps showing it when the screen is closed and opened again.
package watch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

// tab is one run and its panes.
type tab struct {
	info   loop.Info
	agents []loop.AgentState
	gate   *loop.Question
}

// row is a line of the tree: a run (agent < 0) or one of its agents.
type row struct{ tab, agent int }

const (
	browsing = iota
	answering
	starting
)

type model struct {
	o      Options
	tabs   []tab
	rows   []row
	cursor int
	// focus names the row in focus, so it stays in focus when the tree is
	// read again and rows come and go.
	focus string
	pane  []string
	mode  int
	ask   asking
	pick  picking
	note  string
	w, h  int
	look  look
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
	m := model{o: o, w: 100, h: 30, look: newLook()}
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
	case "a":
		t, ok := m.current()
		if !ok || t.gate == nil {
			m.note = i18n.T("watch.no_gate")
			break
		}
		m.mode, m.ask = answering, asking{run: t.info.State.Run, qs: t.gate.Questions, answers: map[string]string{}}
	case "c":
		t, ok := m.current()
		if !ok {
			break
		}
		if err := loop.Cancel(m.o.Root, t.info.State.Run); err != nil {
			m.note = err.Error()
		} else {
			m.note = i18n.T("watch.cancelled", t.info.State.Run)
		}
	case "r":
		t, ok := m.current()
		if !ok || m.o.Start == nil {
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
	m.focus = m.key(m.rows[m.cursor])
}

func (m model) current() (tab, bool) {
	if m.cursor >= len(m.rows) {
		return tab{}, false
	}
	return m.tabs[m.rows[m.cursor].tab], true
}

func (m model) key(r row) string {
	k := m.tabs[r.tab].info.State.Run
	if r.agent >= 0 {
		k += "/" + m.tabs[r.tab].agents[r.agent].ID
	}
	return k
}

// reload reads every run again, keeps the row in focus, and reads its pane.
func (m *model) reload() {
	infos, _ := loop.List(m.o.Root)
	m.tabs, m.rows = nil, nil
	for i, info := range infos {
		t := tab{info: info}
		t.agents, _ = loop.Agents(m.o.Root, info.State.Run)
		if q, ok := loop.OpenGate(m.o.Root, info.State.Run); ok {
			t.gate = q
		}
		m.tabs = append(m.tabs, t)
		m.rows = append(m.rows, row{i, -1})
		for j := range t.agents {
			m.rows = append(m.rows, row{i, j})
		}
	}
	m.cursor = 0
	for i, r := range m.rows {
		if m.key(r) == m.focus {
			m.cursor = i
		}
	}
	if len(m.rows) > 0 {
		m.focus = m.key(m.rows[m.cursor])
	}
	m.pane = m.readPane()
}

func (m model) readPane() []string {
	if len(m.rows) == 0 {
		return []string{m.look.dim.Render(i18n.T("watch.empty"))}
	}
	r := m.rows[m.cursor]
	t := m.tabs[r.tab]
	if r.agent >= 0 {
		a := t.agents[r.agent]
		return transcript(m.look, loop.Transcript(m.o.Root, t.info.State.Run, a.ID))
	}
	return m.runPane(t)
}

// runPane is a run's facts, its open gate, and its last events.
func (m model) runPane(t tab) []string {
	s, k := t.info.State, m.look
	out := []string{
		k.bold.Render(s.Run) + "  " + s.Change + k.dim.Render(fmt.Sprintf("  · %s · %s · iter %d · US$ %.3f", s.Flow, s.Context, s.Iter, s.CostUSD)),
	}
	if s.Outcome != "" {
		out = append(out, k.dim.Render(i18n.T("watch.outcome"))+" "+s.Outcome)
	}
	if s.Verdict != "" {
		out = append(out, k.dim.Render(i18n.T("watch.verdict"))+" "+s.Verdict+k.dim.Render(fmt.Sprintf("  (%d)", len(s.Open))))
	}
	if t.gate != nil {
		out = append(out, "", k.warn.Render(i18n.T("watch.gate", t.gate.Agent)))
		for _, q := range t.gate.Questions {
			line := "  " + q.ID + "  " + q.Text
			if len(q.Options) > 0 {
				line += k.dim.Render("  [" + strings.Join(options(q), " | ") + "]")
			}
			out = append(out, line)
		}
	}
	out = append(out, "")
	evs, _ := loop.Events(m.o.Root, s.Run)
	for _, e := range evs {
		line := k.dim.Render(e.T.Local().Format("15:04:05")) + "  " + e.Ev + " " + e.Node
		if e.To != "" {
			line += " → " + e.To
		}
		if e.Why != "" {
			line += k.dim.Render("  (" + e.Why + ")")
		}
		out = append(out, line)
	}
	return out
}

// transcript renders an agent's turn as it is recorded: the prompt folded to
// one line, the text, each tool call, how it ended.
func transcript(k look, path string) []string {
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
			out = append(out, strings.Split(strings.TrimRight(l.Text, "\n"), "\n")...)
		case "tool":
			out = append(out, k.accent.Render("· ")+strings.ReplaceAll(l.Text, "\n", " "))
		case "done":
			out = append(out, k.good.Render("✓ "+l.Text))
		case "failure":
			out = append(out, k.bad.Render("✗ "+l.Text))
		}
	}
	return out
}

func (m model) View() string {
	k := m.look
	var b strings.Builder
	head := k.bold.Render("gofi watch") + k.dim.Render(" · "+m.o.Project+" · "+i18n.T("watch.count", len(m.tabs)))
	b.WriteString(ansi.Truncate(head, m.w, "…") + "\n")

	// The tree takes up to half the screen, scrolled to keep the focus in it.
	room := max(m.h/2-2, 3)
	first := max(0, min(m.cursor-room/2, len(m.rows)-room))
	for i := first; i < len(m.rows) && i < first+room; i++ {
		b.WriteString(ansi.Truncate(m.treeLine(i), m.w, "…") + "\n")
	}
	b.WriteString(k.border.Render(strings.Repeat("─", max(m.w, 1))) + "\n")

	var pane []string
	switch m.mode {
	case answering:
		pane = m.askView()
	case starting:
		pane = m.pickView()
	default:
		pane = m.pane
	}
	shown := min(len(m.rows), room)
	height := max(m.h-shown-4, 3)
	if len(pane) > height {
		pane = pane[len(pane)-height:] // the end is what is happening now
	}
	for _, l := range pane {
		b.WriteString(ansi.Truncate(l, m.w, "…") + "\n")
	}
	for i := len(pane); i < height; i++ {
		b.WriteString("\n")
	}
	foot := k.dim.Render(i18n.T("watch.keys"))
	if m.note != "" {
		foot = k.warn.Render(m.note) + k.dim.Render("  · ") + foot
	}
	b.WriteString(ansi.Truncate(foot, m.w, "…"))
	return b.String()
}

func (m model) treeLine(i int) string {
	k, r := m.look, m.rows[i]
	t := m.tabs[r.tab]
	mark := "  "
	if i == m.cursor {
		mark = k.accent.Render("▸ ")
	}
	if r.agent < 0 {
		s := t.info.State
		state, where := t.info.Liveness, s.Node
		if state == loop.Ended {
			state, where = s.Outcome, "—"
		}
		return mark + k.dot(runColor(t.info)) + fmt.Sprintf("%-22s %-26s %-13s %s", s.Run, s.Change, state, where) +
			k.dim.Render(fmt.Sprintf("  US$ %.3f", s.CostUSD))
	}
	a := t.agents[r.agent]
	return mark + "    " + k.dot(agentColor(a.Status)) + fmt.Sprintf("%-18s %-12s %-12s", a.ID, a.Role, a.Status) +
		k.dim.Render(fmt.Sprintf("  US$ %.3f", a.CostUSD))
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

// The dot of a run, as the multiplexer colors it: working, blocked on the
// person, stopped, or how it ended.
func runColor(i loop.Info) lipgloss.TerminalColor {
	switch i.Liveness {
	case loop.Running:
		return styles.Accent
	case loop.Blocked:
		return styles.Amber
	case loop.Ended:
		if i.State.Outcome == "accepted" {
			return styles.Good
		}
		return styles.Bad
	}
	return styles.Dim
}

func agentColor(status string) lipgloss.TerminalColor {
	switch status {
	case loop.AgentWorking:
		return styles.Accent
	case loop.AgentBlocked:
		return styles.Amber
	case loop.AgentFailed:
		return styles.Bad
	}
	return styles.Good
}

// look is the screen's styles; plain text with color off.
type look struct {
	color                                      bool
	bold, dim, accent, good, bad, warn, border lipgloss.Style
}

func newLook() look {
	s := lipgloss.NewStyle
	if !styles.Enabled() {
		return look{bold: s(), dim: s(), accent: s(), good: s(), bad: s(), warn: s(), border: s()}
	}
	return look{color: true, bold: s().Bold(true).Foreground(styles.Text), dim: s().Foreground(styles.Dim),
		accent: s().Foreground(styles.Accent), good: s().Foreground(styles.Good), bad: s().Foreground(styles.Bad),
		warn: s().Foreground(styles.Amber), border: s().Foreground(styles.Border)}
}

func (k look) dot(c lipgloss.TerminalColor) string {
	if !k.color {
		return "● "
	}
	return lipgloss.NewStyle().Foreground(c).Render("● ")
}

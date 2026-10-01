// Package chat is gofi's interactive mode: a conversation with the agents in
// the terminal, laid out like the rest of the CLI — the banner on top, every
// finished step printed to the scrollback as a "● … ⎿ …" block, and only the
// answer being written kept live at the bottom.
//
// The conversation runs on an engine.Session; this package only renders it.
// Text in free form goes through the intake first, when there is one: the
// plan is shown, its questions asked, and its phases conducted one turn each
// (0001, D9). /skills go to the engine as typed, so every skill the project
// installed works with no wiring here. A few commands are the chat's own:
// /help, /clear, /exit, /brief, /send, and !command for the shell.
package chat

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/gofi-labs/gofi/cli/internal/approval"
	"github.com/gofi-labs/gofi/cli/internal/engine"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/intake"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"github.com/gofi-labs/gofi/cli/internal/tui/banner"
	"github.com/gofi-labs/gofi/cli/internal/tui/styles"
)

// Options configure a chat.
type Options struct {
	Root      string           // the project root; the engine runs here
	Version   string           // shown in the banner
	Model     string           // configured model, until the engine names its own
	Approvals *approval.Server // where the engine asks before a change; nil when it does not

	// NewSession opens a conversation — a new one, or the saved one with that
	// id. Called at start, and again on /clear and /resume.
	NewSession func(resumeID string) engine.Session
	// Sessions lists the conversations that can be resumed, newest first.
	Sessions func() []engine.Saved
	// Models are offered by /model, in order.
	Models []string
	// Intake plans a request in free text: answers are the person's to the
	// last round's questions, proceed goes on without them. Nil sends every
	// request as typed.
	Intake func(text string, answers map[string]string, proceed bool) (*intake.Result, error)
	// TierModel is the model a tier runs on, for a phase the router raised
	// above its role's contract. Nil keeps the skill's own model.
	TierModel func(host.Tier) string
}

// Run holds the conversation until the user leaves.
func Run(opts Options) error {
	m := newModel(opts, styles.Enabled())
	p := tea.NewProgram(m)
	if opts.Approvals != nil {
		opts.Approvals.OnRequest(func(r *approval.Request) { p.Send(approvalMsg{r}) })
	}
	final, err := p.Run()
	if fm, ok := final.(model); ok && fm.session != nil {
		if fm.plan != nil {
			fm.plan.closePhase()
		}
		_ = fm.session.Close()
	}
	return err
}

// Messages from the engine, the shell and the approval channel back into the
// update loop.
type (
	eventMsg  struct{ event engine.Event }
	turnEnded struct{}
	shellDone struct {
		command string
		output  string
		err     error
	}
	tick        struct{}
	approvalMsg struct{ req *approval.Request }
)

type model struct {
	opts    Options
	look    look
	color   bool
	session engine.Session

	input   textinput.Model
	history []string
	histPos int

	width int

	running bool
	events  <-chan engine.Event
	started time.Time
	frame   int
	live    string // answer text streamed so far, not yet printed
	think   bool   // the model is thinking rather than writing

	tools  map[string]string // tool use id → name, to label its result
	modelN string
	skills []string

	armedExit time.Time // first ctrl+c on an idle prompt
	notice    string    // one-line message in the status slot

	asking []*approval.Request // tool calls waiting for a yes or no, oldest first
	always map[string]bool     // tools approved for the rest of the conversation

	chosenModel string         // picked with /model; outlives /clear and /resume
	listed      []engine.Saved // what the last /resume listed, for /resume <n>

	plan       *plan       // the request being conducted, if any
	wait       string      // what the plan waits for from the person
	planning   bool        // the intake is running
	conducting bool        // the running turn is a phase of the plan
	lastFailed bool        // the last turn ended in error or was cut
	lastDone   engine.Done // how the last turn ended: its time and cost
}

func newModel(opts Options, color bool) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = i18n.T("chat.placeholder")
	ti.Focus()
	k := newLook(color)
	ti.PlaceholderStyle = k.dim
	return model{
		opts:    opts,
		look:    k,
		color:   color,
		session: opts.NewSession(""),
		input:   ti,
		width:   80,
		tools:   map[string]string{},
		always:  map[string]bool{},
		modelN:  opts.Model,
		skills:  projectSkills(opts.Root),
	}
}

func (m model) Init() tea.Cmd {
	b := banner.Banner{
		Title:    "chat",
		Version:  m.opts.Version,
		Subtitle: i18n.T("chat.subtitle"),
		Dir:      m.opts.Root,
	}
	return tea.Batch(textinput.Blink, tea.Println(banner.Render(b, m.color)))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = max(msg.Width-4, 10)
		return m, nil

	case tea.KeyMsg:
		if len(m.asking) > 0 {
			return m.onApprovalKey(msg)
		}
		return m.onKey(msg)

	case approvalMsg:
		if m.always[msg.req.Tool] {
			msg.req.Answer(approval.Decision{Allow: true})
			return m, tea.Println(m.look.nested([]string{i18n.T("chat.approve.auto", msg.req.Tool)}, m.look.dim))
		}
		m.asking = append(m.asking, msg.req)
		return m, nil

	case eventMsg:
		return m.onEvent(msg.event)

	case intakeDone:
		return m.onIntake(msg)

	case turnEnded:
		m.running, m.events, m.think = false, nil, false
		m.asking = nil
		flush := m.flushLive()
		if m.conducting && m.plan != nil {
			next, cmd := m.afterPhase()
			return next, tea.Sequence(flush, cmd)
		}
		return m, flush

	case shellDone:
		m.running = false
		return m, tea.Println(m.shellBlock(msg))

	case tick:
		if !m.running {
			return m, nil
		}
		m.frame++
		return m, tickCmd()
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.running {
			m.interrupt()
			return m, nil
		}
		if m.input.Value() != "" {
			m.input.Reset()
			return m, nil
		}
		if time.Since(m.armedExit) < 2*time.Second {
			return m, tea.Quit
		}
		m.armedExit = time.Now()
		m.notice = i18n.T("chat.exit_hint")
		return m, nil
	case "ctrl+d":
		if m.input.Value() == "" && !m.running {
			return m, tea.Quit
		}
	case "esc":
		if m.running {
			m.interrupt()
		}
		return m, nil
	case "up":
		if m.histPos > 0 {
			m.histPos--
			m.input.SetValue(m.history[m.histPos])
			m.input.CursorEnd()
		}
		return m, nil
	case "down":
		if m.histPos < len(m.history) {
			m.histPos++
			if m.histPos == len(m.history) {
				m.input.SetValue("")
			} else {
				m.input.SetValue(m.history[m.histPos])
			}
			m.input.CursorEnd()
		}
		return m, nil
	case "tab":
		value, options := m.complete(m.input.Value())
		m.input.SetValue(value)
		m.input.CursorEnd()
		m.notice = ""
		if len(options) > 0 {
			if len(options) > 8 {
				options = append(options[:8], "…")
			}
			m.notice = strings.Join(options, "  ")
		}
		return m, nil
	case "enter":
		if m.running {
			return m, nil
		}
		text := strings.TrimSpace(m.input.Value())
		if text == "" && (m.wait == waitConfirm || m.wait == waitContinue) {
			// Enter on an empty prompt takes the plan's default: go on.
			return m.onPlanReply("", nil)
		}
		return m.submit(text)
	}
	m.notice = ""
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// interrupt stops the turn, and denies whatever it was waiting on so no hook
// is left blocking the engine.
func (m *model) interrupt() {
	if m.opts.Approvals != nil {
		m.opts.Approvals.DenyAll(i18n.T("chat.approve.interrupted"))
	}
	m.asking = nil
	m.engine().Cancel()
}

// onApprovalKey answers the oldest pending call.
func (m model) onApprovalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	req := m.asking[0]
	var d approval.Decision
	var note string
	switch msg.String() {
	case "1", "y", "s", "enter":
		d, note = approval.Decision{Allow: true}, i18n.T("chat.approve.allowed")
	case "2", "a":
		m.always[req.Tool] = true
		d, note = approval.Decision{Allow: true}, i18n.T("chat.approve.allowed_always", req.Tool)
	case "3", "n", "esc":
		d, note = approval.Decision{Reason: i18n.T("chat.approve.reason")}, i18n.T("chat.approve.denied")
	case "ctrl+c":
		m.interrupt()
		return m, nil
	default:
		return m, nil
	}
	req.Answer(d)
	m.asking = m.asking[1:]
	style := m.look.good
	if !d.Allow {
		style = m.look.bad
	}
	return m, tea.Println(m.look.nested([]string{note}, style))
}

// submit routes what was typed: a command of the chat's own, the shell, or
// the engine — which also receives every /skill the chat does not know.
func (m model) submit(text string) (tea.Model, tea.Cmd) {
	if text == "" {
		return m, nil
	}
	m.input.Reset()
	m.notice = ""
	if len(m.history) == 0 || m.history[len(m.history)-1] != text {
		m.history = append(m.history, text)
	}
	m.histPos = len(m.history)
	echo := tea.Println("\n" + m.look.userLine(text, m.width))

	switch {
	case text == "/exit" || text == "/quit":
		return m, tea.Quit
	case text == "/help":
		return m, tea.Sequence(echo, tea.Println(m.helpBlock()))
	case text == "/clear":
		m.stopPlan()
		m.replaceSession("")
		return m, tea.Sequence(echo, tea.Println(m.look.nested([]string{i18n.T("chat.cleared")}, m.look.dim)))
	case text == "/model" || strings.HasPrefix(text, "/model "):
		return m, tea.Sequence(echo, tea.Println(m.switchModel(strings.TrimSpace(strings.TrimPrefix(text, "/model")))))
	case text == "/resume" || strings.HasPrefix(text, "/resume "):
		return m, tea.Sequence(echo, tea.Println(m.resume(strings.TrimSpace(strings.TrimPrefix(text, "/resume")))))
	case strings.HasPrefix(text, "!"):
		command := strings.TrimSpace(text[1:])
		if command == "" {
			return m, echo
		}
		m.running, m.started, m.frame = true, time.Now(), 0
		return m, tea.Batch(echo, runShell(m.opts.Root, command), tickCmd())
	case text == "/brief":
		if m.plan == nil || m.plan.res == nil {
			return m, tea.Sequence(echo, tea.Println(m.look.nested([]string{i18n.T("chat.plan.no_brief")}, m.look.dim)))
		}
		return m, tea.Sequence(echo, tea.Println("\n"+m.look.markdown(m.plan.res.Brief, m.width)))
	case strings.HasPrefix(text, "/send "):
		m.stopPlan()
		return m.sendToEngine(strings.TrimSpace(strings.TrimPrefix(text, "/send ")), echo)
	case m.wait != waitNone:
		return m.onPlanReply(text, echo)
	case !strings.HasPrefix(text, "/") && m.opts.Intake != nil:
		return m.startIntake(text, echo)
	}
	return m.sendToEngine(text, echo)
}

func (m model) onEvent(e engine.Event) (tea.Model, tea.Cmd) {
	next := waitEvent(m.events)
	k := m.look
	switch ev := e.(type) {
	case engine.Meta:
		if ev.Model != "" {
			m.modelN = ev.Model
		}
		if len(ev.SlashCommands) > 0 {
			m.skills = ev.SlashCommands
		}
		return m, next

	case engine.Delta:
		if ev.Kind == engine.DeltaThinking {
			m.think = true
			return m, next
		}
		m.think = false
		m.live += ev.Text
		return m, next

	case engine.Message:
		// The finished message is authoritative: its text replaces what was
		// streamed, and its tool calls appear only here.
		var out []string
		for _, b := range ev.Blocks {
			switch b.Kind {
			case engine.BlockText:
				out = append(out, k.markdown(b.Text, m.width))
			case engine.BlockToolUse:
				m.tools[b.ID] = b.Name
				summary := toolSummary(b.Input, m.opts.Root, m.width-len(b.Name)-6)
				call := k.bold.Render(b.Name)
				if summary != "" {
					call += k.dim.Render("(") + summary + k.dim.Render(")")
				}
				out = append(out, k.accent.Render("● ")+call)
			}
		}
		m.live = ""
		if len(out) == 0 {
			return m, next
		}
		return m, tea.Sequence(tea.Println("\n"+strings.Join(out, "\n\n")), next)

	case engine.ToolResult:
		style := k.dim
		if ev.IsError {
			style = k.bad
		}
		lines := preview(ev.Text, 4, func(n int) string { return i18n.T("chat.more_lines", n) })
		if len(lines) == 0 {
			lines = []string{i18n.T("chat.no_output")}
		}
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], m.width-6, "…")
		}
		return m, tea.Sequence(tea.Println(k.nested(lines, style)), next)

	case engine.Done:
		m.lastDone = ev
		var line string
		switch {
		case ev.Cancelled:
			m.lastFailed = true
			line = k.nested([]string{i18n.T("chat.interrupted")}, k.warn)
		case ev.IsError:
			m.lastFailed = true
			line = "\n" + k.bullet(k.bad, ev.Err, k.text, m.width)
		default:
			line = k.dim.Render("  " + turnFooter(ev))
		}
		return m, tea.Sequence(m.flushLive(), tea.Println(line), next)

	case engine.Failure:
		m.lastFailed = true
		out := "\n" + k.bullet(k.bad, ev.Message, k.text, m.width)
		if ev.Hint != "" {
			out += "\n" + k.nested([]string{ev.Hint}, k.dim)
		}
		return m, tea.Sequence(tea.Println(out), next)
	}
	return m, next
}

// flushLive prints streamed text that never got its finished message, so a
// cut-off answer is not lost from the scrollback.
func (m *model) flushLive() tea.Cmd {
	text := strings.TrimSpace(m.live)
	m.live = ""
	if text == "" {
		return nil
	}
	return tea.Println("\n" + m.look.markdown(text, m.width))
}

func turnFooter(d engine.Done) string {
	parts := []string{fmt.Sprintf("%.1fs", d.Duration.Seconds())}
	if d.CostUSD > 0 {
		parts = append(parts, fmt.Sprintf("$%.3f", d.CostUSD))
	}
	return strings.Join(parts, " · ")
}

func (m model) View() string {
	k := m.look
	var b strings.Builder

	if m.running {
		if text := strings.TrimSpace(m.live); text != "" {
			lines := strings.Split(k.markdown(text, m.width), "\n")
			// Keep the live answer to the last lines that fit: the whole of it
			// reaches the scrollback once it is finished.
			if len(lines) > 12 {
				lines = lines[len(lines)-12:]
			}
			b.WriteString("\n" + strings.Join(lines, "\n") + "\n")
		}
		label := i18n.T("chat.working")
		switch {
		case m.planning:
			label = i18n.T("chat.planning")
		case m.think:
			label = i18n.T("chat.thinking")
		}
		secs := int(time.Since(m.started).Seconds())
		spin := string(spinFrames[m.frame%len(spinFrames)])
		status := ansi.Truncate(spin+" "+label+" ("+elapsed(secs)+" · "+i18n.T("chat.esc_hint")+")", max(m.width-1, 10), "…")
		b.WriteString("\n" + k.accent.Render(status) + "\n")
	}

	line := k.border.Render(strings.Repeat("─", max(m.width, 10)))
	if len(m.asking) > 0 {
		b.WriteString("\n" + line + "\n" + m.approvalView(m.asking[0]) + "\n" + line + "\n")
	} else {
		b.WriteString("\n" + line + "\n" + m.input.View() + "\n" + line + "\n")
	}

	status := i18n.T("chat.keys")
	switch {
	case len(m.asking) > 0:
		status = i18n.T("chat.approve.keys")
	case m.wait == waitAnswer || m.wait == waitElicit:
		status = i18n.T("chat.plan.answer_hint")
	case m.wait == waitConfirm || m.wait == waitContinue:
		status = i18n.T("chat.plan.confirm_keys")
	}
	if m.notice != "" {
		status = m.notice
	}
	right := m.modelN
	gap := m.width - 2 - ansi.StringWidth(status) - ansi.StringWidth(right)
	if gap < 2 {
		right, gap = "", 0
	}
	footer := ansi.Truncate("  "+status+strings.Repeat(" ", gap)+right, max(m.width-1, 10), "…")
	b.WriteString(k.dim.Render(footer))
	return b.String()
}

// approvalView asks about one tool call, in place of the prompt.
func (m model) approvalView(r *approval.Request) string {
	k := m.look
	call := k.bold.Render(r.Tool)
	if summary := toolSummary(r.Input, m.opts.Root, m.width-len(r.Tool)-16); summary != "" {
		call += k.dim.Render("(") + summary + k.dim.Render(")")
	}
	question := k.accent.Render("● ") + i18n.T("chat.approve.question", call)
	options := "  " + k.accent.Render("1.") + " " + i18n.T("chat.approve.yes") + "   " +
		k.accent.Render("2.") + " " + i18n.T("chat.approve.always", r.Tool) + "   " +
		k.accent.Render("3.") + " " + i18n.T("chat.approve.no")
	if more := len(m.asking) - 1; more > 0 {
		options += k.dim.Render("  " + i18n.T("chat.approve.more", more))
	}
	width := max(m.width-1, 10)
	return ansi.Truncate(question, width, "…") + "\n" + ansi.Truncate(options, width, "…")
}

var spinFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return tick{} })
}

// waitEvent reads the next event of the running turn.
func waitEvent(ch <-chan engine.Event) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return turnEnded{}
		}
		return eventMsg{e}
	}
}

func runShell(dir, command string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/C", command)
		} else {
			cmd = exec.Command("sh", "-c", command)
		}
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return shellDone{command: command, output: string(out), err: err}
	}
}

func (m model) shellBlock(r shellDone) string {
	k := m.look
	dot := k.good
	if r.err != nil {
		dot = k.bad
	}
	head := dot.Render("● ") + k.bold.Render("Shell") + k.dim.Render("(") + ansi.Truncate(r.command, m.width-12, "…") + k.dim.Render(")")
	lines := preview(r.output, 8, func(n int) string { return i18n.T("chat.more_lines", n) })
	if len(lines) == 0 {
		lines = []string{i18n.T("chat.no_output")}
	}
	if r.err != nil {
		lines = append(lines, r.err.Error())
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], m.width-6, "…")
	}
	return "\n" + head + "\n" + k.nested(lines, k.dim)
}

func (m model) helpBlock() string {
	k := m.look
	rows := [][2]string{
		{"/help", i18n.T("chat.help.help")},
		{"/clear", i18n.T("chat.help.clear")},
		{"/model", i18n.T("chat.help.model")},
		{"/resume", i18n.T("chat.help.resume")},
		{"/brief", i18n.T("chat.help.brief")},
		{"/send", i18n.T("chat.help.send")},
		{"/exit", i18n.T("chat.help.exit")},
		{"!<command>", i18n.T("chat.help.shell")},
		{"esc", i18n.T("chat.help.esc")},
		{"↑ ↓", i18n.T("chat.help.history")},
		{"tab", i18n.T("chat.help.tab")},
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, k.accent.Render(fmt.Sprintf("%-11s", r[0]))+" "+r[1])
	}
	if skills := gofiSkills(m.skills); len(skills) > 0 {
		lines = append(lines, "", i18n.T("chat.help.skills"))
		for _, s := range skills {
			lines = append(lines, k.accent.Render("/"+s))
		}
	}
	return "\n" + k.bullet(k.accent, i18n.T("chat.help.title"), k.bold, m.width) + "\n" + k.nested(lines, k.text)
}

// gofiSkills keeps the project's own skills out of the engine's full list of
// slash commands, which also carries its built-ins.
func gofiSkills(all []string) []string {
	var out []string
	for _, s := range all {
		if strings.HasPrefix(s, "gofi-") {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// projectSkills lists .claude/skills before the engine is up to report them.
func projectSkills(root string) []string {
	entries, err := os.ReadDir(layout.Skills().Abs(root))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

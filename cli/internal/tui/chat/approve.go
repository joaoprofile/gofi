package chat

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/approval"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
)

// The answers of the dialog, in the order they are listed.
const (
	answerYes = iota
	answerAlways
	answerNo
)

// asked is the dialog over the call waiting first in line: the answer in
// focus and, once "no" was chosen, what the person tells the agent to do
// instead.
type asked struct {
	sel    int
	typing bool
	note   prompt
	// since is when the dialog came up, and midText whether the person was
	// writing then: keys already on their way to the prompt must not answer
	// a question they did not see.
	since   time.Time
	midText bool
}

// approvalGuard is how long a dialog that just came up takes no yes: a key
// pressed for the prompt — or for the dialog before it — lands in that time.
var approvalGuard = 600 * time.Millisecond

// open starts the dialog over the call now first in line.
func (m *model) openAsk() {
	m.ask = asked{since: time.Now(), midText: !m.input.Empty()}
}

// scope is what "yes, and do not ask again" covers for a call: never a whole
// tool that runs anything. A shell command covers the commands that begin as
// it does; an edit, the edits of files; the approval of a contract, nothing —
// it is asked every time.
type scope struct {
	key   string // what is remembered; empty when the call cannot be remembered
	label string // how the dialog names it
}

// editTools change files: approving them for the conversation covers them all.
var editTools = map[string]bool{"Edit": true, "MultiEdit": true, "Write": true, "NotebookEdit": true}

// multiCommands are programs whose first argument names what they do: "git
// push" and "git status" are not the same thing to allow.
var multiCommands = map[string]bool{
	"git": true, "go": true, "npm": true, "pnpm": true, "yarn": true, "npx": true, "make": true, "cargo": true,
	"docker": true, "kubectl": true, "gofi": true, "pip": true, "pip3": true, "brew": true, "apt": true, "gh": true,
	"terraform": true, "dotnet": true, "mvn": true, "gradle": true, "bun": true, "deno": true, "uv": true,
}

var (
	// shellJoin is anything that makes a line more than one plain command.
	shellJoin  = regexp.MustCompile("[;&|<>`\n]|\\$\\(")
	subcommand = regexp.MustCompile(`^[a-z][a-z0-9:_-]*$`)
)

// commandPrefix is how a plain shell command begins — its program, and its
// subcommand when the program has them. Empty for a line that joins commands
// or sets variables: its beginning says nothing about what it runs.
func commandPrefix(command string) string {
	command = strings.TrimSpace(command)
	if command == "" || shellJoin.MatchString(command) {
		return ""
	}
	words := strings.Fields(command)
	if strings.Contains(words[0], "=") {
		return ""
	}
	if len(words) > 1 && multiCommands[words[0]] && subcommand.MatchString(words[1]) {
		return words[0] + " " + words[1]
	}
	return words[0]
}

func scopeOf(r *approval.Request) scope {
	switch {
	case r.Tool == approval.ApproveTool:
		return scope{}
	case editTools[r.Tool]:
		return scope{key: "edits", label: i18n.T("chat.approve.scope_edits")}
	case r.Tool == "Bash":
		var in struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal(r.Input, &in)
		if p := commandPrefix(in.Command); p != "" {
			return scope{key: "Bash:" + p, label: i18n.T("chat.approve.scope_command", p)}
		}
		return scope{}
	}
	return scope{key: r.Tool, label: r.Tool}
}

// answers are the dialog's choices for a call: "always" only where there is
// something to remember.
func answers(r *approval.Request) []int {
	if scopeOf(r).key == "" {
		return []int{answerYes, answerNo}
	}
	return []int{answerYes, answerAlways, answerNo}
}

// onApprovalKey answers the oldest pending call: the arrows choose, enter
// takes the choice, a digit or a letter answers at once, esc refuses.
func (m model) onApprovalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	req := m.asking[0]
	if m.ask.typing {
		switch msg.String() {
		case "enter":
			return m.decide(req, answerNo, strings.TrimSpace(m.ask.note.Value()))
		case "esc":
			return m.decide(req, answerNo, "")
		case "ctrl+c":
			m.interrupt()
			return m, nil
		}
		m.ask.note.Key(msg)
		return m, nil
	}
	// A dialog that just came up answers nothing but a no: what is typed in
	// its first moment was meant for something else.
	key := msg.String()
	if time.Since(m.ask.since) < approvalGuard && key != "esc" && key != "ctrl+c" {
		return m, nil
	}
	// With a message half written in the prompt, a letter is more likely
	// the next one of it than an answer: there, only the arrows, the digits
	// and enter answer.
	if m.ask.midText && len(key) == 1 && (key[0] < '0' || key[0] > '9') {
		return m, nil
	}
	choices := answers(req)
	pick := func(answer int) (tea.Model, tea.Cmd) {
		if answer == answerNo {
			// The agent goes on better told what to do than only told no.
			m.ask.sel = len(choices) - 1
			m.ask.typing, m.ask.midText = true, false
			m.ask.note = newPrompt(i18n.T("chat.approve.note_hint"))
			m.ask.note.SetWidth(m.boxWidth() - 8)
			return m, nil
		}
		return m.decide(req, answer, "")
	}
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.ask.sel = (m.ask.sel + len(choices) - 1) % len(choices)
	case "down", "j", "tab":
		m.ask.sel = (m.ask.sel + 1) % len(choices)
	case "enter":
		return pick(choices[min(m.ask.sel, len(choices)-1)])
	case "1", "y", "s":
		return m.decide(req, answerYes, "")
	case "2", "a":
		// An approval of the contract is asked each time, and a line of
		// several commands cannot be remembered: there, 2 answers nothing.
		if len(choices) == 3 {
			return m.decide(req, answerAlways, "")
		}
	case "3":
		return pick(answerNo)
	case "n", "esc":
		return m.decide(req, answerNo, "")
	case "ctrl+c":
		m.interrupt()
	}
	return m, nil
}

// decide answers the call and goes on to the next one waiting.
func (m model) decide(req *approval.Request, answer int, note string) (tea.Model, tea.Cmd) {
	k := m.look
	var d approval.Decision
	var line string
	style := k.good
	switch answer {
	case answerYes:
		d, line = approval.Decision{Allow: true}, i18n.T("chat.approve.allowed")
	case answerAlways:
		sc := scopeOf(req)
		m.always[sc.key] = true
		d, line = approval.Decision{Allow: true}, i18n.T("chat.approve.allowed_always", sc.label)
	default:
		style = k.bad
		d, line = approval.Decision{Reason: i18n.T("chat.approve.reason")}, i18n.T("chat.approve.denied")
		if note != "" {
			d.Reason = i18n.T("chat.approve.reason_with", note)
			line += " — " + note
		}
	}
	req.Answer(d)
	m.asking = m.asking[1:]
	m.openAsk()
	return m, say(k.nested(wrapLines(line, m.width-6), style))
}

// remembered reports a call the person already allowed for the conversation.
func (m model) remembered(r *approval.Request) bool {
	sc := scopeOf(r)
	return sc.key != "" && m.always[sc.key]
}

// boxWidth is how wide the dialog is: the terminal's width, up to the banner's.
func (m model) boxWidth() int { return max(min(m.width-1, 100), 20) }

// approvalView is the dialog, in place of the prompt: what the agent wants to
// do — the command as it will run, the change as a diff —, then the answers.
// limit is how many lines it may take; the preview gives way first.
func (m model) approvalView(r *approval.Request, limit int) []string {
	k := m.look
	inner := m.boxWidth() - 4
	title, body := m.callPreview(r, inner)

	choices := answers(r)
	var foot []string
	question := i18n.T("chat.approve.question")
	if r.Tool == approval.ApproveTool {
		question = i18n.T("chat.approve.contract")
	}
	foot = append(foot, "", k.bold.Render(ansi.Truncate(question, inner, "…")))
	for i, c := range choices {
		var text string
		switch c {
		case answerYes:
			text = i18n.T("chat.approve.yes")
		case answerAlways:
			text = i18n.T("chat.approve.always", scopeOf(r).label)
		default:
			text = i18n.T("chat.approve.no")
		}
		// The digits are the keys: "no" is always 3, as it is typed.
		digit := []string{"1", "2", "3"}[c]
		line := ansi.Truncate(digit+". "+text, inner-2, "…")
		if i == min(m.ask.sel, len(choices)-1) {
			foot = append(foot, k.accent.Render("❯ "+line))
		} else {
			foot = append(foot, "  "+k.text.Render(line))
		}
	}
	if m.ask.typing {
		foot = append(foot, "")
		note := m.ask.note
		foot = append(foot, note.View(k, m.color, 3)...)
	}
	if more := len(m.asking) - 1; more > 0 {
		foot = append(foot, k.dim.Render(i18n.T("chat.approve.more", more)))
	}

	head := []string{k.accent.Bold(true).Render(ansi.Truncate(title, inner, "…"))}
	// The answers always show; the preview takes the lines that are left.
	if room := limit - 2 - len(head) - len(foot) - 1; len(body) > room {
		room = max(room, 1)
		hidden := len(body) - room + 1
		body = append(body[:room-1:room-1], k.dim.Render(i18n.T("chat.more_lines", hidden)))
	}
	lines := append(head, "")
	lines = append(append(lines, body...), foot...)

	frame := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(m.boxWidth() - 2)
	if m.color {
		frame = frame.BorderForeground(styles.Amber)
	}
	return strings.Split(frame.Render(strings.Join(lines, "\n")), "\n")
}

// previewLines is how much of a command or of each side of a change the
// dialog shows before it says how much is left.
const previewLines = 10

// callPreview is a call as the person should read it before answering: a
// title that says what kind of thing it is, and the thing itself.
func (m model) callPreview(r *approval.Request, w int) (title string, body []string) {
	k := m.look
	var in struct {
		Command     string `json:"command"`
		Description string `json:"description"`
		FilePath    string `json:"file_path"`
		Notebook    string `json:"notebook_path"`
		Content     string `json:"content"`
		Old         string `json:"old_string"`
		New         string `json:"new_string"`
		NewSource   string `json:"new_source"`
		Edits       []struct {
			Old string `json:"old_string"`
			New string `json:"new_string"`
		} `json:"edits"`
	}
	_ = json.Unmarshal(r.Input, &in)
	file := relativeTo(m.opts.Root, in.FilePath)
	if file == "" {
		file = relativeTo(m.opts.Root, in.Notebook)
	}
	code := func(text string, limit int) []string {
		var out []string
		for _, l := range preview(text, limit, func(n int) string { return i18n.T("chat.more_lines", n) }) {
			out = append(out, strings.Split(ansi.Hardwrap(strings.ReplaceAll(l, "\t", "  "), max(w-2, 8), true), "\n")...)
		}
		return out
	}
	side := func(sign string, st lipgloss.Style, text string) []string {
		var out []string
		for _, l := range code(text, previewLines) {
			out = append(out, st.Render(sign+" "+l))
		}
		return out
	}
	switch r.Tool {
	case "Bash":
		title = i18n.T("chat.approve.title_bash")
		for _, l := range code(in.Command, previewLines) {
			body = append(body, "  "+k.text.Render(l))
		}
		if d := strings.TrimSpace(in.Description); d != "" {
			body = append(body, "  "+k.dim.Render(ansi.Truncate(d, w-2, "…")))
		}
	case "Write":
		title = i18n.T("chat.approve.title_write", file)
		body = side("+", k.good, in.Content)
	case "Edit":
		title = i18n.T("chat.approve.title_edit", file)
		body = append(side("-", k.bad, in.Old), side("+", k.good, in.New)...)
	case "MultiEdit":
		title = i18n.T("chat.approve.title_edit", file)
		for i, e := range in.Edits {
			if i > 0 {
				body = append(body, k.dim.Render("···"))
			}
			body = append(body, append(side("-", k.bad, e.Old), side("+", k.good, e.New)...)...)
		}
	case "NotebookEdit":
		title = i18n.T("chat.approve.title_edit", file)
		body = side("+", k.good, in.NewSource)
	case approval.ApproveTool:
		title = i18n.T("chat.approve.title_contract")
		body = []string{"  " + k.text.Render(toolSummary(r.Input, m.opts.Root, w-2))}
	default:
		title = r.Tool
		if s := toolSummary(r.Input, m.opts.Root, w-2); s != "" {
			body = []string{"  " + k.text.Render(s)}
		}
	}
	if len(body) == 0 {
		body = []string{"  " + k.dim.Render(i18n.T("chat.no_output"))}
	}
	return title, body
}

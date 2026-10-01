// Package flow runs a question-by-question terminal dialogue laid out like a
// chat transcript. Only the question being asked is live at the bottom of the
// terminal; once answered it is printed to the scrollback as a
//
//	● Question
//	  ⎿  answer
//
// block, so the finished dialogue reads as a log of what was decided. Nothing
// takes over the screen: the terminal keeps its history and can be scrolled
// and copied from as usual.
package flow

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/tui/banner"
)

// ErrCancelled is returned by Run when the user leaves with ctrl+c.
var ErrCancelled = errors.New("cancelled")

// Kind selects how a step is answered.
type Kind int

const (
	Input Kind = iota
	Select
	MultiSelect
	Confirm
)

// Option is one choice of a Select or MultiSelect step.
type Option struct {
	Label string
	Value string
	Hint  string // shown dimmed next to the label
}

// Step is one question. Exactly one of Text, Choice, Choices or Bool must be
// set, matching Kind; the step reads its starting value from it and writes the
// answer back as soon as it is confirmed, so later Skip and *Func callbacks
// already see it.
type Step struct {
	Kind Kind

	Title     string
	TitleFunc func() string // overrides Title when set
	Help      string
	HelpFunc  func() string // overrides Help when set

	Options     []Option // Select and MultiSelect
	Placeholder string   // Input: shown while empty, echoed when left blank

	// Affirmative and Negative label a Confirm step. Default: Yes / No.
	Affirmative string
	Negative    string

	Text    *string
	Choice  *string
	Choices *[]string
	Bool    *bool

	Validate      func(string) error   // Input
	ValidateMulti func([]string) error // MultiSelect

	// Skip hides the step. It is evaluated when the dialogue reaches the step,
	// so it can depend on earlier answers.
	Skip func() bool

	// Echo replaces the answer recorded in the transcript. Multi-line output is
	// indented under the ⎿ marker.
	Echo func() string
}

func (s *Step) title() string {
	if s.TitleFunc != nil {
		return s.TitleFunc()
	}
	return s.Title
}

func (s *Step) help() string {
	if s.HelpFunc != nil {
		return s.HelpFunc()
	}
	return s.Help
}

func (s *Step) skipped() bool { return s.Skip != nil && s.Skip() }

// choices returns the options a cursor moves over; a Confirm step is a
// two-option select.
func (s *Step) choices() []Option {
	if s.Kind != Confirm {
		return s.Options
	}
	yes, no := s.Affirmative, s.Negative
	if yes == "" {
		yes = i18n.T("flow.yes")
	}
	if no == "" {
		no = i18n.T("flow.no")
	}
	return []Option{{Label: yes, Value: "yes"}, {Label: no, Value: "no"}}
}

// Header is the banner printed once, above the first question: the same card
// the splash shows. A zero Header prints nothing, for a question asked in the
// middle of a command's output.
type Header = banner.Banner

// Run asks every visible step in order. It returns ErrCancelled when the user
// quits with ctrl+c; answers given until then are already written to their
// bindings.
func Run(h Header, steps []Step) error {
	m := newModel(h, steps)
	if m.idx < 0 {
		return nil
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	if final.(model).cancelled {
		return ErrCancelled
	}
	return nil
}

// Say prints a transcript bullet outside a dialogue, so the output that
// follows one (progress, results) keeps the same look.
func Say(text string) {
	fmt.Println(newPalette().bullet(text, false))
}

// YesNo asks a single yes/no question in the middle of a command's output.
// def is the answer enter picks; empty labels fall back to Yes / No.
func YesNo(title, help, yes, no string, def bool) (bool, error) {
	v := def
	err := Run(Header{}, []Step{{Kind: Confirm, Title: title, Help: help, Affirmative: yes, Negative: no, Bool: &v}})
	return v, err
}

// Ask reads a single line of text in the middle of a command's output.
func Ask(title, help string, validate func(string) error) (string, error) {
	var v string
	err := Run(Header{}, []Step{{Kind: Input, Title: title, Help: help, Text: &v, Validate: validate}})
	return strings.TrimSpace(v), err
}

// Required is a Validate func that rejects a blank answer.
func Required(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New(i18n.T("flow.required"))
	}
	return nil
}

type model struct {
	header Header
	steps  []Step
	pal    palette

	idx   int   // step being asked; -1 when none is left
	trail []int // steps answered, in order, for going back

	input  textinput.Model
	cursor int
	picked map[string]bool
	err    string

	width     int
	cancelled bool
}

func newModel(h Header, steps []Step) model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()

	m := model{header: h, steps: steps, pal: newPalette(), input: ti, width: 80}
	m.input.PromptStyle = m.pal.text
	m.input.PlaceholderStyle = m.pal.dim
	m.enter(m.next(-1))
	return m
}

// next returns the first visible step after i, or -1.
func (m *model) next(i int) int {
	for j := i + 1; j < len(m.steps); j++ {
		if !m.steps[j].skipped() {
			return j
		}
	}
	return -1
}

// enter makes step i the live question, seeding the widget from its binding.
func (m *model) enter(i int) {
	m.idx = i
	m.err = ""
	m.cursor = 0
	if i < 0 {
		return
	}
	s := &m.steps[i]
	switch s.Kind {
	case Input:
		m.input.Placeholder = s.Placeholder
		m.input.SetValue(deref(s.Text))
		m.input.CursorEnd()
	case Select:
		for j, o := range s.Options {
			if s.Choice != nil && o.Value == *s.Choice {
				m.cursor = j
			}
		}
	case MultiSelect:
		m.picked = map[string]bool{}
		if s.Choices != nil {
			for _, v := range *s.Choices {
				m.picked[v] = true
			}
		}
	case Confirm:
		if s.Bool != nil && !*s.Bool {
			m.cursor = 1
		}
	}
}

func (m model) Init() tea.Cmd {
	if m.header.Empty() {
		return textinput.Blink
	}
	return tea.Batch(textinput.Blink, tea.Println(banner.Render(m.header, m.pal.color)))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.input.Width = max(msg.Width-6, 10)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancelled = true
			m.idx = -1
			return m, tea.Quit
		case "esc":
			return m.back()
		case "enter":
			return m.commit()
		}
		return m.handleKey(msg)
	}

	if m.idx >= 0 && m.steps[m.idx].Kind == Input {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.idx < 0 {
		return m, nil
	}
	s := &m.steps[m.idx]
	if s.Kind == Input {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.err = ""
		return m, cmd
	}

	opts := s.choices()
	key := msg.String()
	switch key {
	case "up", "k", "shift+tab":
		m.cursor = (m.cursor - 1 + len(opts)) % len(opts)
	case "down", "j", "tab":
		m.cursor = (m.cursor + 1) % len(opts)
	case " ", "x":
		if s.Kind == MultiSelect {
			v := opts[m.cursor].Value
			m.picked[v] = !m.picked[v]
			m.err = ""
		}
	case "a":
		if s.Kind == MultiSelect {
			all := true
			for _, o := range opts {
				all = all && m.picked[o.Value]
			}
			for _, o := range opts {
				m.picked[o.Value] = !all
			}
			m.err = ""
		}
	default:
		// A digit picks that option straight away on a single choice, and
		// toggles it on a multiple one.
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			n := int(key[0] - '1')
			if n >= len(opts) {
				return m, nil
			}
			m.cursor = n
			if s.Kind == MultiSelect {
				m.picked[opts[n].Value] = !m.picked[opts[n].Value]
				return m, nil
			}
			return m.commit()
		}
	}
	return m, nil
}

// commit validates the live answer, writes it to its binding, records it in
// the transcript and moves on.
func (m model) commit() (tea.Model, tea.Cmd) {
	if m.idx < 0 {
		return m, nil
	}
	s := &m.steps[m.idx]
	var answer string

	switch s.Kind {
	case Input:
		v := m.input.Value()
		if s.Validate != nil {
			if err := s.Validate(v); err != nil {
				m.err = err.Error()
				return m, nil
			}
		}
		*s.Text = v
		answer = strings.TrimSpace(v)
		if answer == "" {
			answer = s.Placeholder
		}
	case Select:
		o := s.Options[m.cursor]
		*s.Choice = o.Value
		answer = o.Label
	case MultiSelect:
		var vals, labels []string
		for _, o := range s.Options {
			if m.picked[o.Value] {
				vals = append(vals, o.Value)
				labels = append(labels, o.Label)
			}
		}
		if s.ValidateMulti != nil {
			if err := s.ValidateMulti(vals); err != nil {
				m.err = err.Error()
				return m, nil
			}
		}
		*s.Choices = vals
		answer = strings.Join(labels, ", ")
	case Confirm:
		*s.Bool = m.cursor == 0
		answer = s.choices()[m.cursor].Label
	}
	if s.Echo != nil {
		answer = s.Echo()
	}
	if answer == "" {
		answer = "—"
	}

	printed := tea.Println(m.pal.answered(s.title(), answer, m.width))
	m.trail = append(m.trail, m.idx)
	m.enter(m.next(m.idx))
	if m.idx < 0 {
		return m, tea.Sequence(printed, tea.Quit)
	}
	return m, printed
}

// back reopens the previous answered step. The answer already in the
// transcript stays there; the new one is printed below it.
func (m model) back() (tea.Model, tea.Cmd) {
	if len(m.trail) == 0 {
		return m, nil
	}
	prev := m.trail[len(m.trail)-1]
	m.trail = m.trail[:len(m.trail)-1]
	m.enter(prev)
	return m, tea.Println(m.pal.dim.Render("  " + i18n.T("flow.back", m.steps[prev].title())))
}

// position is the live step's place among the visible ones, for the footer.
func (m model) position() (int, int) {
	cur, total := 0, 0
	for i := range m.steps {
		if i != m.idx && m.steps[i].skipped() {
			continue
		}
		total++
		if i <= m.idx {
			cur = total
		}
	}
	return cur, total
}

func (m model) View() string {
	if m.idx < 0 {
		return ""
	}
	s := &m.steps[m.idx]
	p := m.pal
	var b strings.Builder

	b.WriteString("\n" + p.bullet(s.title(), true) + "\n")
	if h := s.help(); h != "" {
		b.WriteString(p.indent(p.dim, h, m.width) + "\n")
	}
	b.WriteString("\n")

	switch s.Kind {
	case Input:
		line := p.border.Render(strings.Repeat("─", max(m.width, 10)))
		b.WriteString(line + "\n" + m.input.View() + "\n" + line + "\n")
	default:
		b.WriteString(m.renderOptions(s))
	}

	if m.err != "" {
		b.WriteString(p.bad.Render("  ✗ "+m.err) + "\n")
	}

	cur, total := m.position()
	keys := i18n.T("flow.keys.input")
	switch s.Kind {
	case Select, Confirm:
		keys = i18n.T("flow.keys.select")
	case MultiSelect:
		keys = i18n.T("flow.keys.multi")
	}
	if len(m.trail) > 0 {
		keys += " · " + i18n.T("flow.keys.back")
	}
	keys += " · " + i18n.T("flow.keys.cancel")
	footer := "  " + keys
	if total > 1 {
		footer = fmt.Sprintf("  %d/%d · %s", cur, total, keys)
	}
	footer = ansi.Truncate(footer, max(m.width-1, 20), "…")
	b.WriteString("\n" + p.dim.Render(footer))
	return b.String()
}

func (m model) renderOptions(s *Step) string {
	p := m.pal
	opts := s.choices()
	width := 0
	for _, o := range opts {
		width = max(width, lipgloss.Width(o.Label))
	}

	var b strings.Builder
	for i, o := range opts {
		focused := i == m.cursor
		pointer := "  "
		if focused {
			pointer = p.accent.Render("❯ ")
		}

		var mark string
		if s.Kind == MultiSelect {
			mark = "◯ "
			if m.picked[o.Value] {
				mark = p.good.Render("◉ ")
			}
		} else {
			mark = fmt.Sprintf("%d. ", i+1)
		}

		label := fmt.Sprintf("%-*s", width, o.Label)
		if focused {
			label = p.accent.Render(label)
		} else {
			label = p.text.Render(label)
		}
		line := "  " + pointer + mark + label
		if o.Hint != "" {
			line += "  " + p.dim.Render(o.Hint)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

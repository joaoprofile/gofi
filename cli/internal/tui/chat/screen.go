package chat

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/tui/watch"
)

// How the conversation reaches the terminal.
//
// The chat is drawn in line with the terminal, not over it: what is said ends
// up in the scrollback, where the terminal's own scrolling, search and
// selection work. What the chat redraws is the screen itself, top to bottom:
// the panel of what is at work pinned to the top, the newest lines of the
// conversation under it with the answer being written after them, and the
// prompt at the bottom. A terminal cannot pin lines over its scrollback, so
// those newest lines are held back while they show and go to the scrollback
// as newer ones push them out, in the order they were said — it never repeats
// a line nor misses one.
//
// On a terminal too short for a panel, a conversation and a prompt, nothing
// is pinned: every line goes straight to the scrollback and only the prompt
// and the answer being written are redrawn, with the panel over them while
// something is at work. Leaving the chat lets go of the lines held back, so
// what stays in the terminal is the conversation.

// sayMsg is a block of text for the conversation, as tea.Println would print
// it; flushed says the terminal has taken the last lines sent to it.
type (
	sayMsg   struct{ text string }
	flushed  struct{}
	boardMsg struct{ board watch.Board }
)

// say adds text to the conversation. It is a command, like tea.Println, so it
// is sequenced with the reading of the next event the same way; what it puts
// on screen goes through the model, which is what lets the board be pinned.
func say(text string) tea.Cmd {
	return func() tea.Msg { return sayMsg{text} }
}

// minPinned is the shortest terminal the panel is pinned on: under it there
// is no room for a panel, a conversation and a prompt.
const minPinned = 18

// emit takes a block of text into the conversation: straight to the queue
// for the scrollback when nothing is pinned, into the window under the panel
// when it is.
func (m *model) emit(text string) {
	lines := strings.Split(text, "\n")
	if !m.pinned {
		m.queue = append(m.queue, lines...)
		return
	}
	// The window is drawn by the chat, which cannot let the terminal wrap: a
	// line wider than it is broken here.
	for _, l := range lines {
		if ansi.StringWidth(l) > m.width {
			m.window = append(m.window, strings.Split(ansi.Hardwrap(l, max(m.width, 10), true), "\n")...)
			continue
		}
		m.window = append(m.window, l)
	}
}

// settle decides whether the panel is pinned and lets go of the lines the
// window has no room for. It runs after every update, so the window never
// holds more than the screen shows.
func (m *model) settle() {
	pin := m.height >= minPinned && !m.leaving
	if pin != m.pinned {
		m.pinned = pin
		if !pin {
			m.queue = append(m.queue, m.window...)
			m.window, m.head = nil, 0
		}
	}
	if !m.pinned {
		return
	}
	room := m.height - m.opts.Inset - len(m.panel(m.panelLimit())) - len(m.liveLines()) - len(m.promptLines())
	over := len(m.window) - max(room, 0)
	if over <= 0 {
		return
	}
	// The banner leaves whole: a card cut in half is a broken card.
	if m.head > 0 {
		over = min(max(over, m.head), len(m.window))
		m.head = 0
	}
	m.queue = append(m.queue, m.window[:over]...)
	m.window = m.window[over:]
}

// pump sends the lines waiting for the scrollback, one batch at a time: the
// next goes only after the terminal took this one, which is what keeps the
// order they were said in. Leaving, the last batch goes and the chat ends
// after it.
func (m model) pump(cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.settle()
	if m.flushing {
		return m, cmd
	}
	if len(m.queue) == 0 {
		if m.leaving {
			return m, tea.Quit
		}
		return m, cmd
	}
	text := strings.Join(m.queue, "\n")
	m.queue = nil
	if m.leaving {
		return m, tea.Sequence(tea.Println(text), tea.Quit)
	}
	m.flushing = true
	return m, tea.Batch(cmd, tea.Sequence(tea.Println(text), func() tea.Msg { return flushed{} }))
}

// watchBoard reads the runs again after a while: every second while
// something runs, so the panel moves with the agents, and seldom otherwise —
// just enough to notice a run that started somewhere else. It reads the disk
// and nothing else.
func watchBoard(root string, after time.Duration) tea.Cmd {
	return tea.Tick(after, func(time.Time) tea.Msg { return boardMsg{watch.Live(root)} })
}

func boardEvery(b watch.Board) time.Duration {
	if b.Active() {
		return time.Second
	}
	return 3 * time.Second
}

// View draws what the chat redraws: pinned, the whole screen — the panel,
// the conversation, the answer being written, and the prompt at the bottom;
// on a short terminal, the prompt and what is live over it.
func (m model) View() string {
	live, prompt := m.liveLines(), m.promptLines()
	if !m.pinned {
		var out []string
		// With no room to pin it, the panel still says what is at work.
		if m.busy() && !m.leaving {
			out = m.panel(5)
		}
		return strings.Join(append(append(out, live...), prompt...), "\n")
	}
	panel := m.panel(m.panelLimit())
	room := max(m.height-m.opts.Inset-len(panel)-len(live)-len(prompt), 0)
	window := m.window
	if len(window) > room {
		window = window[len(window)-room:]
	}
	lines := make([]string, 0, m.height)
	// The rows the terminal draws over stay empty.
	for range m.opts.Inset {
		lines = append(lines, "")
	}
	lines = append(append(append(lines, panel...), window...), live...)
	// The prompt keeps its place at the bottom: the room the conversation
	// does not fill yet is over it.
	for range room - len(window) {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, prompt...), "\n")
}

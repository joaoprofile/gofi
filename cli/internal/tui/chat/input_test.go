package chat

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyOf(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "alt+enter":
		return tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
	case "ctrl+j":
		return tea.KeyMsg{Type: tea.KeyCtrlJ}
	case "ctrl+v":
		return tea.KeyMsg{Type: tea.KeyCtrlV}
	case "ctrl+w":
		return tea.KeyMsg{Type: tea.KeyCtrlW}
	case "ctrl+a":
		return tea.KeyMsg{Type: tea.KeyCtrlA}
	case "ctrl+k":
		return tea.KeyMsg{Type: tea.KeyCtrlK}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestThePromptTakesSeveralLines(t *testing.T) {
	p := newPrompt("ask")
	p.SetWidth(20)
	for _, k := range []string{"first", "alt+enter", "second", "ctrl+j", "third"} {
		if !p.Key(keyOf(k)) {
			t.Fatalf("%q should be the prompt's key", k)
		}
	}
	if p.Value() != "first\nsecond\nthird" || p.Lines() != 3 {
		t.Fatalf("value %q in %d lines", p.Value(), p.Lines())
	}
	if p.Key(keyOf("enter")) || p.Key(keyOf("tab")) || p.Key(keyOf("esc")) {
		t.Error("enter, tab and esc are the chat's keys, not the prompt's")
	}

	// Up and down walk the lines and keep the column; past the ends they are
	// not the prompt's — the chat goes through the history.
	p.Key(keyOf("up"))
	p.Key(keyOf("x"))
	if p.Value() != "first\nseconxd\nthird" {
		t.Errorf("up should keep the column: %q", p.Value())
	}
	p.Key(keyOf("up"))
	if !p.OnFirstRow() || p.Key(keyOf("up")) {
		t.Error("up on the first line leaves the text")
	}
	p.Key(keyOf("down"))
	p.Key(keyOf("down"))
	if !p.OnLastRow() || p.Key(keyOf("down")) {
		t.Error("down on the last line leaves the text")
	}

	// Backspace at the start of a line joins it to the one before.
	p.Key(keyOf("ctrl+a"))
	p.Key(keyOf("backspace"))
	if p.Value() != "first\nseconxdthird" {
		t.Errorf("backspace over a line break: %q", p.Value())
	}
	p.Key(keyOf("ctrl+w"))
	if p.Value() != "first\nthird" {
		t.Errorf("ctrl+w deletes the word before the cursor: %q", p.Value())
	}
	p.Key(keyOf("ctrl+k"))
	if p.Value() != "first\n" {
		t.Errorf("ctrl+k deletes to the end of the line: %q", p.Value())
	}
}

func TestThePromptWrapsToItsWidthAndKeepsWordsWhole(t *testing.T) {
	p := newPrompt("")
	p.SetWidth(12)
	p.Insert("alpha beta gamma delta\nxy")
	var got []string
	for _, r := range p.rows() {
		got = append(got, string(p.text[r.start:r.end]))
	}
	if want := "alpha beta |gamma delta|xy"; strings.Join(got, "|") != want {
		t.Errorf("rows %q, want %q", strings.Join(got, "|"), want)
	}
	// The rows cover the text: nothing is drawn twice, nothing is lost but
	// the line breaks.
	if strings.Join(got, "") != "alpha beta gamma deltaxy" {
		t.Errorf("rows do not cover the text: %q", got)
	}
	for _, l := range p.View(newLook(false), false, 10) {
		if n := len([]rune(l)); n > 12+3 {
			t.Errorf("row wider than the prompt (%d): %q", n, l)
		}
	}
	// More rows than fit: the ones around the cursor show.
	if view := p.View(newLook(false), false, 2); len(view) != 2 || !strings.Contains(view[1], "xy") {
		t.Errorf("a long text shows the rows at the cursor: %q", view)
	}
}

func TestPastedTextKeepsItsLinesAndLosesControlCharacters(t *testing.T) {
	p := newPrompt("")
	p.Key(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune("one\r\ntwo\tx\x1b[31m\rthree")})
	if p.Value() != "one\ntwo    x[31m\nthree" {
		t.Errorf("pasted %q", p.Value())
	}
}

func TestAnEmptyPromptShowsItsPlaceholder(t *testing.T) {
	p := newPrompt("ask the agents")
	view := p.View(newLook(false), false, 5)
	if len(view) != 1 || !strings.HasPrefix(view[0], "> ") || !strings.Contains(view[0], "ask the agents") {
		t.Errorf("view %q", view)
	}
}

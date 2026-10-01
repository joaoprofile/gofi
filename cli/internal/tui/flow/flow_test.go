package flow

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m model, keys ...string) model {
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(model)
	}
	return m
}

func TestInputKeepsSuggestionOnEnter(t *testing.T) {
	name := "shop"
	m := newModel(Header{}, []Step{{Kind: Input, Title: "Name", Text: &name}})
	m = send(m, "enter")
	if name != "shop" || m.idx != -1 {
		t.Fatalf("name = %q, idx = %d", name, m.idx)
	}
}

func TestInputValidationBlocksAdvance(t *testing.T) {
	name := ""
	m := newModel(Header{}, []Step{{Kind: Input, Title: "Name", Text: &name,
		Validate: func(s string) error {
			if s == "" {
				return errors.New("required")
			}
			return nil
		}}})
	m = send(m, "enter")
	if m.idx != 0 || m.err != "required" {
		t.Fatalf("idx = %d, err = %q", m.idx, m.err)
	}
	m = send(m, "a", "b", "enter")
	if name != "ab" || m.idx != -1 {
		t.Fatalf("name = %q, idx = %d", name, m.idx)
	}
}

// A digit answers a single choice directly, and later steps see the answer
// when deciding whether to show themselves.
func TestDigitPicksAndSkipSeesAnswer(t *testing.T) {
	lang, module := "go", ""
	m := newModel(Header{}, []Step{
		{Kind: Select, Title: "Language", Choice: &lang, Options: []Option{{Label: "Go", Value: "go"}, {Label: "Rust", Value: "rust"}}},
		{Kind: Input, Title: "Module", Text: &module, Skip: func() bool { return lang == "rust" }},
	})
	m = send(m, "2")
	if lang != "rust" {
		t.Fatalf("lang = %q", lang)
	}
	if m.idx != -1 {
		t.Fatalf("module step should be skipped for rust, idx = %d", m.idx)
	}
}

func TestMultiSelectToggleAndValidate(t *testing.T) {
	picked := []string{"a"}
	m := newModel(Header{}, []Step{{Kind: MultiSelect, Title: "Pick", Choices: &picked,
		Options: []Option{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}},
		ValidateMulti: func(v []string) error {
			if len(v) == 0 {
				return errors.New("pick one")
			}
			return nil
		}}})
	m = send(m, " ", "enter") // untoggle a → empty
	if m.err != "pick one" {
		t.Fatalf("err = %q", m.err)
	}
	m = send(m, "down", " ", "enter")
	if len(picked) != 1 || picked[0] != "b" || m.idx != -1 {
		t.Fatalf("picked = %v, idx = %d", picked, m.idx)
	}
}

func TestEscGoesBackToPreviousAnswer(t *testing.T) {
	a, b := "x", "y"
	m := newModel(Header{}, []Step{
		{Kind: Input, Title: "A", Text: &a},
		{Kind: Input, Title: "B", Text: &b},
	})
	m = send(m, "enter", "esc")
	if m.idx != 0 || m.input.Value() != "x" {
		t.Fatalf("idx = %d, value = %q", m.idx, m.input.Value())
	}
	m = send(m, "z", "enter", "enter")
	if a != "xz" || m.idx != -1 {
		t.Fatalf("a = %q, idx = %d", a, m.idx)
	}
}

func TestConfirmAndCancel(t *testing.T) {
	ok := true
	m := newModel(Header{}, []Step{{Kind: Confirm, Title: "Apply?", Bool: &ok}})
	m = send(m, "down", "enter")
	if ok {
		t.Fatal("second option should answer no")
	}

	ok = true
	m = newModel(Header{}, []Step{{Kind: Confirm, Title: "Apply?", Bool: &ok}})
	m = send(m, "ctrl+c")
	if !m.cancelled || m.View() != "" {
		t.Fatalf("cancelled = %v", m.cancelled)
	}
}

func TestWrapKeepsTableAligned(t *testing.T) {
	got := wrap("agents  gofi-pd, gofi-spec, gofi-eng, gofi-ui\nplain text that is long enough to wrap", 30)
	want := []string{
		"agents  gofi-pd, gofi-spec,",
		"        gofi-eng, gofi-ui",
		"plain text that is long enough",
		"to wrap",
	}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

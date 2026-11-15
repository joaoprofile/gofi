package banner

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var sample = []Banner{
	{Title: "init", Version: "v0.3.0", Subtitle: "New project · answer, or press enter to keep the suggestion", Dir: "/home/someone/projects/a/very/deep/folder/structure/that/goes/on/and/on"},
	{Subtitle: "Welcome to gofi", Extra: []string{"This is the first run on this machine. Let's configure the CLI — it takes", "a few seconds and is stored in gofi.json next to the executable."}},
	{Version: "dev", Subtitle: "CLI for gofi project lifecycle", Dir: "/tmp"},
	{Title: "chat", Version: "dev", Subtitle: "Talk to the agents: a request, a /skill, or !command for the shell.", Dir: "/home/someone/project",
		Side: []Section{
			{Title: "Keys", Lines: []string{"/\tcommands and skills", "alt+enter\ta new line, and a rather long explanation of it that will not fit", "a line with no key"}},
			{Title: "Recent conversations", Lines: []string{"Refactor the payment module so that it reads the config once · 2h ago"}},
		}},
}

// Every banner in a session is the same card: whatever it says, it is exactly
// min(width, MaxWidth) wide, and nothing in it is wider — a frame the terminal
// has to wrap comes apart. One with a side column takes the width its columns
// can use instead: the whole terminal here, where none is wider than they are.
func TestSameWidthWhateverTheContent(t *testing.T) {
	for _, color := range []bool{true, false} {
		for w := 30; w <= 160; w += 7 {
			for i, b := range sample {
				want := min(w, MaxWidth)
				if len(b.Side) > 0 {
					want = w
				}
				for j, l := range strings.Split(strings.Trim(RenderWidth(b, color, w), "\n"), "\n") {
					if got := lipgloss.Width(l); got != want {
						t.Errorf("color=%v width=%d banner %d line %d: %d wide, want %d", color, w, i, j, got, want)
					}
				}
			}
		}
	}
}

func TestTruncateLeftKeepsTheEnd(t *testing.T) {
	if got := truncateLeft("/a/b/c/project", 10); got != "…c/project" {
		t.Errorf("got %q", got)
	}
	if got := truncateLeft("/short", 10); got != "/short" {
		t.Errorf("got %q", got)
	}
}

func TestEmpty(t *testing.T) {
	if !(Banner{}).Empty() || (Banner{Dir: "/x"}).Empty() {
		t.Error("Empty is wrong")
	}
}

// The side sections stand beside the text in as many columns as the terminal
// has room for — an ordinary 80 columns is room for one —, their keys lined
// up and their titles on the same line; what has no room is left out, the
// last first, and the card never grows taller for them. On a terminal wider
// than its columns can use, the card stops growing.
func TestTheSideColumnsFindTheirPlace(t *testing.T) {
	b := sample[3]
	b.Side = append(b.Side, Section{Title: "No model", Lines: []string{"/find\tasks the index", "/show\tshows a symbol", "/path\tfrom one to another",
		"/impact\twhat depends on it", "/runs\twho is at work", "/cost\twhat it costs"}})
	lines := func(w int) []string { return strings.Split(strings.Trim(RenderWidth(b, false, w), "\n"), "\n") }
	titles := func(w int) (row int, found []string) {
		for i, l := range lines(w) {
			for _, title := range []string{"Keys", "Recent conversations", "No model"} {
				if strings.Contains(l, title) {
					row, found = i, append(found, title)
				}
			}
		}
		return row, found
	}

	if _, found := titles(60); len(found) != 0 || len(lines(60)) > 12 {
		t.Errorf("at 60 columns there is no room for a side column, and the card is no taller:\n%s", strings.Join(lines(60), "\n"))
	}
	if _, found := titles(80); strings.Join(found, ",") != "Keys,Recent conversations" || len(lines(80)) > 14 {
		t.Errorf("at 80 columns one column holds what fits it, in order — got %v in %d lines:\n%s", found, len(lines(80)), strings.Join(lines(80), "\n"))
	}
	wide := lines(200)
	row, found := titles(200)
	if len(found) != 3 || strings.Count(wide[row], "│") != 5 || len(wide) > 12 {
		t.Errorf("at 200 columns each section has its column, the titles on one line — got %v:\n%s", found, strings.Join(wide, "\n"))
	}
	if got := lipgloss.Width(wide[0]); got != 200 {
		t.Errorf("at 200 columns the card takes the terminal's width, got %d", got)
	}
	if !strings.Contains(strings.Join(wide, "\n"), "/          commands and skills") {
		t.Errorf("the keys of a section should line up:\n%s", strings.Join(wide, "\n"))
	}
	if got, same := lipgloss.Width(lines(400)[0]), lipgloss.Width(lines(320)[0]); got != same || got >= 320 {
		t.Errorf("past what its columns can use the card stops growing: %d at 400, %d at 320", got, same)
	}
}

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
// has to wrap comes apart.
func TestSameWidthWhateverTheContent(t *testing.T) {
	for _, color := range []bool{true, false} {
		for w := 30; w <= 160; w += 7 {
			want := min(w, MaxWidth)
			for i, b := range sample {
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

// The side column stands beside the text where the card has room for both —
// an ordinary 80 columns is room —, its keys lined up, and is left out where
// there is none: the card never grows taller for it.
func TestTheSideColumnFindsItsPlace(t *testing.T) {
	b := sample[3]
	wide := strings.Split(strings.Trim(RenderWidth(b, false, 120), "\n"), "\n")
	var beside bool
	for _, l := range wide {
		if strings.Contains(l, "Keys") && strings.Contains(l, "│  ") {
			beside = true
		}
	}
	if !beside || len(wide) > 14 {
		t.Errorf("at 120 columns the side column should stand beside the text (%d lines):\n%s", len(wide), strings.Join(wide, "\n"))
	}
	if at80 := RenderWidth(b, false, 80); !strings.Contains(at80, "Keys") || strings.Count(strings.Trim(at80, "\n"), "\n")+1 > 14 {
		t.Errorf("at 80 columns the side column still fits beside the text:\n%s", at80)
	}
	narrow := RenderWidth(b, false, 60)
	if strings.Contains(narrow, "Keys") || !strings.Contains(narrow, "chat") || strings.Count(strings.Trim(narrow, "\n"), "\n")+1 > 12 {
		t.Errorf("at 60 columns the side column is left out and the card is no taller:\n%s", narrow)
	}
	if !strings.Contains(strings.Join(wide, "\n"), "/          commands and skills") {
		t.Errorf("the keys of a section should line up:\n%s", strings.Join(wide, "\n"))
	}
}

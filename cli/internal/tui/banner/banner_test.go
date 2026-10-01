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

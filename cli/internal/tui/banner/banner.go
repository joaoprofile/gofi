// Package banner draws the gofi header: the mascot and a few lines of context
// in one rounded frame. It is the only place that draws it, so the splash and
// every interactive dialogue open with the same card — same width, same
// layout — whatever they have to say.
package banner

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
)

// Banner is what a header says. Every field is optional. The mascot carries
// the name on its chest plate, so no field repeats it.
type Banner struct {
	Title    string   // what is running, e.g. "init"
	Version  string   // shown dimmed after the title
	Subtitle string   // one sentence on what happens next
	Extra    []string // further text, joined and wrapped as one paragraph
	Dir      string   // the folder it runs in; long paths keep their end
}

// Empty reports whether there is nothing to draw, for a dialogue that asks a
// single question in the middle of a command's output.
func (b Banner) Empty() bool {
	return b.Title == "" && b.Version == "" && b.Subtitle == "" && len(b.Extra) == 0 && b.Dir == ""
}

// Layout. MaxWidth keeps the card from stretching across a wide monitor; below
// it the card takes the terminal's full width, so every banner in a session is
// the same size.
const (
	MaxWidth = 100

	chrome     = 4  // border and padding, both sides
	gap        = 3  // between the mascot and the text
	minTextCol = 24 // narrower than this, the text goes under the mascot
)

// Render draws the banner for the terminal stdout writes to.
func Render(b Banner, color bool) string { return RenderWidth(b, color, terminalWidth()) }

// RenderWidth draws the banner for a terminal width columns wide. The card is
// min(width, MaxWidth) wide and never wider: a frame the terminal has to wrap
// comes apart.
func RenderWidth(b Banner, color bool, width int) string {
	cardW := min(width, MaxWidth)
	innerW := max(cardW-chrome, 10)

	mascot := styles.MascotLines(color)
	mascotW := lipgloss.Width(mascot[0])
	textW := innerW - mascotW - gap
	stacked := textW < minTextCol
	if stacked {
		textW = innerW
	}

	text := lipgloss.JoinVertical(lipgloss.Left, b.lines(textW, color)...)
	var body string
	if stacked {
		body = lipgloss.JoinVertical(lipgloss.Left, strings.Join(mascot, "\n"), "", text)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(mascot, "\n"), strings.Repeat(" ", gap), text)
	}

	frame := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(cardW - 2) // lipgloss counts padding, not the border, in Width
	if color {
		frame = frame.BorderForeground(styles.Accent)
	}
	return "\n" + frame.Render(body) + "\n"
}

// lines lays the text column out at width w: title and version, then the
// subtitle and extra text wrapped, then the folder.
func (b Banner) lines(w int, color bool) []string {
	bold, dim := lipgloss.NewStyle(), lipgloss.NewStyle()
	if color {
		bold = bold.Bold(true).Foreground(styles.Text)
		dim = dim.Foreground(styles.Dim)
	}

	var out []string
	var head []string
	if b.Title != "" {
		head = append(head, bold.Render(b.Title))
	}
	if b.Version != "" {
		head = append(head, dim.Render(b.Version))
	}
	if len(head) > 0 {
		out = append(out, ansi.Truncate(strings.Join(head, " "), w, "…"))
	}
	for _, block := range []string{b.Subtitle, strings.Join(b.Extra, " ")} {
		if block == "" {
			continue
		}
		for _, l := range strings.Split(ansi.Wrap(block, w, "/"), "\n") {
			out = append(out, dim.Render(strings.TrimRight(l, " ")))
		}
	}
	if b.Dir != "" {
		out = append(out, dim.Render(truncateLeft(b.Dir, w)))
	}
	return out
}

// truncateLeft keeps the end of a path, which is the part that tells folders
// apart, and marks what was cut with an ellipsis.
func truncateLeft(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	return "…" + string(r[len(r)-width+1:])
}

// terminalWidth is the width of the terminal stdout writes to, or 80 when it
// is not one.
func terminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}

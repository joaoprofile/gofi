package flow

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
)

type palette struct {
	accent, dim, border, good, bad, text, bold lipgloss.Style
	color                                      bool
}

// newPalette follows the CLI color preference: with color off (NO_COLOR,
// `gofi settings set color never`, a pipe) every style renders plain text.
func newPalette() palette { return paletteFor(styles.Enabled()) }

func paletteFor(color bool) palette {
	if !color {
		plain := lipgloss.NewStyle()
		return palette{plain, plain, plain, plain, plain, plain, plain, false}
	}
	s := lipgloss.NewStyle
	return palette{
		accent: s().Foreground(styles.Accent),
		dim:    s().Foreground(styles.Dim),
		border: s().Foreground(styles.Border),
		good:   s().Foreground(styles.Good),
		bad:    s().Foreground(styles.Bad),
		text:   s().Foreground(styles.Text),
		bold:   s().Bold(true).Foreground(styles.Text),
		color:  true,
	}
}

// bullet renders a "● text" line; live marks the question being asked.
func (p palette) bullet(text string, live bool) string {
	dot := p.good.Render("● ")
	if live {
		dot = p.accent.Render("● ")
	}
	return dot + p.bold.Render(text)
}

// answered is the transcript block of a confirmed step.
func (p palette) answered(title, answer string, width int) string {
	var b strings.Builder
	b.WriteString("\n" + p.bullet(title, false))
	for i, l := range wrap(answer, max(width-5, 20)) {
		prefix := "     "
		if i == 0 {
			prefix = "  ⎿  "
		}
		b.WriteString("\n" + p.dim.Render(prefix) + l)
	}
	return b.String()
}

// indent wraps text to the terminal and indents it under a bullet.
func (p palette) indent(st lipgloss.Style, text string, width int) string {
	lines := wrap(text, max(width-2, 20))
	for i, l := range lines {
		lines[i] = "  " + st.Render(l)
	}
	return strings.Join(lines, "\n")
}

// tableRow matches a "key  value" line: a word, then a gap of two spaces or
// more, then the value.
var tableRow = regexp.MustCompile(`^\S+ {2,}`)

// wrap breaks text to width without padding the lines. A "key  value" line
// wraps with its continuation under the value, so a summary stays aligned
// however narrow the terminal is.
func wrap(text string, width int) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		hang := len(tableRow.FindString(line))
		if hang == 0 || hang > width/2 {
			out = append(out, split(line, width)...)
			continue
		}
		for i, part := range split(line[hang:], width-hang) {
			if i == 0 {
				out = append(out, line[:hang]+part)
			} else {
				out = append(out, strings.Repeat(" ", hang)+part)
			}
		}
	}
	return out
}

func split(s string, width int) []string {
	lines := strings.Split(ansi.Wrap(s, width, "/,"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

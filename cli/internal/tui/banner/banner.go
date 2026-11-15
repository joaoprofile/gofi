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
	// Side is what a dialogue has to say besides who it is — its keys, what
	// was left open —, in columns of their own past a rule, in the order
	// they matter: as many columns as the terminal has room for, and what
	// has no room is left out. It is a convenience, never the only place
	// something is said.
	Side []Section
}

// Section is a titled block of the side columns. A line "key\ttext" is drawn
// as a key and what it does, the keys of a section lined up.
type Section struct {
	Title string
	Lines []string
}

// Empty reports whether there is nothing to draw, for a dialogue that asks a
// single question in the middle of a command's output.
func (b Banner) Empty() bool {
	return b.Title == "" && b.Version == "" && b.Subtitle == "" && len(b.Extra) == 0 && b.Dir == "" && len(b.Side) == 0
}

// Layout. MaxWidth keeps the card from stretching across a wide monitor; below
// it the card takes the terminal's full width, so every banner in a session is
// the same size. A banner with a side column is the one that grows past it: it
// has columns to fill, and takes what they can use.
const (
	MaxWidth = 100

	chrome     = 4  // border and padding, both sides
	gap        = 3  // between the mascot and the text
	minTextCol = 24 // narrower than this, the text goes under the mascot
	maxTextCol = 56 // wider than this, a sentence is a line too long to read
	rule       = 3  // the line between two columns, with its margins
	minSideCol = 30 // narrower than this, a side column is left out
	maxSideCol = 58 // wider than this, a column is mostly empty

	easyTextCol = 32 // the text beside two columns or more
	easySideCol = 44 // a side column wide enough to say a whole line
	sideRows    = 12 // the tallest a side column gets; what does not fit is left out
)

// Render draws the banner for the terminal stdout writes to.
func Render(b Banner, color bool) string { return RenderWidth(b, color, terminalWidth()) }

// RenderWidth draws the banner for a terminal width columns wide. The card is
// never wider than that: a frame the terminal has to wrap comes apart. Without
// a side column it is min(width, MaxWidth) wide; with one, as wide as its
// columns can use — narrower than a very wide terminal, where the caller may
// centre it.
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

	// The side sections stand in as many columns as fit beside the text, and
	// the card takes the width they can use.
	var columns [][]Section
	sideW := 0
	if avail := width - chrome - mascotW - gap; len(b.Side) > 0 && !stacked {
		// One column asks for little; a second and a third only come when
		// each can say a whole line and the text still has its own.
		fit := 0
		for n := min(len(b.Side), 4); n >= 1; n-- {
			need := minTextCol + rule + minSideCol
			if n > 1 {
				need = easyTextCol + n*(rule+easySideCol)
			}
			if avail >= need {
				fit = n
				break
			}
		}
		if columns = pack(b.Side, fit); len(columns) > 0 {
			// Each column gets the width a line of it reads well at; the
			// text takes what is left, and what the text cannot use goes
			// back to the columns.
			k := len(columns)
			room := avail - k*rule
			// A column alone keeps growing until a second one fits, so the
			// card takes the terminal's width all the way there.
			widest := maxSideCol
			if k == 1 && len(b.Side) > 1 {
				widest = max(widest, easyTextCol+2*(rule+easySideCol)-rule-maxTextCol)
			}
			textW = min(max(room-k*easySideCol, minTextCol), maxTextCol)
			sideW = min((room-textW)/k, widest)
			if sideW < widest {
				textW = room - k*sideW // the columns' remainder, a cell or two
			}
			innerW = mascotW + gap + textW + k*(rule+sideW)
			cardW = innerW + chrome
		}
	}

	text := lipgloss.JoinVertical(lipgloss.Left, b.lines(textW, color)...)
	var body string
	if stacked {
		body = lipgloss.JoinVertical(lipgloss.Left, strings.Join(mascot, "\n"), "", text)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Center, strings.Join(mascot, "\n"), strings.Repeat(" ", gap), text)
	}
	if len(columns) > 0 {
		// The text keeps its width whatever it says, so the rules stand in
		// the same place on every card; the columns start on the same line.
		parts := []string{lipgloss.NewStyle().Width(mascotW + gap + textW).Render(body)}
		drawn := make([][]string, len(columns))
		height := lipgloss.Height(parts[0])
		for i, col := range columns {
			drawn[i] = sideLines(col, sideW, color)
			height = max(height, len(drawn[i]))
		}
		bar := lipgloss.NewStyle()
		if color {
			bar = bar.Foreground(styles.Border)
		}
		line := strings.TrimSuffix(strings.Repeat(bar.Render("│")+"\n", height), "\n")
		tallest := 0
		for _, d := range drawn {
			tallest = max(tallest, len(d))
		}
		for _, d := range drawn {
			for len(d) < tallest {
				d = append(d, "")
			}
			parts = append(parts, " ", line, " ", lipgloss.NewStyle().Width(sideW).Render(strings.Join(d, "\n")))
		}
		body = lipgloss.JoinHorizontal(lipgloss.Center, parts...)
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

// pack lays the side sections out in at most k columns, in the order given
// and as even in height as that order allows. Sections that would make a
// column taller than sideRows are left out, the last first — they come in
// the order they matter.
func pack(sections []Section, k int) [][]Section {
	if k <= 0 {
		return nil
	}
	for n := len(sections); n > 0; n-- {
		cols, tallest := split(sections[:n], k)
		if tallest <= sideRows || n == 1 {
			return cols
		}
	}
	return nil
}

// split cuts the sections into at most k runs, the tallest as short as can
// be, and says how tall that one is.
func split(sections []Section, k int) ([][]Section, int) {
	rows := func(run []Section) int {
		n := len(run) - 1 // the blank lines between them
		for _, s := range run {
			n += len(s.Lines) + 1
		}
		return n
	}
	if k == 1 || len(sections) == 1 {
		return [][]Section{sections}, rows(sections)
	}
	var best [][]Section
	tallest := 0
	for cut := 1; cut <= len(sections); cut++ {
		head := sections[:cut]
		cols, t := [][]Section{head}, rows(head)
		if cut < len(sections) {
			rest, restT := split(sections[cut:], k-1)
			cols, t = append(cols, rest...), max(t, restT)
		}
		if best == nil || t < tallest {
			best, tallest = cols, t
		}
	}
	return best, tallest
}

// sideLines lays one side column out at width w: each section its title and
// its lines, a blank line between two.
func sideLines(sections []Section, w int, color bool) []string {
	bold, dim, key := lipgloss.NewStyle(), lipgloss.NewStyle(), lipgloss.NewStyle()
	if color {
		bold = bold.Bold(true).Foreground(styles.Text)
		dim = dim.Foreground(styles.Dim)
		key = key.Foreground(styles.Accent)
	}
	var out []string
	for i, s := range sections {
		if i > 0 {
			out = append(out, "")
		}
		if s.Title != "" {
			out = append(out, bold.Render(ansi.Truncate(s.Title, w, "…")))
		}
		keyW := 0
		for _, l := range s.Lines {
			if k, _, ok := strings.Cut(l, "\t"); ok {
				keyW = max(keyW, ansi.StringWidth(k))
			}
		}
		keyW = min(keyW, w/2)
		for _, l := range s.Lines {
			k, text, ok := strings.Cut(l, "\t")
			if !ok {
				out = append(out, dim.Render(ansi.Truncate(l, w, "…")))
				continue
			}
			k = ansi.Truncate(k, keyW, "…")
			pad := strings.Repeat(" ", keyW-ansi.StringWidth(k)+2)
			out = append(out, key.Render(k)+pad+dim.Render(ansi.Truncate(text, max(w-keyW-2, 1), "…")))
		}
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

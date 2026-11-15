package chat

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// prompt is where the person writes: a text of as many lines as they need,
// wrapped to the terminal and edited in place. Enter is the chat's — it
// sends —, so a line break is typed with alt+enter, ctrl+j or a backslash
// before enter, and a pasted text keeps the breaks it has.
type prompt struct {
	text []rune
	cur  int // the cursor, an index into text
	// width is the columns a row of text has, the mark before it left out.
	width int
	// aim is the column up and down keep to across rows of different
	// lengths; -1 once the cursor moved any other way.
	aim         int
	placeholder string
}

// row is one line on screen: text[start:end]. A hard row ends at a line
// break or at the end of the text; a soft one was wrapped.
type row struct {
	start, end int
	hard       bool
}

func newPrompt(placeholder string) prompt {
	return prompt{width: 76, aim: -1, placeholder: placeholder}
}

func (p *prompt) Value() string { return string(p.text) }

func (p *prompt) SetValue(s string) {
	p.text = []rune(clean(s))
	p.cur, p.aim = len(p.text), -1
}

func (p *prompt) Reset() { p.text, p.cur, p.aim = nil, 0, -1 }

func (p *prompt) Empty() bool { return len(p.text) == 0 }

func (p *prompt) SetWidth(w int) { p.width = max(w, 8) }

// Before is the text up to the cursor: what a completion reads.
func (p *prompt) Before() string { return string(p.text[:p.cur]) }

// Insert puts text at the cursor.
func (p *prompt) Insert(s string) {
	in := []rune(clean(s))
	if len(in) == 0 {
		return
	}
	text := make([]rune, 0, len(p.text)+len(in))
	text = append(append(append(text, p.text[:p.cur]...), in...), p.text[p.cur:]...)
	p.text, p.cur, p.aim = text, p.cur+len(in), -1
}

// Replace swaps text[from:to] for s and leaves the cursor after it.
func (p *prompt) Replace(from, to int, s string) {
	from, to = max(min(from, len(p.text)), 0), max(min(to, len(p.text)), 0)
	if to < from {
		from, to = to, from
	}
	p.text = append(p.text[:from:from], p.text[to:]...)
	p.cur = from
	p.Insert(s)
}

// clean is typed or pasted text as the prompt keeps it: line ends as \n, a
// tab as spaces, and no other control character.
func clean(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// rows lays the text out on screen. Rows cover the text end to end: a soft
// row ends where the next begins, a hard one at its line break.
func (p *prompt) rows() []row {
	var out []row
	start, width, space := 0, 0, -1
	for i, r := range p.text {
		if r == '\n' {
			out = append(out, row{start, i, true})
			start, width, space = i+1, 0, -1
			continue
		}
		rw := runeWidth(r)
		if width+rw > p.width && i > start {
			// A word that does not fit goes to the next row whole, when the
			// row has a space to break at.
			if at := space + 1; space >= start && r != ' ' && at > start {
				out = append(out, row{start, at, false})
				start = at
				width = 0
				for _, c := range p.text[start:i] {
					width += runeWidth(c)
				}
			} else {
				out = append(out, row{start, i, false})
				start, width = i, 0
			}
			space = -1
		}
		if r == ' ' {
			space = i
		}
		width += rw
	}
	return append(out, row{start, len(p.text), true})
}

func runeWidth(r rune) int {
	if r < 0x1100 {
		return 1 // every narrow script, without measuring
	}
	return max(ansi.StringWidth(string(r)), 1)
}

// at is the row the cursor is on and its column there.
func (p *prompt) at(rows []row) (line, col int) {
	for i, r := range rows {
		if p.cur < r.end || (p.cur == r.end && r.hard) {
			line = i
			break
		}
		line = i
	}
	for _, c := range p.text[rows[line].start:p.cur] {
		col += runeWidth(c)
	}
	return line, col
}

// to moves the cursor to a column of a row, or to the row's end when it is
// shorter.
func (p *prompt) to(r row, col int) {
	p.cur = r.start
	for w := 0; p.cur < r.end; p.cur++ {
		w += runeWidth(p.text[p.cur])
		if w > col {
			break
		}
	}
	// A soft row's end is the next row's start: stay on this one.
	if !r.hard && p.cur == r.end && r.end > r.start {
		p.cur--
	}
}

// Lines is how many rows the text takes on screen.
func (p *prompt) Lines() int { return len(p.rows()) }

// OnFirstRow and OnLastRow tell the chat when up and down leave the text for
// the history.
func (p *prompt) OnFirstRow() bool { l, _ := p.at(p.rows()); return l == 0 }

func (p *prompt) OnLastRow() bool {
	rows := p.rows()
	l, _ := p.at(rows)
	return l == len(rows)-1
}

// Key edits the text with one key. It reports whether the key was the
// prompt's: enter, tab, esc and whatever it does not know are the chat's.
func (p *prompt) Key(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		if msg.Alt && !msg.Paste {
			switch string(msg.Runes) {
			case "b":
				p.wordLeft()
			case "f":
				p.wordRight()
			case "d":
				p.deleteTo(p.nextWord())
			default:
				return false
			}
			return true
		}
		p.Insert(string(msg.Runes))
		return true
	}
	key := msg.String()
	if key != "up" && key != "down" {
		p.aim = -1
	}
	switch key {
	case "alt+enter", "ctrl+j", "shift+enter":
		p.Insert("\n")
	case "backspace", "ctrl+h":
		if p.cur > 0 {
			p.deleteTo(p.cur - 1)
		}
	case "delete":
		if p.cur < len(p.text) {
			p.deleteTo(p.cur + 1)
		}
	case "left", "ctrl+b":
		p.cur = max(p.cur-1, 0)
	case "right", "ctrl+f":
		p.cur = min(p.cur+1, len(p.text))
	case "home", "ctrl+a":
		rows := p.rows()
		l, _ := p.at(rows)
		p.cur = rows[l].start
	case "end", "ctrl+e":
		rows := p.rows()
		l, _ := p.at(rows)
		p.to(rows[l], 1<<30)
	case "alt+left", "ctrl+left":
		p.wordLeft()
	case "alt+right", "ctrl+right":
		p.wordRight()
	case "ctrl+w", "alt+backspace":
		p.deleteTo(p.prevWord())
	case "ctrl+u":
		rows := p.rows()
		l, _ := p.at(rows)
		p.deleteTo(rows[l].start)
	case "ctrl+k":
		rows := p.rows()
		l, _ := p.at(rows)
		end := rows[l].end
		if end == p.cur && end < len(p.text) {
			end++ // at the end of a line, it joins the next one
		}
		p.deleteTo(end)
	case "up", "down":
		rows := p.rows()
		l, col := p.at(rows)
		if p.aim < 0 {
			p.aim = col
		}
		next := l - 1
		if key == "down" {
			next = l + 1
		}
		if next < 0 || next >= len(rows) {
			return false
		}
		p.to(rows[next], p.aim)
	default:
		return false
	}
	return true
}

// deleteTo removes the text between the cursor and an index.
func (p *prompt) deleteTo(i int) {
	from, to := min(i, p.cur), max(i, p.cur)
	p.text = append(p.text[:from:from], p.text[to:]...)
	p.cur = from
}

func (p *prompt) prevWord() int {
	i := p.cur
	for i > 0 && unicode.IsSpace(p.text[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(p.text[i-1]) {
		i--
	}
	return i
}

func (p *prompt) nextWord() int {
	i := p.cur
	for i < len(p.text) && unicode.IsSpace(p.text[i]) {
		i++
	}
	for i < len(p.text) && !unicode.IsSpace(p.text[i]) {
		i++
	}
	return i
}

func (p *prompt) wordLeft()  { p.cur = p.prevWord() }
func (p *prompt) wordRight() { p.cur = p.nextWord() }

// View draws the prompt in at most limit rows, the ones around the cursor:
// "> " before the first row of the text and two spaces before the others, so
// a long request reads as one block. The cursor is the character under it
// reversed; with color off, a bar before it.
func (p *prompt) View(k look, color bool, limit int) []string {
	cursor := func(under string) string {
		if !color {
			return "▌" + strings.TrimSuffix(under, " ")
		}
		return lipgloss.NewStyle().Reverse(true).Render(under)
	}
	if len(p.text) == 0 {
		hint := ansi.Truncate(p.placeholder, max(p.width-1, 1), "…")
		return []string{k.accent.Render("> ") + cursor(" ") + k.dim.Render(hint)}
	}
	rows := p.rows()
	line, _ := p.at(rows)
	first := 0
	if limit = max(limit, 1); len(rows) > limit {
		first = min(max(line-limit+1, 0), len(rows)-limit)
		rows = rows[first : first+limit]
	}
	out := make([]string, 0, len(rows))
	for i, r := range rows {
		mark := "  "
		if first+i == 0 {
			mark = k.accent.Render("> ")
		}
		if first+i != line {
			out = append(out, mark+k.text.Render(string(p.text[r.start:r.end])))
			continue
		}
		under, after := " ", ""
		if p.cur < r.end {
			under, after = string(p.text[p.cur]), string(p.text[p.cur+1:r.end])
		}
		out = append(out, mark+k.text.Render(string(p.text[r.start:p.cur]))+cursor(under)+k.text.Render(after))
	}
	return out
}

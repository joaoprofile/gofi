package chat

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/gofi-labs/gofi/cli/internal/tui/styles"
)

// look holds the styles of the transcript. With color off every style renders
// plain text, and the layout alone carries the structure.
type look struct {
	text, bold, dim, accent, good, bad, warn, user lipgloss.Style
	border                                         lipgloss.Style
}

func newLook(color bool) look {
	if !color {
		p := lipgloss.NewStyle()
		return look{p, p, p, p, p, p, p, p, p}
	}
	s := lipgloss.NewStyle
	return look{
		text:   s().Foreground(styles.Text),
		bold:   s().Bold(true).Foreground(styles.Text),
		dim:    s().Foreground(styles.Dim),
		accent: s().Foreground(styles.Accent),
		good:   s().Foreground(styles.Good),
		bad:    s().Foreground(styles.Bad),
		warn:   s().Foreground(styles.Amber),
		user:   s().Background(lipgloss.Color("#353535")).Foreground(styles.Text),
		border: s().Foreground(styles.Border),
	}
}

// wrapLines breaks text to width without padding the lines.
func wrapLines(text string, width int) []string {
	lines := strings.Split(ansi.Wrap(text, max(width, 10), "/"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

// bullet renders "● first line" with the rest of the text indented under it.
func (k look) bullet(dot lipgloss.Style, text string, body lipgloss.Style, width int) string {
	lines := wrapLines(text, width-2)
	var b strings.Builder
	for i, l := range lines {
		if i == 0 {
			b.WriteString(dot.Render("● ") + body.Render(l))
			continue
		}
		b.WriteString("\n  " + body.Render(l))
	}
	return b.String()
}

// nested renders lines under the bullet above them, the first one marked ⎿.
func (k look) nested(lines []string, st lipgloss.Style) string {
	var b strings.Builder
	for i, l := range lines {
		prefix := "     "
		if i == 0 {
			prefix = "  ⎿  "
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(k.dim.Render(prefix) + st.Render(l))
	}
	return b.String()
}

// userLine is the prompt as sent, on a band across the terminal so the turns
// are easy to find when scrolling back.
func (k look) userLine(text string, width int) string {
	var out []string
	for i, l := range wrapLines(text, width-2) {
		prefix := "> "
		if i > 0 {
			prefix = "  "
		}
		out = append(out, k.user.Width(width).Render(prefix+l))
	}
	return strings.Join(out, "\n")
}

// preview keeps the first lines of a tool's output, and says how much it left.
func preview(text string, maxLines int, more func(int) string) []string {
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return lines
	}
	return append(lines[:maxLines], more(len(lines)-maxLines))
}

// toolSummary is the one thing worth reading about a tool call — the file it
// reads, the command it runs, the pattern it searches — shortened to fit.
func toolSummary(input json.RawMessage, root string, width int) string {
	var args map[string]any
	if json.Unmarshal(input, &args) != nil {
		return ""
	}
	pick := ""
	for _, key := range []string{"file_path", "notebook_path", "command", "pattern", "url", "query", "skill", "description", "prompt", "path"} {
		if v, ok := args[key].(string); ok && strings.TrimSpace(v) != "" {
			pick = v
			if strings.HasSuffix(key, "path") {
				pick = relativeTo(root, v)
			}
			break
		}
	}
	if pick == "" {
		for _, v := range args {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				pick = s
				break
			}
		}
	}
	pick = strings.Join(strings.Fields(pick), " ")
	return ansi.Truncate(pick, max(width, 10), "…")
}

func relativeTo(root, path string) string {
	if root == "" || !filepath.IsAbs(path) {
		return path
	}
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// elapsed renders a duration the way a status line wants it: 4s, 1m12s.
func elapsed(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm%02ds", seconds/60, seconds%60)
}

package chat

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// The agents answer in markdown. A full renderer would pull in a markdown
// parser and a syntax highlighter for what the terminal needs from a handful
// of constructs, so this reads line by line and styles just those: headings,
// lists, quotes, fenced code, tables, and bold, code and links inline.
var (
	mdHeading  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	mdBullet   = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdNumbered = regexp.MustCompile(`^(\s*)(\d+[.)])\s+(.*)$`)
	mdQuote    = regexp.MustCompile(`^\s*>\s?(.*)$`)
	mdRule     = regexp.MustCompile(`^\s*([-*_])(\s*[-*_]){2,}\s*$`)
	mdBold     = regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	mdCode     = regexp.MustCompile("`([^`]+)`")
	mdLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
)

// markdown renders an answer under a bullet: "● " before the first line, the
// rest indented to match, everything fitted to width.
func (k look) markdown(text string, width int) string {
	body := k.renderMarkdown(strings.TrimSpace(text), max(width-2, 10))
	lines := strings.Split(body, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = k.text.Render("● ") + lines[i]
		} else if lines[i] != "" {
			lines[i] = "  " + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (k look) renderMarkdown(text string, width int) string {
	var out []string
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			// Code keeps its layout: cut rather than wrapped, which would
			// change what it means.
			out = append(out, k.dim.Render("│ ")+k.accent.Render(ansi.Truncate(line, width-2, "…")))
			continue
		}

		switch {
		case trimmed == "":
			out = append(out, "")
		case mdRule.MatchString(line):
			out = append(out, k.dim.Render(strings.Repeat("─", min(width, 40))))
		case mdHeading.MatchString(line):
			m := mdHeading.FindStringSubmatch(line)
			style := k.bold
			if len(m[1]) <= 2 {
				style = k.accent.Bold(true)
			}
			out = append(out, wrapLines(style.Render(k.inline(m[2])), width)...)
		case strings.HasPrefix(trimmed, "|"):
			// Tables line up by column; wrapping would scramble them.
			out = append(out, ansi.Truncate(k.inline(line), width, "…"))
		case mdBullet.MatchString(line):
			m := mdBullet.FindStringSubmatch(line)
			out = append(out, k.hanging(m[1]+"• ", m[2], width)...)
		case mdNumbered.MatchString(line):
			m := mdNumbered.FindStringSubmatch(line)
			out = append(out, k.hanging(m[1]+m[2]+" ", m[3], width)...)
		case mdQuote.MatchString(line):
			m := mdQuote.FindStringSubmatch(line)
			for _, l := range wrapLines(k.inline(m[1]), width-2) {
				out = append(out, k.dim.Render("│ ")+k.dim.Render(l))
			}
		default:
			out = append(out, wrapLines(k.inline(line), width)...)
		}
	}
	return strings.Join(collapseBlank(out), "\n")
}

// hanging wraps a list item with its continuation under the text, not under
// the marker.
func (k look) hanging(marker, text string, width int) []string {
	indent := strings.Repeat(" ", ansi.StringWidth(marker))
	lines := wrapLines(k.inline(text), width-len(indent))
	for i := range lines {
		if i == 0 {
			lines[i] = k.accent.Render(marker) + lines[i]
		} else {
			lines[i] = indent + lines[i]
		}
	}
	return lines
}

// inline styles bold, code and links inside a line of text.
func (k look) inline(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdCode.ReplaceAllStringFunc(s, func(m string) string {
		return k.accent.Render(strings.Trim(m, "`"))
	})
	s = mdBold.ReplaceAllStringFunc(s, func(m string) string {
		return k.bold.Render(strings.Trim(m, "*_"))
	})
	return s
}

// collapseBlank keeps at most one blank line in a row and none at the ends.
func collapseBlank(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l == "" && (len(out) == 0 || out[len(out)-1] == "") {
			continue
		}
		out = append(out, l)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

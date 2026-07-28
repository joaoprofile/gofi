package help

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const logo = ` ██████╗  ██████╗ ███████╗██╗
██╔════╝ ██╔═══██╗██╔════╝██║
██║  ███╗██║   ██║█████╗  ██║
██║   ██║██║   ██║██╔══╝  ██║
╚██████╔╝╚██████╔╝██║     ██║
 ╚═════╝  ╚═════╝ ╚═╝     ╚═╝`

// The gofi mark, drawn beside the wordmark: cyan bar + sky chevron, six lines
// tall to match the ASCII logo. Same silhouette as assets/gofi-logo.svg and
// the animated mark in the VS Code extension.
var markBar = [...]string{"▐▌", "▐▌", "▐▌", "▐▌", "▐▌", "▐▌"}
var markChev = [...]string{
	"     ",
	" ▄▖  ",
	" ██▖ ",
	" ██▘ ",
	" ▀▘  ",
	"     ",
}

// RenderSplash returns the GOFI mark, wordmark, tagline and version block.
// Empty string when opts.Plain is true.
// Plain ASCII (no color) when opts.NoColor is true.
// Colored gradient when neither flag is set.
func RenderSplash(tagline, version string, opts Options) string {
	if opts.Plain {
		return ""
	}
	logoLines := strings.Split(logo, "\n")

	if opts.NoColor {
		var b strings.Builder
		b.WriteString("\n")
		for i, line := range logoLines {
			b.WriteString(indent + markBar[i] + markChev[i] + " " + line + "\n")
		}
		b.WriteString("\n")
		b.WriteString(indent + tagline)
		if version != "" {
			b.WriteString("  " + version)
		}
		b.WriteString("\n")
		return b.String()
	}

	gradient := []lipgloss.Color{
		lipgloss.Color("39"),
		lipgloss.Color("45"),
		lipgloss.Color("51"),
		lipgloss.Color("87"),
		lipgloss.Color("123"),
		lipgloss.Color("159"),
	}
	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	chevStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)

	var b strings.Builder
	b.WriteString("\n")
	for i, line := range logoLines {
		color := gradient[i%len(gradient)]
		st := lipgloss.NewStyle().Foreground(color).Bold(true)
		b.WriteString(indent + barStyle.Render(markBar[i]) + chevStyle.Render(markChev[i]) + " " + st.Render(line) + "\n")
	}
	b.WriteString("\n")
	taglineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(indent + taglineStyle.Render(tagline))
	if version != "" {
		b.WriteString("  " + versionStyle.Render(version))
	}
	b.WriteString("\n")
	return b.String()
}

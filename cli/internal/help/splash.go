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

// RenderSplash returns the GOFI logo, tagline and version block.
// Empty string when opts.Plain is true.
// Plain ASCII (no color) when opts.NoColor is true.
// Colored gradient when neither flag is set.
func RenderSplash(tagline, version string, opts Options) string {
	if opts.Plain {
		return ""
	}
	if opts.NoColor {
		var b strings.Builder
		b.WriteString("\n")
		for _, line := range strings.Split(logo, "\n") {
			b.WriteString(indent + line + "\n")
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
	var b strings.Builder
	b.WriteString("\n")
	lines := strings.Split(logo, "\n")
	for i, line := range lines {
		color := gradient[i%len(gradient)]
		st := lipgloss.NewStyle().Foreground(color).Bold(true)
		b.WriteString(indent + st.Render(line) + "\n")
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

package help

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

const (
	indent       = "  "
	doubleIndent = "    "
)

type Options struct {
	Plain   bool
	NoColor bool
}

func DetectOptions(cmd *cobra.Command) Options {
	opts := Options{}
	if v, err := cmd.Root().PersistentFlags().GetBool("plain"); err == nil && v {
		opts.Plain = true
	}
	if v, err := cmd.Root().PersistentFlags().GetBool("no-color"); err == nil && v {
		opts.NoColor = true
	}
	if os.Getenv("NO_COLOR") != "" {
		opts.NoColor = true
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		opts.NoColor = true
	}
	return opts
}

type styles struct {
	title   lipgloss.Style
	meta    lipgloss.Style
	heading lipgloss.Style
	command lipgloss.Style
	flag    lipgloss.Style
	example lipgloss.Style
}

func newStyles(opts Options) styles {
	if opts.Plain || opts.NoColor {
		return styles{}
	}
	return styles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12")),
		meta:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		heading: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10")),
		command: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14")),
		flag:    lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
		example: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
	}
}

func RenderRoot(root *cobra.Command, version string, opts Options) string {
	st := newStyles(opts)
	var b strings.Builder

	b.WriteString(RenderSplash(root.Short, version, opts))
	b.WriteString("\n")

	b.WriteString(indent + st.heading.Render("USAGE") + "\n")
	b.WriteString(doubleIndent + root.Use + " <command> [flags]\n\n")

	b.WriteString(indent + st.heading.Render("COMMANDS") + "\n")
	cmds := visibleCommands(root)
	width := maxNameWidth(cmds)
	if w := len("help, h"); w > width {
		width = w
	}
	for _, c := range cmds {
		name := padRight(c.Name(), width+4)
		b.WriteString(doubleIndent + st.command.Render(name) + c.Short + "\n")
	}
	helpName := padRight("help, h", width+4)
	b.WriteString(doubleIndent + st.command.Render(helpName) + "Show help (gofi h <command> for details)\n")
	b.WriteString("\n")

	if root.Example != "" {
		b.WriteString(indent + st.heading.Render("EXAMPLES") + "\n")
		for _, line := range splitNonEmpty(root.Example) {
			b.WriteString(doubleIndent + st.example.Render(strings.TrimSpace(line)) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(indent + st.meta.Render("Run '"+root.Use+" h <command>' for command-specific help.") + "\n")
	return b.String()
}

func RenderCommand(cmd *cobra.Command, opts Options) string {
	st := newStyles(opts)
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(indent + st.title.Render(cmd.CommandPath()+" — "+cmd.Short) + "\n\n")

	if cmd.Long != "" {
		for _, line := range strings.Split(strings.TrimSpace(cmd.Long), "\n") {
			b.WriteString(indent + line + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(indent + st.heading.Render("USAGE") + "\n")
	b.WriteString(doubleIndent + cmd.UseLine() + "\n\n")

	subs := visibleCommands(cmd)
	if len(subs) > 0 {
		b.WriteString(indent + st.heading.Render("COMMANDS") + "\n")
		width := maxNameWidth(subs)
		for _, c := range subs {
			name := padRight(c.Name(), width+4)
			b.WriteString(doubleIndent + st.command.Render(name) + c.Short + "\n")
		}
		b.WriteString("\n")
	}

	flagsBlock := renderFlags(cmd, st)
	if flagsBlock != "" {
		b.WriteString(indent + st.heading.Render("FLAGS") + "\n")
		b.WriteString(flagsBlock)
		b.WriteString("\n")
	}

	if cmd.Example != "" {
		b.WriteString(indent + st.heading.Render("EXAMPLES") + "\n")
		for _, line := range splitNonEmpty(cmd.Example) {
			b.WriteString(doubleIndent + st.example.Render(strings.TrimSpace(line)) + "\n")
		}
		b.WriteString("\n")
	}

	if related, ok := cmd.Annotations["related"]; ok && related != "" {
		b.WriteString(indent + st.heading.Render("RELATED") + "\n")
		for _, line := range splitNonEmpty(related) {
			b.WriteString(doubleIndent + strings.TrimSpace(line) + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderFlags(cmd *cobra.Command, st styles) string {
	var entries []struct{ name, desc string }
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		var name string
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", --" + f.Name
		} else {
			name = "    --" + f.Name
		}
		if t := f.Value.Type(); t != "bool" && t != "" {
			name += " <" + t + ">"
		}
		entries = append(entries, struct{ name, desc string }{name, f.Usage})
	})
	if len(entries) == 0 {
		return ""
	}
	width := 0
	for _, e := range entries {
		if l := len(e.name); l > width {
			width = l
		}
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(doubleIndent + st.flag.Render(padRight(e.name, width+4)) + e.desc + "\n")
	}
	return b.String()
}

func visibleCommands(cmd *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func maxNameWidth(cmds []*cobra.Command) int {
	w := 0
	for _, c := range cmds {
		if l := len(c.Name()); l > w {
			w = l
		}
	}
	return w
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

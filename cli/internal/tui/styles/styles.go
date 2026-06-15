// Package styles centralizes the gofi CLI visual language: a lipgloss palette
// for command output (summaries, next-steps, preflight) and a huh form theme
// for the interactive wizard. Everything degrades to plain text when color is
// disabled (NO_COLOR) or stdout is not a TTY.
package styles

import (
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Brand palette (256-color codes for broad terminal support).
const (
	brand     = lipgloss.Color("99")  // gofi violet
	brandSoft = lipgloss.Color("141") // lighter violet (selection)
	inkMuted  = lipgloss.Color("245") // secondary text
	good      = lipgloss.Color("42")  // success green
	warn      = lipgloss.Color("214") // warning amber
	bad       = lipgloss.Color("203") // error red
)

// Enabled reports whether colored/graphical output should be used. False when
// NO_COLOR is set or stdout is not an interactive terminal.
func Enabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func style() lipgloss.Style { return lipgloss.NewStyle() }

// Header is a bold brand-colored title.
func Header(s string) string {
	if !Enabled() {
		return s
	}
	return style().Bold(true).Foreground(brand).Render(s)
}

// Note renders muted secondary text.
func Note(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(inkMuted).Render(s)
}

// Label renders a key in a summary row.
func Label(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(inkMuted).Render(s)
}

// Value renders a value in a summary row.
func Value(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(brandSoft).Render(s)
}

// Success renders a positive status line.
func Success(s string) string {
	if !Enabled() {
		return s
	}
	return style().Bold(true).Foreground(good).Render(s)
}

// Warn renders a warning status line.
func Warn(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(warn).Render(s)
}

// Error renders a failure status line.
func Error(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(bad).Render(s)
}

// Hint renders an actionable hint.
func Hint(s string) string {
	if !Enabled() {
		return s
	}
	return style().Foreground(brandSoft).Render(s)
}

// Panel wraps content in a rounded brand-colored border. Returns the content
// unchanged when color is disabled (keeps non-TTY output clean and parseable).
func Panel(content string) string {
	if !Enabled() {
		return content
	}
	return style().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(brand).
		Padding(0, 2).
		Render(content)
}

// FormTheme returns the huh theme for the wizard: ThemeCharm with the gofi
// brand applied to titles, notes and selection. Falls back to the uncolored
// base theme when color is disabled.
func FormTheme() *huh.Theme {
	if !Enabled() {
		return huh.ThemeBase()
	}
	t := huh.ThemeCharm()
	t.Focused.Title = t.Focused.Title.Foreground(brand).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(brand).Bold(true)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(brandSoft)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(brandSoft)
	t.Focused.Base = t.Focused.Base.BorderForeground(brand)
	t.Blurred.Title = t.Blurred.Title.Foreground(inkMuted)
	return t
}

// Package styles centralizes the gofi CLI visual language: the palette taken
// from the gofi logo, the pixel mascot, and helpers for command output
// (summaries, next steps, preflight). The interactive dialogues in tui/flow
// draw with the same colors. Everything degrades to plain text when color is
// disabled (NO_COLOR) or stdout is not a TTY.
package styles

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/settings"
)

// Palette. Accent is the cyan of the gofi logo.
var (
	Accent = lipgloss.Color("#22D3EE")
	Dim    = lipgloss.Color("#6B6B6B")
	Border = lipgloss.Color("#4A4A5A")
	Good   = lipgloss.Color("#4CC38A")
	Amber  = lipgloss.Color("#F5A524")
	Bad    = lipgloss.Color("#E5484D")
	Text   = lipgloss.Color("#EDEDED")
)

// Enabled reports whether colored output should be used. NO_COLOR always wins,
// then the persisted preference, with "auto" following the terminal.
func Enabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	switch settings.ColorMode() {
	case settings.ColorNever:
		return false
	case settings.ColorAlways:
		return true
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func render(st lipgloss.Style, s string) string {
	if !Enabled() {
		return s
	}
	return st.Render(s)
}

func style() lipgloss.Style { return lipgloss.NewStyle() }

// Header is a bold title.
func Header(s string) string { return render(style().Bold(true).Foreground(Text), s) }

// Note renders muted secondary text.
func Note(s string) string { return render(style().Foreground(Dim), s) }

// Label renders a key in a summary row.
func Label(s string) string { return render(style().Foreground(Dim), s) }

// Value renders a value in a summary row.
func Value(s string) string { return render(style().Foreground(Accent), s) }

// Success renders a positive status line.
func Success(s string) string { return render(style().Bold(true).Foreground(Good), s) }

// Warn renders a warning status line.
func Warn(s string) string { return render(style().Foreground(Amber), s) }

// Error renders a failure status line.
func Error(s string) string { return render(style().Foreground(Bad), s) }

// Bullet renders a transcript "● text" line, the shape every result of the CLI
// takes. The dot is green for a success, red for a failure and cyan otherwise.
func Bullet(kind BulletKind, s string) string {
	dot := "● "
	if Enabled() {
		c := Accent
		switch kind {
		case Done:
			c = Good
		case Failed:
			c = Bad
		case Warning:
			c = Amber
		}
		dot = style().Foreground(c).Render(dot)
	}
	return dot + Header(s)
}

// Detail renders a line nested under a Bullet.
func Detail(s string) string { return Note("  ⎿  ") + s }

// BulletKind colors a Bullet.
type BulletKind int

const (
	Info BulletKind = iota
	Done
	Failed
	Warning
)

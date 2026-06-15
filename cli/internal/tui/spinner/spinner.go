// Package spinner runs a sequence of named steps, showing a rotating dot
// glyph while each one executes. Falls back to plain "→ name…" lines when
// stdout is not a TTY (CI, pipes).
package spinner

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Step pairs a human-readable description with the work to do.
type Step struct {
	Name string
	Fn   func() error
}

// Result mirrors a Step with its outcome, used by callers that want to
// render a summary table afterwards.
type Result struct {
	Name string
	Err  error
}

// Frames is the rotating sequence used while a step runs (Braille dots).
var Frames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// FrameInterval is how long each frame stays on screen.
const FrameInterval = 80 * time.Millisecond

// Run executes every step sequentially. Errors are not fatal — the next
// step still runs — so the caller can surface multiple failures at once.
// Returns the per-step results in input order.
func Run(steps []Step) []Result {
	results := make([]Result, 0, len(steps))
	tty := term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("NO_COLOR") == ""
	for _, s := range steps {
		err := runStep(os.Stdout, s, tty)
		results = append(results, Result{Name: s.Name, Err: err})
	}
	return results
}

func runStep(out io.Writer, s Step, tty bool) error {
	if !tty {
		fmt.Fprintf(out, "→ %s …\n", s.Name)
		err := s.Fn()
		if err != nil {
			fmt.Fprintf(out, "  ✗ %s\n", err)
		} else {
			fmt.Fprintf(out, "  ok\n")
		}
		return err
	}

	done := make(chan error, 1)
	go func() { done <- s.Fn() }()

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	tick := time.NewTicker(FrameInterval)
	defer tick.Stop()

	i := 0
	for {
		select {
		case err := <-done:
			fmt.Fprint(out, "\r\x1b[2K") // clear current line
			if err != nil {
				fmt.Fprintf(out, "  %s %s — %s\n", failStyle.Render("✗"), s.Name, muted.Render(err.Error()))
			} else {
				fmt.Fprintf(out, "  %s %s\n", okStyle.Render("✓"), s.Name)
			}
			return err
		case <-tick.C:
			frame := string(Frames[i%len(Frames)])
			fmt.Fprintf(out, "\r  %s %s", muted.Render(frame), s.Name)
			i++
		}
	}
}

// AnyFailed reports whether any result carries a non-nil error.
func AnyFailed(results []Result) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

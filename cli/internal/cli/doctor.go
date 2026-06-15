package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the local environment for gofi",
		Long: `Run a series of environment checks and print a status table.

Checks: git on PATH, target language toolchain (go or cargo), claude CLI on PATH,
write access to ~/.cache/gofi/, GitHub API connectivity. Each row reports ok,
warning or error along with a remediation hint.`,
		Example: `gofi doctor
gofi doctor --plain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor()
		},
	}
}

func runDoctor() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		// .gofi.yaml is optional for doctor; just skip toolchain check.
		cfg = nil
	}
	checks := doctor.Run(cfg, doctor.Options{})

	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	render(checks, useColor)

	if anyFailed(checks) {
		return errors.New("one or more checks failed")
	}
	return nil
}

func render(checks []doctor.Check, color bool) {
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	width := 0
	for _, c := range checks {
		if l := len(c.Name); l > width {
			width = l
		}
	}

	header := "Doctor"
	if color {
		header = headerStyle.Render(header)
	}
	fmt.Println()
	fmt.Println("  " + header)

	for _, c := range checks {
		marker := "?"
		var styled string
		switch c.Status {
		case doctor.StatusOK:
			marker = "✓"
			if color {
				styled = okStyle.Render(marker)
			} else {
				styled = marker
			}
		case doctor.StatusWarn:
			marker = "!"
			if color {
				styled = warnStyle.Render(marker)
			} else {
				styled = marker
			}
		case doctor.StatusFail:
			marker = "✗"
			if color {
				styled = failStyle.Render(marker)
			} else {
				styled = marker
			}
		}

		name := padRight(c.Name, width+2)
		detail := c.Detail
		if color {
			detail = mutedStyle.Render(detail)
		}
		fmt.Printf("    %s %s%s\n", styled, name, detail)
		if c.Hint != "" && c.Status != doctor.StatusOK {
			indent := strings.Repeat(" ", 6+width+2)
			line := "→ " + c.Hint
			if color {
				line = mutedStyle.Render(line)
			}
			fmt.Println(indent + line)
		}
	}
	fmt.Println()
}

func anyFailed(checks []doctor.Check) bool {
	for _, c := range checks {
		if c.Status == doctor.StatusFail {
			return true
		}
	}
	return false
}

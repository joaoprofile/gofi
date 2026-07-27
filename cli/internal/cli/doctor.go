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
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: i18n.T("cmd.doctor.short"),
		Long: `Run a series of environment checks and print a status table.

Checks: git on PATH, target language toolchain (go or cargo), claude CLI on PATH,
docker on PATH, write access to ~/.cache/gofi/, GitHub API connectivity, and —
when sources.institutional is set — whether the committed institutional snapshot
is behind the org repo. Each row reports ok, warning or error with a hint.`,
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

	// Institutional freshness — only when the project pins an org repo. The
	// doctor package can't reach sources/installed.yaml (import cycle), so the
	// check is orchestrated here and appended to the report.
	if cfg != nil && cfg.Sources.Institutional != "" {
		checks = append(checks, checkInstitutionalFreshness(cfg))
	}

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

// checkInstitutionalFreshness resolves the configured institutional repo and
// compares its current SHA with the snapshot committed to this project
// (.gofi/installed.yaml). Network resolution failures degrade to a warning —
// doctor should never hard-fail over an institutional gap.
func checkInstitutionalFreshness(cfg *config.GofiConfig) doctor.Check {
	root := projectRootFromCfg(cfg)
	committed := readInstalledInstitutionalSha(root)
	resolved, err := resolveRefSHA(root, cfg.Sources.Institutional)
	return institutionalFreshnessCheck(cfg.Sources.Institutional, committed, resolved, err)
}

// institutionalFreshnessCheck is the pure decision the doctor row is built from,
// factored out so it can be unit-tested without the network.
func institutionalFreshnessCheck(ref, committed, resolved string, resolveErr error) doctor.Check {
	const name = "institutional base"
	switch {
	case resolveErr != nil:
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "could not resolve " + ref,
			Hint:   "check connectivity; then run `gofi institutional update`",
		}
	case resolved == "local":
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "local source (fixture)"}
	case committed == "":
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: "configured but no snapshot recorded",
			Hint:   "run `gofi institutional update` to pull the org base",
		}
	case committed == resolved:
		return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: "up to date (" + short(resolved) + ")"}
	default:
		return doctor.Check{
			Name:   name,
			Status: doctor.StatusWarn,
			Detail: fmt.Sprintf("behind: %s → %s", short(committed), short(resolved)),
			Hint:   "run `gofi institutional update` to sync the org base",
		}
	}
}

func anyFailed(checks []doctor.Check) bool {
	for _, c := range checks {
		if c.Status == doctor.StatusFail {
			return true
		}
	}
	return false
}

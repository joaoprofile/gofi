package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/hsec"
)

func newHsecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hsec",
		Short: "Run Horusec SAST against this project",
		Long: `Run the Horusec static analysis security scanner against the current project.

Configuration lives under the hsec: block in .gofi.yaml. gofi renders that block
into .gofi/horusec-config.json before each run, then invokes the horusec binary
against it.

Without a subcommand, hsec runs the full scan (alias of 'gofi hsec start').`,
		Example: `gofi hsec
gofi hsec start
gofi hsec list
gofi hsec install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHsecStart()
		},
	}
	cmd.AddCommand(newHsecStartCmd(), newHsecInstallCmd(), newHsecListCmd())
	return cmd
}

func newHsecStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Short:   "Run the security scan",
		Long:    `Render .gofi/horusec-config.json from the hsec: block and invoke 'horusec start' against the project.`,
		Example: `gofi hsec start`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHsecStart()
		},
	}
}

func newHsecInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the horusec CLI via the official script",
		Long: `Run the official horusec install script (Linux/macOS only).

The script downloads a release of horusec into a directory on your PATH. Review
https://github.com/ZupIT/horusec before running. On Windows, install via
winget/scoop/brew or download a release binary manually.`,
		Example: `gofi hsec install
gofi hsec install --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			return runHsecInstall(yes)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func newHsecListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List findings from the last scan",
		Long:    `Print findings recorded in .gofi/horusec-output.json by the most recent 'gofi hsec start' run.`,
		Example: `gofi hsec list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHsecList()
		},
	}
}

func runHsecStart() error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	if !cfg.Hsec.Enabled {
		return errors.New("hsec is disabled in .gofi.yaml; set hsec.enabled to true to run")
	}
	if !hsec.IsInstalled() {
		return errors.New("horusec is not installed on PATH; run `gofi hsec install` first")
	}
	configPath, err := hsec.WriteConfig(root, cfg.Hsec)
	if err != nil {
		return fmt.Errorf("write horusec-config.json: %w", err)
	}
	fmt.Printf("Running horusec against %s …\n\n", root)
	if err := hsec.Run(root, configPath, os.Stdout, os.Stderr, os.Stdin); err != nil {
		return err
	}
	fmt.Println("\nScan complete. Run `gofi hsec list` to inspect findings.")
	return nil
}

func runHsecInstall(autoConfirm bool) error {
	if hsec.IsInstalled() {
		fmt.Println("horusec is already installed.")
		return nil
	}
	if !autoConfirm {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("hsec install requires --yes when stdin is not a TTY")
		}
		ok := false
		if err := huh.NewConfirm().
			Title("Install horusec via the official script?").
			Description("Will run: curl -fsSL https://raw.githubusercontent.com/ZupIT/horusec/main/deployments/scripts/install.sh | bash -s latest").
			Affirmative("Install").
			Negative("Cancel").
			Value(&ok).Run(); err != nil {
			return err
		}
		if !ok {
			fmt.Println("install cancelled.")
			return nil
		}
	}
	if err := hsec.InstallScript(os.Stdout, os.Stderr); err != nil {
		return err
	}
	if !hsec.IsInstalled() {
		fmt.Fprintln(os.Stderr, "warning: install completed but `horusec` is still not on PATH; you may need to restart your shell or update PATH.")
	}
	fmt.Println("horusec installed.")
	return nil
}

func runHsecList() error {
	_, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	findings, err := hsec.ParseFindings(root)
	if err != nil {
		return err
	}
	if findings == nil {
		fmt.Println("no scan recorded yet — run `gofi hsec start` first.")
		return nil
	}
	if len(findings) == 0 {
		fmt.Println("no vulnerabilities found.")
		return nil
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
		}
		return findings[i].File < findings[j].File
	})

	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	header := fmt.Sprintf("%d finding(s)", len(findings))
	if useColor {
		header = headerStyle.Render(header)
	}
	fmt.Printf("\n  %s\n", header)
	for _, f := range findings {
		sev := f.Severity
		if useColor {
			sev = severityStyle(f.Severity).Render(sev)
		}
		loc := f.File
		if f.Line != "" {
			loc = f.File + ":" + f.Line
		}
		if useColor {
			loc = mutedStyle.Render(loc)
		}
		fmt.Printf("    %-10s %s\n", sev, loc)
		if f.Details != "" {
			detail := "      " + f.Details
			if useColor {
				detail = mutedStyle.Render(detail)
			}
			fmt.Println(detail)
		}
	}
	fmt.Println()
	return nil
}

func severityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM":
		return 2
	case "LOW":
		return 3
	case "INFO":
		return 4
	}
	return 5
}

func severityStyle(s string) lipgloss.Style {
	switch s {
	case "CRITICAL":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	case "HIGH":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	case "MEDIUM":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	case "LOW":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	}
	return lipgloss.NewStyle()
}

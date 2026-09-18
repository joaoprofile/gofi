package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/hsec"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

func newHsecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hsec",
		Short: i18n.T("cmd.hsec.short"),
		Long: `Run the Horusec static analysis security scanner against the current project.

Configuration lives under the hsec: block in .gofi.yaml. gofi renders that block
into .gofi/horusec-config.json before each run, then invokes the horusec binary
against it.

Without a subcommand, hsec runs the full scan locally (alias of 'gofi hsec
start'). Nothing is sent to a Horusec Manager unless 'start --publish' is used.`,
		Example: `gofi hsec
gofi hsec start
gofi hsec start --publish
gofi hsec list
gofi hsec install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHsecStart(false)
		},
	}
	cmd.AddCommand(newHsecStartCmd(), newHsecInstallCmd(), newHsecListCmd(), newHsecPruneCmd())
	return cmd
}

func newHsecStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: i18n.T("cmd.hsec.start.short"),
		Long: `Render .gofi/horusec-config.json from the hsec: block and invoke 'horusec start' against the project.

With --publish the analysis is also sent to the Horusec Manager configured in
hsec.manager. The repository token is read from HORUSEC_REPOSITORY_AUTHORIZATION
and the Manager must answer its healthcheck before the scan starts. A publishing
failure exits with code 3, distinct from a failing scan.`,
		Example: `gofi hsec start
HORUSEC_REPOSITORY_AUTHORIZATION=<token> gofi hsec start --publish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			publish, _ := cmd.Flags().GetBool("publish")
			return runHsecStart(publish)
		},
	}
	cmd.Flags().Bool("publish", false, "send the analysis to the Horusec Manager in hsec.manager")
	return cmd
}

func newHsecInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: i18n.T("cmd.hsec.install.short"),
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

func newHsecPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "prune",
		Short:   "Remove the isolated Docker daemon and its image cache",
		Long:    `Remove this project's isolated Docker daemon (hsec.docker_runtime: isolated) and the volume that caches the horusec tool images. The next scan recreates both.`,
		Example: `gofi hsec prune`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			if err := hsec.RemoveIsolatedDaemon(root); err != nil {
				return err
			}
			fmt.Println("isolated docker daemon removed.")
			return nil
		},
	}
}

func newHsecListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   i18n.T("cmd.hsec.list.short"),
		Long:    `Print findings recorded in .gofi/horusec-output.json by the most recent 'gofi hsec start' run.`,
		Example: `gofi hsec list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHsecList()
		},
	}
}

// ExitCodeError carries a process exit code distinct from the generic 1, so
// automation can tell a Manager failure apart from a failing scan.
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string { return e.Err.Error() }
func (e *ExitCodeError) Unwrap() error { return e.Err }

const (
	exitCodeManager      = 3
	exitCodeInterrupted  = 130
	managerHealthTimeout = 10 * time.Second
)

func runHsecStart(publish bool) error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	h := cfg.Hsec
	if !h.Enabled {
		return errors.New("hsec is disabled in .gofi.yaml; set hsec.enabled to true to run")
	}
	isolated := h.UseDocker && h.DockerRuntime == config.HsecDockerRuntimeIsolated
	if !isolated && !hsec.IsInstalled() {
		return errors.New("horusec is not installed on PATH; run `gofi hsec install` first")
	}

	var token string
	var fpHashes, raHashes []string
	if publish {
		if h.Manager == nil {
			return errors.New("hsec.manager is not configured in .gofi.yaml; cannot publish")
		}
		if token, err = hsec.ResolveAuthToken(); err != nil {
			return err
		}
		if fpHashes, raHashes, err = hsec.SuppressedHashes(root, h); err != nil {
			return err
		}
		if err := hsec.CheckManager(h.Manager.URL, managerHealthTimeout); err != nil {
			return &ExitCodeError{Code: exitCodeManager, Err: err}
		}
	}
	// Catch Ctrl+C from here on so the scan is interrupted, not the CLI: the
	// deferred isolated-daemon stop and horusec's own cleanup then still run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if h.UseDocker {
		if !hsec.DockerAvailable() {
			return errors.New("hsec.use_docker is true but the Docker daemon is not reachable; start Docker or set hsec.use_docker: false")
		}
		if isolated {
			if err := hsec.EnsureIsolatedDaemon(root, os.Stdout); err != nil {
				return err
			}
			defer func() {
				if err := hsec.StopIsolatedDaemon(root); err != nil {
					fmt.Fprintln(os.Stderr, "warning:", err)
				}
			}()
		} else if err := hsec.CheckHostDockerCompatible(); err != nil {
			return err
		}
	}

	configPath, err := hsec.WriteConfig(root, h)
	if err != nil {
		return fmt.Errorf("write horusec-config.json: %w", err)
	}
	if publish {
		fmt.Printf("Running horusec against %s and publishing to %s as %s …\n\n", root, h.Manager.URL, h.Manager.RepositoryName)
	} else {
		fmt.Printf("Running horusec against %s (local only, nothing is published) …\n\n", root)
	}
	runErr := hsec.Run(ctx, h, hsec.RunOptions{
		ProjectRoot:         root,
		ConfigPath:          configPath,
		Publish:             publish,
		AuthToken:           token,
		FalsePositiveHashes: fpHashes,
		RiskAcceptHashes:    raHashes,
		Stdout:              os.Stdout,
		Stderr:              os.Stderr,
		Stdin:               os.Stdin,
	})
	if errors.Is(runErr, hsec.ErrInterrupted) {
		return &ExitCodeError{Code: exitCodeInterrupted, Err: runErr}
	}
	publishFailed := errors.Is(runErr, hsec.ErrPublishFailed)
	if runErr != nil && !publishFailed {
		return runErr
	}

	if !hsec.HasSuppressions(h) {
		if err := hsec.VerifyAnalysis(root, false); err != nil {
			return err
		}
	} else {
		res, evalErr := hsec.Evaluate(root, h)
		if evalErr == nil || errors.Is(evalErr, hsec.ErrFindings) {
			printHsecSummary(res)
		}
		if evalErr != nil {
			if publishFailed {
				fmt.Fprintln(os.Stderr, "warning:", hsec.ErrPublishFailed)
			}
			return evalErr
		}
	}
	if publishFailed {
		return &ExitCodeError{Code: exitCodeManager, Err: runErr}
	}
	if publish {
		fmt.Printf("\nScan complete and published to the Horusec Manager as %s.\n", h.Manager.RepositoryName)
	} else {
		fmt.Println("\nScan complete. Run `gofi hsec list` to inspect findings.")
	}
	return nil
}

func printHsecSummary(res hsec.Result) {
	fmt.Printf("\n%d finding(s) remaining; %d false positive(s) and %d risk accept(s) out of the verdict.\n", len(res.Remaining), len(res.Suppressed), len(res.RiskAccepted))
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
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	all, err := hsec.ParseFindings(root)
	if err != nil {
		return err
	}
	if all == nil {
		fmt.Println("no scan recorded yet — run `gofi hsec start` first.")
		return nil
	}
	if err := hsec.VerifyAnalysis(root, false); err != nil {
		fmt.Fprintln(os.Stderr, "warning: the last scan is incomplete —", err)
	}
	res, err := hsec.Classify(all, cfg.Hsec)
	if err != nil {
		return err
	}
	findings := res.Remaining
	if len(findings) == 0 {
		fmt.Printf("no vulnerabilities found (%d false positive(s), %d risk accept(s)).\n", len(res.Suppressed), len(res.RiskAccepted))
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

	header := fmt.Sprintf("%d finding(s), %d false positive(s), %d risk accept(s)", len(findings), len(res.Suppressed), len(res.RiskAccepted))
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
		rule := f.RuleID
		if rule == "" {
			rule = "-"
		}
		fmt.Printf("    %-10s %-16s %s\n", sev, rule, loc)
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

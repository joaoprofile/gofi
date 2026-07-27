package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/dotenv"
	"github.com/joaoprofile/gofi-cli/internal/help"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/settings"
	"github.com/joaoprofile/gofi-cli/internal/sources"
	"github.com/joaoprofile/gofi-cli/internal/tui/spinner"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func NewRoot() *cobra.Command {
	// the persisted context decides the language of every description below,
	// so it has to be resolved before the tree is built
	bootstrapSettings()

	root := &cobra.Command{
		Use:   "gofi",
		Short: i18n.T("root.short"),
		Long:  i18n.T("root.long"),
		Example: `gofi init
gofi agent add gofi-pd
gofi train -a pd ./docs/fiscal.md
gofi test cover-html`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			loadDotenv()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootDefault(cmd)
		},
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	root.PersistentFlags().Bool("no-color", false, i18n.T("root.flag.no_color"))
	root.PersistentFlags().Bool("plain", false, i18n.T("root.flag.plain"))

	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		opts := help.DetectOptions(cmd)
		if cmd == cmd.Root() {
			fmt.Print(help.RenderRoot(cmd, Version, opts))
			return
		}
		fmt.Print(help.RenderCommand(cmd, opts))
	})

	root.SetHelpCommand(newHelpCmd())

	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newCommitCmd(),
		newAgentCmd(),
		newRemoteCmd(),
		newTrainCmd(),
		newTestCmd(),
		newUpdateCmd(),
		newInstitutionalCmd(),
		newDoctorCmd(),
		newInstallCmd(),
		newConfigCmd(),
		newSettingsCmd(),
		newHsecCmd(),
		newSonarCmd(),
	)

	return root
}

// loadDotenv loads a .env file before any gofi command runs, so variables like
// SONAR_HOST_URL / SONAR_TOKEN no longer need a manual `set -a; source .env`.
// It looks in the project root first, then the current directory. Variables
// already set in the environment win, and a missing file is a no-op.
func loadDotenv() {
	seen := make(map[string]struct{})
	var paths []string
	if root, err := findProjectRoot(); err == nil {
		paths = append(paths, filepath.Join(root, ".env"))
		seen[root] = struct{}{}
	}
	if cwd, err := os.Getwd(); err == nil {
		if _, ok := seen[cwd]; !ok {
			paths = append(paths, filepath.Join(cwd, ".env"))
		}
	}
	for _, p := range paths {
		if _, err := dotenv.Load(p); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load %s: %v\n", p, err)
		}
	}
}

// runRootDefault is invoked when the user types just `gofi` (no subcommand).
// If we're inside a gofi project, run a quick checkin pinging the configured
// agents source. Then show the splash listing in either case.
func runRootDefault(cmd *cobra.Command) error {
	if cfg, root, err := loadProjectConfig(); err == nil {
		if settings.CheckinEnabled() {
			runCheckin(cfg.Sources.Agents, root)
		}
	} else if !errors.Is(err, ErrNotInProject) {
		fmt.Fprintln(os.Stderr, i18n.T("root.warning", err))
	}

	opts := help.DetectOptions(cmd)
	fmt.Print(help.RenderRoot(cmd, Version, opts))

	if _, err := findProjectRoot(); errors.Is(err, ErrNotInProject) {
		fmt.Println("  " + i18n.T("root.tip_init"))
		fmt.Println()
	}
	return nil
}

// runCheckin pings the configured agents source so the user immediately knows
// the network path is healthy. Failures are logged but never fatal.
func runCheckin(agentsRef, projectRoot string) {
	if agentsRef == "" {
		return
	}
	steps := []spinner.Step{
		{
			Name: i18n.T("root.checkin", agentsRef),
			Fn: func() error {
				ref, err := sources.Parse(agentsRef)
				if err != nil {
					return err
				}
				cache, err := sources.ProjectCache(projectRoot)
				if err != nil {
					return err
				}
				client, err := sources.NewClient(cache)
				if err != nil {
					return err
				}
				_, err = client.Resolve(ref)
				return err
			},
		},
	}
	fmt.Println()
	spinner.Run(steps)
}

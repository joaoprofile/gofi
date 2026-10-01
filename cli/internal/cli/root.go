package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/dotenv"
	"github.com/joaoprofile/gofi/cli/internal/help"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
	"github.com/joaoprofile/gofi/cli/internal/settings"
	"github.com/joaoprofile/gofi/cli/internal/sources"
	"github.com/joaoprofile/gofi/cli/internal/tui/flow"
	"github.com/joaoprofile/gofi/cli/internal/tui/spinner"
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
gofi find "onde o pedido é faturado"
gofi update skills
gofi test cover-html`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			loadDotenv()
			useProjectHome()
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
		newChatCmd(),
		newHookCmd(),
		newMemoryCmd(),
		newTestCmd(),
		newUpdateCmd(),
		newDoctorCmd(),
		newInstallCmd(),
		newIndexCmd(),
		newFindCmd(),
		newIntakeCmd(),
		newAskCmd(),
		newShowCmd(),
		newPathCmd(),
		newMCPCmd(),
		newGuardCmd(),
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
		// Inside a project, a bare `gofi` is most often someone about to talk
		// to the agents: offer the interactive mode before the command list.
		if interactive(cmd) {
			enter, err := flow.YesNo(i18n.T("root.ask_chat"), i18n.T("root.ask_chat_help"), "", "", true)
			if err == nil && enter {
				return runChat(chatFlags{permissionMode: permissionAsk})
			}
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

// interactive reports whether a question can be asked: both ends are a
// terminal and the output is not plain.
func interactive(cmd *cobra.Command) bool {
	if help.DetectOptions(cmd).Plain {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// useProjectHome points every path at the agent folder this project uses,
// which its host decides: .claude/ for Claude Code, .agents/ for the tools on
// the open skills convention. Outside a project, or with a config that does
// not load, the default stands — the command that needs the project reports
// that itself.
func useProjectHome() {
	if cfg, _, err := loadProjectConfig(); err == nil {
		layout.SetHome(layout.HomeFor(cfg.AI.Host))
		if h, ok := host.Get(cfg.AI.Host); ok {
			scaffold.SetSkillModels(h, cfg.AI.Tiers)
		}
	}
}

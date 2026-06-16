package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/help"
	"github.com/joaoprofile/gofi-cli/internal/sources"
	"github.com/joaoprofile/gofi-cli/internal/tui/spinner"
)

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "gofi",
		Short: "CLI for gofi project lifecycle",
		Long:  "gofi is the CLI tool that bootstraps and manages the lifecycle of gofi projects:\nproject scaffolding, agent installation and training, test execution.",
		Example: `gofi init
gofi agent add gofi-pd
gofi train -a pd ./docs/fiscal.md
gofi test cover-html`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootDefault(cmd)
		},
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	root.PersistentFlags().Bool("no-color", false, "disable color output even on TTY")
	root.PersistentFlags().Bool("plain", false, "plain text output (no colors, no boxes) — useful for CI/pipes")

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
		newDoctorCmd(),
		newConfigCmd(),
		newHsecCmd(),
		newSonarCmd(),
	)

	return root
}

// runRootDefault is invoked when the user types just `gofi` (no subcommand).
// If we're inside a gofi project, run a quick checkin pinging the configured
// agents source. Then show the splash listing in either case.
func runRootDefault(cmd *cobra.Command) error {
	if cfg, root, err := loadProjectConfig(); err == nil {
		runCheckin(cfg.Sources.Agents, root)
	} else if !errors.Is(err, ErrNotInProject) {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	opts := help.DetectOptions(cmd)
	fmt.Print(help.RenderRoot(cmd, Version, opts))

	if _, err := findProjectRoot(); errors.Is(err, ErrNotInProject) {
		fmt.Println("  Tip: run `gofi init` to bootstrap a project in the current directory.")
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
			Name: "agents source reachable (" + agentsRef + ")",
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

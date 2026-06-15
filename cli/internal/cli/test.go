package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/runner"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [task] [-- args]",
		Short: "Run language test tasks",
		Long: `Run a named test task declared in .gofi.yaml under test.tasks.

Without an argument, runs the task in test.default. Tasks can declare 'needs:' to
chain other tasks. Pre/post hooks defined in test.hooks run around the chain.

Anything after '--' is passed verbatim to the task command — useful for filtering
test names or adding flags without editing .gofi.yaml.`,
		Example: `gofi test
gofi test cover
gofi test cover-html
gofi test sonar
gofi test unit -- -run TestFoo -v`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			posArgs, extraArgs := splitDashArgs(cmd, args)
			return runTest(posArgs, extraArgs)
		},
	}
	cmd.AddCommand(newTestListCmd())
	return cmd
}

func newTestListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List available test tasks",
		Long:    `Print every task declared under test.tasks with its description and dependencies.`,
		Example: `gofi test list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestList()
		},
	}
}

func runTest(posArgs, extraArgs []string) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}

	taskName := cfg.Test.Default
	if len(posArgs) > 0 {
		taskName = posArgs[0]
	}
	if taskName == "" {
		return fmt.Errorf("no task specified and test.default is empty")
	}

	r := &runner.Runner{
		Cfg:    &cfg.Test,
		Cwd:    projectRootFromCfg(cfg),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}
	return r.Run(taskName, extraArgs)
}

func runTestList() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	r := &runner.Runner{Cfg: &cfg.Test}
	infos := r.List()
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })

	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	nameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	defaultMarker := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	width := 0
	for _, t := range infos {
		if l := len(t.Name); l > width {
			width = l
		}
	}

	fmt.Println()
	for _, t := range infos {
		marker := " "
		if t.Name == cfg.Test.Default {
			marker = "*"
		}
		name := padRight(t.Name, width+2)
		desc := t.Desc
		if desc == "" {
			desc = "(no description)"
		}
		needs := ""
		if len(t.Needs) > 0 {
			needs = "  needs: " + strings.Join(t.Needs, ", ")
		}
		if useColor {
			markerOut := marker
			if marker == "*" {
				markerOut = defaultMarker.Render(marker)
			}
			fmt.Printf("  %s %s%s%s\n", markerOut, nameStyle.Render(name), desc, mutedStyle.Render(needs))
		} else {
			fmt.Printf("  %s %s%s%s\n", marker, name, desc, needs)
		}
	}
	if useColor {
		fmt.Println()
		fmt.Println("  " + mutedStyle.Render("* = default task"))
	} else {
		fmt.Println("\n  * = default task")
	}
	fmt.Println()
	return nil
}

// splitDashArgs separates positional args from anything after `--`.
func splitDashArgs(cmd *cobra.Command, args []string) ([]string, []string) {
	idx := cmd.ArgsLenAtDash()
	if idx < 0 {
		return args, nil
	}
	return args[:idx], args[idx:]
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

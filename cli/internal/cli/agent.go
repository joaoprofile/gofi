package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents (add | remove | list)",
		Long: `Manage which agents are active in this project.

Adding an agent installs its skill file in .claude/commands/ and registers it in
.gofi.yaml. Removing reverses both operations and (with confirmation) deletes any
training topics installed for that agent. Listing shows active vs available agents.

Available agents: gofi-pd, gofi-spec, gofi-eng, gofi-qa.`,
		Example: `gofi agent list
gofi agent add gofi-pd
gofi agent remove gofi-qa`,
	}
	cmd.AddCommand(newAgentAddCmd(), newAgentRemoveCmd(), newAgentListCmd())
	return cmd
}

func newAgentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Install an agent into this project",
		Long: `Install the named agent into this project.

Copies the agent's skill file into .claude/commands/ (fetching the latest from
GitHub when possible, falling back to the embedded snapshot) and registers
the agent in .gofi.yaml. No-op if the agent is already installed.`,
		Example: `gofi agent add gofi-pd
gofi agent add gofi-spec`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentAdd(args[0])
		},
	}
}

func newAgentRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Uninstall an agent from this project",
		Long: `Remove the named agent from this project.

Deletes the agent's skill file from .claude/commands/, removes it from .gofi.yaml
and (after confirmation) deletes the per-agent training knowledge dir.`,
		Example: `gofi agent remove gofi-qa
gofi agent remove gofi-qa --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			return runAgentRemove(args[0], yes)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the training-deletion confirmation")
	return cmd
}

func newAgentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List active and available agents",
		Long:    `Print the agents currently active in this project alongside the full list of available agents.`,
		Example: `gofi agent list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAgentList()
		},
	}
}

func runAgentAdd(name string) error {
	if !scaffold.IsValidAgent(name) {
		return fmt.Errorf("unknown agent %q (available: %s)", name, strings.Join(scaffold.AllAgents(), ", "))
	}
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	if slices.Contains(cfg.Agents, name) {
		fmt.Printf("agent %q is already installed\n", name)
		return nil
	}

	projectRoot := projectRootFromCfg(cfg)
	sha, err := installSingleAgentFromSource(projectRoot, name, cfg.Sources.Agents)
	if err != nil {
		return err
	}

	cfg.Agents = append(cfg.Agents, name)
	sort.Strings(cfg.Agents)
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}

	fmt.Printf("Installed %s (sha=%s).\n", name, short(sha))
	return nil
}

func runAgentRemove(name string, autoConfirm bool) error {
	if !scaffold.IsValidAgent(name) {
		return fmt.Errorf("unknown agent %q (available: %s)", name, strings.Join(scaffold.AllAgents(), ", "))
	}
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	if !slices.Contains(cfg.Agents, name) {
		fmt.Printf("agent %q is not installed\n", name)
		return nil
	}

	projectRoot := projectRootFromCfg(cfg)
	hasTraining := agentHasTraining(cfg, name)

	removeKnowledge := false
	if hasTraining {
		if autoConfirm {
			removeKnowledge = true
		} else {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("agent has training topics; pass --yes to delete them in non-interactive mode")
			}
			ok := false
			if err := huh.NewConfirm().
				Title(fmt.Sprintf("Delete training topics for %s?", name)).
				Description("Removes everything under .claude/knowledge/<short>/.").
				Affirmative("Delete training").
				Negative("Keep training files").
				Value(&ok).Run(); err != nil {
				return err
			}
			removeKnowledge = ok
		}
	}

	if err := scaffold.RemoveAgent(projectRoot, name, removeKnowledge); err != nil {
		return err
	}

	cfg.Agents = removeString(cfg.Agents, name)
	if removeKnowledge {
		clearTrainingForAgent(cfg, name)
	}
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}

	fmt.Printf("Removed %s.\n", name)
	if hasTraining && !removeKnowledge {
		fmt.Println("Training files preserved under .claude/knowledge/.")
	}
	return nil
}

func runAgentList() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	active := map[string]bool{}
	for _, a := range cfg.Agents {
		active[a] = true
	}

	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(int(os.Stdout.Fd()))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	fmt.Println()
	for _, name := range scaffold.AllAgents() {
		status := "available"
		marker := " "
		if active[name] {
			status = "active"
			marker = "✓"
		}
		line := fmt.Sprintf("  %s %-12s  %s", marker, name, status)
		if useColor {
			if active[name] {
				line = activeStyle.Render(line)
			} else {
				line = mutedStyle.Render(line)
			}
		}
		fmt.Println(line)
	}
	fmt.Println()
	return nil
}

// installSingleAgentFromSource fetches the gofi monorepo and installs a
// single agent's skill file from ai/skills/. There is no embedded fallback —
// fetch errors propagate.
func installSingleAgentFromSource(projectRoot, agentName, ref string) (string, error) {
	dir, resolved, err := fetchSource(projectRoot, ref)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", ref, err)
	}
	fsys := os.DirFS(dir)
	agentPath := filepath.Join(dir, "ai", "skills", agentName+".md")
	if _, err := os.Stat(agentPath); err != nil {
		return "", fmt.Errorf("agent %s not found in %s: %w", agentName, ref, err)
	}
	if err := scaffold.InstallAgentFromFS(fsys, ".", projectRoot, agentName); err != nil {
		return "", err
	}
	return resolved.Ref, nil
}

func projectRootFromCfg(cfg *config.GofiConfig) string {
	if cfg.Project.Root != "" {
		return cfg.Project.Root
	}
	cwd, _ := os.Getwd()
	return cwd
}

func removeString(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func agentHasTraining(cfg *config.GofiConfig, agent string) bool {
	switch agent {
	case config.AgentPD:
		return len(cfg.Training.PD) > 0
	case config.AgentSpec:
		return len(cfg.Training.Spec) > 0
	case config.AgentEng:
		return len(cfg.Training.Eng) > 0
	case config.AgentQA:
		return len(cfg.Training.QA) > 0
	}
	return false
}

func clearTrainingForAgent(cfg *config.GofiConfig, agent string) {
	switch agent {
	case config.AgentPD:
		cfg.Training.PD = nil
	case config.AgentSpec:
		cfg.Training.Spec = nil
	case config.AgentEng:
		cfg.Training.Eng = nil
	case config.AgentQA:
		cfg.Training.QA = nil
	}
}

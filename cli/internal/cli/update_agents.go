package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
)

func newUpdateAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: i18n.T("cmd.update.agents.short"),
		Long: `Bring the project's agent instructions to AGENTS.md, at the root, from the
source pinned at 'sources.agents'.

AGENTS.md is the one file every coding agent reads — Claude Code (2.1.277 and
later), Codex, Copilot, Cursor, Windsurf. Claude Code stops reading it while a
CLAUDE.md is anywhere in the project, so the .claude/CLAUDE.md older releases
installed goes:

  unedited              removed, and the release's AGENTS.md written
  edited by the team    its text becomes AGENTS.md, word for word
  AGENTS.md exists too  AGENTS.md stays; the old file is removed

When the project changed host (ai.host), the agents folder moves to the one the
new host reads — .claude/ ↔ .agents/ — leaving out what only the old host used
(Claude Code's settings and subagents, backed up), and gofi's MCP server is
registered where the new host looks for it.

An AGENTS.md the team edited is kept; --force puts the release's back. Every
file replaced or removed is copied to .gofi/backup/ first. A CLAUDE.md or
CLAUDE.local.md at the root is the team's and is left alone — reported,
because it still hides AGENTS.md from Claude Code. A project that uses the
Gemini CLI (.gemini/ exists) gets AGENTS.md added to its context files.`,
		Example: `gofi update agents
gofi update agents --yes
gofi update agents --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runAgentsUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite an AGENTS.md you edited (backed up to .gofi/backup/)")
	return cmd
}

func runAgentsUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	t, err := agentsTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// agentsTarget plans the agent instructions: AGENTS.md, the move off
// CLAUDE.md, and the move of the agents folder when the host changed.
func agentsTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	root := cfg.Project.Root
	srcDir, _, err := fetchSource(root, cfg.Sources.Agents)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", cfg.Sources.Agents, err)
	}
	upstream, err := scaffold.ReadAgentsSource(os.DirFS(srcDir), ".")
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", scaffold.AgentsFile, cfg.Sources.Agents, err)
	}
	plan, err := scaffold.PlanAgents(root, upstream, force)
	if err != nil {
		return nil, err
	}
	h := projectHost()
	moveFrom := previousHome(root, h)

	s := updateScope{Force: force, LeavesAlone: []string{layout.Home() + "/", ".gofi.yaml"}}
	if plan.Write != nil {
		s.write(scaffold.AgentsFile, plan.Note)
	}
	if plan.RemoveLegacy {
		note := "removed — it hid AGENTS.md from Claude Code"
		if plan.LegacyEdited && plan.Write == nil {
			// The team's text leaves the project, if only to the backup.
			s.Keeps, s.Force = []string{scaffold.LegacyInstructions}, true
			note = "removed — your edits are in .gofi/backup/; bring what is still true into AGENTS.md"
		}
		s.write(scaffold.LegacyInstructions, note)
	}
	if moveFrom != "" {
		s.write(h.Home+"/", "moved from "+moveFrom+"/ — the project now runs on "+h.Label)
	}
	if strings.HasPrefix(plan.Note, "kept (edited") {
		s.Keeps = append(s.Keeps, scaffold.AgentsFile)
	}
	return &targetPlan{
		name:  "agents",
		title: "Update the agent instructions?",
		scope: s,
		apply: func() error {
			if moveFrom != "" {
				dropped, err := scaffold.MoveHome(root, moveFrom, h.Home)
				if err != nil {
					return fmt.Errorf("move %s/: %w", moveFrom, err)
				}
				for _, d := range dropped {
					fmt.Println(i18n.T("update.agents.dropped", d, h.Label))
				}
			}
			if err := scaffold.ApplyAgents(root, plan); err != nil {
				return err
			}
			if _, err := h.RegisterMCP(root); err != nil {
				return err
			}
			fmt.Printf("\nAgent instructions now in %s.\n", scaffold.AgentsFile)
			return nil
		},
		after: func() error {
			if changed, err := addGeminiContext(root); err != nil {
				return err
			} else if changed {
				fmt.Println(i18n.T("update.agents.gemini"))
			}
			if plan.Note == "kept (edited, no project block)" {
				fmt.Println(i18n.T("update.agents.no_block", scaffold.AgentsFile))
			}
			for _, f := range plan.Blocking {
				fmt.Println(i18n.T("update.agents.blocking", f, scaffold.AgentsFile))
			}
			return nil
		},
	}, nil
}

// geminiSettings is where the Gemini CLI reads a project's settings.
const geminiSettings = ".gemini/settings.json"

// addGeminiContext makes a project that already uses the Gemini CLI read
// AGENTS.md too: context.fileName gains it, in front of what was there. A
// project without .gemini/ is left without one — it does not use Gemini, and
// gofi does not scaffold tools a team never chose.
func addGeminiContext(root string) (bool, error) {
	if fi, err := os.Stat(filepath.Join(root, ".gemini")); err != nil || !fi.IsDir() {
		return false, nil
	}
	path := filepath.Join(root, geminiSettings)
	doc := map[string]any{}
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &doc); err != nil {
			return false, fmt.Errorf("%s: %w — fix it by hand, it is not gofi's to rewrite", geminiSettings, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	ctx, _ := doc["context"].(map[string]any)
	if ctx == nil {
		ctx = map[string]any{}
	}
	var names []any
	switch v := ctx["fileName"].(type) {
	case string:
		names = []any{v}
	case []any:
		names = v
	default:
		// Gemini's own default, kept so adding AGENTS.md takes nothing away.
		names = []any{"GEMINI.md"}
	}
	if slices.Contains(names, any(scaffold.AgentsFile)) {
		return false, nil
	}
	ctx["fileName"] = append([]any{scaffold.AgentsFile}, names...)
	doc["context"] = ctx
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, append(out, '\n'), 0o644)
}

// previousHome is the agents folder of another host when this host's is
// missing: the sign the project changed host and its content has not moved.
func previousHome(root string, h host.Host) string {
	if _, err := os.Stat(filepath.Join(root, h.Home)); err == nil {
		return ""
	}
	for _, other := range host.All {
		if other.Home == h.Home {
			continue
		}
		if fi, err := os.Stat(filepath.Join(root, other.Home)); err == nil && fi.IsDir() {
			return other.Home
		}
	}
	return ""
}

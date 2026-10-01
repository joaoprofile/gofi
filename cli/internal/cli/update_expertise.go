package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
)

func newUpdateExpertiseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expertise",
		Short: i18n.T("cmd.update.expertise.short"),
		Long: `Refresh the expertise packs — DDD, persistence, messaging, APIs, testing… —
from the source pinned at 'sources.agents'.

Packs are gofi's: an update rewrites them. A pack file the team edited is kept
and reported; --force puts upstream back and copies what it replaced to
.gofi/backup/ first. The place for a team's own rule is knowledge/, where it
takes precedence over the pack — not an edit to the pack.

The changes are listed and confirmed before anything is written. Nothing
outside the expertise folder is touched.`,
		Example: `gofi update expertise
gofi update expertise --yes
gofi update expertise --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runExpertiseUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite pack files you edited (backed up to .gofi/backup/)")
	return cmd
}

func runExpertiseUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	t, err := expertiseTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// expertiseTarget plans the expertise update: the packs, and the retirement
// of the copies older versions seeded into knowledge/.
func expertiseTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	root := cfg.Project.Root
	srcDir, _, err := fetchSource(root, cfg.Sources.Agents)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", cfg.Sources.Agents, err)
	}
	fsys := os.DirFS(srcDir)
	changes, err := scaffold.PlanExpertiseUpdate(fsys, ".", root)
	if err != nil {
		return nil, fmt.Errorf("plan update: %w", err)
	}
	dir := layout.Expertise().Dir
	edited := scaffold.PreservedFilesIn(root, []string{"expertise"})
	printUpdatePlan("expertise", dir, changes, edited)

	var added, changed int
	for _, c := range changes {
		if c.Kind == scaffold.ChangeNew {
			added++
		} else if c.Kind == scaffold.ChangeModified {
			changed++
		}
	}
	// Copies older versions seeded into knowledge/ are the one thing outside
	// the packs this touches: an untouched one is an older duplicate of a
	// section it installs. An edited one is the team's, and says so.
	retire, seededKept := scaffold.PlanSeededKnowledge(root)
	s := updateScope{Keeps: edited, Force: force, Hint: keepsHint, LeavesAlone: []string{layout.Skills().Dir + "/", "AGENTS.md", ".gofi.yaml"}}
	if added+changed > 0 || force {
		s.write(dir+"/", planNote(added, changed))
	}
	if len(retire) > 0 {
		s.write(layout.Knowledge().Dir+"/", fmt.Sprintf("removes %d copy(ies) older versions seeded, now in the packs (backed up)", len(retire)))
	}
	if len(retire) == 0 {
		s.LeavesAlone = append(s.LeavesAlone, layout.Knowledge().Dir+"/")
	}
	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	return &targetPlan{
		name:  "expertise",
		title: "Update the expertise packs?",
		scope: s,
		apply: func() error {
			if _, err := scaffold.InstallExpertiseContent(fsys, ".", root, mode); err != nil {
				return err
			}
			removed, err := scaffold.RetireSeededKnowledge(root)
			if err != nil {
				return fmt.Errorf("retire seeded knowledge: %w", err)
			}
			fmt.Printf("\nExpertise packs updated in %s/.\n", dir)
			if len(removed) > 0 {
				fmt.Printf("Removed %d seeded copy(ies) from %s/ — the packs carry them now.\n", len(removed), layout.Knowledge().Dir)
			}
			return nil
		},
		after: func() error {
			if len(seededKept) > 0 {
				fmt.Printf("Kept %d in %s/ that the team edited or gofi has no record of — they now read as the team's learning over the pack:\n", len(seededKept), layout.Knowledge().Dir)
				for _, k := range seededKept {
					fmt.Println("  " + k)
				}
			}
			return nil
		},
	}, nil
}

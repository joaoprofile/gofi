package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"github.com/gofi-labs/gofi/cli/internal/scaffold"
)

func newUpdateTemplatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "templates",
		Short: i18n.T("cmd.update.templates.short"),
		Long: `Refresh the PRD and spec templates from the source pinned at 'sources.agents'.

The templates are gofi's: they carry the frontmatter and sections the document
index reads, so a project on old templates writes documents the index reads
worse. A template the team edited is kept and reported; --force puts upstream
back and copies what it replaced to .gofi/backup/ first. Documents already
written from a template are never touched.`,
		Example: `gofi update templates
gofi update templates --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runTemplatesUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite templates you edited (backed up to .gofi/backup/)")
	return cmd
}

func runTemplatesUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	t, err := templatesTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// templatesTarget plans the templates update.
func templatesTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	root := cfg.Project.Root
	srcDir, _, err := fetchSource(root, cfg.Sources.Agents)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", cfg.Sources.Agents, err)
	}
	fsys := os.DirFS(srcDir)
	changes, err := scaffold.PlanTemplatesUpdate(fsys, ".", root)
	if err != nil {
		return nil, fmt.Errorf("plan update: %w", err)
	}
	dir := layout.Templates().Dir
	edited := scaffold.PreservedFilesIn(root, []string{"templates"})
	printUpdatePlan("template", dir, changes, edited)

	var added, changed int
	for _, c := range changes {
		if c.Kind == scaffold.ChangeNew {
			added++
		} else if c.Kind == scaffold.ChangeModified {
			changed++
		}
	}
	s := updateScope{Keeps: edited, Force: force, Hint: keepsHint,
		LeavesAlone: []string{"specs/", "prd/", layout.Skills().Dir + "/", ".gofi.yaml"}}
	if added+changed > 0 || force {
		s.write(dir+"/", planNote(added, changed))
	}
	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	return &targetPlan{
		name:  "templates",
		title: "Update the templates?",
		scope: s,
		apply: func() error {
			if _, err := scaffold.InstallTemplatesContent(fsys, ".", root, mode); err != nil {
				return err
			}
			fmt.Printf("\nTemplates updated in %s/.\n", dir)
			return nil
		},
	}, nil
}

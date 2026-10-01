package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
)

func newUpdateDSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ds",
		Short: i18n.T("cmd.update.ds.short"),
		Long: `Refresh the design-system docs of every front-end surface the project
declares — .claude/sdk/web/, .claude/sdk/mobile/ — from 'sources.agents'.

This is what /gofi-ui reads: tokens, components, patterns and rules. It is a
separate target from 'gofi update sdk' because the two move on their own
schedules — a brand refresh has nothing to do with a backend SDK release.

The tokens and rules a project adapts to its own product are exactly what tends
to be edited here, so an edited file is kept and reported. --force puts upstream
back, copying what it replaces to .gofi/backup/ first.`,
		Example: `gofi update ds
gofi update ds --yes
gofi update ds --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runDSUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite tuned design-system docs (backed up to .gofi/backup/)")
	return cmd
}

func runDSUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	if len(uiSurfacesFromConfig(cfg)) == 0 {
		return errors.New("this project declares no front-end surface — there is no design system to update")
	}
	fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	t, err := dsTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// dsTarget plans the design-system update of every front-end surface.
func dsTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	surfaces := uiSurfacesFromConfig(cfg)
	ref := cfg.Sources.Agents

	// The manifest records .claude/sdk/ as one tree, so the surfaces have to be
	// picked out of it: a doc tuned under sdk/<lang>/ belongs to the other
	// target and must not be reported here.
	tuned := filterSurfaces(scaffold.PreservedFilesIn(cfg.Project.Root, []string{scaffold.SDKDir}), surfaces)

	scope := updateScope{
		Keeps: tuned,
		Force: force,
		Hint:  keepsHint,
		LeavesAlone: []string{
			layout.Skills().Dir + "/", layout.SDK().Path("<lang>") + "/", ".gofi/gofi-sdk-<lang>/",
			".gofi.yaml", "knowledge/", "memory/", "institutional/", "the graph",
		},
	}
	for _, s := range surfaces {
		scope.write(layout.SDK().Path(s)+"/", "design system ← "+ref)
	}
	printTunedFiles(tuned)

	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	return &targetPlan{
		name:  "design system",
		title: "Update the design system?",
		scope: scope,
		apply: func() error {
			if err := installDSFromSource(cfg.Project.Root, surfaces, ref, mode); err != nil {
				return err
			}
			fmt.Printf("\nDesign system updated — %s.\n", strings.Join(surfaces, ", "))
			return nil
		},
	}, nil
}

// filterSurfaces keeps the .claude/sdk/ entries that belong to a front-end
// surface, dropping the backend language docs that share the same tree.
func filterSurfaces(files, surfaces []string) []string {
	var out []string
	for _, f := range files {
		for _, s := range surfaces {
			if strings.HasPrefix(f, layout.SDK().Path(s)+"/") {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

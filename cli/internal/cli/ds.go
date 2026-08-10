package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
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

	surfaces := uiSurfacesFromConfig(cfg)
	if len(surfaces) == 0 {
		return errors.New("this project declares no front-end surface — there is no design system to update")
	}
	ref := cfg.Sources.Agents
	fmt.Printf("Resolving %s …\n", ref)

	// The manifest records .claude/sdk/ as one tree, so the surfaces have to be
	// picked out of it: a doc tuned under sdk/<lang>/ belongs to the other
	// target and must not be reported here.
	tuned := filterSurfaces(scaffold.PreservedFilesIn(cfg.Project.Root, []string{scaffold.SDKDir}), surfaces)

	scope := updateScope{
		Keeps: tuned,
		Force: force,
		LeavesAlone: []string{
			".claude/skills/", ".claude/sdk/<lang>/", ".gofi/gofi-sdk-<lang>/",
			".gofi.yaml", "knowledge/", "memory/", "institutional/", "the graph",
		},
	}
	for _, s := range surfaces {
		scope.write(".claude/sdk/"+s+"/", "design system ← "+ref)
	}
	printTunedFiles(tuned)

	ok, err := confirmUpdate("Update the design system?", scope, autoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("design system left as it is.")
		return nil
	}

	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	if err := installDSFromSource(cfg.Project.Root, surfaces, ref, mode); err != nil {
		return fmt.Errorf("ds update: %w", err)
	}
	fmt.Printf("\nDesign system updated — %s.\n", strings.Join(surfaces, ", "))
	noteDrift(cfg)
	return nil
}

// filterSurfaces keeps the .claude/sdk/ entries that belong to a front-end
// surface, dropping the backend language docs that share the same tree.
func filterSurfaces(files, surfaces []string) []string {
	var out []string
	for _, f := range files {
		for _, s := range surfaces {
			if strings.HasPrefix(f, ".claude/sdk/"+s+"/") {
				out = append(out, f)
				break
			}
		}
	}
	return out
}

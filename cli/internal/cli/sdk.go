package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
)

func newUpdateSDKCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: i18n.T("cmd.update.sdk.short"),
		Long: `Refresh the backend SDK: the checkout under .gofi/gofi-sdk-<lang>/ from
'sources.sdk.<lang>', and the docs under .claude/sdk/<lang>/ from that same
override — or from 'sources.agents' when the override ships no docs layout.
go.work is realigned with the new checkout.

The two travel together on purpose: the docs the agents read and the code the
toolchain compiles against have to describe the same release.

A doc you tuned is kept and the rest is refreshed — house rules a project adapts
to its own product survive. --force puts every managed file back to upstream,
copying what it replaces to .gofi/backup/ first.

The front-end design systems are their own target: 'gofi update ds'.`,
		Example: `gofi update sdk
gofi update sdk --yes
gofi update sdk --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runSDKUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite tuned SDK docs (backed up to .gofi/backup/)")
	return cmd
}

func runSDKUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}

	language := backendLang(cfg)
	if language == "" {
		return errors.New("this project has no backend language — there is no SDK to update (see 'gofi update ds' for the front-end design system)")
	}
	agentsRef := cfg.Sources.Agents
	sdkRef := cfg.Sources.SDK[language]

	src := sdkRef
	if src == "" {
		src = agentsRef + " (ai/sdk/" + language + ")"
	}
	fmt.Printf("Resolving %s …\n", src)

	scope := updateScope{
		Keeps: scaffold.PreservedFilesIn(cfg.Project.Root, []string{scaffold.SDKDir}),
		Force: force,
		LeavesAlone: []string{
			".claude/skills/", ".claude/sdk/<surface>/", ".gofi.yaml",
			"knowledge/", "memory/", "institutional/", "the graph",
		},
	}
	scope.write(".claude/sdk/"+language+"/", "docs ← "+src)
	if sdkRef != "" {
		scope.write(".gofi/gofi-sdk-"+language+"/", "checkout ← "+sdkRef)
	}
	if language == config.LanguageGo {
		scope.write("go.work", "realigned with the checkout")
	}
	printTunedFiles(scope.Keeps)

	ok, err := confirmUpdate("Update the SDK?", scope, autoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("sdk left as it is.")
		return nil
	}

	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	if err := installSDKFromSource(cfg.Project.Root, language, agentsRef, sdkRef, mode); err != nil {
		return fmt.Errorf("sdk update: %w", err)
	}
	// The checkout may have gained or lost submodules, and a go.work still
	// pointing at the old shape breaks the build rather than the docs.
	if language == config.LanguageGo {
		if err := scaffold.EnsureGoWorkSDK(cfg.Project.Root, language); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not align go.work with local SDK: %v\n", err)
		}
	}
	// A project scaffolded before v2.4 still carries the flat SDK dirs beside
	// the new tree, and this is the target that owns that layout.
	if removed := scaffold.CleanLegacySDKLayout(cfg.Project.Root); len(removed) > 0 {
		fmt.Printf("Removed %d legacy SDK dir(s) (migrated to .claude/sdk/).\n", len(removed))
	}

	fmt.Println("\nSDK update complete.")
	noteDrift(cfg)
	return nil
}

// printTunedFiles lists the files a target will keep, which is the evidence
// behind the scope block's KEEPS line.
func printTunedFiles(tuned []string) {
	if len(tuned) == 0 {
		return
	}
	fmt.Printf("\n%d file(s) carry your edits:\n\n", len(tuned))
	for _, f := range tuned {
		fmt.Printf("  %s\n", f)
	}
}

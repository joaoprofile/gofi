package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
)

func newUpdateSDKCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: i18n.T("cmd.update.sdk.short"),
		Long: `Refresh the backend SDK: the checkout under .gofi/gofi-sdk-<lang>/ from
'sources.sdk.<lang>', and everything the agents read about it under the
agents folder's sdk/<lang>/:

  api/          the API reference, generated from the checkout's source — one
                section per exported symbol, at the version the project pinned
  api/examples.md  the SDK's runnable examples, pointing at the checkout
  knowledge/    the SDK's conventions and pitfalls (curated)
  boilerplates/ project layer skeletons (curated)

The reference is generated, never hand-written, so it cannot drift from the
code. go.work is realigned with the new checkout.

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
	if backendLang(cfg) == "" {
		return errors.New("this project has no backend language — there is no SDK to update (see 'gofi update ds' for the front-end design system)")
	}
	t, err := sdkTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// sdkTarget plans the SDK update: the checkout, the docs and go.work.
func sdkTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	language := backendLang(cfg)
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
		Hint:  keepsHint,
		LeavesAlone: []string{
			layout.Skills().Dir + "/", layout.SDK().Path("<surface>") + "/", ".gofi.yaml",
			"knowledge/", "memory/", "institutional/", "the graph",
		},
	}
	scope.write(layout.SDK().Path(language)+"/", "docs ← "+src)
	if sdkRef != "" {
		scope.write(".gofi/gofi-sdk-"+language+"/", "checkout ← "+sdkRef)
	}
	if language == config.LanguageGo {
		scope.write("go.work", "realigned with the checkout")
	}
	printTunedFiles(scope.Keeps)

	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	return &targetPlan{
		name:  "sdk",
		title: "Update the SDK?",
		scope: scope,
		apply: func() error {
			if err := installSDKFromSource(cfg.Project.Root, language, agentsRef, sdkRef, mode); err != nil {
				return err
			}
			// The checkout may have gained or lost submodules, and a go.work
			// still pointing at the old shape breaks the build rather than the
			// docs.
			if language == config.LanguageGo {
				if err := scaffold.EnsureGoWorkSDK(cfg.Project.Root, language); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not align go.work with local SDK: %v\n", err)
				}
			}
			if retired, err := scaffold.RetireHandWrittenSDKDocs(cfg.Project.Root, language); err != nil {
				return err
			} else if len(retired) > 0 {
				fmt.Printf("Replaced the hand-written %s with the reference generated from the SDK (%d file(s) backed up in .gofi/backup/).\n",
					layout.SDK().Path(language, "sdk-docs")+"/", len(retired))
			}
			// A project scaffolded before v2.4 still carries the flat SDK dirs
			// beside the new tree, and this is the target that owns that layout.
			if removed := scaffold.CleanLegacySDKLayout(cfg.Project.Root); len(removed) > 0 {
				fmt.Printf("Removed %d legacy SDK dir(s) (migrated to %s/).\n", len(removed), layout.SDK().Dir)
			}
			fmt.Println("\nSDK update complete.")
			return nil
		},
	}, nil
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

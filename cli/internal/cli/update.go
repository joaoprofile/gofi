package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
	"github.com/joaoprofile/gofi-cli/internal/sources"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update agents and SDK to the latest tagged version",
		Long: `Resolve the gofi-agents source pinned in the global config and, after
confirmation, reinstall .claude/ from the new tarball.

User-managed directories under .claude/knowledge/ and .claude/memory/ are
preserved. Everything else (CLAUDE.md, commands/, specs-template/,
prd-template/, sdk/<lang>/) is overwritten from the new ref. Pre-v2.4 layout
dirs (boilerplates/, gofi-sdk-<lang>/, sdk-knowledge/) are removed.

With --training, also revalidates training topics with URL sources (planned).`,
		Example: `gofi update
gofi update --yes
gofi update --training`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			training, _ := cmd.Flags().GetBool("training")
			return runUpdate(yes, training)
		},
	}
	cmd.Flags().Bool("training", false, "also revalidate training topics with URL sources")
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func runUpdate(autoConfirm, training bool) error {
	if training {
		fmt.Fprintln(os.Stderr, "warning: --training is not yet implemented; running update without it")
	}

	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}

	ref := cfg.Sources.Agents
	cache, err := sources.ProjectCache(cfg.Project.Root)
	if err != nil {
		return err
	}
	client, err := sources.NewClient(cache)
	if err != nil {
		return err
	}

	fmt.Printf("Resolving %s …\n", ref)
	parsed, err := sources.Parse(ref)
	if err != nil {
		return fmt.Errorf("parse %s: %w", ref, err)
	}
	resolved, err := client.Resolve(parsed)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", ref, err)
	}

	currentSha := readInstalledSha(cfg.Project.Root)
	if currentSha == resolved.Ref {
		fmt.Printf("Already up to date (sha=%s).\n", short(currentSha))
		// Even when content hasn't changed, realign go.work with the on-disk
		// SDK checkout — handles cases where .gofi/gofi-sdk-<lang>/ gained or
		// lost submodules outside an agents update, or was populated by an
		// older CLI build that didn't wire submodules into go.work.
		if backendLang(cfg) == config.LanguageGo {
			if err := scaffold.EnsureGoWorkSDK(cfg.Project.Root, backendLang(cfg)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not align go.work with local SDK: %v\n", err)
			}
		}
		return nil
	}

	if currentSha == "" {
		fmt.Printf("No previous installation recorded; will install %s.\n", short(resolved.Ref))
	} else {
		fmt.Printf("Update available: %s → %s\n", short(currentSha), short(resolved.Ref))
	}

	sourceRoot := config.DefaultSourceRoot
	if cfg.Backend != nil && cfg.Backend.Path != "" {
		sourceRoot = cfg.Backend.Path
	}
	data := scaffold.TemplateData{
		ProjectName: cfg.Project.Name,
		Language:    backendLang(cfg),
		SourceRoot:  sourceRoot,
		Date:        time.Now().Format("2006-01-02"),
		AIHost:      cfg.AI.Host,
		AIModel:     cfg.AI.Model,
		Agents:      cfg.Agents,
	}

	srcDir, _, err := fetchSource(cfg.Project.Root, ref)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", ref, err)
	}
	plan, err := scaffold.PlanAgentsUpdate(os.DirFS(srcDir), ".", cfg.Project.Root, data)
	if err != nil {
		return fmt.Errorf("plan update: %w", err)
	}
	printUpdatePlan(plan, backendLang(cfg))

	if !autoConfirm {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("update requires --yes when stdin is not a TTY")
		}
		ok := false
		if err := huh.NewConfirm().
			Title("Apply this update?").
			Description("Confirms applying the changes listed above. Knowledge (knowledge/) and memory (memory/) are preserved. Legacy SDK dirs (boilerplates/, gofi-sdk-<lang>/, sdk-knowledge/) are removed.").
			Affirmative("Update").
			Negative("Cancel").
			Value(&ok).Run(); err != nil {
			return err
		}
		if !ok {
			fmt.Println("update cancelled.")
			return nil
		}
	}

	sdkRef := cfg.Sources.SDK[backendLang(cfg)]
	sha, err := installFromSource(cfg.Project.Root, backendLang(cfg), uiSurfacesFromConfig(cfg), ref, sdkRef, data, scaffold.InstallUpdate)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if removed := scaffold.CleanLegacySDKLayout(cfg.Project.Root); len(removed) > 0 {
		fmt.Printf("Removed legacy SDK dirs: %d (migrated to .claude/sdk/%s/).\n", len(removed), backendLang(cfg))
	}
	if backendLang(cfg) == config.LanguageGo {
		if err := scaffold.EnsureGoWorkSDK(cfg.Project.Root, backendLang(cfg)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not align go.work with local SDK: %v\n", err)
		}
	}
	if err := writeInstalledSha(cfg.Project.Root, sha); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", installedFileName, err)
	}

	fmt.Printf("\nUpdate complete — .claude/ now at %s.\n", short(sha))
	return nil
}

// printUpdatePlan renders the diff between upstream and the project's
// current .claude/ files. New + modified entries are listed; unchanged
// files are omitted. The SDK reset line always appears when language is
// set because update wipes and recopies .claude/sdk/<lang>/ unconditionally.
func printUpdatePlan(plan []scaffold.Change, language string) {
	fmt.Println()
	if len(plan) == 0 {
		fmt.Println("No agent files would change in .claude/.")
	} else {
		fmt.Printf("The following %d file(s) in .claude/ would change:\n\n", len(plan))
		for _, c := range plan {
			fmt.Printf("  %-9s %s\n", c.Kind, c.RelPath)
		}
		fmt.Println()
	}
	if language != "" {
		fmt.Printf("Plus: .claude/sdk/%s/ will be reset to upstream.\n", language)
	}
	fmt.Println("Preserved: .claude/knowledge/ and .claude/memory/.")
	fmt.Println()
}

// backendLang returns the project's backend language, or "" for a front-only
// project (no backend: block).
func backendLang(cfg *config.GofiConfig) string {
	if cfg.Backend == nil {
		return ""
	}
	return cfg.Backend.Language
}

func short(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	if sha == "" {
		return "(none)"
	}
	return sha
}

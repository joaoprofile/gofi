package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/audit"
	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
	"github.com/joaoprofile/gofi-cli/internal/sources"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: i18n.T("cmd.update.short"),
		Long: `Resolve the gofi-agents source pinned in the global config and, after
confirmation, reinstall .claude/ from the new tarball.

Nothing the project wrote is lost. .claude/memory/ and .claude/institutional/
are preserved whole; .claude/knowledge/ is filled additively (a file added
upstream arrives, an edited one is left alone); and every other managed file —
CLAUDE.md, skills/, templates/, scripts/, sdk/<lang>/, sdk/<surface>/ — is only
refreshed while it still matches what gofi installed there. Edit one and the
update keeps your version and says so. Pre-v2.4 layout dirs (boilerplates/,
gofi-sdk-<lang>/, sdk-knowledge/) are removed.

Every run also brings the project up to the current standards: .gofi.yaml is
rewritten in the current schema with any missing block seeded (values already
in the file are kept; hand-written comments are not), the graph is rebuilt and
its git hooks reinstalled. It then reports the drift it cannot fix on its own —
document frontmatter and missing INDEXes belong to the agents.

With --audit, only that report runs and nothing is changed.
With --force, edited managed files go back to upstream; the replaced content is
copied to .gofi/backup/ first.
With --training, also revalidates training topics with URL sources (planned).`,
		Example: `gofi update
gofi update --yes
gofi update --audit
gofi update --force
gofi update --training`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if auditOnly, _ := cmd.Flags().GetBool("audit"); auditOnly {
				return runAuditOnly()
			}
			yes, _ := cmd.Flags().GetBool("yes")
			training, _ := cmd.Flags().GetBool("training")
			force, _ := cmd.Flags().GetBool("force")
			return runUpdate(cmd.Context(), yes, training, force)
		},
	}
	cmd.Flags().Bool("training", false, "also revalidate training topics with URL sources")
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("audit", false, "only report structure drift; change nothing")
	cmd.Flags().Bool("force", false, "overwrite managed files that carry local edits (backed up to .gofi/backup/)")
	return cmd
}

// runAuditOnly reports drift without touching the project, which is the only
// way to ask "is this project old?" without also answering it.
func runAuditOnly() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		// A config the current schema cannot read is the drift this command
		// exists to report, so refusing to run here would hide the worst case.
		// Without a parsed config there is no ref to fetch, so the upstream
		// knowledge check is skipped; an absent graph block means enabled.
		fmt.Fprintf(os.Stderr, "warning: %s did not load (%v); reporting what can still be read\n", config.FileName, err)
		printAudit(audit.Run(".", audit.Options{GraphEnabled: true}))
		return nil
	}
	printAudit(runAudit(cfg))
	return nil
}

func runUpdate(ctx context.Context, autoConfirm, training, force bool) error {
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
	// --force is how a project asks for its managed files back, which is a
	// reason to reinstall even when upstream has not moved.
	if currentSha == resolved.Ref && !force {
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
		// A matching sha only says .claude/ is current. The config schema, the
		// git hooks and the graph all drift on their own, and this is the path a
		// project takes most of the time — skipping them here would mean they
		// were only ever repaired when upstream happened to move.
		syncProjectStandards(ctx, cfg)
		// Nothing above rewrites .gofi.yaml's contents, specs/ or prd/, so a
		// project that stopped here for months is exactly the one that drifted.
		printAudit(runAudit(cfg))
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
	printUpdatePlan(plan, scaffold.PreservedFiles(cfg.Project.Root), force)

	mode := scaffold.InstallUpdate
	confirm := "Confirms applying the changes listed above. Memory (memory/) and institutional/ are preserved, knowledge/ only gains what it lacks, and managed files you edited keep your version. .gofi.yaml is rewritten in the current schema (hand-written comments are lost) and the graph is rebuilt. Legacy SDK dirs (boilerplates/, gofi-sdk-<lang>/, sdk-knowledge/) are removed."
	if force {
		mode = scaffold.InstallReset
		confirm = "--force: every managed file goes back to upstream, including the ones you edited. The replaced content is copied to .gofi/backup/ first. Memory (memory/), institutional/ and knowledge/ are still preserved."
	}

	if !autoConfirm {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return errors.New("update requires --yes when stdin is not a TTY")
		}
		ok := false
		if err := huh.NewConfirm().
			Title("Apply this update?").
			Description(confirm).
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
	sha, err := installFromSource(cfg.Project.Root, backendLang(cfg), uiSurfacesFromConfig(cfg), ref, sdkRef, data, mode)
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
	// The SDK checkout just moved, so the SDK scope of the graph describes code
	// that is no longer there. Rebuilding is keyed on the recorded SDK SHA, so
	// this is a no-op when the SDK did not actually change.
	syncProjectStandards(ctx, cfg)

	fmt.Printf("\nUpdate complete — .claude/ now at %s.\n", short(sha))
	// Reported after the install so the findings describe the project as it now
	// stands, not as it was a moment ago.
	printAudit(runAudit(cfg))
	return nil
}

// syncProjectStandards applies what an update can repair on its own: the config
// blocks a newer CLI writes, the .gitignore entry the graph depends on, the
// graph itself and the hooks that keep it in step with the code.
//
// Nothing here authors content. Document frontmatter and knowledge the team
// owns are left to the agents and reported by the audit instead — a repair that
// guesses at meaning is worse than a finding the user reads.
func syncProjectStandards(ctx context.Context, cfg *config.GofiConfig) {
	if note := syncConfig(cfg); note != "" {
		fmt.Println(note)
	}
	// A project scaffolded before the graph existed ignores all of .gofi/, which
	// would keep the graph out of git for good.
	if graphEnabled(cfg) {
		if err := ensureGofiIgnored(cfg.Project.Root); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update .gitignore: %v\n", err)
		}
	}
	if note := buildGraphQuietly(ctx, cfg, cfg.Project.Root); note != "" {
		fmt.Println(note)
	}
	if note := installGraphHooksQuietly(cfg, cfg.Project.Root); note != "" {
		fmt.Println(note)
	}
}

// syncConfig rewrites .gofi.yaml when it still carries an older schema or lacks
// blocks a current `gofi init` writes. Nothing the file already says is lost:
// Backfill only fills gaps, so a block the team turned off stays off, and every
// value written in an older shape is carried into the current one — the legacy
// notes name each move rather than letting it happen silently.
func syncConfig(cfg *config.GofiConfig) string {
	path := filepath.Join(cfg.Project.Root, config.FileName)
	// Load already migrated and backfilled the struct in memory, so the file
	// itself is the only thing that can say what the user is still missing.
	stale := config.FileVersion(path) < config.CurrentVersion
	missing := config.MissingBlocks(path)
	legacy := config.LegacyShapes(path)
	if !stale && len(missing) == 0 && len(legacy) == 0 {
		return ""
	}

	config.Backfill(cfg)
	if err := config.Save(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not rewrite %s: %v\n", config.FileName, err)
		return ""
	}

	note := fmt.Sprintf("Config: rewritten in schema v%d.", config.CurrentVersion)
	if len(missing) > 0 {
		note = fmt.Sprintf("Config: schema v%d, seeded missing block(s): %s.",
			config.CurrentVersion, strings.Join(missing, ", "))
	}
	for _, l := range legacy {
		note += "\n  " + l
	}
	return note + "\n  previous version kept in " + config.BackupDir + "/"
}

// runAudit inspects what the update itself never rewrites: .gofi.yaml, specs/,
// prd/ and the preserved .claude/knowledge/ tree.
func runAudit(cfg *config.GofiConfig) []audit.Finding {
	opts := audit.Options{GraphEnabled: graphEnabled(cfg)}
	// The upstream tree is what reveals which knowledge/shared files never
	// reached the project. It comes from the same cache the update just used,
	// so asking again is cheap; when it cannot be fetched the check is skipped
	// rather than guessed.
	if srcDir, _, err := fetchSource(cfg.Project.Root, cfg.Sources.Agents); err == nil {
		opts.Upstream = os.DirFS(srcDir)
		opts.UpstreamRoot = "."
	}
	return audit.Run(cfg.Project.Root, opts)
}

// printAudit renders the drift report grouped by area. It never fails the
// command: everything it reports is a migration the user chooses to do.
func printAudit(findings []audit.Finding) {
	fmt.Println()
	if len(findings) == 0 {
		fmt.Println("Structure audit: nothing to migrate.")
		return
	}

	var warns int
	for _, f := range findings {
		if f.Severity == audit.SeverityWarn {
			warns++
		}
	}
	fmt.Printf("Structure audit — %d finding(s), %d needing attention:\n", len(findings), warns)

	area := ""
	for _, f := range findings {
		if f.Area != area {
			area = f.Area
			fmt.Printf("\n  %s\n", area)
		}
		mark := "·"
		if f.Severity == audit.SeverityWarn {
			mark = "!"
		}
		fmt.Printf("    %s %s — %s\n", mark, f.Item, f.Detail)
		if f.Hint != "" {
			fmt.Printf("        %s\n", f.Hint)
		}
	}
	fmt.Println()
}

// printUpdatePlan renders the diff between upstream and the project's current
// .claude/ files. New + modified entries are listed; unchanged files are
// omitted. edited lists the managed files that carry local changes — under a
// normal run they are what the update will NOT touch, and under --force they
// are exactly what it is about to replace, which is the one thing the user
// needs to see before confirming.
func printUpdatePlan(plan []scaffold.Change, edited []string, force bool) {
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

	if len(edited) > 0 {
		if force {
			fmt.Printf("--force will overwrite %d file(s) you edited (copy kept in .gofi/backup/):\n\n", len(edited))
		} else {
			fmt.Printf("Kept as-is — %d file(s) carry your edits:\n\n", len(edited))
		}
		for _, f := range edited {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println()
	}

	fmt.Println("Knowledge listed above is only what the project is missing; existing files are never overwritten.")
	fmt.Println("Preserved whole: .claude/memory/ and .claude/institutional/.")
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

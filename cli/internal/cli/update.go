package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/audit"
	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
)

func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: i18n.T("cmd.update.short"),
		Long: `Everything a project can pull from upstream lives here, one target each:

  gofi update skills          the agents themselves, .claude/skills/
  gofi update sdk             the SDK checkout and .claude/sdk/<lang>/
  gofi update ds              the design system docs, .claude/sdk/<surface>/
  gofi update graph           the code graph and its git hooks
  gofi update institutional   the business base, from the org repo
  gofi update audit           report what drifted; change nothing

There is no target-less update, on purpose. "Update the project" is not a
decision anyone can review; "update the skills" is. Name what you mean.

Nothing else moves. After 'gofi init' the project is yours: .gofi.yaml,
CLAUDE.md, templates/, scripts/, knowledge/, memory/ and go.work are changed by
hand, in your own git. Every target prints what it will write, what it keeps
because you edited it, and what it leaves alone — then asks.

Drift no target can repair is reported by 'gofi update audit', each finding
naming the command that closes it, or saying plainly that none does.`,
		Example: `gofi update skills
gofi update skills --yes
gofi update sdk --force
gofi update graph
gofi update audit`,
	}
	cmd.AddCommand(
		newUpdateSkillsCmd(),
		newUpdateSDKCmd(),
		newUpdateDSCmd(),
		newUpdateGraphCmd(),
		newUpdateInstitutionalCmd(),
		newUpdateAuditCmd(),
	)
	return cmd
}

func newUpdateAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: i18n.T("cmd.update.audit.short"),
		Long: `Report where the project's structure lags behind what the current CLI writes,
and change nothing.

This is the other half of the update family: the targets fix what they own, and
this says what nobody owns. Config blocks added after the project was
scaffolded, documents the RAG protocol cannot index, missing INDEXes, knowledge
files the skills cite but the project never received, pre-v2.4 SDK dirs.

Every finding carries the command that closes it — or says plainly that no
command does and the fix is yours to make by hand.`,
		Example: `gofi update audit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuditOnly()
		},
	}
}

func newUpdateGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: i18n.T("cmd.update.graph.short"),
		Long: `Rebuild the code graph and reinstall the git hooks that keep it in step with
the code.

The graph is the one target that refreshes something nobody authored: it is
derived from the code, so a stale one is simply wrong, and the agents read it as
if it were current. That is also why it is the one target with nothing to
preserve.

It is rebuilt in the mode graph.deep declares, never forced to deep, and the
mode is printed: in fast an absent edge is not proof of an absent call, and that
is what the agents read it under. For the full toolbox — scoping, --deep,
queries, opening the report — use 'gofi graph'.`,
		Example: `gofi update graph
gofi update graph --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			return runUpdateGraph(cmd.Context(), yes)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	return cmd
}

func runUpdateGraph(ctx context.Context, autoConfirm bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	if !graphEnabled(cfg) {
		return errors.New("the graph is disabled for this project (graph.enabled in .gofi.yaml)")
	}

	ok, err := confirmUpdate("Rebuild the graph?", graphScope(cfg), autoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("graph left as it is.")
		return nil
	}
	syncGraph(ctx, cfg)
	noteDrift(cfg)
	return nil
}

// graphScope is what `gofi update graph` promises before it runs.
func graphScope(cfg *config.GofiConfig) updateScope {
	s := updateScope{LeavesAlone: []string{"everything else — the graph is derived, not authored"}}
	s.write(".gofi/graph/", "rebuilt from the code")
	if cfg.Graph.HooksOn() {
		s.write("git hooks", "pre-commit, post-checkout, post-merge")
	}
	s.write(".gitignore", "the .gofi/ rules the graph needs to reach git")
	return s
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

// syncGraph rebuilds the code graph and reinstalls the hooks that keep it in
// step with the code. Shared by `gofi update graph` and `gofi init`.
//
// Everything an update used to repair on its own (the .gofi.yaml schema, the
// legacy .claude/ dirs, go.work) is reported by `gofi update audit` instead,
// with the command that fixes it.
func syncGraph(ctx context.Context, cfg *config.GofiConfig) {
	// A project scaffolded before the graph existed ignores all of .gofi/, which
	// would keep the graph out of git for good — the one .gitignore line the
	// graph cannot do without.
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

// runAudit inspects what no update target rewrites: .gofi.yaml, specs/, prd/
// and the preserved .claude/knowledge/ tree.
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

// noteDrift closes a write target with one line saying how far the project has
// drifted — never the report itself.
//
// Splitting the update into targets cost the project the report that used to
// run at the end of every update, and drift nobody is told about is drift
// nobody fixes. This is the cheapest way back: a count, and the command that
// prints the rest. Silent when there is nothing to say, so a clean project
// never pays for the line.
//
// It runs the same checks `gofi update audit` prints, so the count can never
// disagree with the report it points at.
func noteDrift(cfg *config.GofiConfig) {
	if note := driftNote(runAudit(cfg)); note != "" {
		fmt.Println(note)
	}
}

func driftNote(findings []audit.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	var warns int
	for _, f := range findings {
		if f.Severity == audit.SeverityWarn {
			warns++
		}
	}
	if warns > 0 {
		return fmt.Sprintf("%d structure finding(s), %d needing attention — run `gofi update audit`.",
			len(findings), warns)
	}
	return fmt.Sprintf("%d structure finding(s) — run `gofi update audit`.", len(findings))
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
// skills. New + modified entries are listed; unchanged files are omitted.
// edited lists the skills that carry local changes — the evidence behind the
// KEEPS line of the scope block that follows.
func printUpdatePlan(plan []scaffold.Change, edited []string) {
	fmt.Println()
	if len(plan) == 0 {
		fmt.Println("No skill would change in .claude/skills/.")
	} else {
		fmt.Printf("The following %d skill file(s) would change:\n\n", len(plan))
		for _, c := range plan {
			fmt.Printf("  %-9s %s\n", c.Kind, c.RelPath)
		}
	}
	if len(edited) > 0 {
		fmt.Printf("\n%d skill file(s) carry your edits:\n\n", len(edited))
		for _, f := range edited {
			fmt.Printf("  %s\n", f)
		}
	}
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

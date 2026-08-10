package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
)

// newUpdateInstitutionalCmd is the target inside the update family. The old
// `gofi institutional update` still works (see newInstitutionalCmd) so nothing
// scripted against it breaks, but this is the one that shows in help.
func newUpdateInstitutionalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "institutional",
		Short: i18n.T("cmd.update.inst.short"),
		Long: `Fetch the institutional repo pinned at 'sources.institutional' in .gofi.yaml
and REPLACE .claude/institutional/<project.name>/ with its <project.name>/
folder.

The institutional base holds business knowledge — domain, actors, rules,
glossary, roadmap — and is the one target that keeps nothing: it is an
authoritative mirror, wiped and recopied every run, so local edits do not
survive. Business changes belong in the institutional repo, not in the project.

prd/, specs/ and memory/contexts/ are never touched — they belong to the
project's own git.

Errors when no institutional repo is configured; in that case the folder is
maintained by hand in this project.`,
		Example: `gofi update institutional
gofi update institutional --yes
gofi update institutional --ref github.com/org/institutional@v1.2.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			ref, _ := cmd.Flags().GetString("ref")
			return runInstitutionalUpdate(yes, ref)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().String("ref", "", "override the configured source ref for this run")
	return cmd
}

// newInstitutionalCmd is the pre-update-family spelling, kept working and
// hidden from help. Deprecated commands still run, which is the point: a script
// pinned to the old path keeps going and says so once.
func newInstitutionalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "institutional",
		Short:      i18n.T("cmd.institutional.short"),
		Deprecated: "use `gofi update institutional`.",
		Long: `The institutional base holds business/product knowledge (domain, actors,
rules, glossary, roadmap) under .claude/institutional/<project>/.

It is separate from the technical base updated by 'gofi update' (skills, SDK,
frameworks). When 'sources.institutional' is configured in .gofi.yaml, it points
at the org's shared repo and 'gofi institutional update' mirrors this product's
folder from there. When it is not configured, the folder is managed by hand in
this project's own git — there is nothing to pull.`,
		Example: `gofi institutional update
gofi institutional update --yes`,
	}
	cmd.AddCommand(newInstitutionalUpdateCmd())
	return cmd
}

func newInstitutionalUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: i18n.T("cmd.institutional.up.short"),
		// The leaf carries the notice too: cobra announces the deprecation of
		// the command actually executed, and this is the one people type.
		Deprecated: "use `gofi update institutional`.",
		Long: `Fetch the institutional repo pinned at 'sources.institutional' in .gofi.yaml
and REPLACE .claude/institutional/<project.name>/ with its <project.name>/ folder.

This is an authoritative mirror: the destination is wiped and recopied every
run, so local edits do not survive — contribute business changes to the
institutional repo, not to the project. prd/, specs/ and memory/contexts/ are
never touched (they belong to the project's own git).

Errors when no institutional repo is configured — in that case the folder is
maintained manually in this project.`,
		Example: `gofi institutional update
gofi institutional update --yes
gofi institutional update --ref github.com/org/institutional@v1.2.0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			ref, _ := cmd.Flags().GetString("ref")
			return runInstitutionalUpdate(yes, ref)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().String("ref", "", "override the configured source ref for this run")
	return cmd
}

func runInstitutionalUpdate(autoConfirm bool, refOverride string) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}

	ref := cfg.Sources.Institutional
	if refOverride != "" {
		ref = refOverride
	}
	if ref == "" {
		return errors.New("no institutional repo configured (sources.institutional in .gofi.yaml is empty).\n" +
			"This project's .claude/institutional/ is managed by hand in its own git.\n" +
			"To wire an org repo, set sources.institutional and re-run, or pass --ref.")
	}

	root := projectRootFromCfg(cfg)
	name := cfg.Project.Name
	if name == "" {
		return errors.New("project.name is empty in .gofi.yaml; cannot resolve the institutional folder")
	}

	fmt.Printf("Resolving %s …\n", ref)
	dir, resolved, err := fetchSource(root, ref)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", ref, err)
	}

	current := readInstalledInstitutionalSha(root)
	if current == resolved.Ref && resolved.Ref != "local" {
		fmt.Printf("Already up to date (sha=%s).\n", short(current))
		return nil
	}
	if current == "" {
		fmt.Printf("No previous institutional install recorded; will install %s.\n", short(resolved.Ref))
	} else {
		fmt.Printf("Update available: %s → %s\n", short(current), short(resolved.Ref))
	}

	scope := updateScope{
		Replaces: true,
		LeavesAlone: []string{
			"prd/", "specs/", "memory/contexts/", ".claude/skills/",
			".claude/sdk/", ".gofi.yaml", "the graph",
		},
	}
	scope.write(".claude/institutional/"+name+"/", "full wipe + recopy of "+name+"/ from the repo")

	ok, err := confirmUpdate("Replace the institutional folder?", scope, autoConfirm)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("institutional update cancelled.")
		return nil
	}

	data := scaffold.TemplateData{
		ProjectName: name,
		Date:        time.Now().Format("2006-01-02"),
	}
	created, err := scaffold.InstallInstitutionalMirror(os.DirFS(dir), name, root, name, data)
	if err != nil {
		if errors.Is(err, scaffold.ErrNoInstitutionalSubdir) {
			return fmt.Errorf("the institutional repo %s has no %q/ folder — "+
				"the multi-product layout expects a folder named after project.name", ref, name)
		}
		return fmt.Errorf("mirror institutional: %w", err)
	}

	if resolved.Ref != "local" {
		if err := writeInstalledInstitutionalSha(root, resolved.Ref); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not record institutional SHA: %v\n", err)
		}
	}

	fmt.Printf("\nInstitutional base updated — %d file(s) in .claude/institutional/%s/ (now at %s).\n",
		len(created), name, short(resolved.Ref))
	fmt.Println("Review and commit the changes to your project git.")
	noteDrift(cfg)
	return nil
}

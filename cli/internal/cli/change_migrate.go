package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

func newChangeMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: i18n.T("cmd.change.migrate.short"),
		Long: `Move each open change that touches one context from specs/changes/<id>/ to
its context's folder, specs/<context>/changes/<id>/ — where gofi change new puts
it. A change of several contexts stays in specs/changes/. The seal is the hash
of each document's body, so a sealed change stays sealed. A versioned folder is
moved with git mv; a change a run is still working on is left where it is.

--genesis also moves the first change of a context that has no spec yet, made
before gofi wrote it as the context's genesis, into the context's own folder:
specs/<context>/proposal.md, deltas/, design.md. Its id becomes the context's
name — gofi spec check, review and approve take that one from then on —, so a
change another open change depends on by its id is left where it is.`,
		Example: `gofi change migrate --dry-run
gofi change migrate
gofi change migrate --genesis --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			dry, _ := cmd.Flags().GetBool("dry-run")
			genesis, _ := cmd.Flags().GetBool("genesis")
			return migrateChanges(cmd.OutOrStdout(), root, dry, genesis)
		},
	}
	cmd.Flags().Bool("dry-run", false, i18n.T("cmd.change.migrate.flag.dry_run"))
	cmd.Flags().Bool("genesis", false, i18n.T("cmd.change.migrate.flag.genesis"))
	return cmd
}

func migrateChanges(out interface{ Write([]byte) (int, error) }, root string, dry, genesis bool) error {
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	busy := map[string]bool{}
	if runs, err := loop.List(root); err == nil {
		for _, r := range runs {
			if r.State.Node != "" {
				busy[r.State.Change] = true
			}
		}
	}
	changes := contract.Changes(all)
	dependedOn := map[string]bool{}
	for _, c := range changes {
		for _, dep := range c.DependsOn {
			dependedOn[dep] = true
		}
	}
	moved := 0
	for _, c := range changes {
		if len(c.Contexts) != 1 {
			continue
		}
		from, ok := contract.FindChange(root, c.ID)
		if !ok || contract.IsGenesis(from) {
			continue // a genesis is where it belongs
		}
		if genesis && contract.Genesis(root, c.Contexts) {
			to := contract.GenesisDir(c.Contexts[0])
			switch {
			case busy[c.ID]:
				fmt.Fprintln(out, "  "+i18n.T("change.migrate.busy", c.ID))
			case dependedOn[c.ID]:
				fmt.Fprintln(out, "  "+i18n.T("change.migrate.depended", c.ID))
			default:
				if !dry {
					if err := moveInto(root, from, to); err != nil {
						return fmt.Errorf("%s → %s: %w", from, to, err)
					}
				}
				moved++
				fmt.Fprintln(out, "  "+i18n.T("change.migrate.genesis", from, to, c.Contexts[0]))
			}
			continue
		}
		to := contract.NewChangeDir(c.ID, c.Contexts)
		if from == to {
			continue
		}
		switch {
		case busy[c.ID]:
			fmt.Fprintln(out, "  "+i18n.T("change.migrate.busy", c.ID))
			continue
		case exists(filepath.Join(root, filepath.FromSlash(to))):
			fmt.Fprintln(out, "  "+i18n.T("change.migrate.taken", c.ID, to))
			continue
		}
		if !dry {
			if err := moveDir(root, from, to); err != nil {
				return fmt.Errorf("%s → %s: %w", from, to, err)
			}
		}
		moved++
		fmt.Fprintln(out, "  "+i18n.T("change.migrate.moved", from, to))
	}
	if moved == 0 {
		fmt.Fprintln(out, "  "+i18n.T("change.migrate.none"))
	} else if !dry {
		fmt.Fprintln(out, "  "+i18n.T("change.migrate.next"))
	}
	return nil
}

// moveDir moves a folder of the project, by git when git versions it so the
// history follows the files.
func moveDir(root, from, to string) error {
	dst := filepath.Join(root, filepath.FromSlash(to))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if tracked, _ := gitOut(root, "ls-files", "--", from); len(strings.TrimSpace(string(tracked))) > 0 {
		cmd := exec.Command("git", "mv", from, to)
		cmd.Dir = root
		if b, err := cmd.CombinedOutput(); err != nil {
			return errors.New(strings.TrimSpace(string(b)))
		}
		return nil
	}
	return os.Rename(filepath.Join(root, filepath.FromSlash(from)), dst)
}

// moveInto moves what a folder holds into another, entry by entry, and
// removes the folder — and its changes/ parent, when nothing is left in it.
func moveInto(root, from, to string) error {
	src := filepath.Join(root, filepath.FromSlash(from))
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if exists(filepath.Join(root, filepath.FromSlash(to), e.Name())) {
			return fmt.Errorf("%s/%s já existe", to, e.Name())
		}
	}
	for _, e := range entries {
		if err := moveDir(root, from+"/"+e.Name(), to+"/"+e.Name()); err != nil {
			return err
		}
	}
	_ = os.Remove(src)
	_ = os.Remove(filepath.Dir(src)) // changes/, if empty
	return nil
}

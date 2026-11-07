package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

func newChangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "change",
		Short: i18n.T("cmd.change.short"),
		Long: `Changes are how the contract moves (decision 0003): a folder under
specs/<context>/changes/<id>/ — or specs/changes/<id>/ when it touches several
contexts — with a proposal, a delta per context it touches and the
design of how. A new context is a change that only adds, written in the
context's own folder (its genesis, named as the context); an existing one is
altered by a change, never by editing its living spec. Approved, the change is
what the loop implements; accepted, it is archived and its deltas become the
living specs.`,
		Example: `gofi change new adicionar-rate-limit --context pedidos
gofi change list
gofi change approve adicionar-rate-limit
gofi change archive adicionar-rate-limit`,
	}
	cmd.AddCommand(newChangeNewCmd(), newChangeListCmd(), newChangeApproveCmd(), newChangeArchiveCmd(), newChangeMigrateCmd())
	return cmd
}

func newChangeNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <id> --context <ctx>...",
		Short: i18n.T("cmd.change.new.short"),
		Long: `Create the folder of a change from the project's templates — in the
context's folder, specs/<ctx>/changes/<id>/, when it touches one context, and
in specs/changes/<id>/ when it touches several: proposal.md, a deltas/<ctx>.md
for each context, and
design.md from the design template of the backend language's pack (the neutral
one when the pack has none).

The id is kebab-case and starts with a verb: adicionar-rate-limit,
cancelar-pedido-enviado. It must be new — no open or archived change has it.
--no-design declares design: n/a in the proposal and writes no design, for a
change that touches no code.

The first change of a context that has no spec yet is its genesis: it is
written in the context's own folder — specs/<ctx>/proposal.md, deltas/<ctx>.md,
design.md —, with no changes/<id>/ around it, and its id is the context's name
(the id given is its title). Archived, the spec takes its place and the change
goes to specs/<ctx>/changes/archive/. --no-genesis keeps it in
specs/<ctx>/changes/<id>/ as any other change.

The folder is a skeleton: the proposal, deltas and design are then written,
checked (gofi spec check <id>) and approved (gofi change approve <id>).`,
		Example: `gofi change new criar-pedidos --context pedidos
gofi change new adicionar-rate-limit --context pedidos --context clientes
gofi change new remover-desconto-manual --context pedidos --no-design`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			if err := checkAgentsSkew(root); err != nil {
				return err
			}
			contexts, _ := cmd.Flags().GetStringArray("context")
			noDesign, _ := cmd.Flags().GetBool("no-design")
			if all, err := contract.Docs(root); err == nil {
				for _, ctx := range contexts {
					if contract.FormatTwo(all, ctx) {
						return errors.New(i18n.T("change.new.format_two", ctx, ctx))
					}
				}
			}
			noGenesis, _ := cmd.Flags().GetBool("no-genesis")
			nc := contract.NewChange{
				ID: args[0], Contexts: contexts, Language: backendLanguage(cfg),
				NoDesign: noDesign, NoGenesis: noGenesis, Date: time.Now().Format("2006-01-02"),
			}
			id, dir := nc.Placed(root)
			written, err := contract.Create(root, nc)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, p := range written {
				fmt.Fprintln(out, "  "+i18n.T("change.new.wrote", p))
			}
			if contract.IsGenesis(dir) {
				fmt.Fprintln(out, "\n  "+i18n.T("change.new.genesis", contexts[0], dir, id))
			}
			fmt.Fprintln(out, "\n  "+i18n.T("change.new.next", id))
			return nil
		},
	}
	cmd.Flags().StringArray("context", nil, i18n.T("cmd.change.flag.context"))
	cmd.Flags().Bool("no-design", false, i18n.T("cmd.change.flag.no_design"))
	cmd.Flags().Bool("no-genesis", false, i18n.T("cmd.change.flag.no_genesis"))
	return cmd
}

func newChangeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: i18n.T("cmd.change.list.short"),
		Long: `The open changes: their status, the contexts they touch, whether they are
sealed — every document approved as it is now — and their folder. A change edited after its
approval lists the documents whose approval no longer holds.`,
		Example: `gofi change list
gofi change list --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			all, err := contract.Docs(root)
			if err != nil {
				return err
			}
			changes := contract.Changes(all)
			out := cmd.OutOrStdout()
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(changes)
			}
			defer printChangeHazards(out, changeHazards(root, changes))
			if len(changes) == 0 {
				fmt.Fprintln(out, "  "+i18n.T("change.none"))
				return nil
			}
			for _, c := range changes {
				seal := i18n.T("change.unsealed")
				switch {
				case c.Sealed:
					seal = i18n.T("change.sealed")
				case len(c.Stale) > 0:
					seal = i18n.T("change.stale", strings.Join(c.Stale, ", "))
				}
				line := fmt.Sprintf("  %s  %s · %s · %s", c.ID, c.Status, strings.Join(c.Contexts, ", "), seal)
				if dir, ok := contract.FindChange(root, c.ID); ok {
					line += " · " + dir + "/"
				}
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.change.flag.json"))
	return cmd
}

func newChangeApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <id>",
		Short: i18n.T("cmd.change.approve.short"),
		Long: `Approve a change: check every document of it (gofi spec check <id>), then seal
the proposal, the deltas and the design — status: aprovado, the version
approved and the hash of each body. Tasks are derived and never sealed.

All or nothing: when any document has an error, none is approved. Editing a
document afterwards breaks its seal, and the loop will not implement the
change until it is approved again.`,
		Example: `gofi change approve adicionar-rate-limit`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			all, err := contract.Docs(root)
			if err != nil {
				return err
			}
			var docs []contract.Doc
			for _, d := range all {
				if d.Change == args[0] {
					docs = append(docs, d)
				}
			}
			if len(docs) == 0 {
				return errors.New(i18n.T("change.not_found", args[0]))
			}
			return approveDocs(cmd, root, all, docs)
		},
	}
}

func newChangeArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <id>",
		Short: i18n.T("cmd.change.archive.short"),
		Long: `Apply an approved change to the living specs and archive it. Each delta
becomes its context's spec: requirements ADICIONADO after the last one,
MODIFICADO replaced in place, REMOVIDO taken out; a context with no spec yet
gets one from the delta's Propósito. The version goes up one minor step, the
spec is sealed again — it is the mechanical application of what the person
approved — and the folder moves to the archive/ beside it, <date>-<id>/, with the
proposal marked arquivado.

No model is involved, and nothing is written unless all of it can be: a change
that is not sealed as it is, does not pass gofi spec check, or would leave a
spec that does not, is refused untouched.

In the autonomous loop the executor archives a change when it accepts the
delivery; run by hand, it is the step after the delivery is accepted.`,
		Example: `gofi change archive adicionar-rate-limit`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			all, err := contract.Docs(root)
			if err != nil {
				return err
			}
			res, err := contract.Archive(root, all, args[0], time.Now().Format("2006-01-02"))
			if err != nil {
				return &ExitCodeError{Code: 1, Err: err}
			}
			out := cmd.OutOrStdout()
			for _, p := range res.Specs {
				fmt.Fprintln(out, "  "+i18n.T("change.archive.spec", p))
			}
			fmt.Fprintln(out, "  "+i18n.T("change.archive.moved", res.Dir))
			return nil
		},
	}
}

// approveDocs checks the documents and, when none has an error, seals them
// all. It is how a spec and a change are approved alike.
func approveDocs(cmd *cobra.Command, root string, all, docs []contract.Doc) error {
	if err := personApproves(cmd.InOrStdin(), cmd.OutOrStdout(), docs); err != nil {
		return err
	}
	return approveDocsTo(cmd.OutOrStdout(), root, all, docs)
}

// approveDocsTo checks every document and, only when none blocks, seals
// them all — what gofi change approve, gofi spec approve and the MCP approve
// tool do.
func approveDocsTo(out io.Writer, root string, all, docs []contract.Doc) error {
	placeholders, _ := contract.Placeholders(root)
	blocked := 0
	for _, d := range docs {
		for _, f := range contract.Blocking(root, all, d, placeholders) {
			fmt.Fprintf(out, "  %s %s: %s\n", i18n.T("index.check.error"), f.Path, f.Message)
			blocked++
		}
	}
	if blocked > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("spec.approve.refused", blocked))}
	}
	for _, d := range docs {
		wrote, err := contract.Approve(root, d)
		if err != nil {
			return err
		}
		key := "spec.approve.done"
		if !wrote {
			key = "spec.approve.already"
		}
		fmt.Fprintln(out, "  "+i18n.T(key, d.Path, contract.Seal(d.Body)[:15]))
	}
	return nil
}

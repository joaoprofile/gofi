package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

func newSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: i18n.T("cmd.spec.short"),
		Long: `The specs as the contract the loop implements: whether each one is ready to
be approved. A spec that does not pass is a spec the implementation would have
to guess at, so this runs before any model reads it — and costs no tokens.`,
		Example: `gofi spec check
gofi spec check pedidos
gofi spec approve pedidos
gofi spec order`,
	}
	cmd.AddCommand(newSpecCheckCmd(), newSpecApproveCmd(), newSpecOrderCmd(), newSpecMigrateCmd())
	return cmd
}

func newSpecCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [<context|change|path>...]",
		Short: i18n.T("cmd.spec.check.short"),
		Long: `Check whether the contract is ready to be approved: the living specs and the
open changes. With no argument, everything under specs/; a context takes its
living spec and every open change that touches it; a change id takes that
change; a path takes that file.

Format 3 (living specs and changes, decision 0003):
  spec      ## Propósito and ## Requisitos; each requirement
            "### Requisito: REQ-NN — título", with a DEVE, and at least one
            "#### Cenário: REQ-NN.n — título" with DADO, QUANDO and ENTÃO;
            ids unique; a code block is a warning; a living spec edited by
            hand after its approval is an error.
  proposal  ## Por quê and ## O que muda; a delta for each context it names
            and none for one it does not; a design when it adds or modifies
            requirements (or design: n/a); depende_de on changes that exist.
  delta     ADICIONADO only for ids the living spec lacks — and no other open
            change adds; MODIFICADO and REMOVIDO only for ids it has.
  design    ## Abordagem and ## Mapa de requisitos with every requirement the
            change adds or modifies; no requirement cited that does not exist.
Any [PRECISA ESCLARECER: …] outside code is an error.

Format 2 (sdd-<ctx>.md): sections 1, 4 and 5 by number, and RN-xx rules with
a description — the rules it was written under, until it is migrated.

Every document: a placeholder of a template left in, an open marker (TODO,
TBD, FIXME, ???) outside code, and the corpus findings gofi index check
reports for the same file. A stale approval is an error; an approved status
with no seal, a warning.

Exits non-zero when there is an error, for CI and for the loop.`,
		Example: `gofi spec check
gofi spec check pedidos
gofi spec check adicionar-rate-limit
gofi spec check specs/pedidos/spec.md`,
		RunE: runSpecCheck,
	}
}

func runSpecCheck(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	specs, missing := contract.Select(all, args)
	if len(missing) > 0 {
		return errors.New(i18n.T("spec.not_found", strings.Join(missing, ", ")))
	}
	if len(specs) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("spec.none"))
		return nil
	}

	findings := contract.CheckAll(root, all, specs)
	for _, f := range findings {
		kind := i18n.T("index.check.warn")
		if f.Error {
			kind = i18n.T("index.check.error")
		}
		fmt.Fprintf(out, "  %s %s: %s\n", kind, f.Path, f.Message)
	}
	errs := contract.Errors(findings)
	fmt.Fprintln(out, "\n  "+i18n.T("spec.check.summary", len(specs), errs, len(findings)-errs))
	if errs > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("spec.check.failed", errs))}
	}
	return nil
}

func newSpecApproveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve <context|path>... | --all",
		Short: i18n.T("cmd.spec.approve.short"),
		Long: `Approve specs as the contract the loop implements. Each one is checked first
(gofi spec check); then its frontmatter gets status: aprovado, the version
approved (aprovacao_versao) and the hash of its body (aprovacao_sha). No name
and no date: who approved and when is in git.

The hash is the seal. Editing the body afterwards breaks it, and the spec is
no longer approved: gofi spec check and gofi index check say so, and the loop
will not implement it until it is approved again. Changing only the
frontmatter does not break it; changing versao does.

All or nothing: when any spec named has an error, none is approved.`,
		Example: `gofi spec approve pedidos
gofi spec approve pedidos clientes
gofi spec approve --all`,
		RunE: runSpecApprove,
	}
	cmd.Flags().Bool("all", false, i18n.T("cmd.spec.flag.all"))
	return cmd
}

func runSpecApprove(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all == (len(args) > 0) {
		return errors.New(i18n.T("spec.approve.which"))
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	corpus, err := contract.Docs(root)
	if err != nil {
		return err
	}
	specs, missing := contract.Select(corpus, args)
	if len(missing) > 0 {
		return errors.New(i18n.T("spec.not_found", strings.Join(missing, ", ")))
	}
	if len(specs) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("spec.none"))
		return nil
	}

	return approveDocs(cmd, root, corpus, specs)
}

func newSpecOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: i18n.T("cmd.spec.order.short"),
		Long: `The order the contexts are built in, from the dependencies their specs declare
(depende_de: [clientes] in the frontmatter). Each level depends only on the
levels before it, so the contexts of one level can be built side by side;
within a level they are sorted by name, so the same specs always give the same
order. The open changes follow, in levels by the depende_de of their
proposals; a dependency on an archived change is met.

The order comes from declared dependencies only. Two contexts describing the
same table say nothing about which one comes first — gofi spec check warns
about them instead. A cycle has no order: it is an error, and the command
exits 1.`,
		Example: `gofi spec order
gofi spec order --json`,
		Args: cobra.NoArgs,
		RunE: runSpecOrder,
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.spec.flag.json"))
	return cmd
}

func runSpecOrder(cmd *cobra.Command, _ []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	specs, err := contract.Docs(root)
	if err != nil {
		return err
	}
	levels, err := contract.Order(specs)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	changes, err := contract.ChangeOrder(specs)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if levels == nil {
			levels = [][]string{}
		}
		if changes == nil {
			changes = [][]string{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"levels": levels, "depends_on": contract.Deps(specs), "changes": changes})
	}
	if len(levels) == 0 && len(changes) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("spec.none"))
		return nil
	}
	defer func() {
		if len(changes) == 0 {
			return
		}
		fmt.Fprintln(out, "\n  "+i18n.T("spec.order.changes"))
		for i, level := range changes {
			fmt.Fprintf(out, "  %s\n", i18n.T("spec.order.level", i))
			for _, c := range level {
				fmt.Fprintf(out, "    %s\n", c)
			}
		}
	}()
	deps := contract.Deps(specs)
	for i, level := range levels {
		fmt.Fprintf(out, "  %s\n", i18n.T("spec.order.level", i))
		for _, c := range level {
			if d := deps[c]; len(d) > 0 {
				fmt.Fprintf(out, "    %s  ← %s\n", c, strings.Join(d, ", "))
			} else {
				fmt.Fprintf(out, "    %s\n", c)
			}
		}
	}
	return nil
}

func newSpecMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate <context>... | --all",
		Short: i18n.T("cmd.spec.migrate.short"),
		Long: `Turn the format-2 specs of a context (sdd-<ctx>.md) into its living spec,
specs/<ctx>/spec.md, in format 3 (decision 0003). Deterministic, no model:

  - §1 becomes ## Propósito;
  - each RN-NN of §5 becomes REQ-NN with its text — the same number when the
    context had one spec, renumbered in order when it had several (the map is
    printed) — without the implementation notes the rule carried;
  - each requirement gets a scenario marked [PRECISA ESCLARECER], since a
    format-2 rule had none: the spec does not pass gofi spec check until a
    person writes them, and that is the point;
  - the old documents are the how, already in the code: they move whole to
    specs/changes/archive/<date>-baseline-<ctx>/, marked arquivado;
  - the context's memory points at the new spec.

The living spec is born proposto. Write the scenarios (the gofi-migrate skill
drafts them from the code and its tests), then gofi spec check and gofi spec
approve.`,
		Example: `gofi spec migrate pedidos
gofi spec migrate --all`,
		RunE: runSpecMigrate,
	}
	cmd.Flags().Bool("all", false, i18n.T("cmd.spec.flag.migrate_all"))
	return cmd
}

func runSpecMigrate(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all == (len(args) > 0) {
		return errors.New(i18n.T("spec.migrate.which"))
	}
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	corpus, err := contract.Docs(root)
	if err != nil {
		return err
	}
	contexts := args
	if all {
		seen := map[string]bool{}
		for _, d := range corpus {
			if d.Kind == contract.KindSpec && d.Change == "" && d.Format < contract.FormatSplit && !seen[d.Context] {
				seen[d.Context] = true
				contexts = append(contexts, d.Context)
			}
		}
	}
	out := cmd.OutOrStdout()
	if len(contexts) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("spec.migrate.none"))
		return nil
	}
	date := time.Now().Format("2006-01-02")
	for _, ctx := range contexts {
		res, err := contract.Migrate(root, corpus, ctx, date)
		if err != nil {
			return &ExitCodeError{Code: 1, Err: err}
		}
		fmt.Fprintln(out, "  "+i18n.T("spec.migrate.done", res.Spec, res.Archive))
		for _, old := range sortedKeys(res.Renamed) {
			fmt.Fprintf(out, "    %s → %s\n", old, res.Renamed[old])
		}
		fmt.Fprintln(out, "  "+i18n.T("spec.migrate.next", res.Pending, ctx))
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

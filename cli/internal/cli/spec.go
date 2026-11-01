package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	cmd.AddCommand(newSpecCheckCmd(), newSpecApproveCmd(), newSpecOrderCmd())
	return cmd
}

func newSpecCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [<context|path>...]",
		Short: i18n.T("cmd.spec.check.short"),
		Long: `Check whether specs are ready to be approved. With no argument, every spec in
specs/; a context name takes all of its specs; a path takes that file.

Errors block the approval: a required section missing (1 Overview, 4
Operations, 5 Business Rules, matched by number), a business rule RN-xx
declared twice or left without a description, a placeholder of the template
left in ({Contexto}, {Nome da Regra}), an open marker (TODO, TBD, FIXME, ???)
outside a code block, and the corpus findings gofi index check reports for the
same file. A §5 with no RN-xx is a warning: nothing for the loop to trace.

An approved spec that was edited afterwards is an error too: its approval no
longer holds (see gofi spec approve). An approved status with no seal is a
warning.

Exits non-zero when there is an error, for CI and for the loop.`,
		Example: `gofi spec check
gofi spec check pedidos
gofi spec check specs/pedidos/sdd-pedidos.md`,
		RunE: runSpecCheck,
	}
}

func runSpecCheck(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	all, err := contract.Specs(root)
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
	corpus, err := contract.Specs(root)
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

	placeholders, _ := contract.Placeholders(root)
	blocked := 0
	for _, s := range specs {
		for _, f := range contract.Blocking(root, corpus, s, placeholders) {
			fmt.Fprintf(out, "  %s %s: %s\n", i18n.T("index.check.error"), f.Path, f.Message)
			blocked++
		}
	}
	if blocked > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("spec.approve.refused", blocked))}
	}
	for _, s := range specs {
		wrote, err := contract.Approve(root, s)
		if err != nil {
			return err
		}
		key := "spec.approve.done"
		if !wrote {
			key = "spec.approve.already"
		}
		fmt.Fprintln(out, "  "+i18n.T(key, s.Path, contract.Seal(s.Body)[:15]))
	}
	return nil
}

func newSpecOrderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "order",
		Short: i18n.T("cmd.spec.order.short"),
		Long: `The order the contexts are built in, from the dependencies their specs declare
(depende_de: [clientes] in the frontmatter). Each level depends only on the
levels before it, so the contexts of one level can be built side by side;
within a level they are sorted by name, so the same specs always give the same
order.

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
	specs, err := contract.Specs(root)
	if err != nil {
		return err
	}
	levels, err := contract.Order(specs)
	if err != nil {
		return &ExitCodeError{Code: 1, Err: err}
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if levels == nil {
			levels = [][]string{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"levels": levels, "depends_on": contract.Deps(specs)})
	}
	if len(levels) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("spec.none"))
		return nil
	}
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

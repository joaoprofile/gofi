package cli

import (
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
gofi spec check pedidos`,
	}
	cmd.AddCommand(newSpecCheckCmd())
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

	findings := contract.CheckAll(root, specs)
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

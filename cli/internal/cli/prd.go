package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

func newPRDCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prd",
		Short: i18n.T("cmd.prd.short"),
		Long: `The PRD is what a discovery leaves and what the specification is written
from. This is the discovery's sensor: it reads the PRD and says, with no model
asked, whether it is ready for that.`,
		Example: `gofi prd check
gofi prd check pedidos`,
	}
	cmd.AddCommand(newPRDCheckCmd())
	return cmd
}

func newPRDCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [<context>...]",
		Short: i18n.T("cmd.prd.check.short"),
		Long: `Check whether a PRD is ready for the specification. With no argument, every
PRD under prd/; a context takes its own (prd/<context>/prd-<context>.md).

An error asks for a correction before the contract is written from the PRD:
  a section of the template missing (1 to 7, by number)
  a piece of the template left as it came, a [PRECISA ESCLARECER: …] or
  another open marker (TODO, TBD, FIXME, ???) outside code
  the problem not said; no metric with today, the goal and how to measure;
  no persona; nothing in "Dentro" or nothing in "Fora"; no journey with steps
  a requirement or a scenario written in it — those are born in the change

A warning is worth reading: "o sistema deve…" in the text, and no
restriction, premise or risk described.

What a PRD says is judged in the current format only: of an older one, its
header — the fields and the lexicon gofi index check reads — and no more.
Exits non-zero when there is an error, for CI.`,
		Example: `gofi prd check
gofi prd check pedidos`,
		RunE: runPRDCheck,
	}
}

func runPRDCheck(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	paths := contract.PRDs(root)
	if len(args) > 0 {
		paths = paths[:0:0]
		for _, ctx := range args {
			paths = append(paths, contract.PRDPath(ctx))
		}
	}
	checked, older, errs, warns := 0, 0, 0, 0
	var missing []string
	for _, p := range paths {
		findings, state := contract.CheckPRD(root, p)
		if state == contract.PRDMissing {
			if len(args) > 0 {
				missing = append(missing, p)
			}
			continue
		}
		checked++
		if state == contract.PRDOlder {
			older++
		}
		for _, f := range findings {
			kind := i18n.T("index.check.warn")
			if f.Error {
				kind = i18n.T("index.check.error")
			}
			fmt.Fprintf(out, "  %s %s: %s\n", kind, f.Path, f.Message)
		}
		e, w := contract.PRDSummary(findings)
		errs, warns = errs+e, warns+w
	}
	if len(missing) > 0 {
		return errors.New(i18n.T("prd.not_found", strings.Join(missing, ", ")))
	}
	if checked == 0 {
		fmt.Fprintln(out, "  "+i18n.T("prd.none"))
		return nil
	}
	summary := i18n.T("prd.check.summary", checked, errs, warns)
	if older > 0 {
		// Said in the summary, not PRD by PRD: nothing in them is wrong.
		summary += " | " + i18n.T("prd.check.older", older)
	}
	fmt.Fprintln(out, "  "+summary)
	if errs > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("prd.check.failed", errs))}
	}
	return nil
}

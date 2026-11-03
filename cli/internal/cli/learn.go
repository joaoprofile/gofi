package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/learn"
)

func newLearnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "learn",
		Short: i18n.T("cmd.learn.short"),
		Long: `The lessons the runs proposed: what a run had to fix — findings the audit
raised, stages the sensors refused —, written by gofi-learn as the knowledge
file each would become. A lesson is only a proposal: accept writes it into
the project's knowledge, where every role reads it, never over a file the team
already has; reject keeps that it was declined. Without a subcommand, lists
the pending ones.`,
		Example: `gofi learn
gofi learn accept r-20261103-150857 1
gofi learn reject r-20261103-150857 2`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			all, err := learn.All(root)
			if err != nil {
				return err
			}
			out, pending := cmd.OutOrStdout(), 0
			for _, l := range all {
				for i, ls := range l.Lessons {
					if ls.Status != learn.Pending {
						continue
					}
					pending++
					fmt.Fprintf(out, "\n  %s %d — knowledge/%s/%s.md\n    %s\n    evidência: %s\n", l.Run, i+1, ls.Scope, ls.Name, ls.Title, ls.Evidence)
				}
			}
			if pending == 0 {
				fmt.Fprintln(out, "  "+i18n.T("learn.none"))
				return nil
			}
			fmt.Fprintln(out, "\n  "+i18n.T("learn.how"))
			return nil
		},
	}
	decide := func(use string, accept bool) *cobra.Command {
		return &cobra.Command{
			Use:  use + " <run> <n>",
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				root, err := findProjectRoot()
				if err != nil {
					return err
				}
				n, err := strconv.Atoi(args[1])
				if err != nil {
					return errors.New(i18n.T("learn.which"))
				}
				l, err := learn.Load(root, args[0])
				if err != nil {
					return err
				}
				if !accept {
					if err := learn.Reject(root, l, n-1); err != nil {
						return err
					}
					fmt.Fprintln(cmd.OutOrStdout(), "  "+i18n.T("learn.rejected", args[0], n))
					return nil
				}
				rel, err := learn.Accept(root, l, n-1)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "  "+i18n.T("learn.accepted", rel))
				return nil
			},
		}
	}
	accept, reject := decide("accept", true), decide("reject", false)
	accept.Short, reject.Short = i18n.T("cmd.learn.accept.short"), i18n.T("cmd.learn.reject.short")
	accept.Long = `Write a pending lesson into the project's knowledge: knowledge/<scope>/<name>.md,
with its keywords and what it overrides. A file the team already has there is
never written over — edit that one, or reject the lesson. The run and the
number are the ones gofi learn shows.`
	accept.Example = "gofi learn accept r-20261103-150857 1"
	reject.Long = `Decline a pending lesson: it stays in the run's proposals as declined, and
nothing is written.`
	reject.Example = "gofi learn reject r-20261103-150857 2"
	cmd.AddCommand(accept, reject)
	return cmd
}

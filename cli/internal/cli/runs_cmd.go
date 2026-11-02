package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: i18n.T("cmd.ps.short"),
		Long: `The runs of the loop in this project, newest first: the change, the node each
one is at, the iteration, what it has cost and whether a supervisor is carrying
it — running, interrupted (its supervisor is gone: resume it), stopped at a
node, or ended with its outcome.`,
		Example: `gofi ps`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			runs, err := loop.List(root)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(runs) == 0 {
				fmt.Fprintln(out, "  "+i18n.T("ps.none"))
				return nil
			}
			fmt.Fprintf(out, "  %-20s %-24s %-14s %-12s %4s %8s\n", "RUN", "MUDANÇA", "ESTADO", "NÓ", "ITER", "US$")
			for _, r := range runs {
				where := r.State.Node
				state := r.Liveness
				if state == loop.Ended {
					state, where = r.State.Outcome, "—"
				}
				fmt.Fprintf(out, "  %-20s %-24s %-14s %-12s %4d %8.3f\n", r.State.Run, r.State.Change, state, where, r.State.Iter, r.State.CostUSD)
			}
			return nil
		},
	}
}

func newLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <run>",
		Short: i18n.T("cmd.logs.short"),
		Long: `The events of a run: each node it started, the edge it took and why, how it
ended. -f keeps following until the run ends or its supervisor is gone. What
each agent did is in .gofi/runs/<run>/agents/.`,
		Example: `gofi logs r-20261102-101500
gofi logs r-20261102-101500 -f`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			follow, _ := cmd.Flags().GetBool("follow")
			return followRun(cmd.OutOrStdout(), root, args[0], follow, 500*time.Millisecond)
		},
	}
	cmd.Flags().BoolP("follow", "f", false, i18n.T("cmd.logs.flag.follow"))
	return cmd
}

// followRun prints a run's events, and with follow the ones still to come.
func followRun(out io.Writer, root, run string, follow bool, every time.Duration) error {
	if _, err := loop.LoadState(root, run); err != nil {
		return err
	}
	printed := 0
	for {
		evs, _ := loop.Events(root, run)
		for _, e := range evs[printed:] {
			line := fmt.Sprintf("  %s  %-12s %s", e.T.Local().Format("15:04:05"), e.Ev, e.Node)
			if e.To != "" {
				line += " → " + e.To
			}
			if e.Why != "" {
				line += "  (" + e.Why + ")"
			}
			fmt.Fprintln(out, line)
		}
		printed = len(evs)
		if !follow {
			return nil
		}
		s, err := loop.LoadState(root, run)
		if err != nil {
			return err
		}
		l, locked := loop.ReadLock(root, run)
		if s.Node == "" || !locked || !loop.Alive(l.PID) {
			return nil
		}
		time.Sleep(every)
	}
}

func newCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <run>",
		Short: i18n.T("cmd.cancel.short"),
		Long: `Stop a running run: its supervisor saves the state at the node it is in, and
gofi run --resume <run> carries on from there.`,
		Example: `gofi cancel r-20261102-101500`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			if err := loop.Cancel(root, args[0]); err != nil {
				return &ExitCodeError{Code: 1, Err: errors.New(err.Error())}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  "+i18n.T("cancel.sent", args[0], args[0]))
			return nil
		},
	}
}

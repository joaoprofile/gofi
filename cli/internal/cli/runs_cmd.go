package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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

func newAnswerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "answer [<run>]",
		Short: i18n.T("cmd.answer.short"),
		Long: `Answer the decisions a run is waiting on: an agent stopped on what the change
does not cover, and the run waits at its gate (gofi ps shows it bloqueada).
With no run, the one waiting. In a terminal each question is asked; in a
script, --set id=value answers one (repeat it), and a question left out takes
its recommendation, recorded as assumed. --recommended takes them all.`,
		Example: `gofi answer
gofi answer r-20261102-101500 --set q1=paginada --set q2=não
gofi answer r-20261102-101500 --recommended`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			run := ""
			if len(args) > 0 {
				run = args[0]
			} else if run, err = waitingRun(root); err != nil {
				return &ExitCodeError{Code: 1, Err: err}
			}
			q, ok := loop.OpenGate(root, run)
			if !ok {
				return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("answer.none", run))}
			}
			sets, _ := cmd.Flags().GetStringArray("set")
			recommended, _ := cmd.Flags().GetBool("recommended")
			answers := map[string]string{}
			for _, kv := range sets {
				id, v, ok := strings.Cut(kv, "=")
				if !ok {
					return fmt.Errorf("--set %q: id=value", kv)
				}
				answers[strings.TrimSpace(id)] = strings.TrimSpace(v)
			}
			out := cmd.OutOrStdout()
			if len(sets) == 0 && !recommended {
				if f, ok := cmd.InOrStdin().(*os.File); !ok || !term.IsTerminal(int(f.Fd())) {
					printGate(out, q)
					return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("answer.how", run))}
				}
				if answers, err = askQuestions(context.Background(), q.Elicitation().AsQuestions()); err != nil {
					return err
				}
			}
			if err := loop.Answer(root, run, answers); err != nil {
				return &ExitCodeError{Code: 1, Err: err}
			}
			fmt.Fprintln(out, "  "+i18n.T("answer.sent", run))
			return nil
		},
	}
	cmd.Flags().StringArray("set", nil, i18n.T("cmd.answer.flag.set"))
	cmd.Flags().Bool("recommended", false, i18n.T("cmd.answer.flag.recommended"))
	return cmd
}

// waitingRun is the one run waiting at its gate.
func waitingRun(root string) (string, error) {
	all, err := loop.List(root)
	if err != nil {
		return "", err
	}
	var waiting []string
	for _, r := range all {
		if r.Liveness == loop.Blocked {
			waiting = append(waiting, r.State.Run)
		}
	}
	switch len(waiting) {
	case 0:
		return "", errors.New(i18n.T("answer.nothing"))
	case 1:
		return waiting[0], nil
	}
	return "", errors.New(i18n.T("answer.which", strings.Join(waiting, ", ")))
}

// printGate shows what a run is waiting on, for a script to answer.
func printGate(out io.Writer, q *loop.Question) {
	fmt.Fprintln(out, "  "+i18n.T("answer.waiting", q.Run, q.Agent))
	for _, x := range q.Questions {
		fmt.Fprintf(out, "    %s  %s\n", x.ID, x.Text)
		if len(x.Options) > 0 {
			fmt.Fprintf(out, "        %s", strings.Join(x.Options, " | "))
			if x.Recommended != "" {
				fmt.Fprintf(out, "  (%s)", x.Recommended)
			}
			fmt.Fprintln(out)
		}
	}
}

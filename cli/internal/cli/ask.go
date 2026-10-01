package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gofi-labs/gofi/cli/internal/engine"
	"github.com/gofi-labs/gofi/cli/internal/engine/claude"
	"github.com/gofi-labs/gofi/cli/internal/guard"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/intake"
	"github.com/gofi-labs/gofi/cli/internal/runs"
	"github.com/gofi-labs/gofi/cli/internal/tui/flow"
)

// exitQuestions is the exit code of an ask that stopped on open questions, so
// a script can tell "answer me" from a failure.
const exitQuestions = 2

func newAskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask <request>",
		Short: i18n.T("cmd.ask.short"),
		Long: `Carry a request in free text out in one call: plan it as 'gofi intake' does,
then run its phases one after the other on Claude Code, each role at its tier.

Open questions are asked in a terminal; in a script the command stops on them
with exit code 2 — answer with --answer id=value, or --proceed to go on with
each default written down as an assumption.

After a PRD or a spec the plan stops for review: in a terminal it asks, in a
script it stops there unless --yes. There is no screen here to approve each
change, so the agent runs in Claude Code's acceptEdits mode by default: it
edits the project's files, and a shell command that needs approval is refused.
--permission-mode picks another mode.`,
		Example: `gofi ask "altere o fluxo de pricing, adicionando um rate limit"
gofi ask "crie um PRD de sincronização de pedidos" --answer context=novo
gofi ask "corrija o cancelamento de pedidos" --yes --permission-mode plan`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAsk(cmd, strings.Join(args, " "))
		},
	}
	cmd.Flags().StringArray("answer", nil, "answer an open question, as id=value")
	cmd.Flags().Bool("proceed", false, "go on without answering; defaults are written down as assumptions")
	cmd.Flags().BoolP("yes", "y", false, "go past the review stops after a PRD or a spec")
	cmd.Flags().Bool("no-model", false, "plan with the rules only: every open point is a question")
	cmd.Flags().String("permission-mode", "acceptEdits", "Claude Code permission mode for the phases")
	cmd.Flags().String("model", "", "the session's model (default: ai.model in .gofi.yaml)")
	return cmd
}

func runAsk(cmd *cobra.Command, request string) error {
	out := cmd.OutOrStdout()
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	h, ok := host.Get(cfg.AI.Host)
	if !ok || !h.Chat {
		return fmt.Errorf("%s", i18n.T("chat.host_required", h.Label, h.Label))
	}
	if err := requireClaudeCode(os.Getenv(EnvEngine)); err != nil {
		return err
	}
	// The plan is made here: the intake's prompt hook stays quiet on the turns
	// of the Claude Code this conducts.
	_ = os.Setenv(EnvConducting, "1")
	interactive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))

	opts := intake.Options{Language: backendLanguage(cfg), CacheDir: filepath.Join(root, ".gofi", "cache", "intake")}
	opts.Proceed, _ = cmd.Flags().GetBool("proceed")
	if answers, _ := cmd.Flags().GetStringArray("answer"); len(answers) > 0 {
		opts.Answers = map[string]string{}
		for _, a := range answers {
			id, v, ok := strings.Cut(a, "=")
			if !ok {
				return fmt.Errorf("--answer %q: use id=value", a)
			}
			opts.Answers[strings.TrimSpace(id)] = strings.TrimSpace(v)
		}
	}
	if noModel, _ := cmd.Flags().GetBool("no-model"); !noModel {
		opts.Model = lightModelFor(cfg)
	}
	res, err := intake.Run(root, request, opts)
	if err != nil {
		return err
	}
	if !res.Chat {
		printIntake(out, res)
	}

	if res.Ask {
		if !interactive {
			return &ExitCodeError{Code: exitQuestions, Err: errors.New(i18n.T("ask.questions_open"))}
		}
		answers, err := askQuestions(res.Questions)
		if err != nil {
			return err
		}
		opts.Answers = answers
		if res, err = intake.Run(root, request, opts); err != nil {
			return err
		}
		fmt.Fprintln(out)
		printIntake(out, res)
	}
	if res.Answer != "" {
		return nil
	}

	model, _ := cmd.Flags().GetString("model")
	if model == "" {
		model = cfg.AI.Model
	}
	mode, _ := cmd.Flags().GetString("permission-mode")
	// Each phase runs in a session of its own: it reads what the phases before
	// it left on disk, never their conversation.
	open := func(m string) engine.Session {
		return claude.New(claude.Options{Executable: os.Getenv(EnvEngine), Dir: root, Model: m, PermissionMode: mode,
			AllowedTools: guard.WriteRules})
	}

	yes, _ := cmd.Flags().GetBool("yes")
	var tierModel func(host.Tier) string
	if h.SkillModel {
		tierModel = func(t host.Tier) string { return h.Model(t, cfg.AI.Tiers) }
	}
	approve := func(done, next string) bool {
		if yes {
			return true
		}
		if !interactive {
			fmt.Fprintln(out, "\n  "+i18n.T("ask.stopped_for_review", done, next))
			return false
		}
		ok, err := flow.YesNo(i18n.T("ask.review_question", done, next), "", "", "", true)
		return err == nil && ok
	}
	var rec *runs.Run
	if len(res.Plan) > 0 {
		// The record is for calibration; the request does not wait on it.
		rec, _ = runs.Start(root, "ask", res)
	}
	elicit := func(qs []intake.Question) (map[string]string, bool) {
		if interactive {
			answers, err := askQuestions(qs)
			return answers, err == nil
		}
		for _, q := range qs {
			fmt.Fprintf(out, "  • %s  [%s]\n", q.Text, strings.Join(q.Options, "] ["))
		}
		// In a script the decisions are the person's, not the recommendation's:
		// only --proceed takes the recommendations, each written down as assumed.
		return nil, opts.Proceed
	}
	return conduct(out, res, open, model, tierModel, approve, elicit, root, rec)
}

// askQuestions asks each open question in the terminal: its options to pick
// from, or a line of text when it has none.
func askQuestions(qs []intake.Question) (map[string]string, error) {
	answers := map[string]string{}
	for _, q := range qs {
		if len(q.Options) == 0 {
			v, err := flow.Ask(q.Text, "", nil)
			if err != nil {
				return nil, err
			}
			answers[q.ID] = v
			continue
		}
		var choice string
		options := make([]flow.Option, 0, len(q.Options))
		for _, o := range q.Options {
			options = append(options, flow.Option{Label: o, Value: o})
		}
		if err := flow.Run(flow.Header{}, []flow.Step{{Kind: flow.Select, Title: q.Text, Options: options, Choice: &choice}}); err != nil {
			return nil, err
		}
		answers[q.ID] = choice
	}
	return answers, nil
}

// conduct runs the plan's phases in order, each a turn with its role in a
// fresh session at its tier: a phase reads what the ones before it left on
// disk — the spec, the context's memory, the code — never their conversation.
// It stops for review where approve says so, and on the first phase that
// fails. An audit the QA fails runs the retry the plan carries — the
// implementation one tier up, then the audit again. A request with no plan is
// sent as it is. Each phase is recorded in rec, when there is one.
func conduct(out io.Writer, res *intake.Result, open func(model string) engine.Session, sessionModel string, tierModel func(host.Tier) string, approve func(done, next string) bool, elicit func([]intake.Question) (map[string]string, bool), root string, rec *runs.Run) (err error) {
	if len(res.Plan) == 0 {
		s := open(sessionModel)
		defer s.Close()
		turn := res.Request
		if res.Turn != "" {
			turn = res.Turn // a question, with the sections that answer it
		}
		_, err := runTurn(out, s, turn)
		return err
	}
	outcome, verdict, retried := runs.OutcomeStopped, "", false
	defer func() {
		if err != nil {
			outcome = runs.OutcomeFailed
		}
		if rec != nil {
			_ = rec.Finish(outcome, verdict)
		}
	}()
	// The phases as they run: a failed audit splices its retry in. The plan
	// the intake returned stays as it was decided.
	r := *res
	r.Plan = append([]intake.Phase{}, res.Plan...)
	// What each role's elicitation left, and the person's answers: they end
	// the role's turn (intake.WithDecisions).
	type decided struct {
		e       *intake.Elicitation
		answers map[string]string
	}
	decisions := map[string]decided{}
	for i := 0; i < len(r.Plan); i++ {
		ph := r.Plan[i]
		fmt.Fprintf(out, "\n● %s\n", i18n.T("chat.plan.phase", i+1, len(r.Plan), ph.Role, string(ph.Tier)))
		// A raised phase runs on its tier's model, as the role; any other
		// invokes its skill, whose own model wins for the turn.
		model, asRole := sessionModel, false
		if ph.Tier != ph.Base && tierModel != nil {
			if m := tierModel(ph.Tier); m != "" {
				model, asRole = m, true
			}
		}
		before := intake.Verdict(root, &r)
		turn := intake.TurnFor(&r, i, asRole)
		if d, ok := decisions[ph.Role]; ok && !ph.Elicit {
			turn = intake.WithDecisions(turn, d.e, d.answers)
			delete(decisions, ph.Role)
		}
		if ph.ElicitFile != "" {
			intake.ClearElicitation(root, ph.ElicitFile)
		}
		s := open(model)
		done, err := runTurn(out, s, turn)
		_ = s.Close()
		p := runs.Phase{Role: ph.Role, Tier: string(ph.Tier), Why: ph.Why, TierWhy: ph.TierWhy,
			Seconds: done.Duration.Seconds(), CostUSD: done.CostUSD, OK: err == nil}
		if asRole {
			p.Model = model
		}
		if err != nil {
			p.Error = err.Error()
		}
		// What the phase left for the person: an elicitation's decisions, or
		// those an implementation stopped on.
		var e *intake.Elicitation
		if err == nil && ph.ElicitFile != "" {
			e, err = intake.ReadElicitation(root, ph.ElicitFile)
		}
		var stop, blockedAgain bool
		switch {
		case err != nil:
		case ph.Elicit && e == nil:
			fmt.Fprintln(out, "\n  "+i18n.T("ask.elicit.none", ph.Role))
		case e != nil && (ph.Elicit || len(e.Questions) > 0):
			if !ph.Elicit && len(ph.OnBlock) == 0 {
				// Stopped again after the spec was completed: the person's to look at.
				blockedAgain = true
				break
			}
			answers, ok := map[string]string{}, true
			if qs := e.AsQuestions(); len(qs) > 0 {
				fmt.Fprintln(out, "\n  "+i18n.T("ask.elicit.questions", len(qs), ph.Role))
				if elicit != nil {
					answers, ok = elicit(qs)
				}
			}
			p.Asked = len(e.Questions)
			for _, q := range e.Questions {
				if strings.TrimSpace(answers[q.ID]) == "" {
					p.Assumed++
				}
			}
			if ph.Elicit {
				decisions[ph.Role] = decided{e, answers}
			} else {
				// The spec records the answers, and the implementation runs again.
				decisions[ph.OnBlock[0].Role] = decided{e, answers}
				fmt.Fprintln(out, "\n  "+blockedMessage(ph))
				r.Plan = append(r.Plan[:i+1], append(append([]intake.Phase{}, ph.OnBlock...), r.Plan[i+1:]...)...)
			}
			stop = !ok
		}
		if rec != nil {
			_ = rec.Phase(p)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", ph.Role, err)
		}
		if blockedAgain {
			fmt.Fprintln(out, "\n  "+i18n.T("ask.blocked_again", ph.Role, ph.ElicitFile))
			return nil
		}
		if stop {
			fmt.Fprintln(out, "\n  "+i18n.T("ask.elicit.stopped", ph.ElicitFile))
			return nil
		}
		if ph.Produces == "audit" {
			verdict = intake.Status(intake.Verdict(root, &r))
			if intake.Rejected(root, &r, before) {
				if len(ph.OnReject) > 0 {
					fmt.Fprintln(out, "\n  "+i18n.T("chat.plan.rejected", ph.OnReject[0].Role, string(ph.OnReject[0].Tier)))
					r.Plan = append(r.Plan[:i+1], append(append([]intake.Phase{}, ph.OnReject...), r.Plan[i+1:]...)...)
					retried = true
					continue
				}
				if retried {
					// Failed again after the retry: the person's to look at.
					outcome = runs.OutcomeEscalated
					fmt.Fprintln(out, "\n  "+i18n.T("chat.plan.still_rejected"))
					return nil
				}
			}
		}
		if phaseReview(&r, i) && !approve(ph.Produces, r.Plan[i+1].Role) {
			return nil
		}
	}
	outcome = runs.OutcomeDone
	fmt.Fprintln(out, "\n  "+i18n.T("chat.plan.done"))
	return nil
}

// phaseReview is whether the plan stops for review after phase i.
func phaseReview(r *intake.Result, i int) bool {
	return r.Plan[i].ReviewAfter || intake.ApprovalAfter(r, i)
}

// runTurn sends one turn and prints it as it goes: the agent's text, the tools
// it calls, and the turn's cost. A turn that ends in error is an error.
func runTurn(out io.Writer, s engine.Session, prompt string) (engine.Done, error) {
	var done engine.Done
	events, err := s.Send(context.Background(), engine.Turn{Prompt: prompt})
	if err != nil {
		return done, err
	}
	var failure error
	for e := range events {
		switch ev := e.(type) {
		case engine.Message:
			for _, b := range ev.Blocks {
				switch b.Kind {
				case engine.BlockText:
					if t := strings.TrimSpace(b.Text); t != "" {
						fmt.Fprintln(out, "\n"+t)
					}
				case engine.BlockToolUse:
					fmt.Fprintf(out, "  · %s\n", b.Name)
				}
			}
		case engine.Done:
			done = ev
			switch {
			case ev.IsError:
				failure = errors.New(ev.Err)
			case ev.Cancelled:
				failure = errors.New(i18n.T("chat.interrupted"))
			default:
				fmt.Fprintf(out, "  %s\n", turnCost(ev))
			}
		case engine.Failure:
			failure = errors.New(ev.Message)
		}
	}
	return done, failure
}

func turnCost(d engine.Done) string {
	parts := []string{fmt.Sprintf("%.1fs", d.Duration.Seconds())}
	if d.CostUSD > 0 {
		parts = append(parts, fmt.Sprintf("$%.3f", d.CostUSD))
	}
	return strings.Join(parts, " · ")
}

// blockedMessage says what follows a phase that stopped on decisions: the
// spec records them and the phase runs again, or — when the phase itself runs
// again, as documentation does — it only takes the answers.
func blockedMessage(ph intake.Phase) string {
	if next := ph.OnBlock[0].Role; next != ph.Role {
		return i18n.T("ask.blocked", ph.Role, next)
	}
	return i18n.T("ask.blocked_rerun", ph.Role)
}

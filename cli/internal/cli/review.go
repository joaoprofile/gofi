package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/review"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

func newReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review <change>",
		Short: i18n.T("cmd.review.short"),
		Long: `Review a change before it is approved: gofi spec check first, at no cost, then
the reviewer reads it as the implementation would — a requirement two
readers would build differently, two that contradict each other, a scenario
whose outcome no test can observe, a gap against the PRD, a scope the
proposal does not hold to.

Blockers and majors fail the command: correct the change, or approve it as it
is if the finding does not hold. A review is kept with the hash of what it
read, so an unchanged change is not reviewed — nor paid for — twice; --again
reviews it anyway. The review never edits the change.`,
		Example: `gofi review expor-pontos
gofi review expor-pontos --again`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			again, _ := cmd.Flags().GetBool("again")
			r, err := reviewChange(cmd.Context(), cmd.OutOrStdout(), root, cfg, args[0], again)
			if err != nil {
				return err
			}
			printReview(cmd.OutOrStdout(), r)
			if n := len(r.Blocking()); n > 0 {
				return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("review.blocking", n, args[0]))}
			}
			return nil
		},
	}
	cmd.Flags().Bool("again", false, i18n.T("cmd.review.flag.again"))
	return cmd
}

// reviewChange checks a change and has the reviewer read it — or returns
// the review kept of it as it is now. A change gofi spec check refuses is
// not reviewed: what it says is wrong before anyone reads it.
func reviewChange(ctx context.Context, out io.Writer, root string, cfg *config.GofiConfig, change string, again bool) (*review.Review, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return nil, err
	}
	bodies := map[string][]string{}
	var mine []contract.Doc
	for _, d := range all {
		if d.Change == change {
			mine = append(mine, d)
			bodies[d.Path] = d.Body
		}
	}
	if len(mine) == 0 {
		return nil, errors.New(i18n.T("change.not_found", change))
	}
	placeholders, _ := contract.Placeholders(root)
	blocked := 0
	for _, d := range mine {
		for _, f := range contract.Blocking(root, all, d, placeholders) {
			fmt.Fprintf(out, "  %s %s: %s\n", i18n.T("index.check.error"), f.Path, f.Message)
			blocked++
		}
	}
	if blocked > 0 {
		return nil, &ExitCodeError{Code: 1, Err: errors.New(i18n.T("spec.approve.refused", blocked))}
	}
	hash := review.Hash(bodies)
	if kept, err := review.Load(root, change); err == nil && kept != nil && kept.Hash == hash && !again {
		fmt.Fprintln(out, "  "+i18n.T("review.kept", change))
		return kept, nil
	}

	roles, problems := role.Load(layout.RoleDirs(root)...)
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	facts, err := changeFacts(cfg, root, change)
	if err != nil {
		return nil, err
	}
	choices := role.Dispatch(roles, "contract", facts)
	if len(choices) == 0 {
		return nil, errors.New(i18n.T("review.nobody"))
	}
	var rl role.Role
	for _, r := range roles {
		if r.Name == choices[0].Role {
			rl = r
		}
	}
	agentsCallThisGofi()
	open, err := turnSessions(cfg, root, "")
	if err != nil {
		return nil, err
	}
	file := review.Path(root, change)
	_ = os.Remove(file)
	rel, _ := filepath.Rel(root, file)
	fmt.Fprintln(out, "  "+i18n.T("review.started", change, rl.Name))
	agent := &engineAgent{root: root, work: root, open: open, out: out,
		transcript: filepath.Join(filepath.Dir(file), change+".ndjson")}
	res, err := agent.Run(ctx, loop.Turn{Node: "review", Role: rl.Name, Agent: "review-" + change, Iter: 1,
		Tier: rl.Contract.Tier, Tools: rl.Contract.Tools, Prompt: reviewBrief(rl, change, filepath.ToSlash(rel))})
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s não gravou a revisão em %s", rl.Name, filepath.ToSlash(rel))
	}
	r, err := review.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s gravou uma revisão que o gofi não lê: %w", rl.Name, err)
	}
	r.Change, r.Hash = change, hash
	if err := review.Save(root, r); err != nil {
		return nil, err
	}
	fmt.Fprintln(out, "  "+i18n.T("review.cost", res.CostUSD))
	return r, nil
}

// reviewBrief is the reviewer's turn: the change, not yet approved, and the
// one file the turn writes.
func reviewBrief(rl role.Role, change, file string) string {
	return fmt.Sprintf("Siga o papel descrito em %s.\n\n"+
		"Revise a mudança `%s` antes da aprovação — `gofi change list`, a proposta, os deltas e o design em `specs/changes/%s/`. A pessoa ainda não a aprovou: o que você achar é o que ela corrige antes de aprovar.\n\n"+
		"Grave os achados em `%s`, exatamente neste formato — `severity` blocker, major ou minor; `kind` ambiguidade, contradição, não testável, lacuna ou escopo; sem achado, `findings` vazio:\n\n```json\n%s\n```\n\n"+
		"É a única escrita deste turno: não edite a mudança, a spec nem o código.\n",
		layout.RoleSkill(rl.Name, true), change, change, file, review.Example)
}

func printReview(out io.Writer, r *review.Review) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(out, "  "+i18n.T("review.clean", r.Change))
		return
	}
	fmt.Fprintln(out, "  "+i18n.T("review.findings", r.Change, len(r.Findings)))
	for _, f := range r.Findings {
		fmt.Fprintf(out, "\n  [%s] %s — %s\n    %s\n", f.Severity, f.Kind, f.Where, f.Message)
		if f.Suggestion != "" {
			fmt.Fprintf(out, "    → %s\n", f.Suggestion)
		}
	}
	fmt.Fprintln(out)
}

// contractGate is the run's first segment (0002, §6): a change not approved
// as it is now is checked and reviewed, and the person approves it — at the
// terminal, when there is one; else the run stops and says how. Nothing
// approves a change but the person.
func contractGate(cmd *cobra.Command, root string, cfg *config.GofiConfig, change string, interactive bool) error {
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	var found *contract.Change
	for _, c := range contract.Changes(all) {
		if c.ID == change {
			found = &c
		}
	}
	if found == nil || found.Sealed {
		return nil // an unknown change is said by the delivery; a sealed one runs
	}
	out := cmd.OutOrStdout()
	r, err := reviewChange(cmd.Context(), out, root, cfg, change, false)
	if err != nil {
		return err
	}
	printReview(out, r)
	if n := len(r.Blocking()); n > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("review.blocking", n, change))}
	}
	if !interactive || !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.not_sealed", change, change))}
	}
	fmt.Fprint(out, "  "+i18n.T("run.approve_ask", change))
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "s", "sim", "y", "yes":
	default:
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.not_sealed", change, change))}
	}
	var mine []contract.Doc
	for _, d := range all {
		if d.Change == change {
			mine = append(mine, d)
		}
	}
	return approveDocs(cmd, root, all, mine)
}

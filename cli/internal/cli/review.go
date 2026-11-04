package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
		Use:   "review <change> | --context <ctx>",
		Short: i18n.T("cmd.review.short"),
		Long: `Review a change before it is approved: gofi spec check first, at no cost, then
the reviewer reads it as the implementation would — a requirement two
readers would build differently, two that contradict each other, a scenario
whose outcome no test can observe, a gap against the PRD, a scope the
proposal does not hold to.

Blockers and majors fail the command: correct the change, or approve it as it
is if the finding does not hold. A review is kept with the hash of what it
read, so an unchanged change is not reviewed — nor paid for — twice; --again
reviews it anyway. The review never edits the change.

--context reviews every change of a context not approved yet: what a chat
shows at its review stop, before the person approves the spec.`,
		Example: `gofi review expor-pontos
gofi review expor-pontos --again
gofi review --context pontos`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			if ctxName, _ := cmd.Flags().GetString("context"); ctxName != "" {
				if len(args) > 0 {
					return errors.New(i18n.T("review.which"))
				}
				lines, blocking, err := chatReview(root, cfg)(ctxName)
				if len(lines) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
				}
				if err != nil {
					return err
				}
				if blocking > 0 {
					return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("review.blocking", blocking, ctxName))}
				}
				return nil
			}
			if len(args) == 0 {
				return errors.New(i18n.T("review.which"))
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
	cmd.Flags().String("context", "", i18n.T("cmd.review.flag.context"))
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
	dir, err := changeDirOf(root, change)
	if err != nil {
		return nil, err
	}
	agent := &engineAgent{root: root, work: root, open: open, out: out,
		transcript: filepath.Join(filepath.Dir(file), change+".ndjson")}
	res, err := agent.Run(ctx, loop.Turn{Node: "review", Role: rl.Name, Agent: "review-" + change, Iter: 1,
		Tier: rl.Contract.Tier, Tools: rl.Contract.Tools, Prompt: reviewBrief(rl, change, dir, filepath.ToSlash(rel))})
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
func reviewBrief(rl role.Role, change, dir, file string) string {
	return fmt.Sprintf("Siga o papel descrito em %s.\n\n"+
		"Revise a mudança `%s` antes da aprovação — `gofi change list`, a proposta, os deltas e o design em `%s/`. A pessoa ainda não a aprovou: o que você achar é o que ela corrige antes de aprovar.\n\n"+
		"Grave os achados em `%s`, exatamente neste formato — `severity` blocker, major ou minor; `kind` ambiguidade, contradição, não testável, lacuna ou escopo; sem achado, `findings` vazio:\n\n```json\n%s\n```\n\n"+
		"É a única escrita deste turno: não edite a mudança, a spec nem o código.\n",
		layout.RoleSkill(rl.Name, true), change, dir, file, review.Example)
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
	if approvePorts.agent() {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("approve.agent"))}
	}
	return approveDocsTo(out, root, all, mine)
}

// chatReview is the chat's review stop on a spec: every open change of the
// context not approved as it is, checked and reviewed — the lines to show,
// and how many findings block.
func chatReview(root string, cfg *config.GofiConfig) func(string) ([]string, int, error) {
	return func(ctxName string) ([]string, int, error) {
		all, err := contract.Docs(root)
		if err != nil {
			return nil, 0, err
		}
		var out bytes.Buffer
		blocking := 0
		for _, c := range contract.Changes(all) {
			if c.Sealed || !slices.Contains(c.Contexts, ctxName) {
				continue
			}
			r, err := reviewChange(context.Background(), &out, root, cfg, c.ID, false)
			if err != nil {
				return strings.Split(strings.TrimSpace(out.String()), "\n"), 0, err
			}
			printReview(&out, r)
			blocking += len(r.Blocking())
		}
		text := strings.TrimSpace(out.String())
		if text == "" {
			return nil, 0, nil
		}
		return strings.Split(text, "\n"), blocking, nil
	}
}

// changeDirOf is the folder of an open change, as a turn is told where to
// read it.
func changeDirOf(root, change string) (string, error) {
	if dir, ok := contract.FindChange(root, change); ok {
		return dir, nil
	}
	return "", errors.New(i18n.T("change.not_found", change))
}

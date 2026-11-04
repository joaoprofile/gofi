package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/bench"
	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/learn"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
	"github.com/joaoprofile/gofi/cli/internal/worktree"
)

func newBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: i18n.T("cmd.bench.short"),
		Long: `Measure the loop on this project: gofi's suite of changes for the backend's
language — one that should pass at once, a contract with a gap, a trap that
tends to need rework, a change of two layers — is delivered with gofi run --all
on a copy of the project that never lands, and what each run did is read back:
accepted or not, the rounds, the findings, the caveats of its contract, the
lessons it proposed, the roles that implemented and the cost by role.

The copy is a worktree of the head, on a branch of its own, outside the
project; it is discarded at the end unless --keep. The result stays in
.gofi/bench/<id>.md and .json — run it again after changing gofi, and compare.
It spends what the runs spend: the suite is a few dollars.`,
		Example: `gofi bench
gofi bench --parallel 2
gofi bench --keep`,
		Args: cobra.NoArgs,
		RunE: runBench,
	}
	cmd.Flags().Int("parallel", 1, i18n.T("cmd.bench.flag.parallel"))
	cmd.Flags().Bool("keep", false, i18n.T("cmd.bench.flag.keep"))
	cmd.AddCommand(newBenchAuditCmd())
	return cmd
}

func runBench(cmd *cobra.Command, _ []string) error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	lang := backendLanguage(cfg)
	suite, ok, err := bench.Load(lang)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New(i18n.T("bench.no_suite", lang))
	}
	out := cmd.OutOrStdout()
	ctx := cmd.Context()
	id := "b-" + time.Now().Format("20060102-150405")
	tree, err := worktree.Scratch(ctx, root, "bench-"+id)
	if err != nil {
		return err
	}
	keep, _ := cmd.Flags().GetBool("keep")
	defer func() {
		if keep {
			fmt.Fprintln(out, "  "+i18n.T("bench.kept", tree.Dir, tree.Branch))
			return
		}
		_ = tree.Discard(context.Background())
	}()
	fmt.Fprintln(out, i18n.T("bench.started", id, len(suite.Changes), lang, tree.Dir))

	// The suite is the contract: written, committed — a run's worktree
	// starts from the commit — and approved, as a person would.
	if err := suite.Write(tree.Dir); err != nil {
		return err
	}
	gofi := func(w io.Writer, args ...string) error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		c := exec.CommandContext(ctx, exe, args...)
		c.Dir = tree.Dir
		c.Env = append(os.Environ(), config.EnvRoot+"="+tree.Dir)
		c.Stdout, c.Stderr = w, w
		return c.Run()
	}
	git := func(args ...string) error {
		c := exec.CommandContext(ctx, "git", append([]string{"-C", tree.Dir}, args...)...)
		if b, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v\n%s", args, err, b)
		}
		return nil
	}
	for _, c := range suite.Changes {
		if err := gofi(io.Discard, "change", "approve", c.ID); err != nil {
			return fmt.Errorf("gofi change approve %s: %w", c.ID, err)
		}
	}
	if err := git("add", "-A", "specs"); err != nil {
		return err
	}
	if err := git("commit", "--quiet", "--no-verify", "-m", "bench: suíte "+id); err != nil {
		return err
	}

	parallel, _ := cmd.Flags().GetInt("parallel")
	// The batch says how it went change by change; not accepted is a result
	// of the benchmark, not a failure of it.
	_ = gofi(out, "run", "--all", "--parallel", strconv.Itoa(max(parallel, 1)), "--gate-timeout", "0")

	res, err := bench.Collect(tree.Dir, id, suite)
	if err != nil {
		return err
	}
	p, err := res.Save(root)
	if err != nil {
		return err
	}
	// The lessons the runs proposed are the benchmark's, kept beside it: the
	// project's knowledge is not the place for a suite's.
	if all, err := learn.All(tree.Dir); err == nil {
		for _, l := range all {
			dst := filepath.Join(root, filepath.FromSlash(bench.Dir), id+"-lessons", l.Run+".json")
			if os.MkdirAll(filepath.Dir(dst), 0o755) == nil {
				_ = copyFile(learn.Path(tree.Dir, l.Run), dst, 0o644)
			}
		}
	}
	fmt.Fprint(out, "\n"+res.Report())
	rel, _ := filepath.Rel(root, p)
	fmt.Fprintln(out, "\n  "+i18n.T("bench.saved", filepath.ToSlash(rel)))
	return nil
}

func newBenchAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: i18n.T("cmd.bench.audit.short"),
		Long: `Measure the audit's recall: for each of gofi's audit cases — code already
written with defects planted in it, and the change it claims to realize —
the same code is audited twice, in lenses and by the general lens alone,
each on a copy of the project of its own, with nothing implemented or fixed
(the audit flow). A finding finds a planted defect when it is in its file,
within its lines, and says what it is about. The result is the recall of
each, the defects each missed, the findings beyond the planted ones, the
turns and the cost — in .gofi/bench/<id>-audit.md and .json, and each
verdict in .gofi/bench/<id>-audit/.

--judge <id> scores a benchmark's verdicts again, with what the cases say
now — a defect placed better, a matcher fixed —, running nothing.`,
		Example: `gofi bench audit
gofi bench audit --judge b-20261104-082423`,
		Args: cobra.NoArgs,
		RunE: runBenchAudit,
	}
	cmd.Flags().String("judge", "", "score the verdicts of this benchmark again, running nothing")
	return cmd
}

// verdictsDir is where an audit benchmark keeps each case's verdict.
func verdictsDir(root, id string) string {
	return filepath.Join(root, filepath.FromSlash(bench.Dir), id+"-audit")
}

// saveAuditResult writes an audit benchmark's scores and report.
func saveAuditResult(out io.Writer, root, id string, scores []bench.Score) error {
	report := bench.AuditReport(id, scores)
	dir := filepath.Join(root, filepath.FromSlash(bench.Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(map[string]any{"schema": bench.Schema, "id": id, "scores": scores}, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, id+"-audit.json"), append(data, '\n'), 0o644)
	p := filepath.Join(dir, id+"-audit.md")
	if err := os.WriteFile(p, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Fprint(out, "\n"+report)
	rel, _ := filepath.Rel(root, p)
	fmt.Fprintln(out, "\n  "+i18n.T("bench.saved", filepath.ToSlash(rel)))
	return nil
}

// judgeAgain scores the verdicts a benchmark kept with the cases as they
// are: the cost and turns are those it measured.
func judgeAgain(out io.Writer, root, src, lang, id string) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(bench.Dir), id+"-audit.json"))
	if err != nil {
		return err
	}
	var old struct {
		Scores []bench.Score `json:"scores"`
	}
	if err := json.Unmarshal(data, &old); err != nil {
		return err
	}
	cases, err := bench.AuditCases(lang)
	if err != nil {
		return err
	}
	byName := map[string]*bench.AuditCase{}
	for _, c := range cases {
		byName[c.Name] = c
	}
	var scores []bench.Score
	for _, was := range old.Scores {
		c, ok := byName[was.Case]
		if !ok {
			return fmt.Errorf("%s: no audit case %s", id, was.Case)
		}
		raw, err := os.ReadFile(filepath.Join(verdictsDir(root, id), was.Case+"-"+was.Mode+".json"))
		if err != nil {
			return fmt.Errorf("%s kept no verdict of %s (%s) — only a benchmark run since verdicts are kept can be judged again", id, was.Case, was.Mode)
		}
		var v verdict.Verdict
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		// The case is written aside: its defects are placed by its code.
		cc := *c
		cc.Defects = append([]bench.Defect{}, c.Defects...)
		tmp, err := os.MkdirTemp("", "gofi-judge-")
		if err != nil {
			return err
		}
		err = cc.Write(tmp, src)
		if err == nil {
			s := cc.Judge(&v, src)
			s.Mode, s.CostUSD, s.Turns = was.Mode, was.CostUSD, was.Turns
			scores = append(scores, s)
		}
		_ = os.RemoveAll(tmp)
		if err != nil {
			return err
		}
	}
	return saveAuditResult(out, root, id, scores)
}

func runBenchAudit(cmd *cobra.Command, _ []string) error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	lang := backendLanguage(cfg)
	cases, err := bench.AuditCases(lang)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return errors.New(i18n.T("bench.no_suite", lang))
	}
	src := "."
	if cfg.Backend != nil && cfg.Backend.Path != "" {
		src = cfg.Backend.Path
	}
	out := cmd.OutOrStdout()
	if again, _ := cmd.Flags().GetString("judge"); again != "" {
		return judgeAgain(out, root, src, lang, again)
	}
	ctx := cmd.Context()
	id := "b-" + time.Now().Format("20060102-150405")
	fmt.Fprintln(out, i18n.T("bench.audit.started", id, len(cases)))
	var (
		mu     sync.Mutex
		scores []bench.Score
		errs   []error
		wg     sync.WaitGroup
	)
	for _, c := range cases {
		for _, mode := range []string{auditLenses, auditSingle} {
			wg.Go(func() {
				s, err := auditCase(ctx, root, src, id, c, mode)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, fmt.Errorf("%s (%s): %w", c.Name, mode, err))
					return
				}
				fmt.Fprintf(out, "  [%s · %s] recall %.0f%% — achou %v, perdeu %v\n", c.Name, mode, 100*s.Recall(), s.Found, s.Missed)
				scores = append(scores, s)
			})
		}
	}
	wg.Wait()
	if err := saveAuditResult(out, root, id, scores); err != nil {
		return err
	}
	return errors.Join(errs...)
}

// auditCase audits one case's code in one mode, on a copy of its own — of
// the case too: placing its defects writes them.
func auditCase(ctx context.Context, root, src, id string, shared *bench.AuditCase, mode string) (bench.Score, error) {
	cc := *shared
	cc.Defects = append([]bench.Defect{}, shared.Defects...)
	c := &cc
	tree, err := worktree.Scratch(ctx, root, fmt.Sprintf("bench-%s-%s-%s", id, c.Name, mode))
	if err != nil {
		return bench.Score{}, err
	}
	defer func() { _ = tree.Discard(context.Background()) }()
	if err := c.Write(tree.Dir, src); err != nil {
		return bench.Score{}, err
	}
	gofi := func(args ...string) error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, exe, args...)
		cmd.Dir = tree.Dir
		cmd.Env = append(os.Environ(), config.EnvRoot+"="+tree.Dir)
		if b, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gofi %v: %v\n%s", args, err, tailLines(string(b), 20))
		}
		return nil
	}
	if err := gofi("change", "approve", c.Change); err != nil {
		return bench.Score{}, err
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "--quiet", "--no-verify", "-m", "bench: caso " + c.Name}} {
		if b, err := exec.CommandContext(ctx, "git", append([]string{"-C", tree.Dir}, args...)...).CombinedOutput(); err != nil {
			return bench.Score{}, fmt.Errorf("git %v: %v\n%s", args, err, b)
		}
	}
	args := []string{"run", c.Change, "--flow", "audit", "--gate-timeout", "0", "--no-archive"}
	if mode == auditSingle {
		args = append(args, "--audit", auditSingle)
	}
	// The audit refusing the code is the point: the run's outcome is no
	// error of the benchmark — but what it said is why, when it left no
	// verdict.
	runErr := gofi(args...)
	all, err := contract.Docs(tree.Dir)
	if err != nil {
		return bench.Score{}, err
	}
	ctxName := ""
	for _, ch := range contract.Changes(all) {
		if ch.ID == c.Change && len(ch.Contexts) > 0 {
			ctxName = ch.Contexts[0]
		}
	}
	v, err := verdict.Read(tree.Dir, ctxName)
	if err != nil || v == nil {
		return bench.Score{}, fmt.Errorf("the audit wrote no verdict of %s: %v", ctxName, errors.Join(err, runErr))
	}
	// Kept, the verdict is scored again — gofi bench audit --judge — when a
	// case or the matcher changes, with no audit run again.
	if raw, err := json.MarshalIndent(v, "", "  "); err == nil {
		dir := verdictsDir(root, id)
		if err := os.MkdirAll(dir, 0o755); err == nil {
			_ = os.WriteFile(filepath.Join(dir, c.Name+"-"+mode+".json"), append(raw, '\n'), 0o644)
		}
	}
	s := c.Judge(v, src)
	s.Mode = mode
	if infos, err := loop.List(tree.Dir); err == nil && len(infos) > 0 && infos[0].State != nil {
		s.CostUSD = infos[0].State.CostUSD
		agents, _ := loop.Agents(tree.Dir, infos[0].State.Run)
		s.Turns = len(agents)
	}
	return s, nil
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

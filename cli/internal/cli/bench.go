package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/bench"
	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/learn"
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

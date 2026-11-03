package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// BatchesDir is where gofi run --all keeps the summary of each batch, under
// the project root.
const BatchesDir = ".gofi/batches"

// allOpts is one gofi run --all.
type allOpts struct {
	Parallel                int
	Flow, Mode, GateTimeout string
	NoArchive               bool
}

// delivered is one change of a batch, and how it ended.
type delivered struct {
	Change, Run, Outcome, Why string
	CostUSD                   float64
}

// startChange delivers one change of a batch and returns how its run ended.
// gofi run starts each in a process of its own — a run moves the process's
// environment to its worktree —; a test runs it in its own.
var startChange = func(ctx context.Context, root, change, run string, worktree bool, budget float64, o allOpts, progress io.Writer) (*loop.State, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{"run", change, "--events", "--run-id", run}
	if worktree {
		args = append(args, "--worktree")
	}
	if budget > 0 {
		args = append(args, "--budget", strconv.FormatFloat(budget, 'f', 2, 64))
	}
	for flag, v := range map[string]string{"--flow": o.Flow, "--permission-mode": o.Mode, "--gate-timeout": o.GateTimeout} {
		if v != "" {
			args = append(args, flag, v)
		}
	}
	if o.NoArchive {
		args = append(args, "--no-archive")
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	why := ""
	for e := range loop.Stream(stdout) {
		switch e.Ev {
		case "node.start":
			fmt.Fprintf(progress, "  [%s] ● %s\n", change, e.Node)
		case "gate.open":
			fmt.Fprintf(progress, "  [%s] ⏸ %s\n", change, e.Why)
		case "run.result":
			why = e.Why
		}
	}
	_ = cmd.Wait()
	s, err := loop.LoadState(root, run)
	if err != nil {
		if why == "" {
			why = strings.TrimSpace(stderr.String())
		}
		return nil, errors.New(why)
	}
	return s, nil
}

// runAll delivers every open change, in the order their depende_de says:
// level by level, up to o.Parallel at once — each in its own worktree when
// more than one runs —, within loop.run_budget_usd. A change waits for the
// ones it depends on; one whose dependency was not accepted is skipped, and
// the changes that do not depend on it go on.
func runAll(ctx context.Context, out io.Writer, root string, cfg *config.GofiConfig, o allOpts) error {
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	levels, err := contract.ChangeOrder(all)
	if err != nil {
		return err
	}
	changes := map[string]contract.Change{}
	for _, c := range contract.Changes(all) {
		changes[c.ID] = c
	}
	if len(changes) == 0 {
		return errors.New(i18n.T("run.all.none"))
	}
	parallel := o.Parallel
	if parallel == 0 && cfg.Loop != nil {
		parallel = cfg.Loop.MaxAgents
	}
	parallel = max(parallel, 1)
	budget := 0.0
	if cfg.Loop != nil {
		budget = cfg.Loop.RunBudgetUSD
	}
	fmt.Fprintln(out, i18n.T("run.all.started", len(changes), len(levels), parallel))

	var (
		mu      sync.Mutex
		spent   float64
		results = map[string]delivered{}
	)
	ended := func(d delivered) {
		mu.Lock()
		defer mu.Unlock()
		results[d.Change] = d
		spent += d.CostUSD
		fmt.Fprintf(out, "  [%s] %s — US$ %.2f %s\n", d.Change, d.Outcome, d.CostUSD, d.Why)
	}
	for _, level := range levels {
		sem := make(chan struct{}, parallel)
		var wg sync.WaitGroup
		for _, id := range level {
			c := changes[id]
			if why := skipReason(c, results, &mu); why != "" {
				ended(delivered{Change: id, Outcome: "skipped", Why: why})
				continue
			}
			sem <- struct{}{}
			// Each run may spend what is left of the batch's budget when it
			// starts: the runs beside it spend from the same.
			mu.Lock()
			left := budget - spent
			mu.Unlock()
			if budget > 0 && left <= 0 {
				<-sem
				ended(delivered{Change: id, Outcome: "skipped", Why: i18n.T("run.all.budget", budget)})
				continue
			}
			if budget == 0 {
				left = 0
			}
			run := reserveRun(root)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				s, err := startChange(ctx, root, id, run, parallel > 1, left, o, out)
				if err != nil {
					ended(delivered{Change: id, Run: run, Outcome: "stopped", Why: err.Error()})
					return
				}
				d := delivered{Change: id, Run: run, Outcome: s.Outcome, CostUSD: s.CostUSD}
				if s.Outcome == "" {
					d.Outcome = "stopped"
				}
				if s.Outcome != "accepted" {
					// What the person decides on: the run's reason, and the
					// amendment it proposed.
					d.Why = s.Why
					if s.Proposal != "" {
						d.Why = strings.TrimSpace(d.Why + " proposta: " + s.Proposal)
					}
				}
				ended(d)
			}()
		}
		wg.Wait()
		if ctx.Err() != nil {
			break
		}
	}
	report := batchReport(levels, results, spent, budget)
	p := filepath.Join(root, filepath.FromSlash(BatchesDir), "b-"+time.Now().Format("20060102-150405")+".md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err == nil {
		_ = os.WriteFile(p, []byte(report), 0o644)
	}
	fmt.Fprint(out, "\n"+report)
	failed := 0
	for _, d := range results {
		if d.Outcome != "accepted" {
			failed++
		}
	}
	if failed > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.all.not_all", failed, len(results)))}
	}
	return nil
}

// skipReason says why a change of a batch is not run: not approved as it
// is, or a change it depends on was not accepted.
func skipReason(c contract.Change, results map[string]delivered, mu *sync.Mutex) string {
	if !c.Sealed {
		return i18n.T("run.all.unsealed", c.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, dep := range c.DependsOn {
		if d, ok := results[dep]; ok && d.Outcome != "accepted" {
			return i18n.T("run.all.dependency", dep, d.Outcome)
		}
	}
	return ""
}

// reserveRun names a new run and keeps its folder, so two runs started in
// the same second do not take the same name.
func reserveRun(root string) string {
	run := newRunID(root)
	_ = os.MkdirAll(loop.Dir(root, run), 0o755)
	return run
}

// batchReport is the summary of a batch: each change, in order, how it
// ended and what it cost.
func batchReport(levels [][]string, results map[string]delivered, spent, budget float64) string {
	var b strings.Builder
	b.WriteString("# Lote de entregas\n\n| Nível | Mudança | Resultado | Execução | US$ | Por quê |\n|---|---|---|---|---|---|\n")
	counts := map[string]int{}
	for i, level := range levels {
		for _, id := range level {
			d, ok := results[id]
			if !ok {
				d = delivered{Change: id, Outcome: "não iniciada"}
			}
			counts[d.Outcome]++
			run := ""
			if d.Run != "" {
				run = "`" + d.Run + "`"
			}
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %.2f | %s |\n", i, id, d.Outcome, run, d.CostUSD, strings.ReplaceAll(d.Why, "|", "/"))
		}
	}
	var parts []string
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
	}
	fmt.Fprintf(&b, "\n%s · US$ %.2f", strings.Join(parts, " · "), spent)
	if budget > 0 {
		fmt.Fprintf(&b, " de US$ %.2f", budget)
	}
	b.WriteString(" · `gofi report <execução>` para cada uma\n")
	return b.String()
}

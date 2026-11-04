package cli

import (
	"bytes"
	"context"
	"encoding/json"
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
	// Resume is the batch to go on with: what it accepted stays, a run it
	// left midway goes on, and the rest runs again.
	Resume string
}

// delivered is one change of a batch, and how it ended.
type delivered struct {
	Change  string  `json:"change"`
	Run     string  `json:"run,omitempty"`
	Outcome string  `json:"outcome"`
	Why     string  `json:"why,omitempty"`
	CostUSD float64 `json:"cost_usd"`
}

// batchSchema is the version of a batch's state.
const batchSchema = "gofi-batch/v1"

// running is a change whose run is under way: what a batch cut short says.
const running = "running"

// batch is the state of a gofi run --all, written as each change ends: what
// --resume goes on from.
type batch struct {
	Schema string     `json:"schema"`
	ID     string     `json:"id"`
	Levels [][]string `json:"levels"`
	// Done is each change's last word: running while its run is under way.
	Done []delivered `json:"done"`
	// Spent is what each run of the batch spent — every run, those a resume
	// ran anew too: the batch's budget is theirs together.
	Spent map[string]float64 `json:"spent"`
}

func batchPath(root, id string) string {
	return filepath.Join(root, filepath.FromSlash(BatchesDir), id+".json")
}

func loadBatch(root, id string) (*batch, error) {
	data, err := os.ReadFile(batchPath(root, id))
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.T("run.all.no_batch", id, BatchesDir))
	}
	var b batch
	if err := json.Unmarshal(data, &b); err != nil || b.Schema != batchSchema {
		return nil, fmt.Errorf("%s: not a batch of gofi run --all (%v)", batchPath(root, id), err)
	}
	return &b, nil
}

func (b *batch) save(root string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	p := batchPath(root, b.ID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// job is one change a batch delivers: a new run, or one it goes on with.
type job struct {
	Change, Run string
	// Resume goes on with Run instead of starting it.
	Resume   bool
	Worktree bool
	Budget   float64
}

// startChange delivers one change of a batch and returns how its run ended.
// gofi run starts each in a process of its own — a run moves the process's
// environment to its worktree —; a test runs it in its own.
var startChange = func(ctx context.Context, root string, j job, o allOpts, progress io.Writer) (*loop.State, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	change, run := j.Change, j.Run
	args := []string{"run", change, "--events", "--run-id", run}
	if j.Resume {
		// A resumed run keeps the tree it started in.
		args = []string{"run", "--resume", run, "--events"}
	} else if j.Worktree {
		args = append(args, "--worktree")
	}
	if j.Budget > 0 {
		args = append(args, "--budget", strconv.FormatFloat(j.Budget, 'f', 2, 64))
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
// the changes that do not depend on it go on. With o.Resume, a batch goes
// on: its accepted changes stay accepted — spent, not run again —, a run it
// left midway, or accepted and not landed, goes on, and every other change
// runs anew.
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
	b := &batch{Schema: batchSchema, ID: "b-" + time.Now().Format("20060102-150405"), Levels: levels, Spent: map[string]float64{}}
	prior := map[string]delivered{}
	if o.Resume != "" {
		old, err := loadBatch(root, o.Resume)
		if err != nil {
			return err
		}
		b.ID = old.ID
		maps.Copy(b.Spent, old.Spent)
		for _, d := range old.Done {
			prior[d.Change] = d
		}
		fmt.Fprintln(out, i18n.T("run.all.resumed", b.ID))
	}
	fmt.Fprintln(out, i18n.T("run.all.started", len(changes), len(levels), parallel))

	var (
		mu      sync.Mutex
		results = map[string]delivered{}
	)
	spent := func() float64 {
		total := 0.0
		for _, usd := range b.Spent {
			total += usd
		}
		return total
	}
	// record keeps a change's last word, in results unless it is under way.
	record := func(d delivered) {
		if d.Outcome != running {
			results[d.Change] = d
		}
		// A run's cost only grows: a run that stopped with no state to
		// read keeps what it was known to have spent.
		if d.Run != "" {
			b.Spent[d.Run] = max(b.Spent[d.Run], d.CostUSD)
		}
		b.Done = slices.DeleteFunc(b.Done, func(x delivered) bool { return x.Change == d.Change })
		b.Done = append(b.Done, d)
		if err := b.save(root); err != nil {
			fmt.Fprintf(out, "  %s: %v\n", batchPath(root, b.ID), err)
		}
	}
	ended := func(d delivered) {
		mu.Lock()
		defer mu.Unlock()
		record(d)
		fmt.Fprintf(out, "  [%s] %s — US$ %.2f %s\n", d.Change, d.Outcome, d.CostUSD, d.Why)
	}
	// What the batch resumed accepted — archived since, or kept open by
	// --no-archive — is done; a dependency on it holds.
	for id, d := range prior {
		if d.Outcome == "accepted" {
			mu.Lock()
			record(d)
			mu.Unlock()
			if _, open := changes[id]; open {
				fmt.Fprintf(out, "  [%s] %s\n", id, i18n.T("run.all.kept", d.Run))
			}
		}
	}
	for _, level := range levels {
		sem := make(chan struct{}, parallel)
		var wg sync.WaitGroup
		for _, id := range level {
			c := changes[id]
			mu.Lock()
			_, done := results[id]
			mu.Unlock()
			if done {
				continue
			}
			if why := skipReason(c, results, &mu); why != "" {
				ended(delivered{Change: id, Outcome: "skipped", Why: why})
				continue
			}
			sem <- struct{}{}
			// Each run may spend what is left of the batch's budget when it
			// starts: the runs beside it spend from the same.
			j := job{Change: id, Worktree: parallel > 1}
			mu.Lock()
			// A run the batch left midway — or accepted, not landed — goes
			// on, its budget what is left plus what it spent; any other is
			// run anew.
			var spentBefore float64
			if p, ok := prior[id]; ok && p.Run != "" {
				if s, err := loop.LoadState(root, p.Run); err == nil {
					// What it spent is the batch's, the run going on or not —
					// a batch cut short never wrote it.
					b.Spent[p.Run] = s.CostUSD
					if s.Node != "" || landable(s) {
						j.Run, j.Resume, spentBefore = p.Run, true, s.CostUSD
					}
				}
			}
			left := budget - spent()
			mu.Unlock()
			if budget > 0 && left <= 0 {
				<-sem
				ended(delivered{Change: id, Outcome: "skipped", Why: i18n.T("run.all.budget", budget)})
				continue
			}
			if budget == 0 {
				left = 0
			} else {
				left += spentBefore
			}
			j.Budget = left
			if !j.Resume {
				j.Run = reserveRun(root)
			}
			run := j.Run
			mu.Lock()
			record(delivered{Change: id, Run: run, Outcome: running})
			mu.Unlock()
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				s, err := startChange(ctx, root, j, o, out)
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
	report := batchReport(b.ID, levels, results, spent(), budget)
	p := filepath.Join(root, filepath.FromSlash(BatchesDir), b.ID+".md")
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
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("run.all.not_all", failed, len(results)) + " — " + i18n.T("run.all.how_resume", b.ID))}
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
// ended and what it cost — and, resumed, the ones it accepted before and
// archived since.
func batchReport(id string, levels [][]string, results map[string]delivered, spent, budget float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Lote de entregas `%s`\n\n| Nível | Mudança | Resultado | Execução | US$ | Por quê |\n|---|---|---|---|---|---|\n", id)
	counts := map[string]int{}
	row := func(level string, d delivered) {
		counts[d.Outcome]++
		run := ""
		if d.Run != "" {
			run = "`" + d.Run + "`"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %.2f | %s |\n", level, d.Change, d.Outcome, run, d.CostUSD, strings.ReplaceAll(d.Why, "|", "/"))
	}
	listed := map[string]bool{}
	for i, level := range levels {
		for _, id := range level {
			d, ok := results[id]
			if !ok {
				d = delivered{Change: id, Outcome: "não iniciada"}
			}
			listed[id] = true
			row(strconv.Itoa(i), d)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(results)) {
		if !listed[id] {
			row("—", results[id])
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

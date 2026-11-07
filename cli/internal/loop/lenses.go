package loop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/joaoprofile/gofi/cli/internal/role"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// readOnly reports whether a role changes no file: what may run beside
// another on the same tree.
func readOnly(rl role.Role) bool {
	return len(rl.Contract.Tools) > 0 && !slices.Contains(rl.Contract.Tools, "edit") &&
		!slices.Contains(rl.Contract.Tools, "write") && !slices.Contains(rl.Contract.Tools, "shell")
}

// lensable reports whether a node's roles run as lenses of one audit: the
// node runs in parallel, more than one role answers, and none writes.
func lensable(n Node, roles []role.Role) bool {
	if !n.Parallel || len(roles) < 2 {
		return false
	}
	for _, rl := range roles {
		if !readOnly(rl) {
			return false
		}
	}
	return true
}

// runLenses runs the roles of an audit at once, as lenses of one iteration
// (0002, D17): each in a session of its own, writing its own verdict, at
// most loop.audit_parallel at a time. Then, in order, their verdicts merge
// into the context's — by fingerprint, without a model.
func (r *Runner) runLenses(ctx context.Context, name string, roles []role.Role, s *State) error {
	s.Iter++
	s.Agent = name
	s.Lenses = nil
	for _, rl := range roles {
		s.Lenses = append(s.Lenses, rl.Name)
	}
	type job struct {
		a    *AgentState
		turn Turn
		res  TurnResult
		err  error
	}
	jobs := make([]*job, len(roles))
	for i, rl := range roles {
		tier := r.tierOf(rl.Name, s)
		s.Worked = append(s.Worked, Ran{Node: name, Role: rl.Name})
		id := fmt.Sprintf("%s-%s", AgentID(name, s.Iter), rl.Name)
		elicit := filepath.ToSlash(filepath.Join(RunsDir, s.Run, "elicit-"+id+".json"))
		a := &AgentState{ID: id, Node: name, Role: rl.Name, Tier: tier, Iter: s.Iter, Status: AgentWorking, Started: r.now()}
		_ = saveAgent(r.Root, s.Run, a)
		jobs[i] = &job{a: a, turn: Turn{Node: name, Role: rl.Name, Agent: id, Iter: s.Iter, Prompt: withChange(r.Brief(rl, s, elicit), r.work(), s, rl), ElicitFile: elicit, Tier: tier, Tools: rl.Contract.Tools, BudgetUSD: r.Policy.turnBudget(s.CostUSD, len(roles))}}
	}
	limit := r.Policy.AuditParallel
	if limit <= 0 || limit > len(jobs) {
		limit = len(jobs)
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			tctx, why, cancel := r.Policy.turnContext(ctx, j.turn.Role)
			j.res, j.err = r.Agent.Run(tctx, j.turn)
			cancel()
			j.err = why(j.err)
		}()
	}
	wg.Wait()
	var failed []error
	for _, j := range jobs {
		s.CostUSD += j.res.CostUSD
		j.a.Ended, j.a.CostUSD, j.a.Seconds, j.a.Status, j.a.Usage = r.now(), j.res.CostUSD, j.res.Seconds, AgentDone, j.res.Usage
		if j.err != nil {
			j.a.Status, j.a.Error = AgentFailed, j.err.Error()
			failed = append(failed, fmt.Errorf("%s: %w", j.turn.Role, j.err))
		}
		_ = saveAgent(r.Root, s.Run, j.a)
	}
	if len(failed) > 0 {
		return errors.Join(failed...)
	}
	return r.mergeLenses(s, name)
}

// mergeLenses reads the verdict each lens of this iteration wrote and writes
// the context's, merged. A lens that wrote none stops the run, as an audit
// that wrote none does.
func (r *Runner) mergeLenses(s *State, name string) error {
	lenses := append([]string{}, s.Lenses...)
	sort.Strings(lenses)
	var vs []*verdict.Verdict
	var missing []string
	for _, l := range lenses {
		v, err := r.Verdicts.ReadLens(s.Context, l)
		if err != nil {
			return err
		}
		if v == nil || v.Run != s.Run || v.Iter != s.Iter {
			missing = append(missing, l)
			continue
		}
		vs = append(vs, v)
	}
	if len(missing) > 0 {
		return fmt.Errorf("a lente %s não gravou o veredicto desta execução (verdict_write com lens, run %s, iter %d)", strings.Join(missing, ", "), s.Run, s.Iter)
	}
	merged, conflicts := verdict.Merge(s.Context, s.Run, s.Iter, vs)
	if err := r.Verdicts.Write(merged); err != nil {
		return err
	}
	s.Ambiguous = conflicts
	why := fmt.Sprintf("%d lentes, %d achados", len(vs), len(merged.Findings))
	if len(conflicts) > 0 {
		why += fmt.Sprintf(", %d com dono divergente (vale o do contrato)", len(conflicts))
	}
	r.emit(s, Event{Ev: "merge", Node: name, Why: why})
	return nil
}

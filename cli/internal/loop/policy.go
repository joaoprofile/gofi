package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// Policy is the project's convergence policy (0002, D8): what a run may
// spend, how many times it goes back to work, the verdict it accepts on and
// when a role that does not progress runs one tier up. The zero Policy is
// the default: no budget, only the flow's limits, clean, on_no_progress.
type Policy struct {
	MaxIterations int
	BudgetUSD     float64
	Accept        string
	Escalate      string
	// AuditParallel is how many lenses of an audit run at once; 0, all.
	AuditParallel int
	// TurnBudgetUSD bounds what one agent turn may spend; 0, only what is
	// left of BudgetUSD.
	TurnBudgetUSD float64
	// TurnTimeout bounds how long one agent turn may take; 0, no bound.
	TurnTimeout time.Duration
}

// turnBudget is what the next turns may spend, shared by n of them at once:
// the per-turn limit, or what is left of the run's budget when lower. 0 is no
// limit.
func (p Policy) turnBudget(spent float64, n int) float64 {
	b := p.TurnBudgetUSD
	if p.BudgetUSD > 0 {
		left := (p.BudgetUSD - spent) / float64(max(n, 1))
		if b == 0 || left < b {
			b = left
		}
	}
	return max(b, 0)
}

// turnContext bounds a turn by the policy's timeout, and says why it was cut.
func (p Policy) turnContext(ctx context.Context, role string) (context.Context, func(err error) error, context.CancelFunc) {
	if p.TurnTimeout <= 0 {
		return ctx, func(err error) error { return err }, func() {}
	}
	tctx, cancel := context.WithTimeout(ctx, p.TurnTimeout)
	why := func(err error) error {
		if err != nil && errors.Is(tctx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			return fmt.Errorf("o turno de %s passou de %s e foi cortado: %w", role, p.TurnTimeout, ErrTurnTimeout)
		}
		return err
	}
	return tctx, why, cancel
}

// ErrTurnTimeout is a turn cut by the policy's timeout.
var ErrTurnTimeout = errors.New("turn timeout")

// The verdicts a change is accepted on, and when a tier is raised.
const (
	AcceptClean   = "clean"
	AcceptCaveats = "ressalvas"

	EscalateOnNoProgress  = "on_no_progress"
	EscalateOnFirstReject = "on_first_reject"
	EscalateNever         = "never"
)

// stall is how many judgments without progress in a row raise a tier: the
// retry starts on the same tier, with the findings — a higher tier answers
// a lack of progress, not the first failure (0002, D8).
const stall = 2

// Check reports a policy whose values gofi does not know.
func (p Policy) Check() error {
	switch {
	case p.MaxIterations < 0:
		return fmt.Errorf("loop.max_iterations: %d — a positive number, or 0 for the flow's limits", p.MaxIterations)
	case p.TurnBudgetUSD < 0:
		return fmt.Errorf("loop.turn_budget_usd: %v — a positive amount, or 0 for no per-turn limit", p.TurnBudgetUSD)
	case p.TurnTimeout < 0:
		return fmt.Errorf("loop.turn_timeout: %v — a positive duration, or 0 for no limit", p.TurnTimeout)
	case p.BudgetUSD < 0:
		return fmt.Errorf("loop.budget_usd: %v — a positive amount, or 0 for no budget", p.BudgetUSD)
	case p.Accept != "" && p.Accept != AcceptClean && p.Accept != AcceptCaveats:
		return fmt.Errorf("loop.accept: %q — clean or ressalvas", p.Accept)
	case p.Escalate != "" && p.Escalate != EscalateOnNoProgress && p.Escalate != EscalateOnFirstReject && p.Escalate != EscalateNever:
		return fmt.Errorf("loop.escalate: %q — on_no_progress, on_first_reject or never", p.Escalate)
	}
	return nil
}

// accepts reports whether the last verdict is one the policy accepts.
func (p Policy) accepts(verdict string) bool {
	return verdict == "clean" || verdict == "caveats" && p.Accept == AcceptCaveats
}

// Ran is a role's turn on a node, kept until the audit judges the work.
type Ran struct {
	Node string `json:"node"`
	Role string `json:"role"`
}

// tierOf is the tier a role runs on now: its contract's, raised as many
// times as it did not progress, up to deep.
func (r *Runner) tierOf(name string, s *State) host.Tier {
	base := host.Standard
	for _, rl := range r.Roles {
		if rl.Name != name {
			continue
		}
		if rl.Contract.Tier != "" {
			base = rl.Contract.Tier
		}
		if r.wide(rl, s) != "" {
			base = host.Deep
		}
	}
	return raise(base, s.Raised[name])
}

// wide says why a role runs deep from the start: it writes, on the
// standard tier, code that intake.WideReach contexts or more depend on
// besides the change's own — a change wide enough to need it (0002, D11).
// Empty when it is not.
func (r *Runner) wide(rl role.Role, s *State) string {
	c := rl.Contract
	if c.ReadsOnly || c.Tier != host.Standard || len(s.Reach) < intake.WideReach {
		return ""
	}
	return fmt.Sprintf("impacto em %d contextos além de %s (%s): standard → deep", len(s.Reach), s.Context, strings.Join(first(s.Reach, 3), ", "))
}

func first(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func raise(t host.Tier, by int) host.Tier {
	i := slices.Index(host.Tiers, t)
	if i < 0 {
		return t
	}
	return host.Tiers[min(i+by, len(host.Tiers)-1)]
}

// weigh is the policy's answer to a judgment that refused the work: progress
// resets the count, a lack of it adds up and, at stall in a row — at once on
// on_first_reject —, raises the roles judged one tier. It reports whether
// the run is stuck: no progress, and no role left to raise, or a policy that
// raises none.
func (r *Runner) weigh(s *State, stalled *int, progress bool, judged []string) bool {
	need := stall
	if r.Policy.Escalate == EscalateOnFirstReject {
		need, progress = 1, false
	}
	if progress {
		*stalled = 0
		return false
	}
	*stalled++
	if *stalled < need {
		return false
	}
	*stalled = 0
	if r.Policy.Escalate == EscalateNever {
		return true
	}
	raised := false
	for _, name := range judged {
		from := r.tierOf(name, s)
		if from == host.Deep {
			continue
		}
		if s.Raised == nil {
			s.Raised = map[string]int{}
		}
		s.Raised[name]++
		raised = true
		r.emit(s, Event{Ev: "escalate", Node: s.Agent, To: name, Why: fmt.Sprintf("sem progresso: %s → %s", from, r.tierOf(name, s))})
	}
	return !raised
}

// judged are the roles whose work a judgment weighs: the sensors judge the
// turn right before them; the audit, every turn since the last audit but
// its own.
func judged(s *State, bySensor bool) []string {
	var out []string
	for _, w := range s.Worked {
		if (w.Node == s.Agent) == bySensor && !slices.Contains(out, w.Role) {
			out = append(out, w.Role)
		}
	}
	return out
}

// weighSensors weighs a sensor run. A failure is progress when it is
// further on — a later stage fails — or weighs less in the same stage.
func (r *Runner) weighSensors(s *State, stages []string) {
	res := s.Sensor
	if len(res.Failed) == 0 {
		s.SensorMark, s.SensorStalled, s.Stuck = nil, 0, false
		return
	}
	mark := slices.Index(stages, res.Failed[0]) * 1000
	for _, f := range res.Findings {
		mark -= weight[f.Severity]
	}
	progress := s.SensorMark == nil || mark > *s.SensorMark
	s.SensorMark = &mark
	s.Stuck = r.weigh(s, &s.SensorStalled, progress, judged(s, true))
	r.amend(s)
}

// weighVerdict weighs the audit's verdict, once judge has read it: the work
// it judged is done with, accepted or not.
func (r *Runner) weighVerdict(s *State) {
	defer func() { s.Worked = nil }()
	if r.Policy.accepts(s.Verdict) {
		s.Stalled, s.Stuck = 0, false
		return
	}
	progress := s.Score != nil && (s.Previous == nil || *s.Score < *s.Previous)
	s.Stuck = r.weigh(s, &s.Stalled, progress, judged(s, false))
	r.amend(s)
}

// amend names the proposal a run that cannot go on writes for the person
// (0002, D9): the contract is wrong — a finding of the spec or the
// discovery —, or the work is stuck. The contract never changes on its own;
// what to change in it is proposed, and the person decides.
func (r *Runner) amend(s *State) {
	if s.Proposal != "" {
		return
	}
	contract := slices.ContainsFunc(s.Open, func(f Finding) bool { return f.Owner == "spec" || f.Owner == "pd" })
	if !contract && !s.Stuck {
		return
	}
	n := 1
	for _, dir := range []string{r.Root, r.work()} {
		entries, _ := os.ReadDir(filepath.Join(dir, filepath.FromSlash(ProposalsDir)))
		for _, e := range entries {
			num := strings.TrimSuffix(strings.TrimPrefix(e.Name(), s.Context+"-"), ".md")
			if k, err := strconv.Atoi(num); err == nil && k >= n {
				n = k + 1
			}
		}
	}
	s.Proposal = fmt.Sprintf("%s/%s-%d.md", ProposalsDir, s.Context, n)
}

// ProposalsDir is where a run leaves the amendment it proposes, under the
// project root.
const ProposalsDir = ".gofi/proposals"

// keepProposal brings the proposal a turn wrote in the run's worktree to the
// project root, where the person reads it: the worktree of a run that did
// not land is not where anyone looks.
func (r *Runner) keepProposal(s *State) {
	if s.Proposal == "" {
		return
	}
	src := filepath.Join(r.work(), filepath.FromSlash(s.Proposal))
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	if r.work() != r.Root {
		dst := filepath.Join(r.Root, filepath.FromSlash(s.Proposal))
		if os.MkdirAll(filepath.Dir(dst), 0o755) != nil || os.WriteFile(dst, data, 0o644) != nil {
			return
		}
	}
	r.emit(s, Event{Ev: "proposal", Node: s.Agent, To: s.Proposal})
}

// spent reports why the policy lets no agent run again: the budget is gone.
func (r *Runner) spent(s *State) (string, bool) {
	if b := r.Policy.BudgetUSD; b > 0 && s.CostUSD >= b {
		return fmt.Sprintf("orçamento de US$ %.2f gasto (US$ %.2f)", b, s.CostUSD), true
	}
	return "", false
}

// rework reports whether an edge sends the work back after a sensor or the
// audit refused it: a cycle of the flow — it has a limit — out of a sensor or
// the policy. loop.max_iterations bounds them all together.
func (r *Runner) rework(from, to string) bool {
	k := r.Flow.Nodes[from].Kind
	return (k == KindSensor || k == KindPolicy) && r.Flow.limit(from, to) > 0
}

func (p Policy) acceptName() string {
	if p.Accept == "" {
		return AcceptClean
	}
	return p.Accept
}

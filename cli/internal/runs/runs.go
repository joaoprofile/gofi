// Package runs records each plan a conductor carries out — gofi chat, gofi
// ask — in .gofi/runs/, outside git (0001, D10): the request, what the intake
// decided and why, each phase with its tier, model, time and cost, the QA's
// verdict and how the plan ended. It is the ground truth the calibration of
// the intake reads: what was decided against what it cost and how it went.
//
// Written after every step, atomically, so a plan cut short is on record too.
package runs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/intake"
)

// Schema names the shape of a run file.
const Schema = "gofi.run/v1"

// Dir is where runs are kept, relative to the project root.
const Dir = ".gofi/runs"

// Outcomes of a plan.
const (
	OutcomeRunning   = "running"
	OutcomeDone      = "done"
	OutcomeStopped   = "stopped"   // the person stopped it at a review
	OutcomeFailed    = "failed"    // a phase failed
	OutcomeEscalated = "escalated" // the QA failed it past the last retry
)

// Run is one plan carried out.
type Run struct {
	Schema    string    `json:"schema"`
	ID        string    `json:"id"`
	Started   time.Time `json:"started"`
	Finished  time.Time `json:"finished,omitempty"`
	Surface   string    `json:"surface"` // chat, ask
	Request   string    `json:"request"`
	Intent    string    `json:"intent"`
	Artifact  string    `json:"artifact"`
	Context   string    `json:"context,omitempty"`
	Signals   []string  `json:"signals"`
	Questions int       `json:"questions"`
	// Assumed are the defaults taken without an answer.
	Assumed []string `json:"assumed,omitempty"`
	// IntakeUSD is what the light tier cost the intake, when consulted.
	IntakeUSD float64 `json:"intake_usd,omitempty"`
	Phases    []Phase `json:"phases"`
	Outcome   string  `json:"outcome"`
	// Verdict is the QA's last status for the context, when it ran.
	Verdict string  `json:"verdict,omitempty"`
	CostUSD float64 `json:"cost_usd"`

	path string
}

// Phase is one phase as it ran.
type Phase struct {
	Role    string  `json:"role"`
	Tier    string  `json:"tier"`
	Model   string  `json:"model,omitempty"`
	Why     string  `json:"why"`
	TierWhy string  `json:"tier_why"`
	Seconds float64 `json:"seconds"`
	CostUSD float64 `json:"cost_usd"`
	OK      bool    `json:"ok"`
	Error   string  `json:"error,omitempty"`
	// Asked is how many decisions an elicitation left for the person, and
	// Assumed how many of them went on the recommendation, unanswered.
	Asked   int `json:"asked,omitempty"`
	Assumed int `json:"assumed,omitempty"`
}

// Start opens the record of a plan: what the intake decided, before any
// phase runs.
func Start(root, surface string, r *intake.Result) (*Run, error) {
	now := time.Now().UTC()
	run := &Run{
		Schema: Schema, Started: now, Surface: surface,
		Request: r.Request, Intent: r.Intent, Artifact: r.Artifact,
		Signals: append([]string{}, r.Signals...), Questions: len(r.Questions),
		Assumed: r.Assumptions, Phases: []Phase{}, Outcome: OutcomeRunning,
	}
	if r.Context != nil {
		run.Context = r.Context.Name
	}
	if r.Spent != nil {
		run.IntakeUSD = r.Spent.USD
	}
	run.ID = now.Format("20060102-150405") + "-" + slug(r.Request)
	dir := filepath.Join(root, filepath.FromSlash(Dir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	run.path = filepath.Join(dir, run.ID+".json")
	return run, run.save()
}

// Phase records a phase that ended.
func (r *Run) Phase(p Phase) error {
	r.Phases = append(r.Phases, p)
	r.CostUSD += p.CostUSD
	return r.save()
}

// Finish closes the record with the plan's outcome and the QA's verdict.
func (r *Run) Finish(outcome, verdict string) error {
	r.Outcome, r.Verdict, r.Finished = outcome, verdict, time.Now().UTC()
	return r.save()
}

// Path is the file the run is written to.
func (r *Run) Path() string { return r.path }

// save writes the record through a temporary file, so a reader never sees
// half of it.
func (r *Run) save() error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.path)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug is a short, file-safe name from the request.
func slug(s string) string {
	s = nonSlug.ReplaceAllString(strings.ToLower(s), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.Trim(s[:40], "-")
	}
	if s == "" {
		return "pedido"
	}
	return s
}

// Load reads a run file.
func Load(path string) (*Run, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Run
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	r.path = path
	return &r, nil
}

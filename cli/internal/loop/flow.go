// Package loop runs the delivery of an approved change as a declared graph
// (decision 0002, D21): nodes are agents, sensors, gates, the policy and the
// terminals; edges carry conditions on facts the runtime computes, never on
// what a model says. The graph is data — a team changes its process by
// editing a YAML file — and every cycle in it is bounded.
package loop

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Schema names the format of a flow file.
const Schema = "gofi-flow/v1"

// Node kinds the runtime executes. A kind enters this list with the code
// that runs it, never before.
const (
	KindAgent    = "agent"    // a role's turn, in a session of its own
	KindSensor   = "sensor"   // deterministic checks, run by gofi, no model
	KindGate     = "gate"     // a decision that is the person's
	KindPolicy   = "policy"   // progress, oscillation and the verdict, on the record
	KindTerminal = "terminal" // where a run ends
)

// Outcomes a terminal ends a run with.
var outcomes = map[string]bool{"accepted": true, "impasse": true, "exhausted": true, "stopped": true}

// Sensor stages, cheapest first. A sensor node runs the ones it lists.
var stages = map[string]bool{"build": true, "test": true, "static": true, "graph": true, "trace": true, "integrity": true,
	// The optional stages (0002, D6, S7): checked when the project names a
	// command for them in loop.sensors, not checked — never failed — when
	// it does not.
	"security": true, "coverage": true}

// Flow is one declared graph.
type Flow struct {
	Schema string          `yaml:"schema"`
	Name   string          `yaml:"name"`
	Start  string          `yaml:"start"`
	Nodes  map[string]Node `yaml:"nodes"`
	Edges  []Edge          `yaml:"edges"`
	Limits []Limit         `yaml:"limits"`
	// OnLimit is the terminal a run goes to when an edge's condition holds
	// and its limit is spent. A limit never sends a run down the next edge
	// instead: that is how failing code would reach an audit.
	OnLimit string `yaml:"on_limit"`

	// Source is where the flow was read from, for messages.
	Source string `yaml:"-"`
}

// Node is one step of a flow.
type Node struct {
	Kind  string `yaml:"kind"`
	Role  string `yaml:"role,omitempty"`  // agent: one role, fixed
	Stage string `yaml:"stage,omitempty"` // agent: the roles its facts dispatch
	// Optional lets a stage nobody answers pass: a change that has no plan
	// or tests of its own — a screen, a pipeline — goes through it.
	Optional bool `yaml:"optional,omitempty"`
	// Parallel runs the roles a stage dispatches at once when none of them
	// writes: the lenses of an audit (0002, D17).
	Parallel bool     `yaml:"parallel,omitempty"`
	Stages   []string `yaml:"stages,omitempty"`  // sensor
	Outcome  string   `yaml:"outcome,omitempty"` // terminal
}

// Edge is a transition. Edges from a node are tried in the order written and
// the first whose condition holds is taken; one with no condition is the
// default and comes last.
type Edge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	When When   `yaml:"when,omitempty"`
}

// Limit bounds how many times an edge is taken in one run: what keeps every
// cycle of a flow from running forever.
type Limit struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Max  int    `yaml:"max"`
}

// When is a condition: every field set must hold. The vocabulary is closed —
// each field is a fact the runtime computes deterministically.
type When struct {
	// SensorFailed holds when the last sensor run failed in one of the
	// stages listed; an empty list means any stage.
	SensorFailed *[]string `yaml:"sensor_failed,omitempty"`
	// Verdict is the last audit: clean (approved, no caveat), caveats
	// (approved with majors or minors open) or rejected.
	Verdict string `yaml:"verdict,omitempty"`
	// Owner holds when an open finding is owned by one of these: eng, spec, pd.
	Owner []string `yaml:"owner,omitempty"`
	// Progress holds when the open findings weigh less than at the iteration
	// before — or there was none before.
	Progress *bool `yaml:"progress,omitempty"`
	// Oscillation holds when a finding resolved before is open again.
	Oscillation *bool `yaml:"oscillation,omitempty"`
	// Accept holds when the last verdict is one loop.accept accepts: clean,
	// or approved with caveats under ressalvas.
	Accept *bool `yaml:"accept,omitempty"`
	// TestsOnly holds when every finding open is placed in a test file: the
	// tests' to fix, not the code's.
	TestsOnly *bool `yaml:"tests_only,omitempty"`
	// Stuck holds when the work does not progress and no tier is left to
	// raise — or the policy raises none (loop.escalate).
	Stuck *bool `yaml:"stuck,omitempty"`
	// Blocked holds when the agent stopped on a decision and wrote it down.
	Blocked *bool `yaml:"blocked,omitempty"`
	// Answered holds when the person answered the gate.
	Answered *bool `yaml:"answered,omitempty"`
}

// Empty reports whether the condition sets nothing: the default edge.
func (w When) Empty() bool {
	return w.SensorFailed == nil && w.Verdict == "" && len(w.Owner) == 0 &&
		w.Progress == nil && w.Oscillation == nil && w.Accept == nil && w.TestsOnly == nil && w.Stuck == nil && w.Blocked == nil && w.Answered == nil
}

var (
	verdicts = map[string]bool{"clean": true, "caveats": true, "rejected": true}
	owners   = map[string]bool{"eng": true, "spec": true, "pd": true}
)

//go:embed flows/*.yaml
var builtin embed.FS

// Load reads a flow by name: the project's own (.claude/flows/<name>.yaml)
// when it has one, gofi's built-in otherwise.
func Load(root, name string) (*Flow, error) {
	if !reName.MatchString(name) {
		return nil, fmt.Errorf("%q is not a flow name", name)
	}
	own := layout.Flows().Abs(root, name+".yaml")
	if data, err := os.ReadFile(own); err == nil {
		rel, _ := filepath.Rel(root, own)
		return Parse(data, filepath.ToSlash(rel))
	}
	data, err := builtin.ReadFile("flows/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("no flow %q — gofi has %s", name, strings.Join(Builtin(), ", "))
	}
	return Parse(data, "gofi:"+name)
}

// Builtin lists the flows gofi carries.
func Builtin() []string {
	entries, _ := builtin.ReadDir("flows")
	var out []string
	for _, e := range entries {
		out = append(out, strings.TrimSuffix(e.Name(), ".yaml"))
	}
	sort.Strings(out)
	return out
}

// Parse reads a flow file. It does not validate it: Check does.
func Parse(data []byte, source string) (*Flow, error) {
	var f Flow
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}
	f.Source = source
	return &f, nil
}

// outgoing lists the edges from a node, in the order written.
func (f *Flow) outgoing(node string) []Edge {
	var out []Edge
	for _, e := range f.Edges {
		if e.From == node {
			out = append(out, e)
		}
	}
	return out
}

// Walk is the flow's nodes in the order a run first reaches them, from the
// start along the edges as written: how a flow reads top to bottom.
func (f *Flow) Walk() []string {
	seen := map[string]bool{f.Start: true}
	out := []string{f.Start}
	for i := 0; i < len(out); i++ {
		for _, e := range f.outgoing(out[i]) {
			if !seen[e.To] {
				seen[e.To] = true
				out = append(out, e.To)
			}
		}
	}
	return out
}

// limit is the bound of an edge; 0 when it has none.
func (f *Flow) limit(from, to string) int {
	for _, l := range f.Limits {
		if l.From == from && l.To == to {
			return l.Max
		}
	}
	return 0
}

// builds are the artifacts a plan builds: the loop delivers them.
var builds = map[string]bool{"code": true, "ui": true, "infra": true}

// FlowFor names the flow that delivers an artifact — code, ui or infra — and
// reports whether the artifact is one the loop delivers at all. It is one
// flow for all of them: which role implements is the change's surfaces,
// dispatched inside it (0002, D18).
func FlowFor(artifact string) (string, bool) {
	return "delivery", builds[artifact]
}

// Delivers reports whether a plan phase producing artifact is part of a
// delivery the loop carries out: the build itself, or the audit that the
// flow already holds.
func Delivers(artifact string) bool {
	return builds[artifact] || artifact == "audit"
}

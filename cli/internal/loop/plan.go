package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	slash "path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// PlanSchema names the format of a run's plan.
const PlanSchema = "gofi-plan/v1"

// Plan is what the plan stage decomposes a change into (0002, D17): tasks by
// layer, with the requirements each realizes, the files it writes and the
// tasks it waits for — in the order they are done.
type Plan struct {
	Schema string `json:"schema"`
	Tasks  []Task `json:"tasks"`
}

// Task is one piece of the implementation.
type Task struct {
	ID      string   `json:"id"`
	Layer   string   `json:"layer"`
	Summary string   `json:"summary"`
	Reqs    []string `json:"reqs"`
	// Files are the paths the task writes, relative to the project root: a
	// file, or a folder ending in /.
	Files   []string `json:"files"`
	Depends []string `json:"depends,omitempty"`
}

// PlanExample is the shape of a plan, as the plan turn is told to write it.
var PlanExample = fmt.Sprintf(`{"schema": %q, "tasks": [{"id": "T1", "layer": "domain", "summary": "...", "reqs": ["REQ-01"], "files": ["backend/domain/<ctx>/"], "depends": []}]}`, PlanSchema)

// planFile is where the plan of a run is written, relative to the tree the
// agents work in.
func planFile(run string) string { return slash.Join(RunsDir, run, "plan.json") }

// readPlan reads and checks the plan a turn wrote; nil, with no error, when
// it wrote none.
func readPlan(dir, rel string) (*Plan, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if p.Schema != "" && p.Schema != PlanSchema {
		return nil, fmt.Errorf("%s: schema %q, expected %s", rel, p.Schema, PlanSchema)
	}
	p.Schema = PlanSchema
	if len(p.Tasks) == 0 {
		return nil, fmt.Errorf("%s: a plan with no task", rel)
	}
	seen := map[string]bool{}
	var problems []string
	for i, t := range p.Tasks {
		where := fmt.Sprintf("task %d (%s)", i+1, t.ID)
		switch {
		case t.ID == "" || seen[t.ID]:
			problems = append(problems, where+": an id of its own is required")
		case !slices.Contains(role.Layers, t.Layer):
			problems = append(problems, fmt.Sprintf("%s: layer %q — %s", where, t.Layer, strings.Join(role.Layers, ", ")))
		case len(t.Files) == 0:
			problems = append(problems, where+": the files it writes are required")
		}
		for _, d := range t.Depends {
			if !seen[d] {
				problems = append(problems, fmt.Sprintf("%s: depends on %q, which is not a task before it", where, d))
			}
		}
		seen[t.ID] = true
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s:\n  %s", rel, strings.Join(problems, "\n  "))
	}
	return &p, nil
}

// layers are the plan's layers, in the order its tasks first reach them.
func (p *Plan) layers() []string {
	var out []string
	for _, t := range p.Tasks {
		if !slices.Contains(out, t.Layer) {
			out = append(out, t.Layer)
		}
	}
	return out
}

// layerOf is the layer of the task that writes a file — the file itself, or
// a folder that holds it; a test file belongs to the task whose files share
// its folder, the code it tests. Empty when no task writes it.
func (p *Plan) layerOf(file string, isTest func(string) bool) string {
	file = strings.TrimPrefix(filepath.ToSlash(file), "./")
	for _, t := range p.Tasks {
		for _, f := range t.Files {
			f = strings.TrimPrefix(f, "./")
			switch {
			case f == file,
				strings.HasSuffix(f, "/") && strings.HasPrefix(file, f),
				isTest != nil && isTest(file) && slash.Dir(f) == slash.Dir(file),
				isTest != nil && isTest(file) && strings.HasSuffix(f, "/") && slash.Dir(file)+"/" == f:
				return t.Layer
			}
		}
	}
	return ""
}

// due are the layers with work in this pass: every layer the plan has at
// first; on a retry, the layers the findings fall in — every layer when one
// of them falls in none, or names no file, so nothing refused is left to
// nobody.
func (r *Runner) due(s *State) []string {
	all := s.Plan.layers()
	var files []string
	if s.Sensor != nil && len(s.Sensor.Failed) > 0 {
		if len(s.Sensor.Findings) == 0 {
			return all
		}
		for _, f := range s.Sensor.Findings {
			// A requirement nothing implements is the work of the tasks
			// that realize it; a scenario with no test, the test role's.
			switch TraceGap(f) {
			case traceTests:
				continue
			case traceImplements:
				layers := s.Plan.layersOf(traceReq(f))
				if len(layers) == 0 {
					return all
				}
				for _, t := range s.Plan.Tasks {
					if slices.Contains(layers, t.Layer) && len(t.Files) > 0 {
						files = append(files, t.Files[0])
					}
				}
				continue
			}
			files = append(files, f.File)
		}
	} else {
		for _, f := range s.Open {
			files = append(files, f.File)
		}
	}
	if len(files) == 0 {
		return all
	}
	var out []string
	for _, f := range files {
		l := ""
		if f != "" {
			l = s.Plan.layerOf(f, r.IsTest)
		}
		if l == "" {
			return all
		}
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	// In the plan's order, not the findings'.
	slices.SortStableFunc(out, func(a, b string) int { return slices.Index(all, a) - slices.Index(all, b) })
	return out
}

// The gaps a trace finding names, by the directive it asks for.
const (
	traceImplements = "gofi:implements"
	traceTests      = "gofi:tests"
)

// TraceGap is the directive a finding of the trace stage asks for, empty
// for any other finding: its rule is "gofi:tests pontos/REQ-03.1".
func TraceGap(f SensorFinding) string {
	if f.Stage != "trace" {
		return ""
	}
	kind, _, _ := strings.Cut(f.Rule, " ")
	if kind == traceImplements || kind == traceTests {
		return kind
	}
	return ""
}

// traceReq is the requirement a trace finding is about: REQ-03.
func traceReq(f SensorFinding) string {
	_, ref, _ := strings.Cut(f.Rule, " ")
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		ref = ref[i+1:]
	}
	if i := strings.Index(ref, "."); i >= 0 {
		ref = ref[:i]
	}
	return ref
}

// layersOf are the layers of the tasks that realize a requirement.
func (p *Plan) layersOf(req string) []string {
	var out []string
	for _, t := range p.Tasks {
		if slices.Contains(t.Reqs, req) && !slices.Contains(out, t.Layer) {
			out = append(out, t.Layer)
		}
	}
	return out
}

// testsDue reports whether the tests are to be written now: no test turn
// has run, the trace found a scenario no test tests, or the audit found
// something in a test file. A failing test is the code's to fix — the test
// reads the contract, not the code.
func (r *Runner) testsDue(s *State) bool {
	if !s.Tested {
		return true
	}
	if s.Sensor != nil {
		for _, f := range s.Sensor.Findings {
			if TraceGap(f) == traceTests {
				return true
			}
		}
	}
	for _, f := range s.Open {
		if f.File != "" && r.IsTest != nil && r.IsTest(f.File) {
			return true
		}
	}
	return false
}

// testsOnly reports whether what refused the work is the tests' alone:
// every gap of a failing trace a scenario with no test, or — the sensors
// green — every open finding of the audit in a test file.
func (r *Runner) testsOnly(s *State) bool {
	if s.Sensor != nil && len(s.Sensor.Failed) > 0 {
		if s.Sensor.Failed[0] != "trace" || len(s.Sensor.Findings) == 0 {
			return false
		}
		for _, f := range s.Sensor.Findings {
			if TraceGap(f) != traceTests {
				return false
			}
		}
		return true
	}
	if len(s.Open) == 0 || r.IsTest == nil {
		return false
	}
	for _, f := range s.Open {
		if f.File == "" || !r.IsTest(f.File) {
			return false
		}
	}
	return true
}

// tasksOf are the plan's tasks a role does in this pass: those of its
// layer, when the layer has work now.
func tasksOf(s *State, rl role.Role) []Task {
	if s.Plan == nil || rl.Contract.Layer == "" {
		return nil
	}
	var out []Task
	for _, t := range s.Plan.Tasks {
		if t.Layer == rl.Contract.Layer {
			out = append(out, t)
		}
	}
	return out
}

// byLayer orders the roles a stage dispatched as the plan orders their
// layers; a role with no layer keeps its place after them.
func byLayer(roles []role.Role, p *Plan) {
	if p == nil {
		return
	}
	order := p.layers()
	rank := func(rl role.Role) int {
		if i := slices.Index(order, rl.Contract.Layer); i >= 0 {
			return i
		}
		return len(order)
	}
	slices.SortStableFunc(roles, func(a, b role.Role) int { return rank(a) - rank(b) })
}

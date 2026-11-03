package role

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Visibility of a role (0002, D19). A user role is a slash command a person
// may invoke; an internal one is run by the flow alone, installed where the
// host does not list it.
const (
	VisibilityUser     = "user"
	VisibilityInternal = "internal"
)

// Stages are where in the loop a role works. A flow's agent node names a
// stage, and the roles of that stage whose triggers hold are the ones that
// run — nobody picks them (0002, D18).
var Stages = []string{"contract", "plan", "implement", "test", "repair", "triage", "audit", "deliver", "learn"}

// Trigger is when a role of a stage runs: every predicate of All holds, and
// at least one of Any when Any is given. A role with no trigger runs whenever
// its stage does.
type Trigger struct {
	All []string `yaml:"all"`
	Any []string `yaml:"any"`
}

// predicates are the facts a trigger can name, each with the values its
// argument takes; nil takes none. The list is closed: a misspelt predicate is
// an error when the contract is read, never a trigger that silently never
// fires.
var predicates = map[string][]string{
	"change.surface": {"backend", "web", "mobile", "infra"},
	"sensor.green":   nil,
	"sensor.failed":  {"build", "test", "static", "graph", "trace", "integrity"},
	"verdict":        {"clean", "caveats", "rejected"},
	"owner":          {"eng", "spec", "pd"},
	// stuck: the work does not progress and no tier is left to raise.
	"stuck": nil,
	// layer(x): the run's plan has work for layer x in this pass — every
	// layer it plans at first, those the findings point at on a retry.
	"layer": Layers,
	// plan.none: the run has no plan — a flow with no plan stage.
	"plan.none": nil,
	// tests.due: the tests are to be written — no test turn yet, or a
	// finding of the audit is in a test file.
	"tests.due": nil,
	// run.reworked: the run went back to work at least once — a sensor or
	// the audit refused it —: something in it is worth learning.
	"run.reworked": nil,
	// diff.touches(x): the code graph says the change touched layer x —
	// a symbol added, changed or removed in it since the run started.
	"diff.touches": Layers,
}

// Layers are what a plan divides the implementation of a backend change
// into, in the order a plan usually takes them: the data, the domain, the
// integrations with what is outside, the API. A role that implements one
// declares it; the run dispatches it when its layer has work.
var Layers = []string{"data", "domain", "integration", "api"}

// checkPredicate reports whether p is a fact a trigger can name.
func checkPredicate(p string) error {
	name, arg, hasArg := strings.Cut(strings.TrimSuffix(p, ")"), "(")
	values, known := predicates[name]
	switch {
	case !known:
		return fmt.Errorf("%q is not a fact (expected %s)", p, strings.Join(predicateNames(), ", "))
	case values == nil && hasArg:
		return fmt.Errorf("%q: %s takes no argument", p, name)
	case values != nil && (!hasArg || !strings.HasSuffix(p, ")")):
		return fmt.Errorf("%q: %s takes one of %s, as %s(x)", p, name, strings.Join(values, ", "), name)
	case values != nil && !slices.Contains(values, arg):
		return fmt.Errorf("%q: %s takes one of %s", p, name, strings.Join(values, ", "))
	}
	return nil
}

func predicateNames() []string {
	out := make([]string, 0, len(predicates))
	for k := range predicates {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Facts are what holds now, as predicates: change.surface(backend),
// sensor.green… Computed by gofi from the contract, the project and the run,
// never by a model.
type Facts map[string]bool

// Choice is a role dispatched, and the facts that made it run.
type Choice struct {
	Role string
	Why  string
}

// Dispatch is the roles of a stage that run on these facts, in a stable
// order — by name — so the same state always dispatches the same roles.
func Dispatch(roles []Role, stage string, facts Facts) []Choice {
	var out []Choice
	for _, r := range roles {
		if r.Contract.Stage != stage {
			continue
		}
		if why, ok := fires(r.Contract.Triggers, facts); ok {
			out = append(out, Choice{Role: r.Name, Why: why})
		}
	}
	return out
}

// Explain is every role of a stage and whether it runs on these facts, with
// why: what `gofi run --explain` shows.
func Explain(roles []Role, stage string, facts Facts) []string {
	var out []string
	for _, r := range roles {
		if r.Contract.Stage != stage {
			continue
		}
		why, ok := fires(r.Contract.Triggers, facts)
		mark := "✗"
		if ok {
			mark = "✓"
		}
		out = append(out, fmt.Sprintf("%s %s — %s", mark, r.Name, why))
	}
	return out
}

// fires reports whether any trigger holds, and why — the predicates that
// held, or, when none did, what each trigger lacked.
func fires(triggers []Trigger, facts Facts) (string, bool) {
	if len(triggers) == 0 {
		return "sem gatilho: roda sempre no estágio", true
	}
	var missed []string
	for _, t := range triggers {
		var held, lacked []string
		for _, p := range t.All {
			if facts[p] {
				held = append(held, p)
			} else {
				lacked = append(lacked, p)
			}
		}
		anyHeld := len(t.Any) == 0
		for _, p := range t.Any {
			if facts[p] {
				held, anyHeld = append(held, p), true
			}
		}
		if !anyHeld {
			lacked = append(lacked, "um de "+strings.Join(t.Any, ", "))
		}
		if len(lacked) == 0 {
			return strings.Join(held, " e "), true
		}
		missed = append(missed, "falta "+strings.Join(lacked, " e "))
	}
	return strings.Join(missed, "; "), false
}

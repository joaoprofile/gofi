package loop

import (
	"strings"
	"testing"
)

func messages(ps []Problem) string {
	var b strings.Builder
	for _, p := range ps {
		kind := "warn"
		if p.Error {
			kind = "error"
		}
		b.WriteString(kind + " " + p.Message + "\n")
	}
	return b.String()
}

// What gofi carries must pass its own check.
func TestTheBuiltinFlowsAreValid(t *testing.T) {
	for _, name := range Builtin() {
		f, err := Load(t.TempDir(), name)
		if err != nil {
			t.Fatal(err)
		}
		if got := Check(f, []string{"gofi-eng", "gofi-ui", "gofi-ops", "gofi-qa"}); len(got) != 0 {
			t.Errorf("%s:\n%s", name, messages(got))
		}
	}
}

func TestAProjectFlowOverridesTheBuiltin(t *testing.T) {
	root := t.TempDir()
	if f, _ := Load(root, "delivery"); f.Source != "gofi:delivery" {
		t.Errorf("source = %s", f.Source)
	}
	writeFlow(t, root, "delivery", "schema: gofi-flow/v1\nname: delivery\nstart: done\nnodes:\n  done: {kind: terminal, outcome: accepted}\n")
	f, err := Load(root, "delivery")
	if err != nil || f.Source != ".claude/flows/delivery.yaml" || len(f.Nodes) != 1 {
		t.Errorf("flow = %+v, %v", f, err)
	}
	if _, err := Load(root, "nenhum"); err == nil {
		t.Error("an unknown flow loaded")
	}
}

const base = `schema: gofi-flow/v1
name: x
start: a
`

func problems(t *testing.T, yaml string, roles ...string) string {
	t.Helper()
	f, err := Parse([]byte(yaml), "test")
	if err != nil {
		return "parse " + err.Error()
	}
	var r []string
	if len(roles) > 0 {
		r = roles
	}
	return messages(Check(f, r))
}

func TestCheckRefusesWhatCannotRun(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"unknown field":           {base + "nodes:\n  a: {kind: terminal, outcome: accepted, cor: azul}\n", "parse"},
		"unknown kind":            {base + "nodes:\n  a: {kind: robo}\n", `kind "robo"`},
		"agent without role":      {base + "nodes:\n  a: {kind: agent}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z}\n", "names its role"},
		"bad stage":               {base + "nodes:\n  a: {kind: sensor, stages: [lint]}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z}\n", `stage "lint"`},
		"bad outcome":             {base + "nodes:\n  a: {kind: terminal, outcome: ok}\n", `outcome "ok"`},
		"edge out of a terminal":  {base + "nodes:\n  a: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: a}\n", "is a terminal"},
		"no default":              {base + "nodes:\n  a: {kind: policy}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z, when: {verdict: clean}}\n", "no default edge"},
		"default not last":        {base + "nodes:\n  a: {kind: policy}\n  y: {kind: terminal, outcome: impasse}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z}\n  - {from: a, to: y, when: {verdict: clean}}\n", "must come last"},
		"bad condition":           {base + "nodes:\n  a: {kind: policy}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z, when: {verdict: bom, owner: [qa]}}\n  - {from: a, to: z}\n", `verdict "bom"`},
		"unknown node":            {base + "nodes:\n  a: {kind: policy}\nedges:\n  - {from: a, to: b}\n", `to "b" is not a node`},
		"unreachable":             {base + "nodes:\n  a: {kind: terminal, outcome: accepted}\n  b: {kind: terminal, outcome: impasse}\n", "b: unreachable"},
		"no way out":              {base + "nodes:\n  a: {kind: agent, role: r}\n  b: {kind: policy}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: b}\n  - {from: b, to: a}\n  - {from: a, to: z, when: {blocked: true}}\nlimits:\n  - {from: b, to: a, max: 2}\non_limit: z\n", "default edge"},
		"unbounded cycle":         {base + "nodes:\n  a: {kind: agent, role: r}\n  b: {kind: policy}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: b}\n  - {from: b, to: z, when: {verdict: clean}}\n  - {from: b, to: a}\n", "cycle [a b] has no limit"},
		"limits without on_limit": {base + "nodes:\n  a: {kind: agent, role: r}\n  b: {kind: policy}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: b}\n  - {from: b, to: z, when: {verdict: clean}}\n  - {from: b, to: a}\nlimits:\n  - {from: b, to: a, max: 2}\n", "on_limit"},
		"limit on no edge":        {base + "nodes:\n  a: {kind: terminal, outcome: accepted}\nlimits:\n  - {from: a, to: q, max: 1}\non_limit: a\n", "no such edge"},
	} {
		if got := problems(t, tc.yaml); !strings.Contains(got, tc.want) {
			t.Errorf("%s: want %q in:\n%s", name, tc.want, got)
		}
	}
}

func TestARoleNotInstalledIsAWarning(t *testing.T) {
	got := problems(t, base+"nodes:\n  a: {kind: agent, role: gofi-plan}\n  z: {kind: terminal, outcome: accepted}\nedges:\n  - {from: a, to: z}\n", "gofi-eng")
	if got != "warn a: role gofi-plan is not installed\n" {
		t.Errorf("got:\n%s", got)
	}
}

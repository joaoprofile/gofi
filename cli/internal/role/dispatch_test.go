package role

import (
	"strings"
	"testing"
)

func contract(t *testing.T, y string) Contract {
	t.Helper()
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The roles of a stage whose triggers hold are the ones that run, by name;
// the same facts always dispatch the same roles, with the facts that did it.
func TestDispatchRunsTheRolesWhoseTriggersHold(t *testing.T) {
	roles := []Role{
		{Name: "gofi-eng", Contract: contract(t, "tier: standard\nstage: implement\ntriggers:\n  - any: [change.surface(backend)]\n")},
		{Name: "gofi-ops", Contract: contract(t, "tier: standard\nstage: implement\ntriggers:\n  - any: [change.surface(infra)]\n")},
		{Name: "gofi-qa", Contract: contract(t, "tier: standard\nstage: audit\ntriggers:\n  - all: [sensor.green]\n")},
		{Name: "gofi-ui", Contract: contract(t, "tier: standard\nstage: implement\ntriggers:\n  - any: [change.surface(web), change.surface(mobile)]\n")},
	}
	facts := Facts{"change.surface(backend)": true, "change.surface(web)": true}
	got := Dispatch(roles, "implement", facts)
	if len(got) != 2 || got[0].Role != "gofi-eng" || got[1].Role != "gofi-ui" || got[1].Why != "change.surface(web)" {
		t.Fatalf("dispatch = %+v", got)
	}
	if got := Dispatch(roles, "audit", facts); len(got) != 0 {
		t.Errorf("the audit ran with the sensors not green: %+v", got)
	}
	explained := strings.Join(Explain(roles, "implement", facts), "\n")
	if !strings.Contains(explained, "✗ gofi-ops — falta um de change.surface(infra)") {
		t.Errorf("explain:\n%s", explained)
	}
}

// A trigger names only facts gofi computes; anything else is refused when
// the contract is read.
func TestATriggerNamesOnlyKnownFacts(t *testing.T) {
	for y, want := range map[string]string{
		"tier: standard\nstage: implement\ntriggers:\n  - any: [change.surfaces(backend)]\n": "is not a fact",
		"tier: standard\nstage: implement\ntriggers:\n  - any: [change.surface(desktop)]\n":  "takes one of",
		"tier: standard\nstage: audit\ntriggers:\n  - all: [sensor.green(build)]\n":          "takes no argument",
		"tier: standard\ntriggers:\n  - all: [sensor.green]\n":                               "needs the stage",
		"tier: standard\nstage: review\n":                                                    "is not a stage",
		"tier: standard\nvisibility: hidden\n":                                               "visibility",
	} {
		if _, err := Parse([]byte(y)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", y, err, want)
		}
	}
}

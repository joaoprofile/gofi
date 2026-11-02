package role

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// The roles gofi ships parse, and every surface a change can touch has the
// one role that implements it — a surface with none would leave its changes
// with nobody to dispatch.
func TestTheShippedRolesCoverEverySurface(t *testing.T) {
	roles, problems := Load(filepath.Join("..", "..", "..", "ai", "skills"))
	for _, p := range problems {
		t.Error(p)
	}
	for _, surface := range predicates["change.surface"] {
		// A run with no plan: one role implements the whole surface.
		got := Dispatch(roles, "implement", Facts{"change.surface(" + surface + ")": true, "plan.none": true})
		if len(got) != 1 {
			t.Errorf("surface %s dispatches %+v, want exactly one role", surface, got)
		}
	}
	// A backend change is planned, and each layer of the plan has exactly
	// the one role that implements it; the tests have their own.
	backend := Facts{"change.surface(backend)": true}
	if got := Dispatch(roles, "plan", backend); len(got) != 1 || got[0].Role != "gofi-plan" {
		t.Errorf("plan dispatches %+v", got)
	}
	for _, layer := range Layers {
		got := Dispatch(roles, "implement", Facts{"change.surface(backend)": true, "layer(" + layer + ")": true})
		if len(got) != 1 {
			t.Errorf("layer %s dispatches %+v, want exactly one role", layer, got)
		}
	}
	if got := Dispatch(roles, "test", Facts{"change.surface(backend)": true, "tests.due": true}); len(got) != 1 || got[0].Role != "gofi-test" {
		t.Errorf("test dispatches %+v", got)
	}
	// A domain change is audited by the general lens, conformance and tests;
	// data and the API bring their own lenses; a screen, the UI's.
	audit := func(f Facts) string {
		var out []string
		for _, c := range Dispatch(roles, "audit", f) {
			out = append(out, c.Role)
		}
		sort.Strings(out)
		return strings.Join(out, " ")
	}
	if got := audit(Facts{"sensor.green": true, "change.surface(backend)": true, "layer(domain)": true}); got != "gofi-qa gofi-qa-conformance gofi-qa-tests" {
		t.Errorf("a domain change is audited by %s", got)
	}
	if got := audit(Facts{"sensor.green": true, "change.surface(backend)": true, "layer(data)": true, "layer(api)": true}); got != "gofi-qa gofi-qa-conformance gofi-qa-data gofi-qa-security gofi-qa-tests" {
		t.Errorf("a data and API change is audited by %s", got)
	}
	if got := audit(Facts{"sensor.green": true, "change.surface(web)": true}); got != "gofi-qa gofi-qa-conformance gofi-qa-tests gofi-qa-ui" {
		t.Errorf("a screen is audited by %s", got)
	}
	for _, r := range roles {
		if r.Contract.Stage == "audit" && (slices.Contains(r.Contract.Tools, "edit") || slices.Contains(r.Contract.Tools, "write")) {
			t.Errorf("the lens %s may write: it would not run beside the others", r.Name)
		}
	}
	for _, r := range roles {
		if r.Contract.Stage == "implement" && !r.Contract.Internal() {
			t.Errorf("%s implements and is still a slash command", r.Name)
		}
	}
}

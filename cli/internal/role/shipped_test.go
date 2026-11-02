package role

import (
	"path/filepath"
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
		got := Dispatch(roles, "implement", Facts{"change.surface(" + surface + ")": true})
		if len(got) != 1 {
			t.Errorf("surface %s dispatches %+v, want exactly one role", surface, got)
		}
	}
	if got := Dispatch(roles, "audit", Facts{"sensor.green": true}); len(got) != 1 || got[0].Role != "gofi-qa" {
		t.Errorf("audit dispatches %+v", got)
	}
	for _, r := range roles {
		if r.Contract.Stage == "implement" && !r.Contract.Internal() {
			t.Errorf("%s implements and is still a slash command", r.Name)
		}
	}
}

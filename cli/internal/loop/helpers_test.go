package loop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

func writeFlow(t *testing.T, root, name, body string) {
	t.Helper()
	p := layout.Flows().Abs(root, name+".yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// shippedRoles are the implement and audit roles a delivery dispatches, as
// their contracts declare them.
func shippedRoles() []role.Role {
	return []role.Role{
		{Name: "gofi-eng", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "implement",
			Triggers: []role.Trigger{{Any: []string{"change.surface(backend)"}}}}},
		{Name: "gofi-qa", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "audit",
			Triggers: []role.Trigger{{All: []string{"sensor.green"}}}}},
		{Name: "gofi-repair", Contract: role.Contract{Tier: host.Light, Visibility: role.VisibilityInternal, Stage: "repair",
			Triggers: []role.Trigger{{Any: []string{"sensor.failed(build)", "sensor.failed(static)", "sensor.failed(graph)"}}}}},
		{Name: "gofi-triage", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "triage",
			Triggers: []role.Trigger{{Any: []string{"owner(spec)", "owner(pd)", "stuck"}}}}},
		{Name: "gofi-ui", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "implement",
			Triggers: []role.Trigger{{Any: []string{"change.surface(web)", "change.surface(mobile)"}}}}},
	}
}

// layeredRoles are the roles of a backend change split by layer (0002, D17):
// the plan, one implementer per layer, the tests by a role of their own —
// with the audit, the repair and the triage.
func layeredRoles() []role.Role {
	backend := "change.surface(backend)"
	impl := func(name, layer string, extra ...role.Trigger) role.Role {
		return role.Role{Name: name, Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "implement", Layer: layer,
			Triggers: append([]role.Trigger{{All: []string{backend, "layer(" + layer + ")"}}}, extra...)}}
	}
	out := []role.Role{
		{Name: "gofi-plan", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "plan",
			Triggers: []role.Trigger{{All: []string{backend}}}}},
		impl("gofi-eng", "domain", role.Trigger{All: []string{backend, "plan.none"}}),
		impl("gofi-eng-api", "api"),
		impl("gofi-eng-data", "data"),
		impl("gofi-integrate", "integration"),
		{Name: "gofi-test", Contract: role.Contract{Tier: host.Standard, Visibility: role.VisibilityInternal, Stage: "test",
			Triggers: []role.Trigger{{All: []string{backend, "tests.due"}}}}},
	}
	for _, r := range shippedRoles() {
		if r.Contract.Stage != "implement" {
			out = append(out, r)
		}
	}
	return out
}

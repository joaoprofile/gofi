package cli

import (
	"bytes"
	"strings"
	"testing"
)

// gofi role list shows each role's visibility, stage and triggers as its
// contract declares them.
func TestRoleListShowsWhatDispatchesEachRole(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	installRoles(t, root)
	var out bytes.Buffer
	c := newRoleCmd()
	c.SetOut(&out)
	c.SetArgs([]string{"list"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gofi-eng", "internal", "implement", "um de change.surface(backend)", "gofi-qa", "todos de sensor.green"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list lacks %q:\n%s", want, out.String())
		}
	}
}

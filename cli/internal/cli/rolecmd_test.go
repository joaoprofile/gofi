package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
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
	for _, want := range []string{"gofi-eng", "internal", "implement", "todos de change.surface(backend), layer(domain)", "gofi-qa", "todos de sensor.green"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list lacks %q:\n%s", want, out.String())
		}
	}
}

// The skills update says what each area gets: the internal roles land in
// their own, never counted as slash commands.
func TestTheSkillsUpdateCountsEachArea(t *testing.T) {
	p := &skillsPlan{changes: []scaffold.Change{
		{RelPath: layout.Skills().Path("gofi-pd", "SKILL.md"), Kind: scaffold.ChangeNew},
		{RelPath: layout.Roles().Path("gofi-eng", "SKILL.md"), Kind: scaffold.ChangeNew},
		{RelPath: layout.Roles().Path("gofi-qa", "SKILL.md"), Kind: scaffold.ChangeModified},
	}}
	out := p.scope(false).String()
	for _, want := range []string{layout.Skills().Dir + "/", layout.Roles().Dir + "/"} {
		if !strings.Contains(out, want) {
			t.Errorf("scope lacks %s:\n%s", want, out)
		}
	}
}

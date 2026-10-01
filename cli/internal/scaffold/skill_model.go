package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"path"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// skillModel turns a role's tier into the model its installed SKILL.md names.
// Nil, or an empty answer, writes no model: the session's model serves the
// skill. Set once per command, from the project's host and ai.tiers, the way
// layout.SetHome is.
var skillModel func(host.Tier) string

// SetSkillModels makes the installs name a model per skill: the one h serves
// the skill's tier with, the project's ai.tiers first. Only a host whose skill
// format has a model field gets one — for any other the skill's frontmatter
// must stay as the open standard defines it.
func SetSkillModels(h host.Host, tiers map[string]string) {
	if !h.SkillModel {
		skillModel = nil
		return
	}
	skillModel = func(t host.Tier) string { return h.Model(t, tiers) }
}

// modelFor is the model to write into a skill: its contract's tier, through
// skillModel. A skill without a contract — a release from before contracts —
// gets none; a contract that does not parse is a broken release and says so.
func modelFor(agentsFS fs.FS, srcRoot, name string) (string, error) {
	if skillModel == nil {
		return "", nil
	}
	b, err := fs.ReadFile(agentsFS, path.Join(skillSourceDir(srcRoot, name), role.ContractFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	c, err := role.Parse(b)
	if err != nil {
		return "", fmt.Errorf("skill %s: %w", name, err)
	}
	return skillModel(c.Tier), nil
}

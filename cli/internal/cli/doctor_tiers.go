package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/doctor"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// claudeModelAliases are the model names Claude Code resolves on its own, as
// its model configuration documents them. Anything else in ai.tiers has to be
// a full model ID.
var claudeModelAliases = map[string]bool{
	"haiku": true, "sonnet": true, "opus": true, "fable": true,
	"best": true, "default": true, "opusplan": true, "inherit": true,
}

// checkTiers reports how the project's skills map to models: an ai.tiers
// value the host cannot resolve, and skills installed with a model their tier
// no longer maps to — ai.tiers changed and the skills were not reinstalled.
func checkTiers(cfg *config.GofiConfig, root string, h host.Host) doctor.Check {
	const name = "model tiers"
	if !h.SkillModel {
		return doctor.Check{Name: name, Status: doctor.StatusOK,
			Detail: "the session's model serves every skill on " + h.Label}
	}
	var bad []string
	for tier, m := range cfg.AI.Tiers {
		if m = strings.TrimSpace(m); m != "" && !claudeModelAliases[m] && !strings.HasPrefix(m, "claude-") {
			bad = append(bad, tier+": "+m)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return doctor.Check{Name: name, Status: doctor.StatusWarn,
			Detail: "ai.tiers names no Claude model — " + strings.Join(bad, ", "),
			Hint:   "use an alias (haiku, sonnet, opus, fable) or a full ID such as " + config.DefaultModel}
	}
	if stale := staleSkillModels(root, h, cfg.AI.Tiers); len(stale) > 0 {
		return doctor.Check{Name: name, Status: doctor.StatusWarn,
			Detail: fmt.Sprintf("%d skill(s) run on a model their tier no longer maps to: %s", len(stale), strings.Join(stale, ", ")),
			Hint:   "run `gofi update skills`"}
	}
	var parts []string
	for _, t := range host.Tiers {
		parts = append(parts, string(t)+"→"+h.Model(t, cfg.AI.Tiers))
	}
	return doctor.Check{Name: name, Status: doctor.StatusOK, Detail: strings.Join(parts, " · ")}
}

// staleSkillModels lists the installed skills whose SKILL.md names a model
// other than the one their contract's tier maps to now.
func staleSkillModels(root string, h host.Host, tiers map[string]string) []string {
	dir := layout.Skills().Abs(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var stale []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name(), role.ContractFile))
		if err != nil {
			continue
		}
		c, err := role.Parse(b)
		if err != nil {
			continue
		}
		skill, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		if got, want := frontmatterModel(string(skill)), h.Model(c.Tier, tiers); got != want {
			stale = append(stale, e.Name())
		}
	}
	sort.Strings(stale)
	return stale
}

// frontmatterModel is the model: line of a SKILL.md's frontmatter, or "".
func frontmatterModel(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return ""
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return ""
	}
	for _, l := range strings.Split(text[4:4+end], "\n") {
		if v, ok := strings.CutPrefix(l, "model:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}

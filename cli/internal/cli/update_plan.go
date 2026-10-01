package cli

import (
	"fmt"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// targetPlan is one update target, computed and not yet applied: what it
// writes, keeps and leaves alone, and how to write it. Planning prints the
// file-by-file evidence; nothing is written until apply.
//
// Every target is a plan so that `gofi update <target>` and `gofi update`
// cannot disagree on what a target does: the one confirms a plan, the other
// confirms the sum of them.
type targetPlan struct {
	// name is the target as typed ("skills"), for messages.
	name string
	// title is the question asked when the run can destroy something.
	title string
	scope updateScope
	// apply writes the plan. It runs only when the scope has writes and the
	// user said yes.
	apply func() error
	// after runs whether or not anything was written: notes on what the
	// target could not do for the user. Optional.
	after func() error
}

// runTarget confirms one target's plan and applies it.
func runTarget(cfg *config.GofiConfig, t *targetPlan, autoConfirm bool) error {
	ok, err := confirmUpdate(t.title, t.scope, autoConfirm)
	if err != nil {
		return err
	}
	if ok {
		if err := t.apply(); err != nil {
			return fmt.Errorf("%s update: %w", t.name, err)
		}
	} else if len(t.scope.Writes) > 0 {
		fmt.Printf("%s left as it is.\n", t.name)
	}
	if t.after != nil {
		if err := t.after(); err != nil {
			return err
		}
	}
	if ok {
		noteDrift(cfg)
	}
	return nil
}

// gofiZoneTargets plans every target that refreshes the gofi zone, in the
// order they are applied. A target the project has nothing for — no backend,
// no front-end surface — is left out and named in skipped.
func gofiZoneTargets(cfg *config.GofiConfig, force bool) (plans []*targetPlan, skipped []string, err error) {
	planners := []struct {
		name    string
		applies bool
		plan    func() (*targetPlan, error)
	}{
		{"skills", true, func() (*targetPlan, error) { return skillsTarget(cfg, force) }},
		{"agents", true, func() (*targetPlan, error) { return agentsTarget(cfg, force) }},
		{"expertise", true, func() (*targetPlan, error) { return expertiseTarget(cfg, force) }},
		{"templates", true, func() (*targetPlan, error) { return templatesTarget(cfg, force) }},
		{"sdk", backendLang(cfg) != "", func() (*targetPlan, error) { return sdkTarget(cfg, force) }},
		{"ds", len(uiSurfacesFromConfig(cfg)) > 0, func() (*targetPlan, error) { return dsTarget(cfg, force) }},
	}
	for _, p := range planners {
		if !p.applies {
			skipped = append(skipped, p.name)
			continue
		}
		t, err := p.plan()
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p.name, err)
		}
		plans = append(plans, t)
	}
	return plans, skipped, nil
}

// projectZone is what an update of the gofi zone never writes — the
// project's own areas.
func projectZone() []string {
	return []string{
		".gofi.yaml", layout.Knowledge().Dir + "/", layout.Memory().Dir + "/",
		layout.Institutional().Dir + "/", layout.Lexicon().Dir + "/",
		layout.Specs().Dir + "/", layout.PRD().Dir + "/", "the AGENTS.md project block",
	}
}

// runUpdateAll is `gofi update` without a target: every target of the gofi
// zone, planned together, confirmed once, applied in order. The project zone
// is never touched — the institutional mirror included, which replaces a
// folder the project owns and so stays a target of its own.
func runUpdateAll(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	plans, skipped, err := gofiZoneTargets(cfg, force)
	if err != nil {
		return err
	}
	all := updateScope{Force: force, LeavesAlone: projectZone()}
	for _, t := range plans {
		all.Writes = append(all.Writes, t.scope.Writes...)
		all.Keeps = append(all.Keeps, t.scope.Keeps...)
		all.Replaces = all.Replaces || t.scope.Replaces
		if all.Hint == "" {
			all.Hint = t.scope.Hint
		}
	}
	for _, s := range skipped {
		fmt.Printf("%s: nothing in this project to update.\n", s)
	}
	ok, err := confirmUpdate("Update everything gofi owns in this project?", all, autoConfirm)
	if err != nil {
		return err
	}
	for _, t := range plans {
		if ok && len(t.scope.Writes) > 0 {
			if err := t.apply(); err != nil {
				return fmt.Errorf("%s update: %w", t.name, err)
			}
		}
		if t.after != nil {
			if err := t.after(); err != nil {
				return err
			}
		}
	}
	if !ok && len(all.Writes) > 0 {
		fmt.Println("gofi zone left as it is.")
		return nil
	}
	if ok {
		noteDrift(cfg)
	}
	return nil
}

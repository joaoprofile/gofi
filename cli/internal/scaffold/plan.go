package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"io/fs"
	"os"
	"path/filepath"
)

// ChangeKind describes the relationship between an upstream file and the
// project's current copy. Files whose contents are unchanged are omitted
// from the plan entirely.
type ChangeKind string

const (
	ChangeNew      ChangeKind = "new"
	ChangeModified ChangeKind = "modified"
	// ChangeKept marks a file upstream changed that the update will not write:
	// the project edited it, so the local version stays.
	ChangeKept ChangeKind = "kept"
)

// Change is a single entry in the update plan. RelPath is relative to the
// project root (e.g. ".claude/skills/gofi-pd/SKILL.md") so it can be displayed verbatim.
type Change struct {
	RelPath string
	Kind    ChangeKind
}

// PlanSkillsUpdate computes the list of files an InstallSkillsContent run would
// create or modify in projectRoot, writing nothing. It is the plan `gofi update`
// shows: skills are the only thing that command refreshes.
func PlanSkillsUpdate(agentsFS fs.FS, srcRoot, projectRoot string) ([]Change, error) {
	var changes []Change
	p := newPreserver(projectRoot, InstallUpdate)
	if err := planSkills(agentsFS, srcRoot, planAdder(projectRoot, p, &changes)); err != nil {
		return nil, err
	}
	return changes, nil
}

// planSkills feeds add() the rendered SKILL.md of every skill upstream ships,
// mirroring installSkills so the plan reflects exactly what would be written.
func planSkills(agentsFS fs.FS, srcRoot string, add func(rel string, content []byte) error) error {
	skills, err := listSkillNames(agentsFS, srcRoot)
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}
	for _, skill := range skills {
		body, err := readSkillSource(agentsFS, srcRoot, skill)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("read skill %s: %w", skill, err)
		}
		model, err := modelFor(agentsFS, srcRoot, skill)
		if err != nil {
			return err
		}
		if err := add(skillRelPath(skill), renderSkill(skill, body, model)); err != nil {
			return err
		}
		err = walkSkillResources(agentsFS, srcRoot, skill, func(rel string, content []byte) error {
			return add(filepath.Join(skillsDirName, skill, filepath.FromSlash(rel)), content)
		})
		if err != nil {
			return fmt.Errorf("plan resources of skill %s: %w", skill, err)
		}
	}
	return nil
}

// planAdder builds the callback that classifies one upstream file against the
// project's copy and appends the verdict to changes. A file whose contents
// already match is omitted entirely.
func planAdder(projectRoot string, p *preserver, changes *[]Change) func(string, []byte) error {
	return func(claudeRel string, content []byte) error {
		rel := filepath.Join(layout.Home(), claudeRel)
		target := filepath.Join(projectRoot, rel)
		existing, err := os.ReadFile(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				*changes = append(*changes, Change{RelPath: rel, Kind: ChangeNew})
				return nil
			}
			return err
		}
		if bytes.Equal(existing, content) {
			return nil
		}
		kind := ChangeModified
		if p.keeps(filepath.ToSlash(rel), existing) {
			kind = ChangeKept
		}
		*changes = append(*changes, Change{RelPath: rel, Kind: kind})
		return nil
	}
}

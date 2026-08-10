package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
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
// project root (e.g. ".claude/CLAUDE.md") so it can be displayed verbatim.
type Change struct {
	RelPath string
	Kind    ChangeKind
}

// PlanAgentsUpdate computes the list of files that an InstallUpdate run of
// InstallAgentsContent would create or modify in projectRoot. It writes
// nothing — only walks agentsFS and compares rendered content with the
// project's existing files.
//
// The walk mirrors the InstallUpdate branch of InstallAgentsContent:
// CLAUDE.md, skills/ (all of them), templates/*, scripts/* and the
// knowledge/ files the project is missing. memory/ and institutional/ are
// skipped because update preserves them whole.
func PlanAgentsUpdate(agentsFS fs.FS, srcRoot, projectRoot string, data TemplateData) ([]Change, error) {
	var changes []Change
	p := newPreserver(projectRoot, InstallUpdate)
	add := planAdder(projectRoot, p, &changes)

	addNewOnly := func(claudeRel string, _ []byte) error {
		if _, err := os.Stat(filepath.Join(projectRoot, ".claude", claudeRel)); err == nil {
			return nil
		}
		changes = append(changes, Change{
			RelPath: filepath.Join(".claude", claudeRel),
			Kind:    ChangeNew,
		})
		return nil
	}

	if body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "claude", "CLAUDE.md")); err == nil {
		if err := add("CLAUDE.md", body); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	if err := planSkills(agentsFS, srcRoot, add); err != nil {
		return nil, err
	}

	if srcDir := path.Join(srcRoot, "ai", "templates"); dirExistsInFS(agentsFS, srcDir) {
		if err := walkAndPlan(agentsFS, srcDir, "templates", data, add); err != nil {
			return nil, err
		}
	}

	if srcDir := path.Join(srcRoot, "ai", "scripts"); dirExistsInFS(agentsFS, srcDir) {
		if err := walkAndPlan(agentsFS, srcDir, "scripts", data, add); err != nil {
			return nil, err
		}
	}

	// Knowledge arrives additively, so only the files the project never received
	// are listed. A local edit is not a pending change — the update will not
	// touch it, and showing it would promise otherwise.
	if srcDir := path.Join(srcRoot, "ai", "knowledge"); dirExistsInFS(agentsFS, srcDir) {
		if err := walkAndPlan(agentsFS, srcDir, "knowledge", data, addNewOnly); err != nil {
			return nil, err
		}
	}

	return changes, nil
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
		if err := add(skillRelPath(skill), renderSkill(skill, body)); err != nil {
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
		rel := filepath.Join(".claude", claudeRel)
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

// walkAndPlan walks srcDir in agentsFS and invokes add() per file with the
// destination path (under .claude/<claudeSub>/) and rendered content. Mirrors
// the path/template handling of installFS so the plan reflects exactly what
// would be written.
func walkAndPlan(
	agentsFS fs.FS, srcDir, claudeSub string,
	data TemplateData, add func(rel string, content []byte) error,
) error {
	return fs.WalkDir(agentsFS, srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == srcDir || d.IsDir() {
			return nil
		}
		if filepath.Base(p) == GitkeepName {
			return nil
		}

		rel := strings.TrimPrefix(p, srcDir+"/")
		rel = strings.ReplaceAll(rel, ProjectMarker, data.ProjectName)
		if data.SourceRoot != "" {
			rel = strings.ReplaceAll(rel, RootMarker, data.SourceRoot)
		}

		raw, err := fs.ReadFile(agentsFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}

		content := raw
		if strings.HasSuffix(rel, TemplateExt) {
			rel = strings.TrimSuffix(rel, TemplateExt)
			tmpl, err := template.New(filepath.Base(p)).Parse(string(raw))
			if err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return fmt.Errorf("render %s: %w", p, err)
			}
			content = buf.Bytes()
		}
		return add(filepath.Join(claudeSub, rel), content)
	})
}

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
// CLAUDE.md, skills/<agent>.md (selected agents only) and templates/*.
// memory/ and knowledge/ are skipped because update preserves them.
func PlanAgentsUpdate(agentsFS fs.FS, srcRoot, projectRoot string, data TemplateData) ([]Change, error) {
	var changes []Change

	add := func(claudeRel string, content []byte) error {
		target := filepath.Join(projectRoot, ".claude", claudeRel)
		existing, err := os.ReadFile(target)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				changes = append(changes, Change{
					RelPath: filepath.Join(".claude", claudeRel),
					Kind:    ChangeNew,
				})
				return nil
			}
			return err
		}
		if !bytes.Equal(existing, content) {
			changes = append(changes, Change{
				RelPath: filepath.Join(".claude", claudeRel),
				Kind:    ChangeModified,
			})
		}
		return nil
	}

	if body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "claude", "CLAUDE.md")); err == nil {
		if err := add("CLAUDE.md", body); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	for _, agent := range data.Agents {
		body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "skills", agent+".md"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read agent %s: %w", agent, err)
		}
		if err := add(filepath.Join("skills", agent+".md"), body); err != nil {
			return nil, err
		}
	}

	if srcDir := path.Join(srcRoot, "ai", "templates"); dirExistsInFS(agentsFS, srcDir) {
		if err := walkAndPlan(agentsFS, srcDir, "templates", data, add); err != nil {
			return nil, err
		}
	}

	return changes, nil
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

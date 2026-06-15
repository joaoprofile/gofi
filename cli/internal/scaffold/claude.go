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

// allAgents lists every agent shipped in the embedded snapshot. Agents not
// present in selected get their commands/<agent>.md and knowledge/<short>/
// directories removed after install.
var allAgents = []string{
	"gofi-pd", "gofi-spec", "gofi-eng", "gofi-ui",
	"gofi-ops", "gofi-qa", "gofi-doc", "gofi-status",
}

// agentToKnowledgeDir maps an agent name to the per-agent folder under
// .claude/knowledge/. Shared/ is always kept regardless of agent selection.
// Agents without an entry (gofi-doc, gofi-status) have no per-agent knowledge dir.
var agentToKnowledgeDir = map[string]string{
	"gofi-pd":   "pd",
	"gofi-spec": "spec",
	"gofi-eng":  "eng",
	"gofi-ui":   "ui",
	"gofi-ops":  "ops",
	"gofi-qa":   "qa",
}

// InstallMode tells installers whether they're seeding a brand new project
// or refreshing one that already has user-managed content.
type InstallMode int

const (
	// InstallNew copies everything, including memory templates and empty
	// knowledge dirs. Used by `gofi init`.
	InstallNew InstallMode = iota

	// InstallUpdate refreshes only files managed by the source repos and
	// preserves user-managed dirs (knowledge/, memory/). Used by `gofi update`.
	InstallUpdate
)

// InstallAgentsContent copies the gofi-agents tree into <projectRoot>/.claude/.
// agentsFS is rooted at the gofi-agents repo and srcRoot is the relative
// directory inside that fs (typically "." for an extracted tarball).
//
// It installs:
//   - ai/claude/CLAUDE.md       → .claude/CLAUDE.md
//   - ai/skills/<sel>.md        → .claude/skills/<sel>.md (only selected)
//   - ai/templates/             → .claude/templates/
//   - ai/memory/project.md.tmpl → .claude/memory/project.md (InstallNew only)
//   - embedded institutional templates → .claude/institutional/<name>/ (InstallNew only)
//
// On InstallNew, knowledge/shared/ is seeded from <srcRoot>/ai/knowledge/shared/
// (memory/learning protocols, base principles), per-agent knowledge dirs are
// created empty, and the institutional RAG scaffold (README + INDEX) is seeded
// under .claude/institutional/<project name>/ for gofi-pd. On InstallUpdate,
// memory, knowledge and institutional are left untouched.
func InstallAgentsContent(agentsFS fs.FS, srcRoot, projectRoot string, data TemplateData, mode InstallMode) ([]string, error) {
	dest := filepath.Join(projectRoot, ".claude")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}
	var created []string

	// CLAUDE.md
	if data, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "claude", "CLAUDE.md")); err == nil {
		target := filepath.Join(dest, "CLAUDE.md")
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return created, err
		}
		created = append(created, target)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return created, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	// Selected agents
	for _, agent := range data.Agents {
		body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "skills", agent+".md"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return created, fmt.Errorf("read agent %s: %w", agent, err)
		}
		target := filepath.Join(dest, "skills", agent+".md")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return created, err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return created, err
		}
		created = append(created, target)
	}

	// templates/ (PRD + SDD templates)
	if srcDir := path.Join(srcRoot, "ai", "templates"); dirExistsInFS(agentsFS, srcDir) {
		c, err := installFS(agentsFS, srcDir, filepath.Join(dest, "templates"), data, InstallOptions{})
		if err != nil {
			return created, err
		}
		created = append(created, c...)
	}

	// Knowledge dirs. shared/ comes from the source repo (protocols, base
	// principles); per-agent dirs are placeholders the team fills in.
	// On InstallNew, shared/ is seeded from the source so projects start with
	// the upstream knowledge baked in. InstallUpdate leaves it alone — the
	// team's edits live there.
	sharedDest := filepath.Join(dest, "knowledge", "shared")
	if mode == InstallNew {
		sharedSrc := path.Join(srcRoot, "ai", "knowledge", "shared")
		if dirExistsInFS(agentsFS, sharedSrc) {
			c, err := installFS(agentsFS, sharedSrc, sharedDest, data, InstallOptions{})
			if err != nil {
				return created, err
			}
			created = append(created, c...)
		} else {
			if err := os.MkdirAll(sharedDest, 0o755); err != nil {
				return created, err
			}
		}
	} else {
		if err := os.MkdirAll(sharedDest, 0o755); err != nil {
			return created, err
		}
	}
	for _, agent := range data.Agents {
		if short := agentToKnowledgeDir[agent]; short != "" {
			if err := os.MkdirAll(filepath.Join(dest, "knowledge", short), 0o755); err != nil {
				return created, err
			}
		}
	}

	// Memory + memory/contexts (InstallNew only)
	if mode == InstallNew {
		if err := os.MkdirAll(filepath.Join(dest, "memory", "contexts"), 0o755); err != nil {
			return created, err
		}
		raw, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "memory", "project.md.tmpl"))
		if err == nil {
			rendered, err := renderTemplate(raw, data)
			if err != nil {
				return created, fmt.Errorf("render memory/project.md.tmpl: %w", err)
			}
			target := filepath.Join(dest, "memory", "project.md")
			if err := os.WriteFile(target, rendered, 0o644); err != nil {
				return created, err
			}
			created = append(created, target)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return created, fmt.Errorf("read memory template: %w", err)
		}
	}

	// Institutional knowledge (gofi-pd's per-product business RAG). Seeded
	// only on InstallNew, scoped by project name (.claude/institutional/<name>/),
	// matching how /gofi-pd resolves the folder from project.name. We seed only
	// the structure — README + INDEX (RAG manifest) — from the embedded
	// templates; the thematic chunks are filled in by the team during discovery.
	// On InstallUpdate it is left untouched (team-managed, like memory/).
	if mode == InstallNew && data.ProjectName != "" {
		instDest := filepath.Join(dest, "institutional", data.ProjectName)
		c, err := installFS(embeddedFS, "embedded/institutional", instDest, data, InstallOptions{})
		if err != nil {
			return created, fmt.Errorf("seed institutional: %w", err)
		}
		created = append(created, c...)
	}

	return created, nil
}

// InstallSDKContent copies the SDK content into the project's .claude/.
// sdkRoot is the directory inside srcFS that contains boilerplates/,
// sdk-docs/ and knowledge/. Two common cases:
//
//   - default: srcFS = gofi-agents tarball, sdkRoot = "sdk/<lang>"
//   - override: srcFS = gofi-sdk-<lang> tarball, sdkRoot = "."
//
// Layout in the project after install (preserves source structure):
//
//	.claude/sdk/<language>/
//	  boilerplates/                ← <sdkRoot>/boilerplates/
//	  sdk-docs/                    ← <sdkRoot>/sdk-docs/
//	  knowledge/                   ← <sdkRoot>/knowledge/
//
// Every install/update wipes .claude/sdk/<language>/ and recreates it from
// the source. Returns ErrNoSDKLayout when sdkRoot exists but contains none
// of the three expected subdirs — caller decides whether to warn or fall back.
func InstallSDKContent(srcFS fs.FS, sdkRoot, projectRoot, language string) ([]string, error) {
	if language == "" {
		return nil, errors.New("language is required")
	}
	if !dirExistsInFS(srcFS, sdkRoot) {
		return nil, nil
	}

	subdirs := []string{"boilerplates", "sdk-docs", "knowledge"}
	found := false
	for _, sub := range subdirs {
		if dirExistsInFS(srcFS, path.Join(sdkRoot, sub)) {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrNoSDKLayout
	}

	dest := filepath.Join(projectRoot, ".claude", "sdk", language)
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("clear %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}

	var created []string
	for _, sub := range subdirs {
		srcDir := path.Join(sdkRoot, sub)
		if !dirExistsInFS(srcFS, srcDir) {
			continue
		}
		c, err := installFS(srcFS, srcDir, filepath.Join(dest, sub), TemplateData{}, InstallOptions{})
		if err != nil {
			return created, err
		}
		created = append(created, c...)
	}
	return created, nil
}

// ErrNoSDKLayout signals that the configured SDK source exists but does not
// expose the boilerplates/ + sdk-docs/ + knowledge/ trio. Callers can fall
// back to the gofi-agents bundled SDK in this case.
var ErrNoSDKLayout = errors.New("source does not contain a gofi SDK layout (boilerplates/, sdk-docs/, knowledge/)")

// InstallUIContent copies a front-end surface's harness content from the gofi
// monorepo into the project's .claude/. surfaceRoot is the directory inside
// srcFS for the surface (e.g. "ai/sdk/web" or "ai/sdk/mobile"), which holds the
// design system docs plus boilerplates/ and knowledge/. surface is "web" or
// "mobile". The whole subtree is mirrored to .claude/sdk/<surface>/ so the
// gofi-ui agent reads tokens, components, patterns and rules from there.
//
// No-op (nil, nil) when surfaceRoot is absent in srcFS. Wipes and recreates
// the destination on every run so updates stay clean.
func InstallUIContent(srcFS fs.FS, surfaceRoot, projectRoot, surface string) ([]string, error) {
	if surface == "" {
		return nil, errors.New("surface is required")
	}
	if !dirExistsInFS(srcFS, surfaceRoot) {
		return nil, nil
	}
	dest := filepath.Join(projectRoot, ".claude", "sdk", surface)
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("clear %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}
	return installFS(srcFS, surfaceRoot, dest, TemplateData{}, InstallOptions{})
}

// CleanLegacySDKLayout removes the pre-v2.4 flat SDK directories from
// <projectRoot>/.claude/. Safe to call on fresh installs (no-op when the dirs
// are absent). Used by `gofi update` to migrate projects to the new
// .claude/sdk/<lang>/ layout.
func CleanLegacySDKLayout(projectRoot string) []string {
	dest := filepath.Join(projectRoot, ".claude")
	var removed []string
	candidates := []string{
		filepath.Join(dest, "boilerplates"),
		filepath.Join(dest, "sdk-knowledge"),
	}
	entries, _ := os.ReadDir(dest)
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "gofi-sdk-") {
			candidates = append(candidates, filepath.Join(dest, e.Name()))
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := os.RemoveAll(p); err == nil {
				removed = append(removed, p)
			}
		}
	}
	return removed
}

// readFromFS reads p from fsys and returns its bytes (helper around fs.ReadFile
// that exists mostly so callers can be terser).
func readFromFS(fsys fs.FS, p string) ([]byte, error) {
	return fs.ReadFile(fsys, p)
}

// dirExistsInFS reports whether p is a directory inside fsys.
func dirExistsInFS(fsys fs.FS, p string) bool {
	info, err := fs.Stat(fsys, p)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// renderTemplate runs raw through text/template with data.
func renderTemplate(raw []byte, data TemplateData) ([]byte, error) {
	tmpl, err := template.New("tmpl").Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

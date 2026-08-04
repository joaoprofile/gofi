package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// allAgents lists every agent shipped in the embedded snapshot. Agents not
// present in selected get their commands/<agent>.md and knowledge/<short>/
// directories removed after install.
var allAgents = []string{
	"gofi-pd", "gofi-spec", "gofi-eng", "gofi-ui",
	"gofi-ops", "gofi-qa", "gofi-doc", "gofi-status",
	"gofi-full",
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

	// InstallUpdate refreshes files managed by the source repos, preserves
	// memory/ and institutional/, and fills knowledge/ additively — an upstream
	// file the project never received arrives, an edited one is left alone.
	// Managed files carrying local edits are kept too; see preserver.
	// Used by `gofi update`.
	InstallUpdate

	// InstallReset is InstallUpdate without that last guarantee: every managed
	// file goes back to upstream, whatever the project did to it. Replaced
	// content still lands in .gofi/backup/. Used by `gofi update --force`.
	InstallReset
)

// refreshes reports whether the mode is one of the two update flavours, which
// share everything except how they treat a locally edited file.
func (m InstallMode) refreshes() bool { return m == InstallUpdate || m == InstallReset }

// InstallAgentsContent copies the gofi-agents tree into <projectRoot>/.claude/.
// agentsFS is rooted at the gofi-agents repo and srcRoot is the relative
// directory inside that fs (typically "." for an extracted tarball).
//
// It installs:
//   - ai/claude/CLAUDE.md       → .claude/CLAUDE.md
//   - ai/skills/*.md            → .claude/skills/<name>/SKILL.md (all skills, always)
//   - ai/templates/             → .claude/templates/
//   - ai/scripts/               → .claude/scripts/ (RAG index tooling)
//   - ai/memory/project.md.tmpl → .claude/memory/project.md (InstallNew only)
//   - ai/institutional/         → .claude/institutional/<name>/ (InstallNew only;
//     embedded scaffold as fallback — see InstallInstitutionalSeed)
//
// On InstallNew, knowledge/shared/ is seeded from <srcRoot>/ai/knowledge/shared/
// (memory/learning protocols, base principles), per-agent knowledge dirs are
// created empty, and the institutional RAG scaffold (README + INDEX) is seeded
// under .claude/institutional/<project name>/ for gofi-pd. On InstallUpdate,
// memory and institutional are left untouched and knowledge is filled without
// overwriting.
func InstallAgentsContent(agentsFS fs.FS, srcRoot, projectRoot string, data TemplateData, mode InstallMode) (created []string, err error) {
	dest := filepath.Join(projectRoot, ".claude")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}
	// Every managed write goes through the preserver, so a partial install still
	// records what it managed to write and the next update knows about it.
	p := newPreserver(projectRoot, mode)
	defer func() {
		if saveErr := p.save(); saveErr != nil && err == nil {
			err = fmt.Errorf("record installed files: %w", saveErr)
		}
	}()

	// CLAUDE.md
	if body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "claude", "CLAUDE.md")); err == nil {
		target := filepath.Join(dest, "CLAUDE.md")
		written, err := p.write(target, body)
		if err != nil {
			return created, err
		}
		if written {
			created = append(created, target)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return created, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	// Skills — every .md file under ai/skills/ is installed, regardless of the
	// selected agent set. The agent selection only scopes the per-agent
	// knowledge dirs below; all skills (including any agent not in the canonical
	// nine) are always available under .claude/skills/.
	//
	// Each lands as skills/<name>/SKILL.md with the frontmatter Claude Code
	// needs — see skill.go. Anything else is silently ignored by the engine,
	// which is what used to make every gofi slash command "unknown".
	skills, err := listSkillNames(agentsFS, srcRoot)
	if err != nil {
		return created, fmt.Errorf("list skills: %w", err)
	}
	for _, skill := range skills {
		body, err := readFromFS(agentsFS, path.Join(srcRoot, "ai", "skills", skill+".md"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return created, fmt.Errorf("read skill %s: %w", skill, err)
		}
		target := filepath.Join(dest, skillRelPath(skill))
		written, err := p.write(target, renderSkill(skill, body))
		if err != nil {
			return created, err
		}
		// Drop the flat file an older gofi wrote for this same skill.
		if err := pruneLegacySkillFile(dest, skill); err != nil {
			return created, err
		}
		if written {
			created = append(created, target)
		}
	}

	// templates/ (PRD + SDD templates)
	if srcDir := path.Join(srcRoot, "ai", "templates"); dirExistsInFS(agentsFS, srcDir) {
		c, err := installFS(agentsFS, srcDir, filepath.Join(dest, "templates"), data, InstallOptions{preserve: p})
		if err != nil {
			return created, err
		}
		created = append(created, c...)
	}

	// scripts/ (RAG tooling: gen-index.sh regenerates specs/prd INDEX.md from
	// frontmatter). Portable tool code — refreshed on new and update installs.
	if srcDir := path.Join(srcRoot, "ai", "scripts"); dirExistsInFS(agentsFS, srcDir) {
		scriptsDir := filepath.Join(dest, "scripts")
		c, err := installFS(agentsFS, srcDir, scriptsDir, data, InstallOptions{preserve: p})
		if err != nil {
			return created, err
		}
		// Every .sh, not only the ones just written: a run that rewrites nothing
		// is also the run that must not leave a script unexecutable.
		entries, _ := os.ReadDir(scriptsDir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".sh") {
				_ = os.Chmod(filepath.Join(scriptsDir, e.Name()), 0o755)
			}
		}
		created = append(created, c...)
	}

	// Knowledge. On InstallNew, the ENTIRE ai/knowledge/ tree is seeded from the
	// source — shared/ (protocols, base principles) plus every per-agent dir that
	// ships upstream content (eng/, ui/, …) — regardless of the selected agent
	// set, so projects start with all upstream knowledge baked in. Selected
	// agents that have no upstream content still get an empty placeholder dir the
	// team fills in.
	//
	// InstallUpdate walks the same tree with KeepExisting, which is the only way
	// to satisfy both halves of the contract: a file the team edited is never
	// touched, while a file added upstream after the project was scaffolded still
	// arrives. Without the second half, an update ships skills that cite
	// protocols the project does not have.
	knowledgeDest := filepath.Join(dest, "knowledge")
	if knowledgeSrc := path.Join(srcRoot, "ai", "knowledge"); dirExistsInFS(agentsFS, knowledgeSrc) {
		c, err := installFS(agentsFS, knowledgeSrc, knowledgeDest, data, InstallOptions{
			KeepExisting: mode.refreshes(),
			preserve:     p,
		})
		if err != nil {
			return created, err
		}
		created = append(created, c...)
	}
	// shared/ always exists (empty if the source shipped none), and selected
	// agents without upstream content get a placeholder dir.
	if err := os.MkdirAll(filepath.Join(knowledgeDest, "shared"), 0o755); err != nil {
		return created, err
	}
	for _, agent := range data.Agents {
		if short := agentToKnowledgeDir[agent]; short != "" {
			if err := os.MkdirAll(filepath.Join(knowledgeDest, short), 0o755); err != nil {
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

	// Institutional knowledge (gofi-pd's per-product business RAG). Seeded only
	// on InstallNew, scoped by project name (.claude/institutional/<name>/),
	// matching how /gofi-pd resolves the folder from project.name. See
	// InstallInstitutionalSeed for the source/fallback and placeholder handling.
	// On InstallUpdate it is left untouched (team-managed, like memory/). When a
	// real institutional repo is configured, seedInstitutionalFromRepo later
	// wipes and replaces this starter with authoritative company data.
	if mode == InstallNew {
		c, err := InstallInstitutionalSeed(agentsFS, srcRoot, projectRoot, data.ProjectName, data)
		if err != nil {
			return created, err
		}
		created = append(created, c...)
	}

	return created, nil
}

// institutionalNameToken is the literal placeholder in the guided institutional
// template that InstallInstitutionalSeed substitutes with the project name. All
// other {{PLACEHOLDERS}} are preserved verbatim for the team to fill in.
const institutionalNameToken = "{{NOME_DO_PRODUTO}}"

// InstallInstitutionalSeed seeds .claude/institutional/<projectName>/ with the
// guided business-knowledge starter. It prefers ai/institutional/ from the
// source monorepo (agentsFS) — the rich, guided template (domain, glossary,
// actors, business-rules, integrations, metrics, roadmap + INDEX + README with
// {{PLACEHOLDERS}} and [GUIA] prompts) — and falls back to the embedded minimal
// scaffold when the source ships none. Every institutionalNameToken is replaced
// with projectName so the starter is partially pre-filled; all other
// {{PLACEHOLDERS}} stay literal for the team to complete during discovery.
//
// No-op when projectName is empty. Meant for InstallNew only.
func InstallInstitutionalSeed(agentsFS fs.FS, srcRoot, projectRoot, projectName string, data TemplateData) ([]string, error) {
	if projectName == "" {
		return nil, nil
	}
	dest := filepath.Join(projectRoot, ".claude", "institutional", projectName)

	if srcDir := path.Join(srcRoot, "ai", "institutional"); dirExistsInFS(agentsFS, srcDir) {
		c, err := copyInstitutionalTemplate(agentsFS, srcDir, dest, projectName)
		if err != nil {
			return c, fmt.Errorf("seed institutional: %w", err)
		}
		return c, nil
	}

	// Fallback: embedded minimal scaffold (Go-template .tmpl rendering).
	c, err := installFS(embeddedFS, "embedded/institutional", dest, data, InstallOptions{})
	if err != nil {
		return c, fmt.Errorf("seed institutional (embedded): %w", err)
	}
	return c, nil
}

// copyInstitutionalTemplate mirrors srcDir into dest, replacing every
// institutionalNameToken with projectName and preserving all other placeholders
// verbatim. .gitkeep files are dropped; .tmpl files are NOT Go-template rendered
// (the guided template uses literal {{PLACEHOLDERS}}, not text/template fields).
func copyInstitutionalTemplate(srcFS fs.FS, srcDir, dest, projectName string) ([]string, error) {
	var created []string
	err := fs.WalkDir(srcFS, srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == srcDir {
			return nil
		}
		rel := strings.TrimPrefix(p, srcDir+"/")
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if filepath.Base(target) == GitkeepName {
			return nil
		}
		raw, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		content := bytes.ReplaceAll(raw, []byte(institutionalNameToken), []byte(projectName))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		created = append(created, target)
		return nil
	})
	return created, err
}

// SeedCorpusIndex writes the RAG retrieval manifest (INDEX.md) into a corpus
// directory at the project root — "specs" or "prd". The seed is an empty index
// (header + column row) that /gofi-spec and /gofi-pd repopulate via
// .claude/scripts/gen-index.sh as documents are created. It never overwrites an
// existing INDEX.md, so re-running init on a populated project is a no-op.
func SeedCorpusIndex(projectRoot, corpus string) error {
	if corpus != "specs" && corpus != "prd" {
		return fmt.Errorf("unknown corpus %q (want specs|prd)", corpus)
	}
	target := filepath.Join(projectRoot, corpus, "INDEX.md")
	if _, err := os.Stat(target); err == nil {
		return nil // preserve a project's real index
	}
	seed, err := readFromFS(embeddedFS, path.Join("embedded", "corpus", corpus+"-INDEX.md"))
	if err != nil {
		return fmt.Errorf("read %s seed index: %w", corpus, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
	}
	return os.WriteFile(target, seed, 0o644)
}

// InstallInstitutionalMirror replaces .claude/institutional/<projectName>/ with
// the <srcSubdir> subtree of the org's institutional repo (instFS). It is a
// FULL REPLACE: the destination is wiped and recreated on every run, so the
// folder is an authoritative mirror of the upstream — local edits do not
// survive (business changes belong in the institutional repo, not the project).
//
// srcSubdir is the product folder inside the repo (matched by project name in
// the multi-product layout). Returns ErrNoInstitutionalSubdir when that folder
// is absent so the caller can print actionable guidance.
func InstallInstitutionalMirror(instFS fs.FS, srcSubdir, projectRoot, projectName string, data TemplateData) ([]string, error) {
	if projectName == "" {
		return nil, errors.New("project name is required")
	}
	if !dirExistsInFS(instFS, srcSubdir) {
		return nil, fmt.Errorf("%w: %q", ErrNoInstitutionalSubdir, srcSubdir)
	}
	dest := filepath.Join(projectRoot, ".claude", "institutional", projectName)
	if err := os.RemoveAll(dest); err != nil {
		return nil, fmt.Errorf("clear %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}
	return installFS(instFS, srcSubdir, dest, data, InstallOptions{})
}

// ErrNoInstitutionalSubdir signals the institutional repo has no folder for
// this product (multi-product layout expects <project.name>/ at the repo root).
var ErrNoInstitutionalSubdir = errors.New("institutional repo has no folder for this product")

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
// The tree is filled in place rather than wiped: on InstallUpdate a file the
// project tuned (design tokens, house rules) is kept and everything else is
// refreshed. Returns ErrNoSDKLayout when sdkRoot exists but contains none
// of the three expected subdirs — caller decides whether to warn or fall back.
func InstallSDKContent(srcFS fs.FS, sdkRoot, projectRoot, language string, mode InstallMode) (created []string, err error) {
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
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}

	p := newPreserver(projectRoot, mode)
	defer func() {
		if saveErr := p.save(); saveErr != nil && err == nil {
			err = fmt.Errorf("record installed files: %w", saveErr)
		}
	}()
	for _, sub := range subdirs {
		srcDir := path.Join(sdkRoot, sub)
		if !dirExistsInFS(srcFS, srcDir) {
			continue
		}
		c, err := installFS(srcFS, srcDir, filepath.Join(dest, sub), TemplateData{}, InstallOptions{preserve: p})
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
// No-op (nil, nil) when surfaceRoot is absent in srcFS. Like the SDK tree, the
// destination is filled in place: the design tokens and rules a project adapts
// to its own product survive an update.
func InstallUIContent(srcFS fs.FS, surfaceRoot, projectRoot, surface string, mode InstallMode) (created []string, err error) {
	if surface == "" {
		return nil, errors.New("surface is required")
	}
	if !dirExistsInFS(srcFS, surfaceRoot) {
		return nil, nil
	}
	dest := filepath.Join(projectRoot, ".claude", "sdk", surface)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dest, err)
	}
	p := newPreserver(projectRoot, mode)
	defer func() {
		if saveErr := p.save(); saveErr != nil && err == nil {
			err = fmt.Errorf("record installed files: %w", saveErr)
		}
	}()
	return installFS(srcFS, surfaceRoot, dest, TemplateData{}, InstallOptions{preserve: p})
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

// listSkillNames returns the base names (without the .md extension) of every
// skill file under <srcRoot>/ai/skills/ in agentsFS, sorted. Directories and
// non-.md files are ignored. A missing ai/skills/ dir yields an empty slice,
// not an error, so callers degrade gracefully on sources without skills.
func listSkillNames(agentsFS fs.FS, srcRoot string) ([]string, error) {
	dir := path.Join(srcRoot, "ai", "skills")
	entries, err := fs.ReadDir(agentsFS, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(names)
	return names, nil
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

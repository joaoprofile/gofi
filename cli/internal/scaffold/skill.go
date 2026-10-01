package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Claude Code discovers a skill as a directory holding a SKILL.md whose YAML
// frontmatter declares both `name` and `description`. Every other shape is
// silently ignored — no warning, the slash command simply does not exist.
//
// gofi used to install `.claude/skills/<name>.md`, a flat file, which meant no
// gofi skill was ever invocable. The helpers here own the correct layout so
// the places that write skills (fresh install, update plan) cannot disagree
// about it.
const (
	skillsDirName = "skills"
	skillFileName = "SKILL.md"
)

// skillRelPath is the path of a skill inside .claude/, e.g.
// `skills/gofi-eng/SKILL.md`.
func skillRelPath(name string) string {
	return filepath.Join(skillsDirName, name, skillFileName)
}

// A skill in the gofi monorepo is a folder too — ai/skills/<name>/SKILL.md,
// plus whatever resources sit beside it — so the source mirrors the layout it
// is installed as. Older tarballs shipped a flat ai/skills/<name>.md and an
// update pinned to one of those refs still has to install, so both shapes are
// read; only the folder one can carry resources.
func skillSourceDir(srcRoot, name string) string {
	return path.Join(srcRoot, "ai", skillsDirName, name)
}

// readSkillSource returns the raw body of a skill from the source tree,
// preferring the folder shape and falling back to the flat file.
func readSkillSource(agentsFS fs.FS, srcRoot, name string) ([]byte, error) {
	body, err := fs.ReadFile(agentsFS, path.Join(skillSourceDir(srcRoot, name), skillFileName))
	if err == nil {
		return body, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return fs.ReadFile(agentsFS, skillSourceDir(srcRoot, name)+".md")
}

// walkSkillResources invokes fn for every file the skill's folder holds beside
// SKILL.md, with rel relative to that folder. A skill bundling references or
// scripts is the whole reason the source is a folder; installing only the
// manifest would drop them without a word. No-op for the flat shape.
func walkSkillResources(agentsFS fs.FS, srcRoot, name string, fn func(rel string, content []byte) error) error {
	dir := skillSourceDir(srcRoot, name)
	if !dirExistsInFS(agentsFS, dir) {
		return nil
	}
	return fs.WalkDir(agentsFS, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == skillFileName || filepath.Base(rel) == GitkeepName {
			return nil
		}
		content, err := fs.ReadFile(agentsFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		return fn(rel, content)
	})
}

// skillHeading matches the title the gofi skills open with, e.g.
// `# /gofi-eng — Context Engineer`.
var skillHeading = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// renderSkill returns the body to write as SKILL.md, guaranteeing the
// frontmatter Claude Code requires.
//
// A source file that already declares frontmatter keeps it, with only the
// missing keys filled in — curated `description` text is better than anything
// derived here, and `name` must match the folder either way. A file without
// frontmatter gets one synthesised from its heading.
//
// model, when set, is the model the host runs the skill on — the one its tier
// maps to — and replaces any model the source declares. Unset, the source's
// frontmatter is left as it is.
func renderSkill(name string, body []byte, model string) []byte {
	text := string(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")))

	front, rest, ok := splitFrontmatter(text)
	if !ok {
		front, rest = "", text
	}

	var lines []string
	if front != "" {
		lines = strings.Split(front, "\n")
	}
	if model != "" {
		kept := lines[:0]
		for _, line := range lines {
			if !strings.HasPrefix(line, "model:") {
				kept = append(kept, line)
			}
		}
		lines = kept
	}
	if !ok {
		lines = append(lines, "name: "+name, "description: "+quoteYAML(describeSkill(name, text)))
		if model != "" {
			lines = append(lines, "model: "+model)
		}
		return []byte("---\n" + strings.Join(lines, "\n") + "\n---\n\n" + text)
	}
	hasName, hasDescription := false, false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "name:"):
			// The folder decides the slash command, so the declared name must
			// agree with it or the two would disagree about what to invoke.
			lines[i] = "name: " + name
			hasName = true
		case strings.HasPrefix(line, "description:"):
			hasDescription = true
		}
	}
	if !hasName {
		lines = append([]string{"name: " + name}, lines...)
	}
	if !hasDescription {
		lines = append(lines, "description: "+quoteYAML(describeSkill(name, rest)))
	}
	if model != "" {
		lines = append(lines, "model: "+model)
	}

	return []byte("---\n" + strings.Join(lines, "\n") + "\n---\n" + rest)
}

// splitFrontmatter separates a leading `---` block from the rest of the body.
func splitFrontmatter(text string) (front, rest string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text, false
	}
	end := strings.Index(text[4:], "\n---")
	if end == -1 {
		return "", text, false
	}
	front = strings.Trim(text[4:4+end], "\n")
	rest = strings.TrimPrefix(text[4+end+4:], "\n")
	return front, rest, true
}

// describeSkill derives the `description` Claude Code uses to decide when a
// skill is relevant. The gofi skills open with `# /gofi-eng — Context
// Engineer`, so the role after the dash is the honest one-line summary; the
// slug is the fallback when a skill has no such heading.
func describeSkill(name, body string) string {
	role := ""
	if m := skillHeading.FindStringSubmatch(body); m != nil {
		title := strings.TrimSpace(m[1])
		if parts := regexp.MustCompile(`\s+[—–-]\s+`).Split(title, 2); len(parts) == 2 {
			role = strings.TrimSpace(parts[1])
		} else {
			role = strings.TrimSpace(strings.TrimPrefix(title, "/"))
		}
	}
	if role == "" {
		role = name
	}
	return fmt.Sprintf("%s — agente do projeto gofi, invocado por /%s.", role, name)
}

// quoteYAML wraps a scalar in double quotes when it could otherwise be
// misparsed — descriptions routinely contain `:` and `#`.
func quoteYAML(value string) string {
	if !strings.ContainsAny(value, `:#"'\`+"\n") {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", " ")
	return `"` + escaped + `"`
}

// pruneLegacySkillFile removes the pre-fix `.claude/skills/<name>.md` left by
// an older gofi. Leaving it behind is harmless to Claude Code — it ignores the
// file — but it makes `.claude/skills/` list every skill twice and keeps the
// broken shape looking authoritative.
//
// Only files whose name matches a skill gofi just installed are removed, so a
// hand-written skill next to them is untouched.
func pruneLegacySkillFile(claudeDir, name string) error {
	legacy := filepath.Join(claudeDir, skillsDirName, name+".md")
	if err := os.Remove(legacy); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove legacy skill %s: %w", legacy, err)
	}
	return nil
}

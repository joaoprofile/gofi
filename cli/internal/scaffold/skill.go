package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"os"
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
// the three places that write skills (fresh install, update plan, `gofi agent
// add`) cannot disagree about it.
const (
	skillsDirName = "skills"
	skillFileName = "SKILL.md"
)

// skillRelPath is the path of a skill inside .claude/, e.g.
// `skills/gofi-eng/SKILL.md`.
func skillRelPath(name string) string {
	return filepath.Join(skillsDirName, name, skillFileName)
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
func renderSkill(name string, body []byte) []byte {
	text := string(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")))

	front, rest, ok := splitFrontmatter(text)
	if !ok {
		return []byte(buildFrontmatter(name, describeSkill(name, text)) + text)
	}

	lines := strings.Split(front, "\n")
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

func buildFrontmatter(name, description string) string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n", name, quoteYAML(description))
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

// MigrateSkillsLayout converts a project installed by an older gofi — flat
// `.claude/skills/<name>.md` files — to the folder layout Claude Code
// discovers. `gofi update` reinstalls skills anyway; this exists so a project
// can be repaired without a full update, and so the conversion is testable.
func MigrateSkillsLayout(claudeDir string) error {
	skillsDir := filepath.Join(claudeDir, skillsDirName)
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read %s: %w", skillsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// A folder is the right shape, but only if its manifest is spelled
			// SKILL.md — `skill.md` is ignored just as silently as a flat file,
			// and is an easy thing to get wrong by hand.
			if err := fixSkillFileCase(filepath.Join(skillsDir, entry.Name())); err != nil {
				return err
			}
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		body, err := os.ReadFile(filepath.Join(skillsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read skill %s: %w", name, err)
		}
		target := filepath.Join(claudeDir, skillRelPath(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, renderSkill(name, body), 0o644); err != nil {
			return err
		}
		if err := pruneLegacySkillFile(claudeDir, name); err != nil {
			return err
		}
	}
	return nil
}

// fixSkillFileCase renames a case-variant manifest (skill.md, Skill.md) to
// SKILL.md. Claude Code matches the name exactly, so the wrong case is
// invisible in a directory listing but fatal to discovery.
func fixSkillFileCase(skillDir string) error {
	if _, err := os.Stat(filepath.Join(skillDir, skillFileName)); err == nil {
		return nil // already correct
	}
	entries, err := os.ReadDir(skillDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", skillDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(entry.Name(), skillFileName) {
			continue
		}
		from := filepath.Join(skillDir, entry.Name())
		to := filepath.Join(skillDir, skillFileName)
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("rename %s to %s: %w", from, to, err)
		}
		// Re-render so the frontmatter is there too — a hand-made manifest
		// usually has none, which is the other half of the same trap.
		body, err := os.ReadFile(to)
		if err != nil {
			return err
		}
		return os.WriteFile(to, renderSkill(filepath.Base(skillDir), body), 0o644)
	}
	return nil
}

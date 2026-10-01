package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// LegacyInstructions is where releases before AGENTS.md installed a project's
// instructions. Claude Code stops reading AGENTS.md while any CLAUDE.md is in
// the project, so a project cannot have both.
const LegacyInstructions = ".claude/CLAUDE.md"

// BlockingInstructions are the files, other than the legacy one, that make
// Claude Code skip AGENTS.md. They are the team's, never gofi's to remove.
var BlockingInstructions = []string{"CLAUDE.md", "CLAUDE.local.md"}

// AgentsPlan is what `gofi update agents` is about to do, computed before
// anything is written so the user confirms exactly what happens.
type AgentsPlan struct {
	// Write is the content AGENTS.md will hold; nil leaves it as it is.
	Write []byte
	// Note says why: "new", "updated", "moved from .claude/CLAUDE.md", or,
	// when Write is nil, "unchanged" or "kept (edited)".
	Note string
	// FromUpstream marks a Write that is the release's own text, whose hash is
	// recorded so a later update can tell the team's edits from it.
	FromUpstream bool
	// RemoveLegacy removes .claude/CLAUDE.md, after a backup.
	RemoveLegacy bool
	// LegacyEdited is set when the removed file carried the team's edits.
	LegacyEdited bool
	// Blocking lists the team's own CLAUDE.md files still in the way.
	Blocking []string
}

// PlanAgents decides how a project moves to, or stays current on, AGENTS.md.
// upstream is the release's AGENTS.md. A file the manifest does not record
// was written by the team and is theirs; force puts upstream back anyway,
// with everything it replaces backed up.
func PlanAgents(projectRoot string, upstream []byte, force bool) (AgentsPlan, error) {
	var p AgentsPlan
	man := LoadManifest(projectRoot)
	read := func(rel string) ([]byte, bool, error) {
		b, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return b, err == nil, err
	}
	edited := func(rel string, content []byte) bool {
		if recorded, ok := man[rel]; ok {
			return recorded != hashBytes(content)
		}
		return !bytes.Equal(content, upstream)
	}
	// AGENTS.md is compared without the project's block: that part is the
	// team's by contract, and editing it is not editing gofi's text.
	agentsEdited := func(content []byte) bool {
		if recorded, ok := man[AgentsFile]; ok {
			return recorded != hashBytes(gofiPart(content))
		}
		return !bytes.Equal(gofiPart(content), gofiPart(upstream))
	}

	agents, hasAgents, err := read(AgentsFile)
	if err != nil {
		return p, err
	}
	legacy, hasLegacy, err := read(LegacyInstructions)
	if err != nil {
		return p, err
	}

	if hasLegacy {
		p.RemoveLegacy = true
		p.LegacyEdited = edited(LegacyInstructions, legacy)
		// The team's instructions are the project's; the file name is what is
		// wrong. Carried over as they are, never merged with upstream text.
		if p.LegacyEdited && !hasAgents && !force {
			p.Write, p.Note = legacy, "moved from "+LegacyInstructions
			p.Blocking = blocking(projectRoot)
			return p, nil
		}
	}

	_, hasBlock := projectBlock(agents)
	switch {
	case !hasAgents:
		p.Write, p.Note, p.FromUpstream = upstream, "new", true
	case bytes.Equal(gofiPart(agents), gofiPart(upstream)):
		p.Note = "unchanged"
	case agentsEdited(agents) && !force:
		p.Note = "kept (edited)"
		if !hasBlock {
			p.Note = "kept (edited, no project block)"
		}
	default:
		// gofi's text is replaced; the project's block always stays.
		p.Write, p.Note, p.FromUpstream = adopt(upstream, agents), "updated", true
		if !hasBlock {
			p.Write = upstream
		}
	}
	p.Blocking = blocking(projectRoot)
	return p, nil
}

func blocking(projectRoot string) []string {
	var out []string
	for _, rel := range BlockingInstructions {
		if _, err := os.Stat(filepath.Join(projectRoot, rel)); err == nil {
			out = append(out, rel)
		}
	}
	return out
}

// Empty reports whether the plan changes nothing.
func (p AgentsPlan) Empty() bool { return p.Write == nil && !p.RemoveLegacy }

// ApplyAgents carries a plan out. Whatever it replaces or removes is copied to
// .gofi/backup/ first.
func ApplyAgents(projectRoot string, p AgentsPlan) error {
	pr := newPreserver(projectRoot, InstallReset)
	if p.Write != nil {
		target := filepath.Join(projectRoot, AgentsFile)
		if old, err := os.ReadFile(target); err == nil && !bytes.Equal(old, p.Write) {
			pr.backup(AgentsFile, old)
		}
		if err := os.WriteFile(target, p.Write, 0o644); err != nil {
			return err
		}
		if p.FromUpstream {
			pr.record(AgentsFile, gofiPart(p.Write))
		}
	}
	if p.RemoveLegacy {
		legacy := filepath.Join(projectRoot, filepath.FromSlash(LegacyInstructions))
		if old, err := os.ReadFile(legacy); err == nil {
			pr.backup(LegacyInstructions, old)
		}
		if err := os.Remove(legacy); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return pr.save()
}

// ReadAgentsSource returns the release's AGENTS.md from a source tree.
func ReadAgentsSource(agentsFS fs.FS, srcRoot string) ([]byte, error) {
	return readAgentsFile(agentsFS, srcRoot)
}

// BackupDir is where ApplyAgents and the installs copy what they replace.
func BackupDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".gofi", "backup")
}

// hostOnly are the entries of an agents folder that belong to one host and
// mean nothing to another: Claude Code's settings and subagents.
var hostOnly = map[string][]string{
	".claude": {"settings.json", "settings.local.json", "agents"},
}

// MoveHome moves a project's agents folder when its host changed: the content
// is the project's whatever the host, only the folder's name follows the host.
// What only the old host reads is backed up and left out. It reports what it
// dropped. A target that already exists is never merged into.
func MoveHome(projectRoot, from, to string) ([]string, error) {
	src, dst := filepath.Join(projectRoot, from), filepath.Join(projectRoot, to)
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("%s/ already exists — merge it with %s/ by hand, gofi will not choose between them", to, from)
	}
	pr := newPreserver(projectRoot, InstallReset)
	var dropped []string
	for _, name := range hostOnly[from] {
		p := filepath.Join(src, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		_ = filepath.WalkDir(p, func(f string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if b, err := os.ReadFile(f); err == nil {
					pr.backup(relSlash(projectRoot, f), b)
				}
			}
			return nil
		})
		if err := os.RemoveAll(p); err != nil {
			return dropped, err
		}
		dropped = append(dropped, from+"/"+name)
	}
	return dropped, os.Rename(src, dst)
}

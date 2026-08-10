package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// IsValidAgent reports whether name is one of the supported agent names.
func IsValidAgent(name string) bool {
	for _, a := range allAgents {
		if a == name {
			return true
		}
	}
	return false
}

// AllAgents returns the canonical list of agent names.
func AllAgents() []string {
	return append([]string(nil), allAgents...)
}

// InstallAgentFromFS reads ai/skills/<name>/SKILL.md from the given gofi
// monorepo tree (rooted at agentsRoot inside srcFS) and writes it to
// <projectRoot>/.claude/skills/<name>/SKILL.md — the layout Claude Code
// discovers — along with any resource the skill folder bundles. Also creates
// the per-agent knowledge directory.
func InstallAgentFromFS(srcFS fs.FS, agentsRoot, projectRoot, agentName string) error {
	if !IsValidAgent(agentName) {
		return fmt.Errorf("unknown agent %q", agentName)
	}
	body, err := readSkillSource(srcFS, agentsRoot, agentName)
	if err != nil {
		return fmt.Errorf("read skill %s: %w", agentName, err)
	}
	claudeDir := filepath.Join(projectRoot, ".claude")
	dst := filepath.Join(claudeDir, skillRelPath(agentName))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, renderSkill(agentName, body), 0o644); err != nil {
		return err
	}
	err = walkSkillResources(srcFS, agentsRoot, agentName, func(rel string, content []byte) error {
		resource := filepath.Join(claudeDir, skillsDirName, agentName, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(resource), 0o755); err != nil {
			return err
		}
		return os.WriteFile(resource, content, 0o644)
	})
	if err != nil {
		return fmt.Errorf("install resources of skill %s: %w", agentName, err)
	}
	if err := pruneLegacySkillFile(claudeDir, agentName); err != nil {
		return err
	}
	if short := agentToKnowledgeDir[agentName]; short != "" {
		if err := os.MkdirAll(filepath.Join(projectRoot, ".claude", "knowledge", short), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAgent deletes skills/<agentName>/ and (optionally) the per-agent
// knowledge directory. Returns an error if the agent name is unknown.
func RemoveAgent(projectRoot, agentName string, removeKnowledge bool) error {
	if !IsValidAgent(agentName) {
		return fmt.Errorf("unknown agent %q", agentName)
	}
	claudeDir := filepath.Join(projectRoot, ".claude")
	skillDir := filepath.Join(claudeDir, skillsDirName, agentName)
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("remove skill: %w", err)
	}
	// Also clear the flat file an older gofi may have left here.
	if err := pruneLegacySkillFile(claudeDir, agentName); err != nil {
		return err
	}
	if removeKnowledge {
		if short := agentToKnowledgeDir[agentName]; short != "" {
			dir := filepath.Join(projectRoot, ".claude", "knowledge", short)
			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("remove knowledge dir: %w", err)
			}
		}
	}
	return nil
}

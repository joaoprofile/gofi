package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// IsValidAgent reports whether name is one of the four supported agent names.
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

// InstallAgentFromFS reads ai/skills/<name>.md from the given gofi monorepo
// tree (rooted at agentsRoot inside srcFS) and writes it to
// <projectRoot>/.claude/skills/<name>.md. Also creates the per-agent
// knowledge directory.
func InstallAgentFromFS(srcFS fs.FS, agentsRoot, projectRoot, agentName string) error {
	if !IsValidAgent(agentName) {
		return fmt.Errorf("unknown agent %q", agentName)
	}
	src := path.Join(agentsRoot, "ai", "skills", agentName+".md")
	body, err := fs.ReadFile(srcFS, src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	dst := filepath.Join(projectRoot, ".claude", "skills", agentName+".md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		return err
	}
	if short := agentToKnowledgeDir[agentName]; short != "" {
		if err := os.MkdirAll(filepath.Join(projectRoot, ".claude", "knowledge", short), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAgent deletes commands/<agentName>.md and (optionally) the per-agent
// knowledge directory. Returns an error if the agent name is unknown.
func RemoveAgent(projectRoot, agentName string, removeKnowledge bool) error {
	if !IsValidAgent(agentName) {
		return fmt.Errorf("unknown agent %q", agentName)
	}
	skill := filepath.Join(projectRoot, ".claude", "skills", agentName+".md")
	if err := os.Remove(skill); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove skill: %w", err)
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

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/doctor"
	"github.com/joaoprofile/gofi/cli/internal/host"
)

func tieredProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".claude/skills/gofi-pd/SKILL.md"), "---\nname: gofi-pd\ndescription: PRD.\nmodel: opus\n---\n# /gofi-pd\n")
	writeFile(t, filepath.Join(root, ".claude/skills/gofi-pd/contract.yaml"), "tier: deep\n")
	writeFile(t, filepath.Join(root, ".claude/skills/gofi-status/SKILL.md"), "---\nname: gofi-status\ndescription: Status.\nmodel: haiku\n---\n# /gofi-status\n")
	writeFile(t, filepath.Join(root, ".claude/skills/gofi-status/contract.yaml"), "tier: light\n")
	return root
}

func TestCheckTiers(t *testing.T) {
	root := tieredProject(t)
	cfg := &config.GofiConfig{}

	if c := checkTiers(cfg, root, host.ClaudeCode); c.Status != doctor.StatusOK || !strings.Contains(c.Detail, "deep→opus") {
		t.Errorf("skills in step: %+v", c)
	}

	cfg.AI.Tiers = map[string]string{"deep": "fable"}
	c := checkTiers(cfg, root, host.ClaudeCode)
	if c.Status != doctor.StatusWarn || !strings.Contains(c.Detail, "gofi-pd") || strings.Contains(c.Detail, "gofi-status") || !strings.Contains(c.Hint, "gofi update skills") {
		t.Errorf("a remapped tier must name the stale skill: %+v", c)
	}

	cfg.AI.Tiers = map[string]string{"light": "gpt-5-mini"}
	if c := checkTiers(cfg, root, host.ClaudeCode); c.Status != doctor.StatusWarn || !strings.Contains(c.Detail, "gpt-5-mini") {
		t.Errorf("a model Claude cannot resolve must be named: %+v", c)
	}

	cfg.AI.Tiers = map[string]string{"light": "claude-haiku-4-5-20251001"}
	if c := checkTiers(cfg, root, host.ClaudeCode); c.Status != doctor.StatusWarn || !strings.Contains(c.Detail, "gofi-status") {
		t.Errorf("a full ID is valid, and still a remap: %+v", c)
	}

	if c := checkTiers(&config.GofiConfig{}, root, host.Codex); c.Status != doctor.StatusOK || !strings.Contains(c.Detail, "session") {
		t.Errorf("a host without a model field: %+v", c)
	}
}

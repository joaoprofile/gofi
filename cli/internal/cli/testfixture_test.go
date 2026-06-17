package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// fixtureRepoFiles is the minimum gofi monorepo tree the CLI tests need: 4
// skills, the AI/Claude CLAUDE.md, the templates, the memory template and a
// small ai/sdk/go/ tree. All harness content lives under ai/. It is
// intentionally tiny so tests stay fast and the shape stays obvious;
// production repos will have far richer content.
var fixtureRepoFiles = map[string]string{
	"ai/skills/gofi-pd.md":                  "# /gofi-pd — fixture skill",
	"ai/skills/gofi-spec.md":                "# /gofi-spec — fixture skill",
	"ai/skills/gofi-eng.md":                 "# /gofi-eng — fixture skill",
	"ai/skills/gofi-qa.md":                  "# /gofi-qa — fixture skill",
	"ai/claude/CLAUDE.md":                   "# CLAUDE — fixture",
	"ai/claude/README.md":                   "# README — fixture",
	"ai/templates/sdd-template.md":          "# SDD — fixture",
	"ai/templates/prd-template.md":          "# PRD — fixture",
	"ai/memory/project.md.tmpl":             "# Memory — {{.ProjectName}}",
	"ai/sdk/go/boilerplates/model.md":       "fixture model boilerplate",
	"ai/sdk/go/sdk-docs/overview.md":        "fixture sdk overview",
	"ai/sdk/go/knowledge/error-handling.md": "fixture error handling knowledge",
	"env/localstack/.env-example":           "APP_NAME=fixture\nLOG_LEVEL=debug\n",
	"env/localstack/docker-compose.yml":     "services: {}\n",
	"env/localstack/prometheus.yml":         "global: {}\n",
}

// writeFixtureRepo materialises fixtureRepoFiles in a temp directory and
// returns its path. Tests then point GOFI_AGENTS_LOCAL_DIR at it so the CLI
// reads from there instead of fetching the real GitHub repo.
func writeFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range fixtureRepoFiles {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

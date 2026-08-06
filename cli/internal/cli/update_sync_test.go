package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
)

// The regression this guard exists for: a project that adapted its design-system
// docs to its own product used to find them wiped by the next update, because the
// surface tree was removed and recopied wholesale.
func TestUpdateKeepsTunedSurfaceDocs(t *testing.T) {
	repo := writeFixtureRepo(t)
	t.Setenv("GOFI_AGENTS_LOCAL_DIR", repo)
	upstreamTokens := filepath.Join(repo, "ai/sdk/web/knowledge/design-tokens.md")
	writeFile(t, upstreamTokens, "upstream tokens v1")

	root := t.TempDir()
	data := scaffold.TemplateData{ProjectName: "svc", Language: "go"}
	ref := "github.com/joaoprofile/gofi-agents@main"
	if _, err := installFromSource(root, "go", []string{"web"}, ref, "", data, scaffold.InstallNew); err != nil {
		t.Fatalf("install: %v", err)
	}

	tuned := filepath.Join(root, ".claude/sdk/web/knowledge/design-tokens.md")
	writeFile(t, tuned, "our own tokens")
	writeFile(t, upstreamTokens, "upstream tokens v2")
	writeFile(t, filepath.Join(repo, "ai/sdk/go/sdk-docs/overview.md"), "fixture sdk overview v2")

	if _, err := installFromSource(root, "go", []string{"web"}, ref, "", data, scaffold.InstallUpdate); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got := readFile(t, tuned); got != "our own tokens" {
		t.Errorf("the tuned tokens were replaced: %q", got)
	}
	// The untouched neighbour still has to move, or the guard would have frozen
	// the whole tree instead of protecting one file.
	if got := readFile(t, filepath.Join(root, ".claude/sdk/go/sdk-docs/overview.md")); got != "fixture sdk overview v2" {
		t.Errorf("an untouched doc should have been refreshed, got %q", got)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeProjectConfig materialises a .gofi.yaml verbatim and returns its root,
// so a test can start from the exact bytes an older CLI would have written.
func writeProjectConfig(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	body = strings.ReplaceAll(body, "{{ROOT}}", root)
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const v1Config = `version: 1
project:
  name: svc
  root: {{ROOT}}
  language: go
  path: src
ai:
  host: claude-vscode
  model: claude-opus-5
agents: [gofi-eng]
sources:
  agents: github.com/joaoprofile/gofi-agents@main
git:
  remote: origin
`

func TestSyncConfigRewritesAnOldSchema(t *testing.T) {
	root := writeProjectConfig(t, v1Config)
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	note := syncConfig(cfg)

	if note == "" {
		t.Fatal("rewriting an old config should be reported to the user")
	}
	if got := config.FileVersion(filepath.Join(root, config.FileName)); got != config.CurrentVersion {
		t.Errorf("on-disk version = %d, want %d", got, config.CurrentVersion)
	}
	body, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, block := range []string{"backend:", "hsec:", "sonar:", "test:", "ops:", "graph:"} {
		if !strings.Contains(string(body), block) {
			t.Errorf("%s missing from the rewritten config:\n%s", block, body)
		}
	}
}

// The rewrite must be a repair, not a reset: a project already on the current
// schema with every block written is left alone, so `gofi update` does not
// churn .gofi.yaml on every run.
func TestSyncConfigIsSilentOnACurrentProject(t *testing.T) {
	root := writeProjectConfig(t, v1Config)
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	syncConfig(cfg)

	reloaded, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatal(err)
	}

	if note := syncConfig(reloaded); note != "" {
		t.Errorf("a second run should change nothing, got %q", note)
	}

	after, err := os.ReadFile(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the file was rewritten with nothing to repair")
	}
}

// Seeding fills gaps. A team that disabled a block finds it disabled after the
// update — otherwise the repair would silently re-enable a scan they turned off.
func TestSyncConfigKeepsATeamsChoice(t *testing.T) {
	root := writeProjectConfig(t, v1Config+`hsec:
  enabled: false
  severity_threshold: LOW
  output_format: text
`)
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	syncConfig(cfg)

	reloaded, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Hsec.Enabled || reloaded.Hsec.SeverityThreshold != "LOW" {
		t.Errorf("hsec was overwritten: %+v", reloaded.Hsec)
	}
}

// The v1 shape the old gofi-ui material produced: two web surfaces under `ui:`,
// each with brand as a token block. The current schema has one web block, so a
// straight rewrite would delete the second surface.
const v1TwoSurfaces = `version: 1
project:
  name: svc
  root: {{ROOT}}
  language: go
  path: src
ui:
  web:
    framework: react
    path: frontend/web
    brand:
      surface: "#dcebfb"
      action: "#025cb2"
  backoffice:
    framework: react
    path: frontend/backoffice
ai:
  host: claude-vscode
  model: claude-opus-5
agents: [gofi-eng]
sources:
  agents: github.com/joaoprofile/gofi-agents@main
`

func TestSyncConfigFoldsABrandBlock(t *testing.T) {
	body := strings.Replace(v1TwoSurfaces,
		"  backoffice:\n    framework: react\n    path: frontend/backoffice\n", "", 1)
	root := writeProjectConfig(t, body)
	path := filepath.Join(root, config.FileName)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	note := syncConfig(cfg)

	if !strings.Contains(note, "ui.web.brand") {
		t.Errorf("the fold must be named to the user, note = %q", note)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Quoted either way — a bare #rrggbb would read back as a comment.
	if !strings.Contains(string(out), `brand: '#dcebfb'`) && !strings.Contains(string(out), `brand: "#dcebfb"`) {
		t.Errorf("brand should be rewritten as the quoted seed:\n%s", out)
	}
	if strings.Contains(string(out), "onBrand") || strings.Contains(string(out), "\nui:") {
		t.Errorf("the legacy shape should be gone:\n%s", out)
	}
}

// The whole point of the rewrite: an old project comes out on the current
// schema with nothing dropped. The third surface has no named block, so it
// keeps its own name under surfaces: rather than being deleted.
func TestSyncConfigKeepsASurfaceTheSchemaCannotName(t *testing.T) {
	root := writeProjectConfig(t, v1TwoSurfaces)
	path := filepath.Join(root, config.FileName)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	note := syncConfig(cfg)
	if !strings.Contains(note, "surfaces.backoffice") {
		t.Errorf("the move must be named to the user, note = %q", note)
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	s := reloaded.Surfaces["backoffice"]
	if s == nil {
		t.Fatalf("the surface was dropped: %+v", reloaded.Surfaces)
	}
	if s.Path != "frontend/backoffice" || s.Framework != "react" {
		t.Errorf("the surface must arrive whole, got %+v", s)
	}
	if reloaded.Frontend == nil || reloaded.Frontend.Path != "frontend/web" {
		t.Errorf("web still belongs in frontend:, got %+v", reloaded.Frontend)
	}
}

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/audit"
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

// The rule the whole update rests on: once `gofi init` has run, the project's
// own files are the team's. An update that quietly rewrote .gofi.yaml — even
// to repair it — would be editing a file nobody asked it to touch. The schema
// drift is reported by the audit instead, naming the command that fixes it.
func TestUpdateLeavesTheConfigAlone(t *testing.T) {
	root := writeProjectConfig(t, v1Config)
	path := filepath.Join(root, config.FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	syncGraph(context.Background(), cfg)

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf(".gofi.yaml was rewritten by the update:\n%s", after)
	}
	if got := config.FileVersion(path); got != 1 {
		t.Errorf("on-disk version = %d, want the v1 the project had", got)
	}

	// And the drift is still visible, or leaving the file alone would just be
	// silence.
	var reported bool
	for _, f := range audit.Run(root, audit.Options{GraphEnabled: true}) {
		if f.Area == "config" && f.Item == "version" {
			reported = true
		}
	}
	if !reported {
		t.Error("the stale schema must show up in the audit")
	}
}

package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// project writes a fixture where keys are slash-separated paths relative to a
// fresh temp dir.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

// find returns the first finding whose Item matches, so assertions name the
// thing they care about instead of an index into a slice.
func find(fs []Finding, item string) (Finding, bool) {
	for _, f := range fs {
		if f.Item == item {
			return f, true
		}
	}
	return Finding{}, false
}

const currentConfig = `version: 2
project:
  name: x
  root: /r
ai:
  host: claude-vscode
  model: opus
  models: [claude-opus-4-8]
graph:
  enabled: true
hsec:
  enabled: false
sonar:
  enabled: false
ops:
  cloud: oci
test:
  default: go test ./...
`

func TestConfigOnOldSchema(t *testing.T) {
	root := project(t, map[string]string{".gofi.yaml": "version: 1\nproject:\n  name: x\n"})
	got := Run(root, Options{})

	f, ok := find(got, "version")
	if !ok {
		t.Fatalf("an old schema must be reported, got %+v", got)
	}
	if f.Severity != SeverityWarn {
		t.Errorf("severity = %v, want warn", f.Severity)
	}
	if !strings.Contains(f.Detail, "v1") {
		t.Errorf("detail should name the version found: %q", f.Detail)
	}
	// The blocks that came later are the other half of "written by an old CLI".
	for _, key := range []string{"hsec:", "sonar:", "test:", "ops:"} {
		if _, ok := find(got, key); !ok {
			t.Errorf("%s absent from an old config should be reported", key)
		}
	}
}

// The block's absence still means every default, so nothing breaks without it —
// but the default scan mode is fast, and a project that cannot see the setting
// cannot weigh what its agents may conclude from the graph.
func TestConfigWithoutGraphBlockIsReported(t *testing.T) {
	cfg := strings.Replace(currentConfig, "graph:\n  enabled: true\n", "", 1)
	root := project(t, map[string]string{".gofi.yaml": cfg})
	f, ok := find(Run(root, Options{}), "graph:")
	if !ok {
		t.Fatal("an absent graph block should be reported")
	}
	if f.Severity != SeverityInfo {
		t.Errorf("severity = %v, want info: the project still works without it", f.Severity)
	}
}

func TestConfigCurrentIsSilent(t *testing.T) {
	root := project(t, map[string]string{".gofi.yaml": currentConfig})
	for _, f := range Run(root, Options{}) {
		if f.Area == "config" {
			t.Errorf("a current config should raise nothing, got %+v", f)
		}
	}
}

// A missing ai.models list is what makes the panel's model picker fall back to
// its built-ins, so it is worth naming even though nothing breaks.
func TestConfigWithoutModelList(t *testing.T) {
	cfg := strings.Replace(currentConfig, "  models: [claude-opus-4-8]\n", "", 1)
	root := project(t, map[string]string{".gofi.yaml": cfg})
	if _, ok := find(Run(root, Options{}), "ai.models:"); !ok {
		t.Error("a config without ai.models should be reported")
	}
}

func TestDocWithoutFrontmatterIsInvisible(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml":                   currentConfig,
		"specs/billing/sdd-billing.md": "# SDD — billing\n\nsem frontmatter.\n",
		"specs/INDEX.md":               "index\n",
	})
	f, ok := find(Run(root, Options{}), "specs/billing/sdd-billing.md")
	if !ok {
		t.Fatal("a document without frontmatter must be reported")
	}
	if f.Severity != SeverityWarn {
		t.Errorf("severity = %v, want warn — retrieval cannot see it", f.Severity)
	}
}

func TestDocMissingFrontmatterKeys(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml":                 currentConfig,
		"prd/billing/prd-billing.md": "---\ntipo: prd\ncontexto: billing\nversao: \"1.0\"\n---\n# PRD\n",
		"prd/INDEX.md":               "index\n",
	})
	f, ok := find(Run(root, Options{}), "prd/billing/prd-billing.md")
	if !ok {
		t.Fatal("a document missing keywords/status should be reported")
	}
	if !strings.Contains(f.Detail, "keywords") || !strings.Contains(f.Detail, "status") {
		t.Errorf("detail should name the missing keys, got %q", f.Detail)
	}
}

func TestDocComplete(t *testing.T) {
	const doc = `---
tipo: spec
contexto: billing
versao: "1.0"
status: aprovado
keywords: [faturamento, cobranca]
atualizado: 2026-08-03
---
# SDD — billing
`
	root := project(t, map[string]string{
		".gofi.yaml":                   currentConfig,
		"specs/billing/sdd-billing.md": doc,
		"specs/INDEX.md":               "index\n",
	})
	for _, f := range Run(root, Options{}) {
		if f.Area == "specs" {
			t.Errorf("a complete spec should raise nothing, got %+v", f)
		}
	}
}

// The pre-RAG templates carried author/version/date in the body. Superseded,
// not broken — so it is info, and it must not drown out real problems.
func TestDocWithLegacyProvenance(t *testing.T) {
	const doc = `---
tipo: spec
contexto: billing
versao: "1.0"
status: aprovado
keywords: [faturamento]
---
# SDD — billing

**Autor:** alguem
`
	root := project(t, map[string]string{
		".gofi.yaml":                   currentConfig,
		"specs/billing/sdd-billing.md": doc,
		"specs/INDEX.md":               "index\n",
	})
	f, ok := find(Run(root, Options{}), "specs/billing/sdd-billing.md")
	if !ok {
		t.Fatal("legacy provenance should be reported")
	}
	if f.Severity != SeverityInfo {
		t.Errorf("severity = %v, want info — the document still works", f.Severity)
	}
}

func TestMissingIndexOnlyWhenDocsExist(t *testing.T) {
	withDocs := project(t, map[string]string{
		".gofi.yaml":                   currentConfig,
		"specs/billing/sdd-billing.md": "---\ncontexto: billing\nversao: \"1\"\nstatus: x\nkeywords: [a]\n---\n",
	})
	if _, ok := find(Run(withDocs, Options{}), "specs/INDEX.md"); !ok {
		t.Error("specs with documents and no INDEX should be reported")
	}

	// An empty specs/ is a project that has not specced yet, not a stale one.
	empty := project(t, map[string]string{".gofi.yaml": currentConfig, "specs/.keep": ""})
	if _, ok := find(Run(empty, Options{}), "specs/INDEX.md"); ok {
		t.Error("an empty specs/ should not demand an INDEX")
	}
}

// The reason this audit exists: update preserves .claude/knowledge/, so a file
// added upstream after scaffolding never lands, while the skills that ship in
// the same update already cite it.
func TestKnowledgeFileNeverArrived(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml": currentConfig,
		".claude/knowledge/shared/ddd-principles.md": "x\n",
		".claude/scripts/gen-index.sh":               "x\n",
	})
	upstream := fstest.MapFS{
		"ai/knowledge/shared/ddd-principles.md":           {Data: []byte("x")},
		"ai/knowledge/shared/graph-retrieval-protocol.md": {Data: []byte("x")},
	}

	f, ok := find(Run(root, Options{Upstream: upstream, UpstreamRoot: "."}), ".claude/knowledge/shared/")
	if !ok {
		t.Fatal("a knowledge file present upstream and absent locally must be reported")
	}
	if !strings.Contains(f.Detail, "graph-retrieval-protocol.md") {
		t.Errorf("detail should name the missing file, got %q", f.Detail)
	}
	if strings.Contains(f.Detail, "ddd-principles.md") {
		t.Errorf("a file that is present must not be listed as missing: %q", f.Detail)
	}
}

func TestKnowledgeInSyncIsSilent(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml": currentConfig,
		".claude/knowledge/shared/ddd-principles.md": "x\n",
		".claude/scripts/gen-index.sh":               "x\n",
	})
	upstream := fstest.MapFS{"ai/knowledge/shared/ddd-principles.md": {Data: []byte("x")}}
	if _, ok := find(Run(root, Options{Upstream: upstream, UpstreamRoot: "."}), ".claude/knowledge/shared/"); ok {
		t.Error("a knowledge tree in sync should raise nothing")
	}
}

// Without the upstream tree there is no way to know what is missing, and
// guessing would produce a report the user cannot trust.
func TestNoUpstreamSkipsKnowledgeCheck(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml":                   currentConfig,
		".claude/scripts/gen-index.sh": "x\n",
	})
	if _, ok := find(Run(root, Options{}), ".claude/knowledge/shared/"); ok {
		t.Error("the knowledge check must not run without an upstream tree")
	}
}

func TestGraphEnabledButNeverBuilt(t *testing.T) {
	root := project(t, map[string]string{".gofi.yaml": currentConfig})
	if _, ok := find(Run(root, Options{GraphEnabled: true}), ".gofi/graph/"); !ok {
		t.Error("an enabled graph that was never built should be reported")
	}
	if _, ok := find(Run(root, Options{GraphEnabled: false}), ".gofi/graph/"); ok {
		t.Error("a disabled graph should not be reported")
	}
}

func TestMissingConfigIsFatal(t *testing.T) {
	f, ok := find(Run(t.TempDir(), Options{}), ".gofi.yaml")
	if !ok {
		t.Fatal("a project without .gofi.yaml must be reported")
	}
	if f.Severity != SeverityWarn {
		t.Errorf("severity = %v, want warn", f.Severity)
	}
}

// A brand written as a token map is what the gofi-ui material used to document,
// so real projects carry it — and it is the one drift that stops config.Load
// entirely, leaving the audit as the only command that can name the key.
func TestBrandWrittenAsABlockIsNamed(t *testing.T) {
	const cfg = `version: 2
project:
  name: x
  root: /r
ui:
  web:
    framework: react
    path: web
    brand:
      surface: "#dcebfb"
      action: "#025cb2"
  backoffice:
    framework: react
    path: backoffice
    brand: blue
`
	root := project(t, map[string]string{".gofi.yaml": cfg})
	got := Run(root, Options{})

	f, ok := find(got, "ui.web.brand")
	if !ok {
		t.Fatalf("a brand written as a block must be named, got %+v", got)
	}
	// Info, not warn: the loader folds it and update writes it back as a string,
	// so it is drift the CLI closes rather than something to go fix by hand.
	if f.Severity != SeverityInfo {
		t.Errorf("severity = %v, want info", f.Severity)
	}
	if _, ok := find(got, "ui.backoffice.brand"); ok {
		t.Error("a brand already written as a string is not drift")
	}
	if _, ok := find(got, "ui.web"); ok {
		t.Error("a sub-surface is a surface, not a mistyped field")
	}
}

// The surfaces the current schema names must be scanned the same way as the
// legacy ui: block — a project on frontend:/mobile: is not exempt.
func TestFrontendSurfaceIsScannedToo(t *testing.T) {
	const cfg = `version: 2
project:
  name: x
frontend:
  framework: react
  path: web
  styling:
    engine: tailwind
`
	root := project(t, map[string]string{".gofi.yaml": cfg})
	if _, ok := find(Run(root, Options{}), "frontend.styling"); !ok {
		t.Error("a frontend field written as a block must be named")
	}
}

// The pre-v2.4 dirs are not harmless leftovers: the agents may read them as if
// they were the live tree, so the audit has to name them and point at the one
// command that clears them.
func TestLegacySDKLayoutIsReported(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml":                          currentConfig,
		".claude/scripts/gen-index.sh":        "x\n",
		".claude/boilerplates/model.md":       "old\n",
		".claude/gofi-sdk-go/sdk-docs/x.md":   "old\n",
		".claude/sdk/go/sdk-docs/overview.md": "current\n",
	})

	f, ok := find(Run(root, Options{}), ".claude/")
	if !ok {
		t.Fatal("a project still carrying the pre-v2.4 dirs must be told")
	}
	for _, want := range []string{".claude/boilerplates/", ".claude/gofi-sdk-go/"} {
		if !strings.Contains(f.Detail, want) {
			t.Errorf("detail should name %s, got %q", want, f.Detail)
		}
	}
	if !strings.Contains(f.Hint, "gofi update sdk") {
		t.Errorf("the hint must name the command that removes them, got %q", f.Hint)
	}
}

func TestCurrentSDKLayoutIsSilent(t *testing.T) {
	root := project(t, map[string]string{
		".gofi.yaml":                          currentConfig,
		".claude/scripts/gen-index.sh":        "x\n",
		".claude/sdk/go/sdk-docs/overview.md": "current\n",
	})
	if f, ok := find(Run(root, Options{}), ".claude/"); ok {
		t.Errorf("a project on the current layout should raise nothing, got %q", f.Detail)
	}
}

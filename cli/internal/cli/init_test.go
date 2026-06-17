package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/toolchain"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

// forceToolchain overrides the preflight so pipeline tests don't depend on the
// host having (or lacking) Go/Node, and never shell out to npm/npx.
func forceToolchain(t *testing.T, p toolchain.Preflight) {
	t.Helper()
	orig := detectToolchain
	detectToolchain = func(toolchain.Needs) toolchain.Preflight { return p }
	t.Cleanup(func() { detectToolchain = orig })
}

func TestExecutePipeline_WebOnly_NodeMissing(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: false})
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Environments = []string{wizard.EnvWeb}
	r.Language = ""
	r.WebPath = "web"
	r.WebDS = config.DSWeb

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(target, config.FileName))
	for _, want := range []string{"frontend:", "ops:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in .gofi.yaml:\n%s", want, data)
		}
	}
	for _, d := range []string{"ops", "specs", "prd"} {
		if _, err := os.Stat(filepath.Join(target, d, ".gitkeep")); err != nil {
			t.Errorf("expected %s/.gitkeep: %v", d, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "web")); !os.IsNotExist(err) {
		t.Errorf("web app must not be created when Node is missing")
	}
	if _, err := os.Stat(filepath.Join(target, "go.work")); !os.IsNotExist(err) {
		t.Errorf("go.work must not exist for a front-only project")
	}
	if len(r.Skipped) == 0 {
		t.Errorf("expected web recorded as skipped")
	}
}

func TestExecutePipeline_WebAndMobile_NodeMissing(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: false})
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Environments = []string{wizard.EnvWeb, wizard.EnvMobile}
	r.Language = ""
	r.WebPath, r.WebDS = "web", config.DSWeb
	r.MobilePath, r.MobileDS = "mobile", config.DSMobile

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(target, config.FileName))
	for _, want := range []string{"frontend:", "mobile:"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in .gofi.yaml:\n%s", want, data)
		}
	}
	if len(r.Skipped) != 2 {
		t.Errorf("expected web+mobile skipped, got %v", r.Skipped)
	}
}

func TestExecutePipeline_AlwaysCreatesOpsDir(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: true})
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target) // back+go only
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "ops", ".gitkeep")); err != nil {
		t.Errorf("ops/ must always be created: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(target, config.FileName))
	if !strings.Contains(string(data), "ops:") {
		t.Errorf("expected ops: block in .gofi.yaml")
	}
}

func goWizardResult(target string) *wizard.Result {
	return &wizard.Result{
		AIHost:         "claude-vscode",
		AIModel:        "claude-opus-4-8",
		Name:           "my-svc",
		Root:           target,
		Environments:   []string{wizard.EnvBack},
		Language:       "go",
		SourcePath:     "src",
		GoModule:       "github.com/acme/my-svc",
		Agents:         []string{"gofi-pd", "gofi-spec", "gofi-eng", "gofi-qa"},
		AgentsRef:      "github.com/joaoprofile/gofi@main",
		CreateSpecsDir: true,
		CreatePrdDir:   true,
	}
}

// useFixtureRepo points the CLI at an in-memory gofi-agents tree so tests can
// exercise the fetch path without hitting GitHub.
func useFixtureRepo(t *testing.T) {
	t.Helper()
	t.Setenv("GOFI_AGENTS_LOCAL_DIR", writeFixtureRepo(t))
}

func TestExecutePipeline_Go(t *testing.T) {
	useFixtureRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")

	r := goWizardResult(target)
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	for _, p := range []string{
		".git",
		".gofi.yaml",
		".gitignore",
		".env",
		"go.work",
		"src/go.mod",
		"src/my-svc/main.go",
		".claude/CLAUDE.md",
		".claude/skills/gofi-pd.md",
		".claude/skills/gofi-spec.md",
		".claude/skills/gofi-eng.md",
		".claude/skills/gofi-qa.md",
		".claude/templates/sdd-template.md",
		".claude/memory/project.md",
		".claude/knowledge/shared",
		".claude/knowledge/pd",
		".claude/sdk/go/boilerplates/model.md",
		".claude/sdk/go/sdk-docs/overview.md",
		".claude/sdk/go/knowledge/error-handling.md",
	} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
	// .env should be gitignored so local-only config never lands in git.
	gitignore, err := os.ReadFile(filepath.Join(target, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignore), ".env") {
		t.Errorf("expected .env in .gitignore, got:\n%s", gitignore)
	}

	// Backend projects seed ops/localstack/ with the gofi repo's localstack
	// config files (docker-compose + observability), copied from env/localstack/.
	for _, p := range []string{
		"ops/localstack/docker-compose.yml",
		"ops/localstack/prometheus.yml",
	} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
	// The .env template is the source of the project .env — it must not leak
	// into ops/localstack/.
	if _, err := os.Stat(filepath.Join(target, "ops/localstack/.env-example")); err == nil {
		t.Errorf(".env-example should not be copied into ops/localstack/")
	}
	// The project-root .env is seeded from env/localstack/.env-example, not empty.
	env, err := os.ReadFile(filepath.Join(target, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	if !strings.Contains(string(env), "APP_NAME=") {
		t.Errorf("expected .env populated from .env-example, got:\n%s", env)
	}

	// Pre-v2.4 flat dirs must NOT appear under the new layout.
	for _, gone := range []string{
		".claude/boilerplates",
		".claude/gofi-sdk-go",
		".claude/sdk-knowledge",
	} {
		if _, err := os.Stat(filepath.Join(target, gone)); err == nil {
			t.Errorf("legacy path %s should not exist after fresh install", gone)
		}
	}
}

func TestExecutePipeline_GoWithRemote(t *testing.T) {
	useFixtureRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")

	r := goWizardResult(target)
	r.GitRemote = "git@github.com:acme/my-svc.git"
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	// .gofi.yaml records the remote
	data, err := os.ReadFile(filepath.Join(target, ".gofi.yaml"))
	if err != nil {
		t.Fatalf("read .gofi.yaml: %v", err)
	}
	if !strings.Contains(string(data), "git@github.com:acme/my-svc.git") {
		t.Errorf("expected remote in .gofi.yaml, got:\n%s", data)
	}
}

// Preview backend languages (no scaffold yet) no longer abort init — the
// pipeline records the backend as skipped, still writes .gofi.yaml and the
// .claude/ harness, and creates no Go workspace.
func TestExecutePipeline_PreviewBackendIsSkipped(t *testing.T) {
	useFixtureRepo(t)
	for _, lang := range []string{"rust", "java", "csharp", "nodejs"} {
		t.Run(lang, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "my-svc")
			r := goWizardResult(target)
			r.Language = lang
			r.GoModule = ""
			if err := executePipeline(r); err != nil {
				t.Fatalf("pipeline should succeed for preview backend %q: %v", lang, err)
			}
			if _, err := os.Stat(filepath.Join(target, config.FileName)); err != nil {
				t.Errorf("expected .gofi.yaml written: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, "go.work")); !os.IsNotExist(err) {
				t.Errorf("go.work should not exist for preview backend %q", lang)
			}
			if len(r.Skipped) == 0 {
				t.Errorf("expected a skipped surface for preview backend %q", lang)
			}
		})
	}
}

func TestExecutePipeline_GoWithSDKOverride(t *testing.T) {
	useFixtureRepo(t)

	sdkDir := t.TempDir()
	for rel, body := range map[string]string{
		"boilerplates/model.md":       "override model boilerplate",
		"sdk-docs/overview.md":        "override sdk overview",
		"knowledge/error-handling.md": "override error handling",
		"go.mod":                      "module github.com/joaoprofile/gofi\n\ngo 1.25\n",
		"sqln/go.mod":                 "module github.com/joaoprofile/gofi/sqln\n\ngo 1.25\n",
		"sqln/sqln.go":                "package sqln\n",
		"iam/go.mod":                  "module github.com/joaoprofile/gofi/iam\n\ngo 1.25\n",
		"iam/iam.go":                  "package iam\n",
	} {
		full := filepath.Join(sdkDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOFI_SDK_LOCAL_DIR", sdkDir)

	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")
	r := goWizardResult(target)
	r.SDKURLs = map[string]string{r.Language: "github.com/acme/gofi-sdk-go@main"}
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	for _, p := range []string{
		".gofi/gofi-sdk-go/go.mod",
		".gofi/gofi-sdk-go/sqln/sqln.go",
		".gofi/gofi-sdk-go/iam/iam.go",
		".gofi/gofi-sdk-go/boilerplates/model.md",
		".claude/sdk/go/boilerplates/model.md",
		".claude/sdk/go/sdk-docs/overview.md",
		".claude/sdk/go/knowledge/error-handling.md",
	} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}

	work, err := os.ReadFile(filepath.Join(target, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	for _, want := range []string{
		"./src",
		"./.gofi/gofi-sdk-go",
		"./.gofi/gofi-sdk-go/sqln",
		"./.gofi/gofi-sdk-go/iam",
	} {
		if !strings.Contains(string(work), want) {
			t.Errorf("expected %q in go.work, got:\n%s", want, work)
		}
	}

	// Override docs come from sdk fixture, not the agents fixture's sdk/go/.
	body, err := os.ReadFile(filepath.Join(target, ".claude/sdk/go/boilerplates/model.md"))
	if err != nil {
		t.Fatalf("read installed boilerplate: %v", err)
	}
	if !strings.Contains(string(body), "override") {
		t.Errorf("expected SDK override content in installed boilerplate, got: %s", body)
	}
}

// TestExecutePipeline_GoWithoutSDKOverride pins the no-override behavior:
// .gofi/gofi-sdk-<lang>/ is NOT created, go.work stays single-line, docs come
// from the gofi-agents bundled SDK.
func TestExecutePipeline_GoWithoutSDKOverride(t *testing.T) {
	useFixtureRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")

	r := goWizardResult(target)
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, ".gofi/gofi-sdk-go")); !os.IsNotExist(err) {
		t.Errorf(".gofi/gofi-sdk-go should not exist when no SDK override is configured (err=%v)", err)
	}
	work, err := os.ReadFile(filepath.Join(target, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	if strings.Contains(string(work), ".gofi/gofi-sdk-go") {
		t.Errorf("go.work should not reference local SDK without override, got:\n%s", work)
	}
}

func TestExecutePipeline_AgentFiltering(t *testing.T) {
	useFixtureRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")
	r := goWizardResult(target)
	r.Agents = []string{"gofi-spec", "gofi-eng"}
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, kept := range []string{"gofi-spec.md", "gofi-eng.md"} {
		if _, err := os.Stat(filepath.Join(target, ".claude/skills", kept)); err != nil {
			t.Errorf("expected %s kept: %v", kept, err)
		}
	}
	for _, gone := range []string{"gofi-pd.md", "gofi-qa.md"} {
		if _, err := os.Stat(filepath.Join(target, ".claude/skills", gone)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed", gone)
		}
	}
}

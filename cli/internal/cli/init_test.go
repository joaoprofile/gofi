package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/detect"
	"github.com/joaoprofile/gofi/cli/internal/gitops"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/toolchain"
	"github.com/joaoprofile/gofi/cli/internal/tui/wizard"
)

// forceToolchain overrides the preflight so pipeline tests don't depend on the
// host having (or lacking) Go/Node, and never shell out to npm/npx.
func forceToolchain(t *testing.T, p toolchain.Preflight) {
	t.Helper()
	orig := detectToolchain
	// Claude Code is required, not skipped around: a test about Go or Node
	// has it present unless it says otherwise.
	p.ClaudeOK = true
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
		Module:         "github.com/acme/my-svc",
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
		"AGENTS.md",
		".claude/skills/gofi-pd/SKILL.md",
		".claude/skills/gofi-spec/SKILL.md",
		".claude/skills/gofi-eng/SKILL.md",
		".claude/skills/gofi-qa/SKILL.md",
		".claude/templates/sdd-template.md",
		".claude/memory/project.md",
		".claude/knowledge/shared",
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

// TestExecutePipeline_AdoptsExistingGoTree covers `gofi init` inside a
// repository that already has Go code: the harness is installed around it, and
// nothing of the user's tree is rewritten or joined by scaffold leftovers.
func TestExecutePipeline_AdoptsExistingGoTree(t *testing.T) {
	useFixtureRepo(t)
	target := t.TempDir()

	mainGo := "package main\n\nfunc main() { println(\"legacy\") }\n"
	for rel, body := range map[string]string{
		"go.mod":      "module github.com/acme/legacy\n\ngo 1.24\n",
		"cmd/main.go": mainGo,
		"README.md":   "# legacy\n",
		".gitignore":  "*.log\n",
	} {
		full := filepath.Join(target, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r := goWizardResult(target)
	r.SourcePath = "."
	r.Detected = detect.Scan(target)
	if r.Detected.Backend.Path != "." {
		t.Fatalf("fixture should be detected at the root, got %+v", r.Detected.Backend)
	}

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	// The user's files are untouched.
	if body, _ := os.ReadFile(filepath.Join(target, "go.mod")); !strings.Contains(string(body), "acme/legacy") {
		t.Errorf("go.mod was rewritten: %s", body)
	}
	if body, _ := os.ReadFile(filepath.Join(target, "cmd/main.go")); string(body) != mainGo {
		t.Errorf("main.go was rewritten: %s", body)
	}
	if body, _ := os.ReadFile(filepath.Join(target, "README.md")); !strings.Contains(string(body), "# legacy") {
		t.Errorf("README.md was rewritten: %s", body)
	}

	// No scaffold leftovers.
	for _, junk := range []string{"src", "domain", ".migrations", "my-svc"} {
		if _, err := os.Stat(filepath.Join(target, junk)); err == nil {
			t.Errorf("%s belongs to the scaffold and must not appear in an adopted tree", junk)
		}
	}

	// The harness is installed, and go.work now wires the adopted module.
	for _, p := range []string{".gofi.yaml", "AGENTS.md", "go.work"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
	work, _ := os.ReadFile(filepath.Join(target, "go.work"))
	if !strings.Contains(string(work), "use .") {
		t.Errorf("go.work should use the root module, got:\n%s", work)
	}
	// The pre-existing .gitignore keeps its rules and gains gofi's.
	gitignore, _ := os.ReadFile(filepath.Join(target, ".gitignore"))
	if !strings.Contains(string(gitignore), "*.log") {
		t.Errorf("existing .gitignore rules were dropped:\n%s", gitignore)
	}
}

// TestExecutePipeline_AdoptsExistingWebApp covers the surface where adoption is
// most dangerous: create-vite writes into the folder it is handed and the design
// system starter replaces the app entry files, so an existing app must never
// reach the scaffolder — even with Node present and the DS selected.
func TestExecutePipeline_AdoptsExistingWebApp(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: true})
	scaffolded := false
	origVite := createViteApp
	createViteApp = func(root, path string, useDS bool) error {
		scaffolded = true
		return nil
	}
	t.Cleanup(func() { createViteApp = origVite })

	target := t.TempDir()
	appTSX := "export default function App() { return <h1>legacy</h1> }\n"
	for rel, body := range map[string]string{
		"apps/web/package.json":  `{"name":"legacy-web","dependencies":{"react":"^18.0.0","vite":"^5.0.0"}}`,
		"apps/web/src/App.tsx":   appTSX,
		"apps/web/src/main.tsx":  "import App from './App'\n",
		"apps/web/src/index.css": ":root { color: rebeccapurple }\n",
	} {
		full := filepath.Join(target, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	r := goWizardResult(target)
	r.Environments = []string{wizard.EnvWeb}
	r.Language = ""
	r.WebPath = "apps/web"
	r.WebDS = config.DSWeb
	r.Detected = detect.Scan(target)
	if r.Detected.Web.Path != "apps/web" {
		t.Fatalf("fixture should be detected as a web app, got %+v", r.Detected.Web)
	}

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	if scaffolded {
		t.Error("create-vite ran over an adopted app and would have overwritten it")
	}
	if body, _ := os.ReadFile(filepath.Join(target, "apps/web/src/App.tsx")); string(body) != appTSX {
		t.Errorf("App.tsx was rewritten: %s", body)
	}
	if body, _ := os.ReadFile(filepath.Join(target, "apps/web/package.json")); !strings.Contains(string(body), "legacy-web") {
		t.Errorf("package.json was rewritten: %s", body)
	}
	// The harness still lands, and the config records the adopted path.
	for _, p := range []string{".gofi.yaml", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
	cfgBody, _ := os.ReadFile(filepath.Join(target, config.FileName))
	if !strings.Contains(string(cfgBody), "apps/web") {
		t.Errorf("adopted web path missing from .gofi.yaml:\n%s", cfgBody)
	}
}

// TestExecutePipeline_AdoptedMobileIsNotScaffolded is the same guarantee for
// Expo, which additionally deletes a nested .claude/ from the app folder.
func TestExecutePipeline_AdoptedMobileIsNotScaffolded(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: true})
	scaffolded := false
	origExpo := createExpoApp
	createExpoApp = func(root, path string, useDS bool) error {
		scaffolded = true
		return nil
	}
	t.Cleanup(func() { createExpoApp = origExpo })

	target := t.TempDir()
	appDir := filepath.Join(target, "apps", "mobile")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := `{"name":"legacy-mobile","dependencies":{"expo":"^51.0.0","react-native":"0.74.0"}}`
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}

	r := goWizardResult(target)
	r.Environments = []string{wizard.EnvMobile}
	r.Language = ""
	r.MobilePath = "apps/mobile"
	r.Detected = detect.Scan(target)
	if r.Detected.Mobile.Path != "apps/mobile" {
		t.Fatalf("fixture should be detected as a mobile app, got %+v", r.Detected.Mobile)
	}

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if scaffolded {
		t.Error("create-expo-app ran over an adopted app")
	}
	if body, _ := os.ReadFile(filepath.Join(appDir, "package.json")); !strings.Contains(string(body), "legacy-mobile") {
		t.Errorf("package.json was rewritten: %s", body)
	}
}

// TestExecutePipeline_AdoptedNonGoBackendIsNotReportedAsMissing guards the
// message: an adopted Python tree has nothing left to scaffold, so telling the
// user a scaffold was skipped describes a gap that does not exist.
func TestExecutePipeline_AdoptedNonGoBackendIsNotReportedAsMissing(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: true})
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "pyproject.toml"), []byte("[project]\nname = \"legacy\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := goWizardResult(target)
	r.Language = config.LanguagePython
	r.SourcePath = "."
	r.Detected = detect.Scan(target)
	if r.Detected.Backend.Language != config.LanguagePython {
		t.Fatalf("fixture should be detected as python, got %+v", r.Detected.Backend)
	}

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, s := range r.Skipped {
		if strings.Contains(s, "scaffold not implemented") {
			t.Errorf("adopted backend reported as a skipped scaffold: %q", s)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "go.work")); err == nil {
		t.Error("a python project must not receive a go.work")
	}
}

func TestCheckRootIsUsable_AdoptsDetectedProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkRootIsUsable(root, detect.Scan(root)); err != nil {
		t.Fatalf("a recognised codebase is the reason to init there: %v", err)
	}
	// The same folder, unrecognised, is still refused.
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "notes.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkRootIsUsable(other, detect.Scan(other)); err == nil {
		t.Error("a non-empty folder with nothing recognisable should be refused")
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
	// git keeps the remote; .gofi.yaml does not repeat it
	if got, _ := gitops.GetRemote(target, "origin"); got != r.GitRemote {
		t.Errorf("origin = %q, want %q", got, r.GitRemote)
	}
	data, err := os.ReadFile(filepath.Join(target, ".gofi.yaml"))
	if err != nil {
		t.Fatalf("read .gofi.yaml: %v", err)
	}
	if strings.Contains(string(data), "git@github.com:acme/my-svc.git") {
		t.Errorf(".gofi.yaml should not record the remote, got:\n%s", data)
	}
}

// A backend in a language without a gofi SDK gets no skeleton, but init still
// writes the harness, says what it skipped, and never leaves a go.work behind —
// a Java or Node project that carried one would confuse every tool that looks
// for it.
func TestExecutePipeline_NonGoBackendsAreReported(t *testing.T) {
	useFixtureRepo(t)
	cases := []struct {
		language string
		module   string
		manifest string
	}{
		{"csharp", "Acme.MySvc", "src/my-svc/my-svc.csproj"},
	}
	for _, tc := range cases {
		t.Run(tc.language, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "my-svc")
			r := goWizardResult(target)
			r.Language = tc.language
			r.Module = tc.module
			if err := executePipeline(r); err != nil {
				t.Fatalf("pipeline: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, config.FileName)); err != nil {
				t.Errorf("expected .gofi.yaml written: %v", err)
			}
			if _, err := os.Stat(filepath.Join(target, tc.manifest)); !os.IsNotExist(err) {
				t.Errorf("%s should not be scaffolded", tc.manifest)
			}
			if _, err := os.Stat(filepath.Join(target, "go.work")); !os.IsNotExist(err) {
				t.Errorf("go.work should not exist for a %s backend", tc.language)
			}
			if len(r.Skipped) != 1 {
				t.Errorf("Skipped = %v, want the missing skeleton reported", r.Skipped)
			}
		})
	}
}

// C# has no scaffold. Init must still complete — the .gofi.yaml and the
// .claude/ harness are the point — and say so instead of writing nothing
// silently.
func TestExecutePipeline_BackendWithoutScaffoldIsReported(t *testing.T) {
	useFixtureRepo(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "my-svc")
	r := goWizardResult(target)
	r.Language = config.LanguageCSharp
	r.Module = "Acme.MySvc"

	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, config.FileName)); err != nil {
		t.Errorf("expected .gofi.yaml written: %v", err)
	}
	if len(r.Skipped) == 0 {
		t.Error("expected the csharp backend to be reported as skipped")
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
		"sqln/sqln.go":                "// Package sqln is the SQL layer.\npackage sqln\n\n// Open opens the connection.\nfunc Open() error { return nil }\n",
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

	// The API reference is generated from the checkout the project pinned.
	ref, err := os.ReadFile(filepath.Join(target, ".claude/sdk/go/api/sqln.md"))
	if err != nil || !strings.Contains(string(ref), "### sqln.Open") || !strings.Contains(string(ref), "Open opens the connection.") {
		t.Errorf("generated reference = %q (%v)", ref, err)
	}
	if _, err := os.Stat(filepath.Join(target, ".claude/sdk/go/api/INDEX.md")); err != nil {
		t.Errorf("reference index missing: %v", err)
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

// init installs every skill: there is no agent selection to honour.
func TestExecutePipeline_InstallsEverySkill(t *testing.T) {
	useFixtureRepo(t)
	target := filepath.Join(t.TempDir(), "my-svc")
	if err := executePipeline(goWizardResult(target)); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, kept := range []string{"gofi-pd/SKILL.md", "gofi-spec/SKILL.md", "gofi-eng/SKILL.md", "gofi-qa/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(target, ".claude/skills", kept)); err != nil {
			t.Errorf("expected %s installed: %v", kept, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(target, config.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"\nagents:", "\ntraining:", "\ngit:"} {
		if strings.Contains(string(b), gone) {
			t.Errorf(".gofi.yaml still writes %q", strings.TrimSpace(gone))
		}
	}
}

// Without Claude Code at the minimum there are no agents to scaffold for, and
// stopping before anything is written leaves nothing to clean up.
func TestExecutePipeline_StopsWithoutClaudeCode(t *testing.T) {
	useFixtureRepo(t)
	orig := detectToolchain
	detectToolchain = func(toolchain.Needs) toolchain.Preflight {
		return toolchain.Preflight{GoOK: true, NodeOK: true, ClaudeOK: false}
	}
	t.Cleanup(func() { detectToolchain = orig })

	target := filepath.Join(t.TempDir(), "my-svc")
	err := executePipeline(goWizardResult(target))
	if err == nil || !strings.Contains(err.Error(), toolchain.MinClaudeCode) {
		t.Fatalf("err = %v, want the minimum Claude Code named", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("init created the project before checking Claude Code")
	}
}

// A project on Codex gets one agents folder, .agents/, and its MCP config in
// Codex's format — and nothing of Claude Code's.
func TestExecutePipeline_CodexHost(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{GoOK: true, NodeOK: true})
	t.Cleanup(func() { layout.SetHome(layout.DefaultHome) })
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.AIHost = host.Codex.ID

	if err := executePipeline(r); err != nil {
		t.Fatalf("executePipeline: %v", err)
	}
	for _, p := range []string{".agents/skills", "AGENTS.md", ".codex/config.toml", ".gofi.yaml"} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(p))); err != nil {
			t.Errorf("expected %s: %v", p, err)
		}
	}
	for _, p := range []string{".claude", ".mcp.json"} {
		if _, err := os.Stat(filepath.Join(target, p)); !os.IsNotExist(err) {
			t.Errorf("%s was created for a Codex project", p)
		}
	}
	cfg, err := config.Load(filepath.Join(target, config.FileName))
	if err != nil || cfg.AI.Host != host.Codex.ID {
		t.Errorf("ai.host = %v (%v)", cfg, err)
	}
}

// A NestJS backend gets the starter its pack's sensors run — tsc, ESLint,
// Jest with JUnit — and its dependencies installed, so the first run has
// them.
func TestExecutePipeline_NestJSBackend(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{NodeOK: true})
	var installed []string
	orig := npmInstall
	npmInstall = func(dir string) error { installed = append(installed, dir); return nil }
	t.Cleanup(func() { npmInstall = orig })
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Language, r.Module = "nodejs", "@acme/my-svc"
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, p := range []string{"src/package.json", "src/tsconfig.json", "src/eslint.config.mjs", "src/src/main.ts", "src/src/app.module.ts"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("%s not scaffolded: %v", p, err)
		}
	}
	pkg, _ := os.ReadFile(filepath.Join(target, "src/package.json"))
	for _, want := range []string{`"name": "my-svc"`, `"jest-junit"`, `"@microsoft/eslint-formatter-sarif"`} {
		if !strings.Contains(string(pkg), want) {
			t.Errorf("package.json lacks %s:\n%s", want, pkg)
		}
	}
	if len(installed) != 1 || installed[0] != filepath.Join(target, "src") {
		t.Errorf("npm install ran in %v", installed)
	}
	if _, err := os.Stat(filepath.Join(target, "go.work")); !os.IsNotExist(err) {
		t.Error("go.work written for a Node backend")
	}
}

// A FastAPI backend gets the starter its pack's sensors run — mypy, ruff,
// pytest with JUnit — and its virtual environment.
func TestExecutePipeline_FastAPIBackend(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{})
	var made []string
	orig := pythonEnv
	pythonEnv = func(dir string) error { made = append(made, dir); return nil }
	t.Cleanup(func() { pythonEnv = orig })
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Language, r.Module = "python", ""
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, p := range []string{"src/pyproject.toml", "src/app/main.py", "src/app/__init__.py"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("%s not scaffolded: %v", p, err)
		}
	}
	if py, _ := os.ReadFile(filepath.Join(target, "src/pyproject.toml")); !strings.Contains(string(py), `name = "my-svc"`) {
		t.Errorf("pyproject:\n%s", py)
	}
	if len(made) != 1 || made[0] != filepath.Join(target, "src") {
		t.Errorf("venv made in %v", made)
	}
}

// A Quarkus backend gets the starter under its module's package, and the
// module is kept in .gofi.yaml: where the pack reads a context's code.
func TestExecutePipeline_QuarkusBackend(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{})
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Language, r.Module = "java", "com.acme.mysvc"
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, p := range []string{"src/pom.xml", "src/src/main/java/com/acme/mysvc", "src/src/test/java/com/acme/mysvc", "src/src/main/resources/application.properties"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("%s not scaffolded: %v", p, err)
		}
	}
	pom, _ := os.ReadFile(filepath.Join(target, "src/pom.xml"))
	if !strings.Contains(string(pom), "<groupId>com.acme</groupId>") || !strings.Contains(string(pom), "<artifactId>mysvc</artifactId>") {
		t.Errorf("pom:\n%s", pom)
	}
	cfg, err := config.Load(filepath.Join(target, config.FileName))
	if err != nil || cfg.Backend.Module != "com.acme.mysvc" {
		t.Errorf("backend.module = %+v, %v", cfg.Backend, err)
	}
}

// An Axum backend gets the starter its pack's sensors run — cargo build,
// clippy, cargo test —, the contexts declared in its library.
func TestExecutePipeline_AxumBackend(t *testing.T) {
	useFixtureRepo(t)
	forceToolchain(t, toolchain.Preflight{})
	target := filepath.Join(t.TempDir(), "my-svc")
	r := goWizardResult(target)
	r.Language, r.Module = "rust", ""
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	for _, p := range []string{"src/Cargo.toml", "src/src/main.rs", "src/src/lib.rs", "src/migrations"} {
		if _, err := os.Stat(filepath.Join(target, p)); err != nil {
			t.Errorf("%s not scaffolded: %v", p, err)
		}
	}
	if c, _ := os.ReadFile(filepath.Join(target, "src/Cargo.toml")); !strings.Contains(string(c), `name = "my-svc"`) {
		t.Errorf("Cargo.toml:\n%s", c)
	}
	if _, err := os.Stat(filepath.Join(target, "go.work")); !os.IsNotExist(err) {
		t.Error("go.work written for a Rust backend")
	}
}

// A run had no budget and only the flow's limits unless the team wrote them:
// a change that did not converge spent until every rework edge was used. A
// new project's .gofi.yaml is written bounded, and the bounds load back.
func TestBuildConfig_NewProjectRunsAreBounded(t *testing.T) {
	cfg := buildConfig(&wizard.Result{
		AIHost: config.AIHostClaudeVSCode, AIModel: config.DefaultModel,
		Name: "acme", Root: t.TempDir(), Environments: []string{wizard.EnvBack},
		Language: config.LanguageGo, SourcePath: "backend", Module: "example.com/acme",
		AgentsRef: config.DefaultAgentsRef,
	})
	path := filepath.Join(t.TempDir(), config.FileName)
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Loop == nil || got.Loop.BudgetUSD != config.DefaultLoopBudgetUSD || got.Loop.MaxIterations != config.DefaultLoopMaxIterations {
		t.Errorf("a new project's runs must start bounded, got loop %+v", got.Loop)
	}
}

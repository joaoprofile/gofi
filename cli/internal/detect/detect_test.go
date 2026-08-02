package detect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// tree writes a fixture where keys are slash-separated paths relative to a
// fresh temp dir. Content is irrelevant to detection except for package.json.
func tree(t *testing.T, files map[string]string) string {
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

func pkg(deps string) string {
	return `{"name":"x","dependencies":{` + deps + `}}`
}

func TestScanBackendLanguages(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		language string
		marker   string
	}{
		{"go", map[string]string{"go.mod": "module x\n"}, config.LanguageGo, "go.mod"},
		{"rust", map[string]string{"Cargo.toml": ""}, config.LanguageRust, "Cargo.toml"},
		{"java-maven", map[string]string{"pom.xml": ""}, config.LanguageJava, "pom.xml"},
		{"java-gradle", map[string]string{"build.gradle.kts": ""}, config.LanguageJava, "build.gradle.kts"},
		{"csharp", map[string]string{"Api.csproj": ""}, config.LanguageCSharp, "Api.csproj"},
		{"python", map[string]string{"pyproject.toml": ""}, config.LanguagePython, "pyproject.toml"},
		{"nodejs", map[string]string{"package.json": pkg(`"express":"4"`)}, config.LanguageNodeJS, "package.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Scan(tree(t, tc.files))
			if !got.Backend.Found() {
				t.Fatalf("expected a backend, got %+v", got)
			}
			if got.Backend.Path != "." {
				t.Errorf("Path = %q, want %q", got.Backend.Path, ".")
			}
			if got.Backend.Language != tc.language {
				t.Errorf("Language = %q, want %q", got.Backend.Language, tc.language)
			}
			if got.Backend.Marker != tc.marker {
				t.Errorf("Marker = %q, want %q", got.Backend.Marker, tc.marker)
			}
		})
	}
}

func TestScanFrontendFrameworks(t *testing.T) {
	cases := []struct {
		name      string
		deps      string
		framework string
	}{
		{"react", `"react":"18"`, config.FrameworkReact},
		{"vue", `"vue":"3"`, config.FrameworkVue},
		{"angular", `"@angular/core":"18"`, config.FrameworkAngular},
		{"svelte", `"svelte":"4"`, config.FrameworkSvelte},
		{"astro", `"astro":"4","react":"18"`, config.FrameworkAstro},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Scan(tree(t, map[string]string{"package.json": pkg(tc.deps)}))
			if !got.Web.Found() {
				t.Fatalf("expected a web surface, got %+v", got)
			}
			if got.Web.Framework != tc.framework {
				t.Errorf("Framework = %q, want %q", got.Web.Framework, tc.framework)
			}
		})
	}
}

func TestScanExpoIsMobileNotWeb(t *testing.T) {
	got := Scan(tree(t, map[string]string{
		"package.json": pkg(`"expo":"51","react":"18","react-native":"0.74"`),
	}))
	if !got.Mobile.Found() {
		t.Fatalf("expected a mobile surface, got %+v", got)
	}
	if got.Mobile.Framework != config.FrameworkReactNative {
		t.Errorf("Framework = %q, want %q", got.Mobile.Framework, config.FrameworkReactNative)
	}
	if got.Web.Found() {
		t.Errorf("an Expo app is not a web surface, got %+v", got.Web)
	}
}

func TestScanMonorepo(t *testing.T) {
	got := Scan(tree(t, map[string]string{
		"backend/go.mod":        "module x\n",
		"frontend/index.html":   "",
		"frontend/package.json": pkg(`"react":"18"`),
		"mobile/package.json":   pkg(`"expo":"51"`),
	}))
	if got.Backend.Path != "backend" || got.Backend.Language != config.LanguageGo {
		t.Errorf("backend = %+v", got.Backend)
	}
	if got.Web.Path != "frontend" || got.Web.Framework != config.FrameworkReact {
		t.Errorf("web = %+v", got.Web)
	}
	if got.Mobile.Path != "mobile" {
		t.Errorf("mobile = %+v", got.Mobile)
	}
}

func TestScanNestedWorkspace(t *testing.T) {
	// A workspace root whose package.json is nothing but tooling must not claim
	// the web surface away from the real app two levels down.
	got := Scan(tree(t, map[string]string{
		"package.json":                `{"name":"root","workspaces":["apps/*"],"devDependencies":{"turbo":"2"}}`,
		"apps/web/package.json":       pkg(`"vue":"3"`),
		"services/api/pyproject.toml": "",
	}))
	if got.Web.Path != "apps/web" || got.Web.Framework != config.FrameworkVue {
		t.Errorf("web = %+v", got.Web)
	}
	if got.Backend.Path != "services/api" || got.Backend.Language != config.LanguagePython {
		t.Errorf("backend = %+v", got.Backend)
	}
}

func TestScanShallowestWins(t *testing.T) {
	got := Scan(tree(t, map[string]string{
		"go.mod":          "module x\n",
		"examples/go.mod": "module x/examples\n",
	}))
	if got.Backend.Path != "." {
		t.Errorf("Path = %q, want %q", got.Backend.Path, ".")
	}
}

func TestScanIgnoresDependencyDirs(t *testing.T) {
	got := Scan(tree(t, map[string]string{
		"node_modules/left-pad/package.json": pkg(`"react":"18"`),
		"vendor/dep/go.mod":                  "module dep\n",
	}))
	if got.Any() {
		t.Errorf("nothing should be detected inside dependency dirs, got %+v", got)
	}
}

func TestScanEmptyDir(t *testing.T) {
	if got := Scan(t.TempDir()); got.Any() {
		t.Errorf("expected nothing, got %+v", got)
	}
}

func TestWebFrameworkDefaultsToReact(t *testing.T) {
	dir := t.TempDir()
	if got := WebFramework(dir); got != config.FrameworkReact {
		t.Errorf("no package.json: got %q, want %q", got, config.FrameworkReact)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := WebFramework(dir); got != config.FrameworkReact {
		t.Errorf("broken package.json: got %q, want %q", got, config.FrameworkReact)
	}
}

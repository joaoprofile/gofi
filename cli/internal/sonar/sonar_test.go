package sonar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func TestBuildProperties_Defaults(t *testing.T) {
	c := config.DefaultSonar("my-service", &config.Backend{Language: config.LanguageGo, Path: "src"}, nil, nil)
	body, err := BuildProperties(c, config.LanguageGo)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out := string(body)

	for _, want := range []string{
		"sonar.projectKey=my-service",
		"sonar.projectName=my-service",
		"sonar.sources=src",
		"sonar.go.coverage.reportPaths=coverage.out",
		"sonar.sourceEncoding=UTF-8",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected properties to contain %q:\n%s", want, out)
		}
	}

	// First-party scoping: tests, mocks and the SDK checkout are excluded.
	for _, want := range []string{"**/*_test.go", "**/mocks/**", "**/.gofi/**"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected exclusions to contain %q:\n%s", want, out)
		}
	}
	// Exclusions also feed coverage.exclusions so excluded files don't skew it.
	if !strings.Contains(out, "sonar.coverage.exclusions=") {
		t.Errorf("expected coverage.exclusions to be set:\n%s", out)
	}
}

func TestBuildProperties_MultiSurfaceSources(t *testing.T) {
	c := config.DefaultSonar("svc",
		&config.Backend{Language: config.LanguageGo, Path: "backend"},
		&config.UISurface{Path: "web"},
		&config.UISurface{Path: "mobile"},
	)
	body, err := BuildProperties(c, config.LanguageGo)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(string(body), "sonar.sources=backend,web,mobile") {
		t.Errorf("expected combined sources, got:\n%s", body)
	}
}

func TestBuildProperties_FrontOnlyNoCoverageKey(t *testing.T) {
	c := config.DefaultSonar("svc", nil, &config.UISurface{Path: "web"}, nil)
	body, err := BuildProperties(c, "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(string(body), "coverage.reportPaths") {
		t.Errorf("front-only project should not emit a coverage report path:\n%s", body)
	}
	if !strings.Contains(string(body), "sonar.sources=web") {
		t.Errorf("expected sources=web, got:\n%s", body)
	}
}

func TestBuildProperties_RequiresProjectKey(t *testing.T) {
	if _, err := BuildProperties(config.SonarConfig{}, ""); err == nil {
		t.Fatal("expected error when project key is empty")
	}
}

func TestWriteConfig_RoundTrip(t *testing.T) {
	root := t.TempDir()
	c := config.DefaultSonar("svc", &config.Backend{Language: config.LanguageGo, Path: "src"}, nil, nil)
	path, err := WriteConfig(root, c, config.LanguageGo)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if path != filepath.Join(root, ConfigFileName) {
		t.Errorf("unexpected path: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(b), "sonar.projectKey=svc") {
		t.Errorf("expected key in output:\n%s", b)
	}
}

func TestMissingEnv(t *testing.T) {
	t.Setenv("SONAR_TOKEN", "")
	t.Setenv("SONAR_HOST_URL", "")

	// Host pinned in config removes the need for SONAR_HOST_URL.
	got := MissingEnv("https://sonar.example.com")
	if len(got) != 1 || got[0] != "SONAR_TOKEN" {
		t.Errorf("expected only SONAR_TOKEN missing, got %v", got)
	}

	// No host anywhere — both are missing.
	got = MissingEnv("")
	if len(got) != 2 {
		t.Errorf("expected both env vars missing, got %v", got)
	}

	t.Setenv("SONAR_TOKEN", "secret")
	t.Setenv("SONAR_HOST_URL", "https://sonar.example.com")
	if got := MissingEnv(""); len(got) != 0 {
		t.Errorf("expected nothing missing, got %v", got)
	}
}

func TestIsInstalled_Smoke(t *testing.T) {
	_ = IsInstalled()
}

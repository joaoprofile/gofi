package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validConfig() *GofiConfig {
	return &GofiConfig{
		Version: CurrentVersion,
		Project: Project{Name: "my-service", Root: "/abs/path/my-service"},
		Backend: &Backend{Language: LanguageGo, Path: "src"},
		AI:      AI{Host: AIHostClaudeVSCode, Model: ModelOpus47},
		Agents:  []string{AgentPD, AgentSpec, AgentEng, AgentQA},
		Sources: Sources{Agents: "github.com/joaoprofile/gofi-agents@v0.1.0"},
		Test: TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit":  {Desc: "unit tests", Run: "go test ./..."},
				"cover": {Desc: "cover", Run: "go test -cover ./..."},
				"sonar": {Desc: "sonar", Run: "sonar-scanner", Needs: []string{"cover"}},
			},
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidate_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*GofiConfig)
	}{
		{"bad version", func(c *GofiConfig) { c.Version = 99 }},
		{"empty name", func(c *GofiConfig) { c.Project.Name = "" }},
		{"name with uppercase", func(c *GofiConfig) { c.Project.Name = "MyService" }},
		{"bad language", func(c *GofiConfig) { c.Backend.Language = "haskell" }},
		{"empty root", func(c *GofiConfig) { c.Project.Root = "" }},
		{"empty path", func(c *GofiConfig) { c.Backend.Path = "" }},
		{"path with slash", func(c *GofiConfig) { c.Backend.Path = "src/inner" }},
		{"no backend no surface", func(c *GofiConfig) { c.Backend = nil; c.Frontend = nil; c.Mobile = nil }},
		{"bad host", func(c *GofiConfig) { c.AI.Host = "cursor" }},
		{"bad model", func(c *GofiConfig) { c.AI.Model = "gpt-5" }},
		{"empty agents", func(c *GofiConfig) { c.Agents = nil }},
		{"unknown agent", func(c *GofiConfig) { c.Agents = []string{"gofi-foo"} }},
		{"duplicated agent", func(c *GofiConfig) { c.Agents = []string{AgentPD, AgentPD} }},
		{"bad source format", func(c *GofiConfig) { c.Sources.Agents = "https://github.com/x/y" }},
		{"test default missing", func(c *GofiConfig) { c.Test.Default = "ghost" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validConfig()
			tc.mutate(c)
			if err := c.Validate(); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestValidate_SonarEnabledRequiresKey(t *testing.T) {
	c := validConfig()
	c.Sonar = SonarConfig{Enabled: true} // no project key
	if err := c.Validate(); err == nil {
		t.Fatal("expected error when sonar is enabled without a project key")
	}

	c.Sonar.ProjectKey = "my-service"
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid with project key set: %v", err)
	}

	// Disabled sonar is always valid regardless of missing fields.
	c.Sonar = SonarConfig{Enabled: false}
	if err := c.Validate(); err != nil {
		t.Fatalf("expected disabled sonar to validate: %v", err)
	}
}

func TestDefaultSonar_ScopesAndExcludes(t *testing.T) {
	s := DefaultSonar("my-service", &Backend{Language: LanguageGo, Path: "src"}, nil, nil)
	if !s.Enabled {
		t.Error("expected sonar enabled by default")
	}
	if s.ProjectKey != "my-service" {
		t.Errorf("expected project key from name, got %q", s.ProjectKey)
	}
	if len(s.Sources) != 1 || s.Sources[0] != "src" {
		t.Errorf("expected sources scoped to backend path, got %v", s.Sources)
	}
	if s.CoverageReport != "coverage.out" {
		t.Errorf("expected go coverage report, got %q", s.CoverageReport)
	}
	wantExcl := map[string]bool{"**/*_test.go": false, "**/mocks/**": false, "**/.gofi/**": false}
	for _, e := range s.Exclusions {
		if _, ok := wantExcl[e]; ok {
			wantExcl[e] = true
		}
	}
	for e, found := range wantExcl {
		if !found {
			t.Errorf("expected default exclusions to drop %q", e)
		}
	}
}

func TestValidate_TestTaskCycle(t *testing.T) {
	c := validConfig()
	c.Test.Tasks = map[string]TestTask{
		"a": {Run: "echo a", Needs: []string{"b"}},
		"b": {Run: "echo b", Needs: []string{"a"}},
	}
	c.Test.Default = "a"
	if err := c.Validate(); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidate_TrainingDuplicateTopic(t *testing.T) {
	c := validConfig()
	c.Training.PD = []TrainingItem{
		{Topic: "domain", Source: "a.md", InstalledAt: "2026-04-25", Hash: "x"},
		{Topic: "domain", Source: "b.md", InstalledAt: "2026-04-25", Hash: "y"},
	}
	if err := c.Validate(); err == nil {
		t.Fatal("expected duplicate topic error")
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gofi.yaml")
	original := validConfig()
	if err := Save(path, original); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Project.Name != original.Project.Name {
		t.Errorf("name mismatch: %s vs %s", loaded.Project.Name, original.Project.Name)
	}
	if len(loaded.Agents) != len(original.Agents) {
		t.Errorf("agents length mismatch")
	}
	if loaded.Test.Default != original.Test.Default {
		t.Errorf("test.default mismatch")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	if _, err := Load("/does/not/exist/.gofi.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLoad_LegacyProjectPath verifies the in-memory migration of pre-v2.5
// configs: when only `path` was written and held an absolute workspace, Load
// must move it into `root` and seed `path` with the default source folder.
func TestLoad_LegacyProjectPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gofi.yaml")
	legacy := []byte(`version: 1
project:
  name: my-service
  language: go
  path: /abs/legacy/my-service
ai:
  host: claude-vscode
  model: claude-opus-4-7
agents: [gofi-pd, gofi-spec, gofi-eng, gofi-qa]
sources:
  agents: github.com/joaoprofile/gofi-agents@v0.1.0
git:
  remote: ""
test:
  default: unit
  hooks:
    pre: []
    post: []
  tasks:
    unit:
      desc: unit tests
      run: go test ./...
hsec:
  enabled: false
  severity_threshold: HIGH
  return_error_on_finding: true
  output_format: text
`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Project.Root != "/abs/legacy/my-service" {
		t.Errorf("expected Root migrated from legacy path, got %q", loaded.Project.Root)
	}
	if loaded.Backend == nil || loaded.Backend.Language != "go" {
		t.Fatalf("expected backend.language migrated to go, got %+v", loaded.Backend)
	}
	if loaded.Backend.Path != "src" {
		t.Errorf("expected backend.path defaulted to %q, got %q", "src", loaded.Backend.Path)
	}
	if loaded.Version != CurrentVersion {
		t.Errorf("expected version bumped to %d, got %d", CurrentVersion, loaded.Version)
	}
}

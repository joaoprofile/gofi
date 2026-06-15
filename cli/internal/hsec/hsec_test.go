package hsec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func TestSeveritiesBelow(t *testing.T) {
	cases := map[string][]string{
		"CRITICAL": {"HIGH", "MEDIUM", "LOW", "INFO"},
		"HIGH":     {"MEDIUM", "LOW", "INFO"},
		"MEDIUM":   {"LOW", "INFO"},
		"LOW":      {"INFO"},
		"INFO":     {},
	}
	for th, want := range cases {
		got := severitiesBelow(th)
		if len(got) != len(want) {
			t.Errorf("%s: got %v want %v", th, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: got %v want %v", th, got, want)
			}
		}
	}
}

func TestBuildHorusecConfig_Defaults(t *testing.T) {
	c := config.DefaultHsec()
	body, err := BuildHorusecConfig(c, "/tmp/out.json")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var got horusecJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.PrintOutputType != "json" {
		t.Errorf("expected json, got %s", got.PrintOutputType)
	}
	if !got.ReturnErrorIfFoundVulnerability {
		t.Errorf("expected return on finding")
	}
	if got.JsonOutputFilepath != "/tmp/out.json" {
		t.Errorf("unexpected output filepath: %s", got.JsonOutputFilepath)
	}
	if !contains(got.SeveritiesToIgnore, "MEDIUM") {
		t.Errorf("HIGH threshold should ignore MEDIUM, got %v", got.SeveritiesToIgnore)
	}
	if contains(got.SeveritiesToIgnore, "HIGH") {
		t.Errorf("HIGH threshold should NOT ignore HIGH, got %v", got.SeveritiesToIgnore)
	}
}

func TestBuildHorusecConfig_InvalidSeverity(t *testing.T) {
	c := config.DefaultHsec()
	c.SeverityThreshold = "BANANA"
	if _, err := BuildHorusecConfig(c, ""); err == nil {
		t.Fatal("expected error for unknown severity")
	}
}

func TestWriteConfig_RoundTrip(t *testing.T) {
	root := t.TempDir()
	c := config.DefaultHsec()
	path, err := WriteConfig(root, c)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if path != filepath.Join(root, ConfigFileName) {
		t.Errorf("unexpected path: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file: %v", err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "horusecCliReturnErrorIfFoundVulnerability") {
		t.Errorf("expected key in output:\n%s", b)
	}
}

func TestParseFindings_NoFile(t *testing.T) {
	root := t.TempDir()
	got, err := ParseFindings(root)
	if err != nil {
		t.Fatalf("expected nil error when file is missing, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil findings, got %v", got)
	}
}

func TestParseFindings_FromFixture(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gofi"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
		"analysisVulnerabilities": [
			{"vulnerabilities": {"vulnerabilityID":"id-1","severity":"HIGH","file":"src/foo.go","line":"10","details":"hardcoded secret"}},
			{"vulnerabilities": {"vulnerabilityID":"id-2","severity":"CRITICAL","file":"src/bar.go","line":"42","details":"sql injection"}}
		]
	}`)
	if err := os.WriteFile(filepath.Join(root, OutputFileName), body, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ParseFindings(root)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(got))
	}
	if got[0].Severity != "HIGH" || got[1].Severity != "CRITICAL" {
		t.Errorf("unexpected ordering: %+v", got)
	}
}

func TestIsInstalled_Smoke(t *testing.T) {
	// We can't assert true/false reliably (depends on the test machine), but
	// the function must not panic.
	_ = IsInstalled()
}

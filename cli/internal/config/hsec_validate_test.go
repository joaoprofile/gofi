package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func validHsec() HsecConfig {
	return HsecConfig{
		Enabled:           true,
		SeverityThreshold: "HIGH",
		OutputFormat:      "json",
		TimeoutSeconds:    600,
		UseDocker:         true,
		Manager: &HsecManagerConfig{
			URL:            "https://horusec.example.com",
			RepositoryName: "MY-REPO",
			TimeoutSeconds: 300,
		},
	}
}

func TestValidate_HsecAcceptsFullBlock(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected full hsec block to validate: %v", err)
	}
}

func TestValidate_HsecRejectsUnknownSeverityAtEditTime(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.SeverityThreshold = "BANANA"
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "hsec.severity_threshold") {
		t.Fatalf("expected severity_threshold error, got %v", err)
	}
}

func TestValidate_HsecSeverityIsCaseInsensitiveLikeRender(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.SeverityThreshold = "high"
	if err := c.Validate(); err != nil {
		t.Fatalf("lowercase severity is accepted at render time, must validate too: %v", err)
	}
}

func TestValidate_HsecRejectsUnknownOutputFormat(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.OutputFormat = "xml"
	if err := c.Validate(); err == nil {
		t.Fatal("expected output_format error")
	}
}

func TestValidate_HsecManagerRequiresAbsoluteURLAndRepositoryName(t *testing.T) {
	cases := map[string]func(*HsecManagerConfig){
		"relative url":     func(m *HsecManagerConfig) { m.URL = "horusec.example.com" },
		"non-http scheme":  func(m *HsecManagerConfig) { m.URL = "ftp://horusec.example.com" },
		"empty url":        func(m *HsecManagerConfig) { m.URL = "" },
		"empty repo name":  func(m *HsecManagerConfig) { m.RepositoryName = "  " },
		"negative timeout": func(m *HsecManagerConfig) { m.TimeoutSeconds = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := validConfig()
			c.Hsec = validHsec()
			mutate(c.Hsec.Manager)
			if err := c.Validate(); err == nil {
				t.Fatalf("expected error for %s", name)
			}
		})
	}
}

func TestValidate_HsecDisabledSkipsChecks(t *testing.T) {
	c := validConfig()
	c.Hsec = HsecConfig{Enabled: false, SeverityThreshold: "BANANA", Manager: &HsecManagerConfig{}}
	if err := c.Validate(); err != nil {
		t.Fatalf("disabled hsec must always validate: %v", err)
	}
}

func TestHsecConfig_ParsesManagerBlockFromYAML(t *testing.T) {
	src := `
enabled: true
use_docker: true
enable_git_history: true
enable_commit_author: true
enable_owasp_dependency_check: true
severity_threshold: HIGH
manager:
    url: https://horusec.example.com
    repository_name: MY-REPO
    timeout_seconds: 300
`
	var h HsecConfig
	if err := yaml.Unmarshal([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !h.UseDocker || !h.EnableGitHistory || !h.EnableCommitAuthor || !h.EnableOwaspDependencyCheck {
		t.Errorf("analysis flags not parsed: %+v", h)
	}
	if h.Manager == nil || h.Manager.URL != "https://horusec.example.com" || h.Manager.RepositoryName != "MY-REPO" || h.Manager.TimeoutSeconds != 300 {
		t.Errorf("manager block not parsed: %+v", h.Manager)
	}
}

func TestHsecConfig_OldBlockWithoutNewFieldsStaysLocalOnly(t *testing.T) {
	src := `
enabled: true
severity_threshold: HIGH
return_error_on_finding: true
`
	var h HsecConfig
	if err := yaml.Unmarshal([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.UseDocker || h.Manager != nil {
		t.Errorf("a pre-existing block must keep the old behaviour (no docker, no manager): %+v", h)
	}
	out, err := yaml.Marshal(h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, k := range []string{"use_docker", "manager", "enable_git_history"} {
		if strings.Contains(string(out), k) {
			t.Errorf("saving an old block must not add %q:\n%s", k, out)
		}
	}
}

func TestValidate_HsecDockerRuntime(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.DockerRuntime = HsecDockerRuntimeIsolated
	if err := c.Validate(); err != nil {
		t.Fatalf("isolated with use_docker must validate: %v", err)
	}
	c.Hsec.UseDocker = false
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "use_docker") {
		t.Fatalf("isolated without use_docker must fail, got %v", err)
	}
	c.Hsec.UseDocker = true
	c.Hsec.DockerRuntime = "podman"
	if err := c.Validate(); err == nil {
		t.Fatal("unknown runtime must fail")
	}
}

func TestValidate_HsecSuppressionNeedsRuleAndReason(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.FalsePositives = []HsecSuppression{{Rule: "HS-LEAKS-26", Path: "**/locales/*.json", Reason: "i18n labels"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("complete rule must validate: %v", err)
	}
	c.Hsec.FalsePositives[0].Reason = " "
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("a suppression without a reason must fail, got %v", err)
	}
	c.Hsec.FalsePositives[0] = HsecSuppression{Reason: "x"}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "rule") {
		t.Fatalf("a suppression without a rule must fail, got %v", err)
	}
}

// RN-14: a dependency advisory is fixed by updating, never suppressed.
func TestValidate_HsecRejectsDependencyAdvisoryAsFalsePositive(t *testing.T) {
	for _, rule := range []string{"CVE-2026-14257", "cve-2023-45133", "GHSA-5p4m-2wfm-xmqj", "GO-2026-5932", "1112052"} {
		c := validConfig()
		c.Hsec = validHsec()
		c.Hsec.FalsePositives = []HsecSuppression{{Rule: rule, Reason: "x"}}
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "dependency advisory") {
			t.Errorf("%s must be rejected, got %v", rule, err)
		}
	}
	for _, rule := range []string{"G101", "G404", "HS-LEAKS-26", "HS-JAVASCRIPT-16"} {
		c := validConfig()
		c.Hsec = validHsec()
		c.Hsec.FalsePositives = []HsecSuppression{{Rule: rule, Reason: "x"}}
		if err := c.Validate(); err != nil {
			t.Errorf("%s is a code rule and must be accepted: %v", rule, err)
		}
	}
}

// Regression: publishing shipped finding snippets to the Manager, and .env was
// scanned by default — a secret could leave the machine.
func TestDefaultHsec_IgnoresEnvFiles(t *testing.T) {
	if !slices.Contains(DefaultHsec().IgnorePaths, "**/.env*") {
		t.Fatal("default hsec must ignore **/.env*")
	}
}

func TestValidate_HsecRiskAcceptsFollowSameRules(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.RiskAccepts = []HsecSuppression{{Rule: "G402", Path: "backend/**/unblocker.go", Reason: "provider intercepts TLS"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("complete risk accept must validate: %v", err)
	}
	c.Hsec.RiskAccepts[0].Reason = ""
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "risk_accepts[0].reason") {
		t.Fatalf("risk accept without reason must fail, got %v", err)
	}
	c.Hsec.RiskAccepts[0] = HsecSuppression{Rule: "GO-2026-5932", Path: "backend/go.sum", Reason: "vulnerable package of the module is not imported"}
	if err := c.Validate(); err != nil {
		t.Fatalf("a dependency advisory that does not apply may be risk-accepted: %v", err)
	}
	c.Hsec.RiskAccepts[0].Reason = " "
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("an accepted advisory still needs a reason, got %v", err)
	}
}

// Regression: git history analysis with .git excluded made GitLeaks fail with
// "not a valid git repository" while the scan still reported success.
func TestValidate_HsecGitHistoryConflictsWithIgnoredGitDir(t *testing.T) {
	c := validConfig()
	c.Hsec = validHsec()
	c.Hsec.EnableGitHistory = true
	c.Hsec.IgnorePaths = []string{"**/node_modules/**", "**/.git/**"}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "enable_git_history") {
		t.Fatalf("expected conflict error, got %v", err)
	}
	c.Hsec.EnableGitHistory = false
	if err := c.Validate(); err != nil {
		t.Fatalf("excluding .git without history analysis is valid: %v", err)
	}
	c.Hsec.EnableGitHistory = true
	c.Hsec.IgnorePaths = []string{"**/.github/**", "**/node_modules/**"}
	if err := c.Validate(); err != nil {
		t.Fatalf(".github is not .git: %v", err)
	}
}

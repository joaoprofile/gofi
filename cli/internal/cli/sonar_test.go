package cli

import (
	"strings"
	"testing"
)

func TestWarnSonarEnv_MissingHostAndToken(t *testing.T) {
	t.Setenv("SONAR_HOST_URL", "")
	t.Setenv("SONAR_TOKEN", "")

	var b strings.Builder
	warnSonarEnv(&b, "")
	out := b.String()

	for _, want := range []string{"SONAR_HOST_URL", "SONAR_TOKEN", "sonarcloud.io", "sonar.organization"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected warning to mention %q:\n%s", want, out)
		}
	}
}

func TestWarnSonarEnv_HostPinnedInConfig(t *testing.T) {
	t.Setenv("SONAR_HOST_URL", "")
	t.Setenv("SONAR_TOKEN", "secret")

	// Host pinned in the sonar: block removes the need for SONAR_HOST_URL, and
	// with the token set there is nothing left to warn about.
	var b strings.Builder
	warnSonarEnv(&b, "http://localhost:9000")
	if out := b.String(); out != "" {
		t.Errorf("expected no warning when host is pinned and token set, got:\n%s", out)
	}
}

func TestWarnSonarEnv_FullyConfigured(t *testing.T) {
	t.Setenv("SONAR_HOST_URL", "http://localhost:9000")
	t.Setenv("SONAR_TOKEN", "secret")

	var b strings.Builder
	warnSonarEnv(&b, "")
	if out := b.String(); out != "" {
		t.Errorf("expected no warning when fully configured, got:\n%s", out)
	}
}

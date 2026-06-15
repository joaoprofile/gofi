package wizard

import (
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func TestNewDefaultResult(t *testing.T) {
	r := newDefaultResult()
	if r.AIModel != config.ModelOpus48 {
		t.Errorf("default model = %q, want %q", r.AIModel, config.ModelOpus48)
	}
	if !r.Has(EnvBack) {
		t.Errorf("default should include backend")
	}
	if len(r.Agents) != 8 {
		t.Errorf("default should activate all 8 agents, got %d", len(r.Agents))
	}
	if r.AgentsRef != config.DefaultAgentsRef {
		t.Errorf("default agents ref = %q", r.AgentsRef)
	}
}

func TestCollectSources(t *testing.T) {
	sdk := collectSources("  github.com/o/r@main  ")
	if sdk[config.LanguageGo] != "github.com/o/r@main" {
		t.Errorf("sdk go = %q", sdk[config.LanguageGo])
	}
	if len(collectSources("")) != 0 {
		t.Errorf("empty override should yield no sdk entries")
	}
}

func TestValidateEnvironments(t *testing.T) {
	if err := validateEnvironments(nil); err == nil {
		t.Error("empty selection should fail")
	}
	if err := validateEnvironments([]string{EnvWeb}); err != nil {
		t.Errorf("one surface should pass: %v", err)
	}
}

func TestValidateSurfacePath(t *testing.T) {
	for _, ok := range []string{"", "web", "services", "back-end"} {
		if err := validateSurfacePath(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"apps/web", "Web", "1web", "a/b"} {
		if err := validateSurfacePath(bad); err == nil {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestSeedFromConfig_MultiSurface(t *testing.T) {
	r := newDefaultResult()
	cfg := &config.GofiConfig{
		Project:  config.Project{Name: "x", Root: "/r"},
		Backend:  &config.Backend{Language: config.LanguageGo, Path: "services"},
		Frontend: &config.UISurface{Framework: config.FrameworkReact, Path: "apps-web", DS: config.DSWeb},
		Mobile:   &config.UISurface{Framework: config.FrameworkReactNative, Path: "apps-mobile", DS: ""},
		Agents:   []string{config.AgentEng},
		Sources: config.Sources{Agents: config.DefaultAgentsRef},
	}
	seedFromConfig(r, cfg)
	if !r.Has(EnvBack) || !r.Has(EnvWeb) || !r.Has(EnvMobile) {
		t.Fatalf("expected all three surfaces, got %v", r.Environments)
	}
	if r.SourcePath != "services" || r.WebPath != "apps-web" || r.MobilePath != "apps-mobile" {
		t.Errorf("paths not seeded: %+v", r.Environments)
	}
	if r.WebDS != config.DSWeb || r.MobileDS != "" {
		t.Errorf("DS not seeded: web=%q mobile=%q", r.WebDS, r.MobileDS)
	}
}

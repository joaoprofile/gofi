package wizard

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/detect"
)

func TestNewDefaultResult(t *testing.T) {
	r := newDefaultResult()
	if r.AIModel != config.DefaultModel {
		t.Errorf("default model = %q, want %q", r.AIModel, config.DefaultModel)
	}
	if !r.Has(EnvBack) {
		t.Errorf("default should include backend")
	}
	if len(r.Agents) != len(config.AllAgents()) {
		t.Errorf("default should activate every agent (%d), got %d", len(config.AllAgents()), len(r.Agents))
	}
	if r.AgentsRef != config.DefaultAgentsRef {
		t.Errorf("default agents ref = %q", r.AgentsRef)
	}
}

// The wizard's pick is what lands in .gofi.yaml as ai.models, which is the list
// the extension's /model picker reads. A model the wizard omits is a model the
// user can never reach without editing YAML by hand.
func TestModelOptionsOfferEveryModel(t *testing.T) {
	opts := modelOptions()
	if len(opts) != len(config.Models()) {
		t.Fatalf("picker offers %d models, table has %d", len(opts), len(config.Models()))
	}
	var marked int
	for i, m := range config.Models() {
		if opts[i].Value != m.ID {
			t.Errorf("option %d = %q, want %q", i, opts[i].Value, m.ID)
		}
		if !strings.HasPrefix(opts[i].Key, m.Label) {
			t.Errorf("option %d label = %q, want it to start with %q", i, opts[i].Key, m.Label)
		}
		if strings.Contains(opts[i].Key, "default") {
			marked++
			if m.ID != config.DefaultModel {
				t.Errorf("%q is marked default, but the default is %q", m.ID, config.DefaultModel)
			}
		}
	}
	if marked != 1 {
		t.Errorf("%d options marked default, want exactly 1", marked)
	}
}

func TestSeedFromDetect(t *testing.T) {
	r := newDefaultResult()
	seedFromDetect(r, detect.Result{
		Backend: detect.Surface{Path: "services/api", Marker: "go.mod", Language: config.LanguageGo},
		Web:     detect.Surface{Path: "apps/web", Marker: "package.json", Framework: config.FrameworkVue},
	})

	if r.SourcePath != "services/api" || r.Language != config.LanguageGo {
		t.Errorf("backend = %q/%q", r.Language, r.SourcePath)
	}
	if r.WebPath != "apps/web" {
		t.Errorf("web path = %q", r.WebPath)
	}
	if !r.Has(EnvBack) || !r.Has(EnvWeb) {
		t.Errorf("environments = %v, want backend and web", r.Environments)
	}
	if r.Has(EnvMobile) {
		t.Errorf("mobile was not detected but is selected: %v", r.Environments)
	}
}

// The scanned identifier replaces the placeholder module, but only when the
// marker actually carried one — otherwise the question is still a real one and
// the example is the better prompt.
func TestSeedFromDetect_Identifiers(t *testing.T) {
	placeholder := newDefaultResult().Module

	withModule := newDefaultResult()
	seedFromDetect(withModule, detect.Result{
		Name:    "billing-api",
		Backend: detect.Surface{Path: ".", Marker: "go.mod", Language: config.LanguageGo, Module: "github.com/acme/billing"},
	})
	if withModule.Module != "github.com/acme/billing" {
		t.Errorf("Module = %q, want the one read from go.mod", withModule.Module)
	}
	if withModule.Name != "billing-api" {
		t.Errorf("Name = %q, want billing-api", withModule.Name)
	}

	withoutModule := newDefaultResult()
	seedFromDetect(withoutModule, detect.Result{
		Backend: detect.Surface{Path: ".", Marker: "build.gradle", Language: config.LanguageJava},
	})
	if withoutModule.Module != placeholder {
		t.Errorf("Module = %q, want the placeholder %q", withoutModule.Module, placeholder)
	}
}

func TestSeedFromDetect_EmptyScanKeepsDefaults(t *testing.T) {
	r := newDefaultResult()
	seedFromDetect(r, detect.Result{})
	if r.SourcePath != config.DefaultBackendPath || !r.Has(EnvBack) {
		t.Errorf("a brand-new project must keep its defaults, got %q / %v", r.SourcePath, r.Environments)
	}
}

func TestAdopted(t *testing.T) {
	r := newDefaultResult()
	r.Detected = detect.Result{Backend: detect.Surface{Path: "src", Marker: "go.mod", Language: config.LanguageGo}}

	r.SourcePath = "src"
	if !r.Adopted(EnvBack) {
		t.Error("the detected path should count as adopted")
	}
	// Overriding the detected path points at a folder gofi still has to create.
	r.SourcePath = "backend"
	if r.Adopted(EnvBack) {
		t.Error("a hand-typed path is not an adopted tree")
	}
	if r.Adopted(EnvWeb) || r.Adopted(EnvMobile) {
		t.Error("undetected surfaces are never adopted")
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
	for _, ok := range []string{"", "web", "services", "back-end", ".", "apps/web", "Web"} {
		if err := validateSurfacePath(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"../up", "/abs", `a\b`} {
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
		Sources:  config.Sources{Agents: config.DefaultAgentsRef},
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

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func baseConfig() *GofiConfig {
	return &GofiConfig{
		Version: CurrentVersion,
		Project: Project{Name: "my-svc", Root: "/tmp/x"},
		Backend: &Backend{Language: LanguageGo, Path: "src"},
		AI:      AI{Host: AIHostClaudeVSCode, Model: ModelOpus48},
		Agents:  []string{AgentPD, AgentUI, AgentOps, AgentDoc, AgentStatus},
		Sources: Sources{Agents: DefaultAgentsRef},
		Test:    DefaultTestSection(LanguageGo, "src"),
		Hsec:    DefaultHsec(),
	}
}

func TestValidate_NewModelAndAgents(t *testing.T) {
	c := baseConfig()
	if err := c.Validate(); err != nil {
		t.Fatalf("expected valid config: %v", err)
	}
}

func TestSaveLoad_MultiSurfaceRoundTrip(t *testing.T) {
	c := baseConfig()
	c.Frontend = &UISurface{Framework: FrameworkReact, Path: "web", Brand: BrandBlue, Styling: StylingTailwind, State: StateTanstackQuery, Testing: TestingVitest, DS: DSWeb}
	c.Mobile = &UISurface{Framework: FrameworkReactNative, Path: "mobile", Brand: BrandBlue, Styling: StylingStylesheet, State: StateTanstackQuery, Testing: TestingJest, DS: DSMobile}
	c.Ops = DefaultOps()
	c.Sources.UI = map[string]string{DSWeb: DefaultAgentsRef}

	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, c); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, _ := os.ReadFile(path)
	for _, want := range []string{"frontend:", "mobile:", "ops:", "ds: gofi-ui", "ds: gofi-ui-native"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("expected %q in yaml:\n%s", want, data)
		}
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Frontend == nil || got.Frontend.DS != DSWeb {
		t.Errorf("frontend surface not round-tripped: %+v", got.Frontend)
	}
	if got.Mobile == nil || got.Mobile.Path != "mobile" {
		t.Errorf("mobile surface not round-tripped: %+v", got.Mobile)
	}
	if got.Ops == nil || got.Ops.Path != "ops" {
		t.Errorf("ops not round-tripped: %+v", got.Ops)
	}
}

func TestSave_BackOnlyOmitsSurfacesAndOps(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, baseConfig()); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, _ := os.ReadFile(path)
	for _, unwanted := range []string{"frontend:", "mobile:", "ops:"} {
		if strings.Contains(string(data), unwanted) {
			t.Errorf("back-only config should not emit %q:\n%s", unwanted, data)
		}
	}
}

func TestValidate_FrontOnly(t *testing.T) {
	c := baseConfig()
	c.Backend = nil
	c.Frontend = &UISurface{Framework: FrameworkReact, Path: "web", DS: DSWeb}
	if err := c.Validate(); err != nil {
		t.Fatalf("front-only config should be valid: %v", err)
	}
}

func TestValidate_RejectsBadEnumsAndSources(t *testing.T) {
	cases := map[string]func(*GofiConfig){
		"bad brand": func(c *GofiConfig) {
			c.Frontend = &UISurface{Framework: FrameworkReact, Path: "web", Brand: "neon"}
		},
		"bad ds": func(c *GofiConfig) {
			c.Frontend = &UISurface{Framework: FrameworkReact, Path: "web", DS: "nope"}
		},
		"bad ops cloud": func(c *GofiConfig) { c.Ops = &Ops{Cloud: "moon", Path: "ops"} },
		"bad sources.ui key": func(c *GofiConfig) {
			c.Sources.UI = map[string]string{"bogus": DefaultAgentsRef}
		},
		"no backend no surface": func(c *GofiConfig) { c.Backend = nil; c.Frontend = nil; c.Mobile = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := baseConfig()
			mutate(c)
			if err := c.Validate(); err == nil {
				t.Errorf("expected validation error for %q", name)
			}
		})
	}
}

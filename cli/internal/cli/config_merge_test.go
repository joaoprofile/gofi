package cli

import (
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

// projectOwnedFrontend is a surface that brought its own stack — the shape
// `gofi config --wizard` must not flatten back to the gofi presets.
func projectOwnedFrontend() *config.UISurface {
	return &config.UISurface{
		Framework: config.FrameworkReact, Path: "frontend",
		Brand: "acme", Styling: "scss-modules", State: "axios-hooks",
		Forms: "react-final-form", I18n: "react-i18next",
		Testing: "jest", DS: "acme-ui", Legacy: "antd",
	}
}

func baseWizardResult() *wizard.Result {
	return &wizard.Result{
		Name: "acme", Root: "/abs/acme",
		Environments: []string{wizard.EnvBack, wizard.EnvWeb},
		Language:     config.LanguageGo, SourcePath: "backend",
		WebPath: "frontend", WebDS: "acme-ui",
		AIHost: config.AIHostClaudeVSCode, AIModel: config.ModelOpus48,
		Agents: config.AllAgents(), AgentsRef: config.DefaultAgentsRef,
	}
}

func TestMergeWizardIntoConfig_KeepsProjectOwnedSurfaceStack(t *testing.T) {
	cfg := &config.GofiConfig{
		Version:  config.CurrentVersion,
		Project:  config.Project{Name: "acme", Root: "/abs/acme"},
		Backend:  &config.Backend{Language: config.LanguageGo, Path: "backend"},
		Frontend: projectOwnedFrontend(),
	}

	got := mergeWizardIntoConfig(cfg, baseWizardResult())

	if got.Frontend == nil {
		t.Fatal("frontend dropped by merge")
	}
	if *got.Frontend != *projectOwnedFrontend() {
		t.Errorf("frontend = %+v, want %+v", got.Frontend, projectOwnedFrontend())
	}
}

func TestMergeWizardIntoConfig_SeedsNewSurfaceFromPresets(t *testing.T) {
	cfg := &config.GofiConfig{
		Version: config.CurrentVersion,
		Project: config.Project{Name: "acme", Root: "/abs/acme"},
		Backend: &config.Backend{Language: config.LanguageGo, Path: "backend"},
	}
	r := baseWizardResult()
	r.WebDS = config.DSWeb

	got := mergeWizardIntoConfig(cfg, r)

	if got.Frontend == nil {
		t.Fatal("newly selected web surface was not created")
	}
	want := config.UISurface{
		Framework: config.FrameworkReact, Path: "frontend",
		Brand: config.BrandBlue, Styling: config.StylingTailwind,
		State: config.StateTanstackQuery, Testing: config.TestingVitest,
		DS: config.DSWeb,
	}
	if *got.Frontend != want {
		t.Errorf("frontend = %+v, want %+v", got.Frontend, want)
	}
}

func TestMergeWizardIntoConfig_DropsDeselectedSurface(t *testing.T) {
	cfg := &config.GofiConfig{
		Version:  config.CurrentVersion,
		Project:  config.Project{Name: "acme", Root: "/abs/acme"},
		Backend:  &config.Backend{Language: config.LanguageGo, Path: "backend"},
		Frontend: projectOwnedFrontend(),
		Mobile:   &config.UISurface{Framework: config.FrameworkReactNative, Path: "mobile"},
	}
	r := baseWizardResult() // web + back only, no mobile

	got := mergeWizardIntoConfig(cfg, r)

	if got.Mobile != nil {
		t.Errorf("deselected mobile surface should be dropped, got %+v", got.Mobile)
	}
	if got.Frontend == nil {
		t.Error("web surface was still selected and should survive")
	}
}

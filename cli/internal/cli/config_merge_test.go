package cli

import (
	"path/filepath"
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

// The wizard never asks which web framework a project uses, so every repository
// would be recorded as React and its graph stamped with the wrong framework. The
// package.json beside the code is the answer.
func TestMergeWizardIntoConfig_SeedsFrameworkFromThePackageJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps string
		want string
	}{
		{"angular", `{"@angular/core":"^19.0.0"}`, config.FrameworkAngular},
		{"vue", `{"vue":"^3.5.0"}`, config.FrameworkVue},
		{"svelte", `{"svelte":"^5.0.0"}`, config.FrameworkSvelte},
		// An Astro site pulls in the framework whose components it renders, so
		// the more specific dependency has to win.
		{"astro over vue", `{"astro":"^5.0.0","vue":"^3.5.0"}`, config.FrameworkAstro},
		{"react", `{"react":"^19.0.0"}`, config.FrameworkReact},
		{"nothing recognised", `{}`, config.FrameworkReact},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "frontend", "package.json"),
				`{"name":"acme-admin","dependencies":`+tc.deps+`}`)

			cfg := &config.GofiConfig{
				Version: config.CurrentVersion,
				Project: config.Project{Name: "acme", Root: root},
			}
			r := baseWizardResult()
			r.Root = root

			got := mergeWizardIntoConfig(cfg, r)

			if got.Frontend == nil {
				t.Fatal("web surface was not created")
			}
			if got.Frontend.Framework != tc.want {
				t.Errorf("framework = %q, want %q", got.Frontend.Framework, tc.want)
			}
		})
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

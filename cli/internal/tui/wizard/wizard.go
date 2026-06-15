// Package wizard implements the interactive `gofi init` flow with huh forms.
package wizard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/tui/styles"
)

// Environment slugs — the surfaces a project can include.
const (
	EnvBack   = "back"
	EnvWeb    = "web"
	EnvMobile = "mobile"
)

// Result holds the user's choices from the wizard, ready to drive scaffold and
// gitops execution.
//
//   - Root is the workspace folder (absolute after Run); the wizard labels it
//     "Root path" and defaults to the current working directory when blank.
//   - Environments is the set of selected surfaces (back/web/mobile). Each
//     surface contributes its own path and options below.
type Result struct {
	AIHost    string
	AIModel   string
	Name      string
	Root      string // workspace folder, absolute after Run
	AgentsRef string // skills/agents source URL (gofi monorepo) pinned in .gofi.yaml

	// Environments selected — any combination of back/web/mobile.
	Environments []string

	// Backend (when EnvBack selected).
	Language   string // go|rust|nodejs|java|csharp
	SourcePath string // backend source folder inside Root (default "backend")
	GoModule   string // only when Language == go

	// Web (when EnvWeb selected).
	WebPath string // web app folder inside Root (default "frontend")
	WebDS   string // config.DSWeb or "" (no design system)

	// Mobile (when EnvMobile selected). MobileDS is gofi-ui-native (always set
	// when mobile is selected; the lib is an npm package, not a git source).
	MobilePath string
	MobileDS   string

	Agents    []string
	GitRemote string

	// SDKURLs carries an optional override URL per backend language (Go's SDK is
	// cloned into .gofi/gofi-sdk-go/). Web/mobile design systems are npm
	// packages, not git sources. Empty values are dropped.
	SDKURLs map[string]string

	// CreateSpecsDir / CreatePrdDir control seeding <root>/specs and <root>/prd.
	// The ops/ folder is always created (no prompt).
	CreateSpecsDir bool
	CreatePrdDir   bool

	// ClaudeSource records "fetch:<sha>" set by the init pipeline.
	ClaudeSource string

	// Skipped lists surfaces the pipeline could not create (missing toolchain),
	// surfaced in the next-steps. Filled by the init pipeline.
	Skipped []string
}

// Has reports whether env is among the selected environments.
func (r *Result) Has(env string) bool { return slices.Contains(r.Environments, env) }

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]+$`)

// ErrCancelled is returned when the user picks "Cancel" on the final confirm
// step. It is a clean abort, not a true error.
var ErrCancelled = errors.New("init cancelled")

// Run displays the interactive wizard and returns the user's choices, or an
// error if the user cancels (Ctrl+C) or input fails validation.
//
// When initial != nil, its values pre-populate the form (edit mode used by
// `gofi config --wizard`); when nil, fresh defaults are used.
func Run(initial *config.GofiConfig) (*Result, error) {
	r := newDefaultResult()
	if initial != nil {
		seedFromConfig(r, initial)
	}

	configureRemote := r.GitRemote != ""
	proceed := true

	// Go SDK source override, read back in post-processing. Web/mobile design
	// systems are npm packages (gofi-ui / gofi-ui-native), not git sources.
	sdkGo := r.SDKURLs[config.LanguageGo]

	has := func(env string) bool { return slices.Contains(r.Environments, env) }
	backGo := func() bool { return has(EnvBack) && r.Language == config.LanguageGo }

	form := huh.NewForm(
		// 1 — Project identity
		huh.NewGroup(
			huh.NewNote().
				Title("Project").
				Description("Identity and location of the monorepo."),
			huh.NewInput().
				Title("Project name").
				Description("Lowercase letters, digits, hyphens. Must start with a letter.").
				Validate(validateSlug).
				Value(&r.Name),
			huh.NewInput().
				Title("Root path").
				Description("Workspace folder — where .gofi.yaml, .claude/ and the surfaces live. Blank = current folder. ~ is expanded.").
				Value(&r.Root),
		),
		// 2 — Skills repository
		huh.NewGroup(
			huh.NewNote().
				Title("Skills repository").
				Description("The gofi monorepo the CLI fetches skills, SDK docs and templates from (under ai/)."),
			huh.NewInput().
				Title("Repository").
				Description("github.com/<org>/<repo>@<ref>").
				Value(&r.AgentsRef),
		),
		// 3 — Environments (multi-select)
		huh.NewGroup(
			huh.NewNote().
				Title("Environments").
				Description("Which surfaces to create in this monorepo. Pick any combination."),
			huh.NewMultiSelect[string]().
				Title("Surfaces").
				Description("Space to toggle. At least one is required.").
				Options(
					huh.NewOption("Backend", EnvBack).Selected(has(EnvBack)),
					huh.NewOption("Web (front-end)", EnvWeb).Selected(has(EnvWeb)),
					huh.NewOption("Mobile", EnvMobile).Selected(has(EnvMobile)),
				).
				Validate(validateEnvironments).
				Value(&r.Environments),
		),
		// 4 — Backend config
		huh.NewGroup(
			huh.NewNote().Title("Backend").Description("Language and source folder."),
			huh.NewSelect[string]().
				Title("Language").
				Description("Only Go has a working scaffold today; others are saved but not bootstrapped yet.").
				Options(
					huh.NewOption("Go", config.LanguageGo),
					huh.NewOption("Rust   (preview)", config.LanguageRust),
					huh.NewOption("Node.js (preview)", config.LanguageNodeJS),
					huh.NewOption("Java   (preview)", config.LanguageJava),
					huh.NewOption("C#     (preview)", config.LanguageCSharp),
				).
				Value(&r.Language),
			huh.NewInput().
				Title("Backend path").
				Description("Source folder inside the root, e.g. backend, services, src. Blank = backend.").
				Validate(validateSurfacePath).
				Value(&r.SourcePath),
		).WithHideFunc(func() bool { return !has(EnvBack) }),
		// 5 — Web config (always Vite + React + TS + gofi-ui)
		huh.NewGroup(
			huh.NewNote().Title("Web").Description("Vite + React + TypeScript, with gofi-ui installed."),
			huh.NewInput().
				Title("Web path").
				Description("App folder inside the root. Blank = frontend.").
				Validate(validateSurfacePath).
				Value(&r.WebPath),
		).WithHideFunc(func() bool { return !has(EnvWeb) }),
		// 6 — Mobile config (always Expo + gofi-ui-native)
		huh.NewGroup(
			huh.NewNote().Title("Mobile").Description("React Native (Expo) + TypeScript, with gofi-ui-native installed."),
			huh.NewInput().
				Title("Mobile path").
				Description("App folder inside the root. Blank = mobile.").
				Validate(validateSurfacePath).
				Value(&r.MobilePath),
		).WithHideFunc(func() bool { return !has(EnvMobile) }),
		// 7 — Go SDK source (only Go backend)
		huh.NewGroup(
			huh.NewNote().Title("Source · Go SDK").Description("Repo for the Go SDK (gofi-sdk-go), wired into go.work."),
			huh.NewInput().Title("gofi-sdk-go").Description("github.com/<org>/<repo>@<ref>").Value(&sdkGo),
		).WithHideFunc(func() bool { return !backGo() }),
		// 8 — AI host + model
		huh.NewGroup(
			huh.NewNote().Title("AI host").Description("Where the agents run. Claude Code on VSCode in v1."),
			huh.NewSelect[string]().
				Title("AI host").
				Options(huh.NewOption("Claude Code (VSCode)", config.AIHostClaudeVSCode)).
				Value(&r.AIHost),
			huh.NewSelect[string]().
				Title("Claude model").
				Description("Recorded in .gofi.yaml; change later in .claude/settings.json.").
				Options(
					huh.NewOption("Opus 4.8   — most capable (default)", config.ModelOpus48),
					huh.NewOption("Opus 4.7", config.ModelOpus47),
					huh.NewOption("Sonnet 4.6 — fast & sharp", config.ModelSonnet46),
					huh.NewOption("Haiku 4.5  — fastest", config.ModelHaiku45),
				).
				Value(&r.AIModel),
		),
		// 9 — Agents
		huh.NewGroup(
			huh.NewNote().Title("Agents").Description("Which gofi agents to activate as skills."),
			huh.NewMultiSelect[string]().
				Title("Agents to activate").
				Description("Space to toggle. At least one is required.").
				Options(buildAgentOptions(r.Agents)...).
				Validate(validateAgents).
				Value(&r.Agents),
		),
		// 10 — Doc folders
		huh.NewGroup(
			huh.NewNote().Title("Folders").Description("ops/ is always created. Choose specs/ and prd/."),
			huh.NewConfirm().Title("Create specs/ folder?").Description("Where /gofi-spec writes specs.").Affirmative("Yes").Negative("No").Value(&r.CreateSpecsDir),
			huh.NewConfirm().Title("Create prd/ folder?").Description("Where /gofi-pd writes PRDs.").Affirmative("Yes").Negative("No").Value(&r.CreatePrdDir),
		),
		// 11 — Git remote
		huh.NewGroup(
			huh.NewConfirm().Title("Configure git remote now?").Description("You can also set it later with `gofi remote add <url>`.").Affirmative("Yes").Negative("Skip").Value(&configureRemote),
		),
		huh.NewGroup(
			huh.NewInput().Title("Git remote URL").Description("https://, git@ or github.com/org/repo.").Value(&r.GitRemote),
		).WithHideFunc(func() bool { return !configureRemote }),
		// 12 — Go module (last, only Go backend)
		huh.NewGroup(
			huh.NewNote().Title("Go module").Description("Goes into go.mod for the backend."),
			huh.NewInput().Title("Module path").Description("e.g. github.com/org/repo. Edit later if needed.").Validate(validateGoModule).Value(&r.GoModule),
		).WithHideFunc(func() bool { return !backGo() }),
		// 13 — Review
		huh.NewGroup(
			huh.NewNote().Title("Review").Description("Confirm to apply. Cancel keeps everything untouched."),
			huh.NewConfirm().Title("Apply this configuration?").Description("Creates the selected surfaces, .gofi.yaml, .claude/, specs/ prd/ ops/.").Affirmative("Apply").Negative("Cancel").Value(&proceed),
		),
	).WithTheme(styles.FormTheme()).WithAccessible(!styles.Enabled())

	if err := form.Run(); err != nil {
		return nil, err
	}
	if !proceed {
		return nil, ErrCancelled
	}

	// post-processing
	r.Name = strings.TrimSpace(r.Name)
	r.Root = strings.TrimSpace(r.Root)
	r.SourcePath = strings.TrimSpace(r.SourcePath)
	r.WebPath = strings.TrimSpace(r.WebPath)
	r.MobilePath = strings.TrimSpace(r.MobilePath)
	r.GoModule = strings.TrimSpace(r.GoModule)
	r.GitRemote = strings.TrimSpace(r.GitRemote)
	r.AgentsRef = strings.TrimSpace(r.AgentsRef)

	r.SDKURLs = collectSources(sdkGo)

	// Default per-surface paths when blank, and pin the design system for each
	// selected surface (web → gofi-ui, mobile → gofi-ui-native, always).
	if r.Has(EnvBack) && r.SourcePath == "" {
		r.SourcePath = config.DefaultBackendPath
	}
	if r.Has(EnvWeb) {
		if r.WebPath == "" {
			r.WebPath = config.DefaultFrontendPath
		}
		r.WebDS = config.DSWeb
	} else {
		r.WebDS = ""
	}
	if r.Has(EnvMobile) {
		if r.MobilePath == "" {
			r.MobilePath = config.DefaultMobilePath
		}
		r.MobileDS = config.DSMobile
	} else {
		r.MobileDS = ""
	}

	if r.Root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve current directory: %w", err)
		}
		r.Root = cwd
	}
	expanded, err := expandPath(r.Root)
	if err != nil {
		return nil, fmt.Errorf("expand root path: %w", err)
	}
	r.Root = expanded

	if !configureRemote {
		r.GitRemote = ""
	}
	return r, nil
}

// collectSources turns the Go SDK override input into the SDK map, dropping an
// empty value so the resulting .gofi.yaml carries only an explicit override.
// Extracted for testability.
func collectSources(sdkGo string) map[string]string {
	sdk := map[string]string{}
	if v := strings.TrimSpace(sdkGo); v != "" {
		sdk[config.LanguageGo] = v
	}
	return sdk
}

func newDefaultResult() *Result {
	return &Result{
		AIHost:         config.AIHostClaudeVSCode,
		AIModel:        config.ModelOpus48,
		Environments:   []string{EnvBack},
		Language:       config.LanguageGo,
		SourcePath:     config.DefaultBackendPath,
		GoModule:       "github.com/your-org/your-repo",
		WebPath:        config.DefaultFrontendPath,
		WebDS:          config.DSWeb,
		MobilePath:     config.DefaultMobilePath,
		MobileDS:       config.DSMobile,
		Agents:         config.AllAgents(),
		AgentsRef:      config.DefaultAgentsRef,
		SDKURLs:        map[string]string{config.LanguageGo: config.DefaultSDKGoRef},
		CreateSpecsDir: true,
		CreatePrdDir:   true,
	}
}

// seedFromConfig copies non-empty fields from cfg into r so the wizard pre-
// populates inputs in edit mode.
func seedFromConfig(r *Result, cfg *config.GofiConfig) {
	if cfg.AI.Host != "" {
		r.AIHost = cfg.AI.Host
	}
	if cfg.AI.Model != "" {
		r.AIModel = cfg.AI.Model
	}
	if cfg.Project.Name != "" {
		r.Name = cfg.Project.Name
	}
	if cfg.Project.Root != "" {
		r.Root = cfg.Project.Root
	}

	var envs []string
	if cfg.Backend != nil && cfg.Backend.Language != "" {
		envs = append(envs, EnvBack)
		r.Language = cfg.Backend.Language
		if cfg.Backend.Path != "" {
			r.SourcePath = cfg.Backend.Path
		}
	}
	if cfg.Frontend != nil {
		envs = append(envs, EnvWeb)
		r.WebPath = cfg.Frontend.Path
		r.WebDS = cfg.Frontend.DS
	}
	if cfg.Mobile != nil {
		envs = append(envs, EnvMobile)
		r.MobilePath = cfg.Mobile.Path
		r.MobileDS = cfg.Mobile.DS
	}
	if len(envs) > 0 {
		r.Environments = envs
	}

	if len(cfg.Agents) > 0 {
		r.Agents = append([]string(nil), cfg.Agents...)
	}
	if cfg.Sources.Agents != "" {
		r.AgentsRef = cfg.Sources.Agents
	}
	for lang, url := range cfg.Sources.SDK {
		r.SDKURLs[lang] = url
	}
	if cfg.Git.Remote != "" {
		r.GitRemote = cfg.Git.Remote
	}
}

// buildAgentOptions returns the eight agent options, marking each selected when
// present in the current selection (used to seed the wizard from a config).
func buildAgentOptions(selected []string) []huh.Option[string] {
	type entry struct{ slug, label string }
	all := []entry{
		{config.AgentPD, "gofi-pd     — Product Discovery"},
		{config.AgentSpec, "gofi-spec   — Specification Architect"},
		{config.AgentEng, "gofi-eng    — Context Engineer"},
		{config.AgentUI, "gofi-ui     — UI/UX Engineer"},
		{config.AgentOps, "gofi-ops    — Platform & Delivery"},
		{config.AgentQA, "gofi-qa     — Quality Auditor"},
		{config.AgentDoc, "gofi-doc    — Documentation Generator"},
		{config.AgentStatus, "gofi-status — Context Index"},
	}
	sel := map[string]bool{}
	for _, s := range selected {
		sel[s] = true
	}
	out := make([]huh.Option[string], 0, len(all))
	for _, e := range all {
		opt := huh.NewOption(e.label, e.slug)
		if sel[e.slug] {
			opt = opt.Selected(true)
		}
		out = append(out, opt)
	}
	return out
}

func validateSlug(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("required")
	}
	if !slugRe.MatchString(s) {
		return errors.New("must match ^[a-z][a-z0-9-]+$")
	}
	return nil
}

// validateSurfacePath accepts an empty value (the wizard fills the default
// later) or a single-folder slug.
func validateSurfacePath(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.ContainsAny(s, "/\\") {
		return errors.New("must be a single folder name (no slashes)")
	}
	if !slugRe.MatchString(s) {
		return errors.New("must match ^[a-z][a-z0-9-]+$")
	}
	return nil
}

func validateGoModule(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("required")
	}
	if !strings.Contains(s, "/") {
		return errors.New("must look like a module path (e.g. github.com/org/repo)")
	}
	return nil
}

func validateEnvironments(envs []string) error {
	if len(envs) == 0 {
		return errors.New("select at least one surface")
	}
	return nil
}

func validateAgents(agents []string) error {
	if len(agents) == 0 {
		return errors.New("select at least one agent")
	}
	return nil
}

func expandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		p = filepath.Join(home, p[2:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}

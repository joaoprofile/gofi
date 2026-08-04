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
	"github.com/joaoprofile/gofi-cli/internal/detect"
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

	// InstitutionalRef is the optional org business-knowledge repo. Blank means
	// institutional/ is managed by hand in this project's own git (no upstream).
	InstitutionalRef string

	// Environments selected — any combination of back/web/mobile.
	Environments []string

	// Backend (when EnvBack selected).
	Language   string // go|rust|nodejs|java|csharp
	SourcePath string // backend source folder inside Root (default "backend")
	Module     string // module identifier spelled per language; empty for Rust

	// Web (when EnvWeb selected).
	WebPath string // web app folder inside Root (default "frontend")
	WebDS   string // config.DSWeb or "" (no design system)

	// Mobile (when EnvMobile selected). MobileDS is gofi-ui-native (set when
	// mobile is selected and the surface declares no other design system; the
	// lib is an npm package, not a git source).
	MobilePath string
	MobileDS   string

	// seededWeb / seededMobile record that the surface came from an existing
	// .gofi.yaml (`gofi config --wizard`) rather than being created here. A
	// seeded surface keeps its declared design system — including an explicit
	// "none" — because the wizard never asks about it.
	seededWeb    bool
	seededMobile bool

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

	// Detected is what the scan of the root found before the wizard ran. It
	// seeds the surface paths, and the init pipeline reads it back to tell an
	// adopted tree from one it is about to create.
	Detected detect.Result
}

// Adopted reports whether the surface at the given path is code that was
// already there. The path is compared because the user may have overridden the
// detected one in the wizard, and a hand-typed path points at a folder gofi
// still has to scaffold.
func (r *Result) Adopted(env string) bool {
	switch env {
	case EnvBack:
		return r.Detected.Backend.Found() && r.Detected.Backend.Path == r.SourcePath
	case EnvWeb:
		return r.Detected.Web.Found() && r.Detected.Web.Path == r.WebPath
	case EnvMobile:
		return r.Detected.Mobile.Found() && r.Detected.Mobile.Path == r.MobilePath
	}
	return false
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
// `gofi config --wizard`); when nil, fresh defaults are used, refined by found
// — what a scan of the target folder recognised, so a repository that already
// exists is described back to the user instead of being asked about.
func Run(initial *config.GofiConfig, found detect.Result) (*Result, error) {
	r := newDefaultResult()
	if initial != nil {
		seedFromConfig(r, initial)
	} else {
		seedFromDetect(r, found)
	}
	r.Detected = found

	configureRemote := r.GitRemote != ""
	proceed := true

	// Go SDK source override, read back in post-processing. Web/mobile design
	// systems are npm packages (gofi-ui / gofi-ui-native), not git sources.
	sdkGo := r.SDKURLs[config.LanguageGo]

	has := func(env string) bool { return slices.Contains(r.Environments, env) }
	backGo := func() bool { return has(EnvBack) && r.Language == config.LanguageGo }
	needsModule := func() bool { return has(EnvBack) && r.Language != config.LanguageRust }

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
		// 2b — Institutional repository (optional)
		huh.NewGroup(
			huh.NewNote().
				Title("Institutional base (optional)").
				Description("Org repo with business/product knowledge, maintained by the company independent of any product. Multi-product layout: a <project-name>/ folder per product. Blank = manage institutional/ by hand in this project's git."),
			huh.NewInput().
				Title("Institutional repository").
				Description("github.com/<org>/<repo>@<ref> — or blank").
				Value(&r.InstitutionalRef),
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
			huh.NewNote().Title("Backend").
				Description(surfaceNote("Language and source folder.", found.Backend, "existing backend code")),
			huh.NewSelect[string]().
				Title("Language").
				Description("Every language gets a project skeleton. Only Go ships a gofi SDK; the others carry conventions only.").
				Options(
					huh.NewOption("Go", config.LanguageGo),
					huh.NewOption("Rust", config.LanguageRust),
					huh.NewOption("Node.js", config.LanguageNodeJS),
					huh.NewOption("Java", config.LanguageJava),
					huh.NewOption("C#", config.LanguageCSharp),
				).
				Value(&r.Language),
			huh.NewInput().
				Title("Backend path").
				Description("Source folder inside the root, e.g. backend, services/api, src. Use . when the code is at the root. Blank = backend.").
				Validate(validateSurfacePath).
				Value(&r.SourcePath),
		).WithHideFunc(func() bool { return !has(EnvBack) }),
		// 5 — Web config (always Vite + React + TS + gofi-ui)
		huh.NewGroup(
			huh.NewNote().Title("Web").
				Description(surfaceNote("Vite + React + TypeScript, with gofi-ui installed.", found.Web, "an existing web app")),
			huh.NewInput().
				Title("Web path").
				Description("App folder inside the root, e.g. frontend, apps/web. Blank = frontend.").
				Validate(validateSurfacePath).
				Value(&r.WebPath),
		).WithHideFunc(func() bool { return !has(EnvWeb) }),
		// 6 — Mobile config (always Expo + gofi-ui-native)
		huh.NewGroup(
			huh.NewNote().Title("Mobile").
				Description(surfaceNote("React Native (Expo) + TypeScript, with gofi-ui-native installed.", found.Mobile, "an existing mobile app")),
			huh.NewInput().
				Title("Mobile path").
				Description("App folder inside the root, e.g. mobile, apps/mobile. Blank = mobile.").
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
				Options(modelOptions()...).
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
		// 12 — Backend module identifier (last). Skipped for Rust, whose crate is
		// named after the project, so there is nothing extra to ask.
		huh.NewGroup(
			huh.NewNote().Title("Backend module").Description(moduleNote(found.Backend)),
			huh.NewInput().
				TitleFunc(func() string { t, _ := moduleQuestion(r.Language); return t }, &r.Language).
				DescriptionFunc(func() string { _, d := moduleQuestion(r.Language); return d }, &r.Language).
				Validate(func(s string) error { return validateModule(r.Language, s) }).
				Value(&r.Module),
		).WithHideFunc(func() bool { return !needsModule() }),
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
	r.Module = strings.TrimSpace(r.Module)
	r.GitRemote = strings.TrimSpace(r.GitRemote)
	r.AgentsRef = strings.TrimSpace(r.AgentsRef)
	r.InstitutionalRef = strings.TrimSpace(r.InstitutionalRef)

	r.SDKURLs = collectSources(sdkGo)

	// Default per-surface paths when blank, and pin the gofi design system on a
	// selected surface that has none yet (web → gofi-ui, mobile →
	// gofi-ui-native). A surface seeded from an existing config keeps the design
	// system it already declares — the wizard never asks about it, so it must
	// not replace a project's own package with the gofi one.
	if r.Has(EnvBack) && r.SourcePath == "" {
		r.SourcePath = config.DefaultBackendPath
	}
	if r.Has(EnvWeb) {
		if r.WebPath == "" {
			r.WebPath = config.DefaultFrontendPath
		}
		if !r.seededWeb {
			r.WebDS = config.DSWeb
		}
	} else {
		r.WebDS = ""
	}
	if r.Has(EnvMobile) {
		if r.MobilePath == "" {
			r.MobilePath = config.DefaultMobilePath
		}
		if !r.seededMobile {
			r.MobileDS = config.DSMobile
		}
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
		AIModel:        config.DefaultModel,
		Environments:   []string{EnvBack},
		Language:       config.LanguageGo,
		SourcePath:     config.DefaultBackendPath,
		Module:         "github.com/your-org/your-repo",
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

// seedFromDetect replaces the defaults with what was actually found on disk, so
// `gofi init` on an existing repository pre-fills the surfaces instead of
// proposing a layout the project does not have. A scan that recognised nothing
// leaves the defaults alone — that is the brand-new-project case.
func seedFromDetect(r *Result, found detect.Result) {
	if !found.Any() {
		return
	}
	if found.Name != "" {
		r.Name = found.Name
	}
	var envs []string
	if s := found.Backend; s.Found() {
		envs = append(envs, EnvBack)
		r.Language = s.Language
		r.SourcePath = s.Path
		// An empty module leaves the placeholder in place: the marker carried no
		// identifier gofi can use, so the question is still a real one.
		if s.Module != "" {
			r.Module = s.Module
		}
	}
	if s := found.Web; s.Found() {
		envs = append(envs, EnvWeb)
		r.WebPath = s.Path
	}
	if s := found.Mobile; s.Found() {
		envs = append(envs, EnvMobile)
		r.MobilePath = s.Path
	}
	r.Environments = envs
}

// moduleNote tells the user whether the identifier below was typed by gofi or
// is still an example, which is the difference between confirming a value and
// supplying one.
func moduleNote(s detect.Surface) string {
	if s.Found() && s.Module != "" {
		return fmt.Sprintf("Read from %s at %s — change it only if it is wrong.", s.Marker, displayPath(s.Path))
	}
	return "The identifier the backend manifest is built around."
}

// surfaceNote describes a surface group's header: what was found and the file
// that proved it, so the user can judge the guess before accepting it.
func surfaceNote(base string, s detect.Surface, what string) string {
	if !s.Found() {
		return base
	}
	return fmt.Sprintf("Found %s at %s (%s) — gofi will adopt it, not overwrite it.", what, displayPath(s.Path), s.Marker)
}

// displayPath renders a surface path for a prompt, naming the root explicitly
// because a bare "." reads like a typo.
func displayPath(p string) string {
	if p == "." {
		return "the workspace root"
	}
	return "./" + p
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
		r.seededWeb = true
	}
	if cfg.Mobile != nil {
		envs = append(envs, EnvMobile)
		r.MobilePath = cfg.Mobile.Path
		r.MobileDS = cfg.Mobile.DS
		r.seededMobile = true
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

// modelOptions renders the picker from config.Models(), so the wizard offers
// exactly what the extension's /model does. Labels are padded to a common width
// so the notes line up regardless of how many models the table carries.
func modelOptions() []huh.Option[string] {
	models := config.Models()
	width := 0
	for _, m := range models {
		if len(m.Label) > width {
			width = len(m.Label)
		}
	}
	out := make([]huh.Option[string], 0, len(models))
	for _, m := range models {
		label := m.Label
		note := m.Note
		if m.ID == config.DefaultModel {
			if note == "" {
				note = "default"
			} else {
				note += " (default)"
			}
		}
		if note != "" {
			label = fmt.Sprintf("%-*s — %s", width, label, note)
		}
		out = append(out, huh.NewOption(label, m.ID))
	}
	return out
}

// buildAgentOptions returns the nine agent options, marking each selected when
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
		{config.AgentFull, "gofi-full   — Full-Cycle Orchestrator"},
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
// later) or any path config would accept — including "." for a repository
// whose code already sits at the root.
func validateSurfacePath(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !config.ValidSurfacePath(s) {
		return errors.New("must be a relative folder path (e.g. src, services/api) or . for the root")
	}
	return nil
}

// moduleQuestion spells the one backend identifier the way its ecosystem does.
// Rust is absent on purpose: a crate is named after the project, so there is
// nothing extra to ask.
func moduleQuestion(language string) (title, desc string) {
	switch language {
	case config.LanguageJava:
		return "Base package", "e.g. com.acme.myservice — becomes the Maven groupId and the src/main/java tree."
	case config.LanguageCSharp:
		return "Root namespace", "e.g. Acme.MyService — becomes the namespace in Program.cs."
	case config.LanguageNodeJS:
		return "Package name", "e.g. @acme/my-service — goes into package.json."
	default:
		return "Module path", "e.g. github.com/org/repo — goes into go.mod."
	}
}

func validateModule(language, s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("required")
	}
	switch language {
	case config.LanguageJava, config.LanguageCSharp:
		if !strings.Contains(s, ".") {
			return errors.New("must be dotted (e.g. com.acme.myservice)")
		}
	case config.LanguageNodeJS:
		// npm accepts a bare name as readily as a scoped one.
	default:
		if !strings.Contains(s, "/") {
			return errors.New("must look like a module path (e.g. github.com/org/repo)")
		}
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

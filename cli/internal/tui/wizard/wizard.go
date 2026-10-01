// Package wizard implements the interactive `gofi init` and
// `gofi config --wizard` dialogue.
package wizard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/detect"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/tui/flow"
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

	// GitRemote is the origin init adds; git keeps it, not .gofi.yaml.
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

// ErrCancelled is returned when the user quits or picks "Cancel" on the
// review. It is a clean abort, not a true error.
var ErrCancelled = errors.New("init cancelled")

// Meta describes the invocation the wizard runs for: the version shown in the
// header and, for `gofi init <name>`, the name and folder the user already
// typed on the command line.
type Meta struct {
	Version string
	Name    string
	Root    string
}

// Run displays the interactive wizard and returns the user's choices, or
// ErrCancelled when the user quits or declines the review.
//
// When initial != nil, its values pre-populate the questions (edit mode used by
// `gofi config --wizard`); when nil, fresh defaults are used, refined by found
// — what a scan of the target folder recognised, so a repository that already
// exists is described back to the user instead of being asked about.
func Run(initial *config.GofiConfig, found detect.Result, meta Meta) (*Result, error) {
	r := newDefaultResult()
	r.AgentsRef = config.AgentsRefFor(meta.Version)
	if initial != nil {
		seedFromConfig(r, initial)
	} else {
		seedFromDetect(r, found)
	}
	r.Detected = found
	if meta.Name != "" {
		r.Name = meta.Name
	}
	if meta.Root != "" {
		r.Root = meta.Root
	}

	configureRemote := r.GitRemote != ""
	proceed := true

	// Go SDK source override, read back in post-processing. Web/mobile design
	// systems are npm packages (gofi-ui / gofi-ui-native), not git sources.
	sdkGo := r.SDKURLs[config.LanguageGo]

	cwd, _ := os.Getwd()
	header := flow.Header{
		Title:    "init",
		Version:  meta.Version,
		Subtitle: i18n.T("wizard.subtitle.init"),
		Dir:      cwd,
	}
	if initial != nil {
		header.Title = "config"
		header.Subtitle = i18n.T("wizard.subtitle.config")
	}

	steps := buildSteps(r, found, &sdkGo, &configureRemote, &proceed, initial != nil)
	if err := flow.Run(header, steps); err != nil {
		if errors.Is(err, flow.ErrCancelled) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	if !proceed {
		return nil, ErrCancelled
	}
	if err := r.finalize(sdkGo, configureRemote); err != nil {
		return nil, err
	}
	return r, nil
}

// buildSteps lays out the questions in the order they are asked. Surface
// questions hide themselves when the surface is not selected, and the review
// comes last so it can show everything that was answered.
func buildSteps(r *Result, found detect.Result, sdkGo *string, configureRemote, proceed *bool, editing bool) []flow.Step {
	has := func(env string) bool { return slices.Contains(r.Environments, env) }
	backGo := func() bool { return has(EnvBack) && r.Language == config.LanguageGo }
	needsModule := func() bool { return has(EnvBack) && r.Language != config.LanguageRust }

	return []flow.Step{
		{
			Kind:     flow.Input,
			Title:    i18n.T("wizard.name.title"),
			Help:     i18n.T("wizard.name.help"),
			Text:     &r.Name,
			Validate: validateSlug,
		},
		{
			Kind:        flow.Input,
			Title:       i18n.T("wizard.root.title"),
			Help:        i18n.T("wizard.root.help"),
			Placeholder: currentDir(),
			Text:        &r.Root,
		},
		{
			Kind:  flow.MultiSelect,
			Title: i18n.T("wizard.surfaces.title"),
			Help:  i18n.T("wizard.surfaces.help"),
			Options: []flow.Option{
				{Label: "Backend", Value: EnvBack, Hint: surfaceHint(found.Backend)},
				{Label: "Web", Value: EnvWeb, Hint: surfaceHint(found.Web)},
				{Label: "Mobile", Value: EnvMobile, Hint: surfaceHint(found.Mobile)},
			},
			Choices:       &r.Environments,
			ValidateMulti: validateEnvironments,
		},
		{
			Kind:  flow.Select,
			Title: i18n.T("wizard.lang.title"),
			HelpFunc: func() string {
				return surfaceNote(i18n.T("wizard.lang.help"), found.Backend, "wizard.found.back")
			},
			Options: []flow.Option{
				{Label: "Go", Value: config.LanguageGo, Hint: i18n.T("wizard.lang.sdk")},
				{Label: "Rust", Value: config.LanguageRust},
				{Label: "Node.js", Value: config.LanguageNodeJS},
				{Label: "Java", Value: config.LanguageJava},
				{Label: "C#", Value: config.LanguageCSharp},
			},
			Choice: &r.Language,
			Skip:   func() bool { return !has(EnvBack) },
		},
		{
			Kind:        flow.Input,
			Title:       i18n.T("wizard.backpath.title"),
			Help:        i18n.T("wizard.backpath.help"),
			Placeholder: config.DefaultBackendPath,
			Text:        &r.SourcePath,
			Validate:    validateSurfacePath,
			Skip:        func() bool { return !has(EnvBack) },
		},
		{
			Kind:      flow.Input,
			TitleFunc: func() string { t, _ := moduleQuestion(r.Language); return t },
			HelpFunc: func() string {
				_, d := moduleQuestion(r.Language)
				return moduleNote(found.Backend) + " " + d
			},
			Text:     &r.Module,
			Validate: func(s string) error { return validateModule(r.Language, s) },
			Skip:     func() bool { return !needsModule() },
		},
		{
			Kind:        flow.Input,
			Title:       i18n.T("wizard.webpath.title"),
			Help:        surfaceNote(i18n.T("wizard.webpath.help"), found.Web, "wizard.found.web"),
			Placeholder: config.DefaultFrontendPath,
			Text:        &r.WebPath,
			Validate:    validateSurfacePath,
			Skip:        func() bool { return !has(EnvWeb) },
		},
		{
			Kind:        flow.Input,
			Title:       i18n.T("wizard.mobilepath.title"),
			Help:        surfaceNote(i18n.T("wizard.mobilepath.help"), found.Mobile, "wizard.found.mobile"),
			Placeholder: config.DefaultMobilePath,
			Text:        &r.MobilePath,
			Validate:    validateSurfacePath,
			Skip:        func() bool { return !has(EnvMobile) },
		},
		{
			Kind:    flow.Select,
			Title:   i18n.T("wizard.host.title"),
			Help:    i18n.T("wizard.host.help"),
			Options: hostOptions(),
			Choice:  &r.AIHost,
		},
		{
			Kind:    flow.Select,
			Title:   i18n.T("wizard.model.title"),
			Help:    i18n.T("wizard.model.help"),
			Options: modelOptions(),
			Choice:  &r.AIModel,
			// The model list is Claude's; other hosts pick theirs in their own
			// terms, in ai.tiers or the host's settings.
			Skip: func() bool { return !host.IsClaude(r.AIHost) },
		},
		{
			Kind:  flow.Confirm,
			Title: i18n.T("wizard.specs.title"),
			Help:  i18n.T("wizard.specs.help"),
			Bool:  &r.CreateSpecsDir,
		},
		{
			Kind:  flow.Confirm,
			Title: i18n.T("wizard.prd.title"),
			Help:  i18n.T("wizard.prd.help"),
			Bool:  &r.CreatePrdDir,
		},
		{
			Kind:     flow.Confirm,
			Title:    i18n.T("wizard.remote.title"),
			Help:     i18n.T("wizard.remote.help"),
			Negative: i18n.T("wizard.remote.later"),
			Bool:     configureRemote,
			// Editing, there is nothing to add: the repository has its remote.
			Skip: func() bool { return editing },
		},
		{
			Kind:     flow.Input,
			Title:    i18n.T("wizard.remoteurl.title"),
			Help:     i18n.T("wizard.remoteurl.help"),
			Text:     &r.GitRemote,
			Validate: flow.Required,
			Skip:     func() bool { return editing || !*configureRemote },
		},
		{
			Kind:  flow.Input,
			Title: i18n.T("wizard.skills.title"),
			Help:  i18n.T("wizard.skills.help"),
			Text:  &r.AgentsRef,
		},
		{
			Kind:  flow.Input,
			Title: i18n.T("wizard.sdk.title"),
			Help:  i18n.T("wizard.sdk.help"),
			Text:  sdkGo,
			Skip:  func() bool { return !backGo() },
		},
		{
			Kind:        flow.Input,
			Title:       i18n.T("wizard.inst.title"),
			Help:        i18n.T("wizard.inst.help"),
			Placeholder: i18n.T("wizard.inst.none"),
			Text:        &r.InstitutionalRef,
		},
		{
			Kind: flow.Confirm,
			TitleFunc: func() string {
				switch {
				case editing:
					return i18n.T("wizard.review.config")
				case r.Detected.Any():
					return i18n.T("wizard.review.adopt")
				}
				return i18n.T("wizard.review.create")
			},
			HelpFunc: func() string {
				c := *r
				_ = c.finalize(*sdkGo, *configureRemote)
				return c.Summary()
			},
			Affirmative: i18n.T("wizard.review.apply"),
			Negative:    i18n.T("wizard.review.cancel"),
			Bool:        proceed,
			Echo: func() string {
				if !*proceed {
					return i18n.T("wizard.review.cancelled")
				}
				c := *r
				_ = c.finalize(*sdkGo, *configureRemote)
				return c.Summary()
			},
		},
	}
}

// finalize trims the answers and fills what a blank answer stands for, so the
// Result describes exactly what will be written. The review calls it on a copy
// to show the same values the pipeline will use.
func (r *Result) finalize(sdkGo string, configureRemote bool) error {
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
			return fmt.Errorf("resolve current directory: %w", err)
		}
		r.Root = cwd
	}
	expanded, err := expandPath(r.Root)
	if err != nil {
		return fmt.Errorf("expand root path: %w", err)
	}
	r.Root = expanded

	if !configureRemote {
		r.GitRemote = ""
	}
	return nil
}

// Summary lists what the project will be created with, one "key  value" row
// per line. Call it on a finalized Result.
func (r *Result) Summary() string {
	type row struct{ k, v string }
	dsLabel := func(ds string) string {
		if ds == "" {
			return i18n.T("wizard.sum.no_ds")
		}
		return ds
	}
	rows := []row{{i18n.T("wizard.sum.name"), r.Name}, {i18n.T("wizard.sum.root"), r.Root}}
	if r.Has(EnvBack) {
		b := r.Language + " (" + r.SourcePath + "/)"
		if r.Module != "" && r.Language != config.LanguageRust {
			b += "  " + r.Module
		}
		rows = append(rows, row{i18n.T("wizard.sum.backend"), b})
	}
	if r.Has(EnvWeb) {
		rows = append(rows, row{i18n.T("wizard.sum.web"), "react (" + r.WebPath + "/)  " + dsLabel(r.WebDS)})
	}
	if r.Has(EnvMobile) {
		rows = append(rows, row{i18n.T("wizard.sum.mobile"), "expo (" + r.MobilePath + "/)  " + dsLabel(r.MobileDS)})
	}
	var folders []string
	if r.CreateSpecsDir {
		folders = append(folders, "specs/")
	}
	if r.CreatePrdDir {
		folders = append(folders, "prd/")
	}
	folders = append(folders, "ops/")
	rows = append(rows,
		row{i18n.T("wizard.sum.folders"), strings.Join(folders, " ")},
		row{i18n.T("wizard.sum.model"), r.AIModel},
		row{i18n.T("wizard.sum.skills"), r.AgentsRef},
	)
	if v := r.SDKURLs[config.LanguageGo]; v != "" && r.Has(EnvBack) && r.Language == config.LanguageGo {
		rows = append(rows, row{i18n.T("wizard.sum.gosdk"), v})
	}
	inst := i18n.T("wizard.sum.manual")
	if r.InstitutionalRef != "" {
		inst = r.InstitutionalRef
	}
	rows = append(rows, row{i18n.T("wizard.sum.institutional"), inst})
	if r.GitRemote != "" {
		rows = append(rows, row{i18n.T("wizard.sum.remote"), r.GitRemote})
	}

	width := 0
	for _, x := range rows {
		width = max(width, utf8.RuneCountInString(x.k))
	}
	lines := make([]string, len(rows))
	for i, x := range rows {
		lines[i] = x.k + strings.Repeat(" ", width-utf8.RuneCountInString(x.k)+2) + x.v
	}
	return strings.Join(lines, "\n")
}

func currentDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "current folder"
	}
	return cwd
}

// surfaceHint marks a surface option that a scan of the root already found.
func surfaceHint(s detect.Surface) string {
	if !s.Found() {
		return ""
	}
	return i18n.T("wizard.surface.found", displayPath(s.Path))
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
		AIHost:         host.ClaudeCode.ID,
		AIModel:        config.DefaultModel,
		Environments:   []string{EnvBack},
		Language:       config.LanguageGo,
		SourcePath:     config.DefaultBackendPath,
		Module:         "github.com/your-org/your-repo",
		WebPath:        config.DefaultFrontendPath,
		WebDS:          config.DSWeb,
		MobilePath:     config.DefaultMobilePath,
		MobileDS:       config.DSMobile,
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
		return i18n.T("wizard.module.read", s.Marker, displayPath(s.Path))
	}
	return i18n.T("wizard.module.note")
}

// surfaceNote describes a surface group's header: what was found and the file
// that proved it, so the user can judge the guess before accepting it.
func surfaceNote(base string, s detect.Surface, foundKey string) string {
	if !s.Found() {
		return base
	}
	return i18n.T(foundKey, displayPath(s.Path), s.Marker)
}

// displayPath renders a surface path for a prompt, naming the root explicitly
// because a bare "." reads like a typo.
func displayPath(p string) string {
	if p == "." {
		return i18n.T("wizard.path.root")
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

	if cfg.Sources.Agents != "" {
		r.AgentsRef = cfg.Sources.Agents
	}
	for lang, url := range cfg.Sources.SDK {
		r.SDKURLs[lang] = url
	}
}

// modelOptions renders the picker from config.Models(), so the wizard offers
// exactly what the extension's /model does.
func modelOptions() []flow.Option {
	models := config.Models()
	out := make([]flow.Option, 0, len(models))
	for _, m := range models {
		note := m.Note
		if m.ID == config.DefaultModel {
			if note == "" {
				note = i18n.T("wizard.model.default")
			} else {
				note += " (" + i18n.T("wizard.model.default") + ")"
			}
		}
		out = append(out, flow.Option{Label: m.Label, Value: m.ID, Hint: note})
	}
	return out
}

// hostOptions offers the hosts, each saying what it gets: the folder gofi
// creates for it and how much of gofi it can run.
func hostOptions() []flow.Option {
	out := make([]flow.Option, 0, len(host.All))
	for _, h := range host.All {
		out = append(out, flow.Option{Label: h.Label, Value: h.ID, Hint: i18n.T("wizard.host."+h.ID, h.Home+"/")})
	}
	return out
}

func validateSlug(s string) error {
	if err := flow.Required(s); err != nil {
		return err
	}
	if !slugRe.MatchString(strings.TrimSpace(s)) {
		return errors.New(i18n.T("wizard.err.slug"))
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
		return errors.New(i18n.T("wizard.err.path"))
	}
	return nil
}

// moduleQuestion spells the one backend identifier the way its ecosystem does.
// Rust is absent on purpose: a crate is named after the project, so there is
// nothing extra to ask.
func moduleQuestion(language string) (title, desc string) {
	switch language {
	case config.LanguageJava:
		return i18n.T("wizard.module.java.title"), i18n.T("wizard.module.java.help")
	case config.LanguageCSharp:
		return i18n.T("wizard.module.csharp.title"), i18n.T("wizard.module.csharp.help")
	case config.LanguageNodeJS:
		return i18n.T("wizard.module.nodejs.title"), i18n.T("wizard.module.nodejs.help")
	default:
		return i18n.T("wizard.module.go.title"), i18n.T("wizard.module.go.help")
	}
}

func validateModule(language, s string) error {
	if err := flow.Required(s); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	switch language {
	case config.LanguageJava, config.LanguageCSharp:
		if !strings.Contains(s, ".") {
			return errors.New(i18n.T("wizard.err.dotted"))
		}
	case config.LanguageNodeJS:
		// npm accepts a bare name as readily as a scoped one.
	default:
		if !strings.Contains(s, "/") {
			return errors.New(i18n.T("wizard.err.module"))
		}
	}
	return nil
}

func validateEnvironments(envs []string) error {
	if len(envs) == 0 {
		return errors.New(i18n.T("wizard.err.surfaces"))
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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FileName       = ".gofi.yaml"
	CurrentVersion = 2

	AIHostClaudeVSCode = "claude-vscode"

	LanguageGo     = "go"
	LanguageRust   = "rust"
	LanguageJava   = "java"
	LanguageCSharp = "csharp"
	LanguagePython = "python"
	LanguageNodeJS = "nodejs"

	ModelFable5   = "claude-fable-5"
	ModelOpus5    = "claude-opus-5"
	ModelOpus48   = "claude-opus-4-8"
	ModelOpus47   = "claude-opus-4-7"
	ModelSonnet5  = "claude-sonnet-5"
	ModelSonnet46 = "claude-sonnet-4-6"
	ModelHaiku45  = "claude-haiku-4-5"

	AgentPD     = "gofi-pd"
	AgentSpec   = "gofi-spec"
	AgentEng    = "gofi-eng"
	AgentUI     = "gofi-ui"
	AgentOps    = "gofi-ops"
	AgentQA     = "gofi-qa"
	AgentDoc    = "gofi-doc"
	AgentStatus = "gofi-status"
	AgentFull   = "gofi-full"

	// UI design systems, keyed by surface.
	DSWeb    = "gofi-ui"
	DSMobile = "gofi-ui-native"

	// UI surface frameworks.
	FrameworkReact       = "react"
	FrameworkReactNative = "react-native"

	// UI brand presets (gofi-ui skill).
	BrandBlue   = "blue"
	BrandViolet = "violet"
	BrandGreen  = "green"

	// UI styling / state / testing defaults per surface.
	StylingTailwind    = "tailwind"
	StylingStylesheet  = "stylesheet"
	StateTanstackQuery = "tanstack-query"
	TestingVitest      = "vitest"
	TestingJest        = "jest"

	// Ops enums (gofi-ops skill). First-class values shown; others accepted.
	CloudOCI   = "oci"
	CloudAWS   = "aws"
	CloudGCP   = "gcp"
	CloudAzure = "azure"

	IaCTerraform = "terraform"
	IaCOpenTofu  = "opentofu"
	IaCPulumi    = "pulumi"

	CICDGitHubActions = "github-actions"
	CICDAzureDevOps   = "azure-devops"
	CICDGitLabCI      = "gitlab-ci"
	CICDOCIDevOps     = "oci-devops"

	// Deploy runtime targets (gofi-ops `ops.target`).
	TargetK8s                = "k8s"
	TargetOKE                = "oke"
	TargetEKS                = "eks"
	TargetGKE                = "gke"
	TargetSwarm              = "swarm"
	TargetContainerInstances = "container-instances"
	TargetPaaS               = "paas"

	// Image registries (gofi-ops `ops.registry`).
	RegistryOCIR = "ocir"
	RegistryECR  = "ecr"
	RegistryGAR  = "gar"
	RegistryACR  = "acr"

	DefaultOpsPath = "ops"
)

// AllAgents returns the canonical list of the nine gofi agent slugs, in
// pipeline order. gofi-full comes last: it is the orchestrator that chains the
// others rather than a phase of its own.
func AllAgents() []string {
	return []string{
		AgentPD, AgentSpec, AgentEng, AgentUI,
		AgentOps, AgentQA, AgentDoc, AgentStatus,
		AgentFull,
	}
}

// AllModels returns the canonical Claude model IDs the /model picker offers.
// Single source of truth: `gofi init` seeds it into .gofi.yaml as `ai.models`,
// and the gofi-ai extension reads that list. Adding a new model = one entry
// here (plus a Model* const above). Ordered by family (flagship → light) and,
// within a family, newest → oldest.
func AllModels() []string {
	return []string{
		ModelFable5,
		ModelOpus5, ModelOpus48, ModelOpus47,
		ModelSonnet5, ModelSonnet46,
		ModelHaiku45,
	}
}

type GofiConfig struct {
	Version  int         `yaml:"version"`
	Project  Project     `yaml:"project"`
	Backend  *Backend    `yaml:"backend,omitempty"`
	Frontend *UISurface  `yaml:"frontend,omitempty"`
	Mobile   *UISurface  `yaml:"mobile,omitempty"`
	Ops      *Ops        `yaml:"ops,omitempty"`
	AI       AI          `yaml:"ai"`
	Agents   []string    `yaml:"agents"`
	Sources  Sources     `yaml:"sources"`
	Git      Git         `yaml:"git"`
	Training Training    `yaml:"training,omitempty"`
	Test     TestSection `yaml:"test"`
	Hsec     HsecConfig  `yaml:"hsec"`
	Sonar    SonarConfig `yaml:"sonar"`
}

// Backend carries the backend language and its source folder. nil for a
// front-only project (web and/or mobile, no server-side code).
//
//   - Language is the backend toolchain (go | rust | java | csharp | python | nodejs).
//   - Path is the source folder inside Project.Root that holds the code
//     (e.g. "src", "services", "backend"). Defaults to "src".
type Backend struct {
	Language string `yaml:"language"`
	Path     string `yaml:"path"`
}

// UISurface is one front-end surface — the shape of both the top-level
// `frontend:` (web) and `mobile:` blocks. Mirrors the gofi-ui skill schema.
//
// Only Framework and Path are constrained. Brand, Styling, State, Forms, I18n,
// Testing and DS are free-form: `gofi init` seeds the gofi presets (blue,
// tailwind, tanstack-query, gofi-ui …), but a project that brought its own
// design system or stack writes its own values here and the skill reads them
// verbatim. DS is the design system package the surface uses, or "" when the
// surface opts out of a design system.
//
// Legacy names a stack the surface is migrating away from (e.g. "antd"); the
// gofi-ui skill treats anything built on it as code to replace, not to extend.
type UISurface struct {
	Framework string `yaml:"framework"`
	Path      string `yaml:"path"`
	Brand     string `yaml:"brand,omitempty"`
	Styling   string `yaml:"styling,omitempty"`
	State     string `yaml:"state,omitempty"`
	Forms     string `yaml:"forms,omitempty"`
	I18n      string `yaml:"i18n,omitempty"`
	Testing   string `yaml:"testing,omitempty"`
	DS        string `yaml:"ds,omitempty"`
	Legacy    string `yaml:"legacy,omitempty"`
}

// Ops carries the platform/delivery block the gofi-ops skill reads. `gofi init`
// seeds the first-class stack (see DefaultOps); the user adjusts it afterwards
// via `gofi config` or by editing .gofi.yaml — the inline comments emitted by
// MarshalYAML list the accepted values for each field.
type Ops struct {
	Cloud    string `yaml:"cloud,omitempty"`
	IaC      string `yaml:"iac,omitempty"`
	Target   string `yaml:"target,omitempty"`
	CICD     string `yaml:"cicd,omitempty"`
	Registry string `yaml:"registry,omitempty"`
	Path     string `yaml:"path,omitempty"`
}

// MarshalYAML emits the ops block with an inline comment on each field listing
// the accepted values, so a freshly written .gofi.yaml documents the options
// the user can switch to. Empty fields are omitted (mirrors the omitempty tags).
func (o Ops) MarshalYAML() (interface{}, error) {
	fields := []struct{ key, val, opts string }{
		{"cloud", o.Cloud, "oci | aws | gcp | azure"},
		{"iac", o.IaC, "terraform | opentofu | pulumi"},
		{"target", o.Target, "k8s | oke | eks | gke | swarm | container-instances | paas"},
		{"cicd", o.CICD, "github-actions | azure-devops | gitlab-ci | oci-devops"},
		{"registry", o.Registry, "ocir | ecr | gar | acr"},
		{"path", o.Path, ""},
	}
	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, f := range fields {
		if f.val == "" {
			continue
		}
		key := &yaml.Node{Kind: yaml.ScalarNode, Value: f.key}
		val := &yaml.Node{Kind: yaml.ScalarNode, Value: f.val}
		if f.opts != "" {
			val.LineComment = f.opts
		}
		node.Content = append(node.Content, key, val)
	}
	return node, nil
}

// HsecConfig drives the `gofi hsec` command (Horusec SAST). gofi renders this
// into a horusec-config.json under <project>/.gofi/ at install time and
// invokes the horusec binary against it.
type HsecConfig struct {
	Enabled              bool     `yaml:"enabled"`
	IgnorePaths          []string `yaml:"ignore_paths,omitempty"`
	SeverityThreshold    string   `yaml:"severity_threshold"` // CRITICAL | HIGH | MEDIUM | LOW
	ReturnErrorOnFinding bool     `yaml:"return_error_on_finding"`
	OutputFormat         string   `yaml:"output_format"` // text | json | sarif
	OutputFile           string   `yaml:"output_file,omitempty"`
	TimeoutSeconds       int      `yaml:"timeout_seconds,omitempty"`
}

// SonarConfig drives the `gofi sonar` command (SonarQube / SonarCloud static
// analysis). gofi renders this into a sonar-project.properties under
// <project>/.gofi/ every time `gofi sonar` runs, then invokes the sonar-scanner
// binary against it.
//
// The server URL and authentication token are read from the environment
// (SONAR_HOST_URL / SONAR_TOKEN — typically exported from .env) and are never
// stored in the YAML. HostURL here is only an optional non-secret override for
// the server URL when you prefer to pin it per project.
//
// Sources scopes analysis to the project's own code folders; Exclusions drops
// everything that is not first-party project code (tests, mocks, generated
// code, the vendored SDK under .gofi/, build artefacts), so reports reflect
// only the code the team actually owns.
type SonarConfig struct {
	Enabled        bool     `yaml:"enabled"`
	ProjectKey     string   `yaml:"project_key"`
	ProjectName    string   `yaml:"project_name,omitempty"`
	HostURL        string   `yaml:"host_url,omitempty"`
	Sources        []string `yaml:"sources,omitempty"`
	Exclusions     []string `yaml:"exclusions,omitempty"`
	CoverageReport string   `yaml:"coverage_report,omitempty"`
}

// Project carries the general identity of a gofi project: its name and the
// workspace folder. Language and source layout moved to the `backend:` block;
// front-end surfaces live in the top-level `frontend:` / `mobile:` blocks.
//
//   - Root is the workspace folder (where .gofi.yaml, .claude/ and the
//     language workspace file live). Absolute after `gofi init`.
type Project struct {
	Name string `yaml:"name"`
	Root string `yaml:"root"`
}

// AI captures the AI host and model used by the project.
//
// Models is the list of model IDs the gofi-ai extension's /model picker
// offers (the active picks). Any ID in AllModels() that isn't in Models is
// still written to the file as a `# - id` comment — the "cardápio" — so the
// user can discover options and opt in by uncommenting.
type AI struct {
	Host   string   `yaml:"host"`
	Model  string   `yaml:"model"`
	Models []string `yaml:"models,omitempty"`
}

// MarshalYAML renders the AI block with the picker "menu" comment: active
// models are written as normal sequence items, and every known model
// (AllModels()) that isn't active is glued after the last active item as a
// commented line — go-yaml prefixes each newline of a FootComment with `# `.
//
// This means: adding a Model* to AllModels() automatically shows up as a
// commented entry the next time any project's config is (re)saved. No
// per-project edit needed to advertise new models.
func (a AI) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, val string) {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: val},
		)
	}
	add("host", a.Host)
	add("model", a.Model)

	if len(a.Models) == 0 && len(AllModels()) == 0 {
		return node, nil
	}

	active := make(map[string]bool, len(a.Models))
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.LiteralStyle}
	seq.Style = 0 // block style (default) — one item per line
	for _, m := range a.Models {
		active[m] = true
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: m})
	}

	// Known-but-inactive models become commented items glued after the last
	// active one. If nothing is active (shouldn't happen — init seeds
	// a.Model), fall back to a HeadComment on the (empty) sequence so the
	// menu is still visible.
	var inactive []string
	for _, m := range AllModels() {
		if !active[m] {
			inactive = append(inactive, "- "+m)
		}
	}
	if len(inactive) > 0 {
		commented := strings.Join(inactive, "\n")
		if len(seq.Content) > 0 {
			seq.Content[len(seq.Content)-1].FootComment = commented
		} else {
			seq.HeadComment = commented
		}
	}

	key := &yaml.Node{Kind: yaml.ScalarNode, Value: "models"}
	key.HeadComment = "Modelos exibidos pelo /model no gofi-ai. Descomente para adicionar ao picker."
	node.Content = append(node.Content, key, seq)
	return node, nil
}

type Sources struct {
	Agents string `yaml:"agents"`
	// Institutional is the optional business-knowledge repo, maintained by the
	// org/company independent of any product. When set, `gofi institutional
	// update` mirrors its <project.name>/ subfolder into
	// .claude/institutional/<project.name>/ (full replace). When empty,
	// institutional/ is seeded locally at init and managed by hand in the
	// project's own git — there is no upstream to pull from.
	Institutional string `yaml:"institutional,omitempty"`
	// SDK is an optional per-language override for the SDK content. When a
	// language is present in this map, gofi fetches that repo separately
	// and uses its root for boilerplates/, sdk-docs/, knowledge/ — instead
	// of `gofi-agents/sdk/<lang>/`. Useful when the org has a fork or a
	// pinned branch of the language SDK.
	SDK map[string]string `yaml:"sdk,omitempty"`
	// UI is an optional per-design-system override for the UI library source.
	// Keys are DS package names (gofi-ui, gofi-ui-native); values are
	// github.com/<org>/<repo>@<ref>. Empty means the DS ships in the gofi
	// monorepo alongside the skills.
	UI map[string]string `yaml:"ui,omitempty"`
}

type Git struct {
	Remote string `yaml:"remote"`
}

type Training struct {
	// AutoInvoke controls whether `gofi train` automatically invokes the
	// active AI host's CLI to ask the agent to read new content. nil
	// (yaml absence) defaults to true; users opt out by setting false.
	AutoInvoke *bool `yaml:"auto_invoke,omitempty"`

	Shared []TrainingItem `yaml:"shared,omitempty"`
	PD     []TrainingItem `yaml:"pd,omitempty"`
	Spec   []TrainingItem `yaml:"spec,omitempty"`
	Eng    []TrainingItem `yaml:"eng,omitempty"`
	QA     []TrainingItem `yaml:"qa,omitempty"`
}

type TrainingItem struct {
	Topic       string `yaml:"topic"`
	Source      string `yaml:"source"`
	InstalledAt string `yaml:"installed_at"`
	Hash        string `yaml:"hash"`
}

type TestSection struct {
	Default string              `yaml:"default"`
	Hooks   TestHooks           `yaml:"hooks"`
	Tasks   map[string]TestTask `yaml:"tasks"`
}

type TestHooks struct {
	Pre  []string `yaml:"pre"`
	Post []string `yaml:"post"`
}

type TestTask struct {
	Desc  string   `yaml:"desc"`
	Run   string   `yaml:"run"`
	Needs []string `yaml:"needs,omitempty"`
}

func Load(path string) (*GofiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg GofiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := migrate(&cfg, data); err != nil {
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &cfg, nil
}

// legacyConfig probes the pre-v2 schema (version 1): backend identity lived in
// project.language/project.path and front-end surfaces lived under `ui:`. We
// unmarshal the raw bytes into it to lift those values into the current grouped
// shape (backend:, frontend:, mobile:).
type legacyConfig struct {
	Project struct {
		Language string `yaml:"language"`
		Path     string `yaml:"path"`
		Root     string `yaml:"root"`
	} `yaml:"project"`
	UI *legacyUI `yaml:"ui"`
}

// legacyUI covers both v1 spellings of the front-end block: the single-surface
// form, whose fields sit directly under `ui:` (inlined UISurface), and the
// two-surface form nesting them under `ui.web` / `ui.mobile`.
type legacyUI struct {
	UISurface `yaml:",inline"`
	Web       *UISurface `yaml:"web"`
	Mobile    *UISurface `yaml:"mobile"`
}

// migrate upgrades an on-disk config to the current schema in-memory.
//
// v1 → v2: project.language/path → backend{language,path}; ui.web → frontend;
// ui.mobile → mobile. Also folds the even older pre-root layout (absolute
// workspace stored in project.path, no project.root) into root + "src".
func migrate(cfg *GofiConfig, data []byte) error {
	// Default the backend source folder for already-current configs.
	if cfg.Version >= CurrentVersion {
		if cfg.Backend != nil && cfg.Backend.Path == "" {
			cfg.Backend.Path = DefaultSourceRoot
		}
		return nil
	}

	var lg legacyConfig
	if err := yaml.Unmarshal(data, &lg); err != nil {
		return err
	}

	sourcePath := lg.Project.Path
	// Pre-root layout: absolute workspace in project.path, no project.root.
	if cfg.Project.Root == "" && filepath.IsAbs(sourcePath) {
		cfg.Project.Root = sourcePath
		sourcePath = ""
	}
	if lg.Project.Language != "" {
		if sourcePath == "" {
			sourcePath = DefaultSourceRoot
		}
		cfg.Backend = &Backend{Language: lg.Project.Language, Path: sourcePath}
	}
	if lg.UI != nil {
		switch {
		case lg.UI.Web != nil || lg.UI.Mobile != nil:
			if lg.UI.Web != nil {
				cfg.Frontend = lg.UI.Web
			}
			if lg.UI.Mobile != nil {
				cfg.Mobile = lg.UI.Mobile
			}
		case lg.UI.Framework != "":
			// Single-surface form: the framework decides which block it
			// becomes, so a lone react-native app lands in mobile:, not
			// frontend:.
			surface := lg.UI.UISurface
			if isMobileFramework(surface.Framework) {
				cfg.Mobile = &surface
			} else {
				cfg.Frontend = &surface
			}
		}
	}

	cfg.Version = CurrentVersion
	return nil
}

// isMobileFramework reports whether a framework name denotes a mobile surface.
// Used only when migrating a v1 single-surface `ui:` block, which does not say
// which of frontend:/mobile: it should become.
func isMobileFramework(framework string) bool {
	switch framework {
	case FrameworkReactNative, "expo":
		return true
	}
	return false
}

func Save(path string, cfg *GofiConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

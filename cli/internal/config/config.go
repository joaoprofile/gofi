package config

import (
	"fmt"
	"os"
	"path/filepath"

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

	ModelOpus48   = "claude-opus-4-8"
	ModelOpus47   = "claude-opus-4-7"
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

// AllAgents returns the canonical list of the eight gofi agent slugs, in
// pipeline order.
func AllAgents() []string {
	return []string{
		AgentPD, AgentSpec, AgentEng, AgentUI,
		AgentOps, AgentQA, AgentDoc, AgentStatus,
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
// `frontend:` (web) and `mobile:` blocks. Mirrors the gofi-ui skill schema. DS
// is the design system package the surface uses, or "" when the surface opts
// out of the gofi design system.
type UISurface struct {
	Framework string `yaml:"framework"`
	Path      string `yaml:"path"`
	Brand     string `yaml:"brand,omitempty"`
	Styling   string `yaml:"styling,omitempty"`
	State     string `yaml:"state,omitempty"`
	Testing   string `yaml:"testing,omitempty"`
	DS        string `yaml:"ds,omitempty"`
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

type AI struct {
	Host  string `yaml:"host"`
	Model string `yaml:"model"`
}

type Sources struct {
	Agents string `yaml:"agents"`
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
// project.language/project.path and front-end surfaces nested under ui.web /
// ui.mobile. We unmarshal the raw bytes into it to lift those values into the
// current grouped shape (backend:, frontend:, mobile:).
type legacyConfig struct {
	Project struct {
		Language string `yaml:"language"`
		Path     string `yaml:"path"`
		Root     string `yaml:"root"`
	} `yaml:"project"`
	UI *struct {
		Web    *UISurface `yaml:"web"`
		Mobile *UISurface `yaml:"mobile"`
	} `yaml:"ui"`
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
		if lg.UI.Web != nil {
			cfg.Frontend = lg.UI.Web
		}
		if lg.UI.Mobile != nil {
			cfg.Mobile = lg.UI.Mobile
		}
	}

	cfg.Version = CurrentVersion
	return nil
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

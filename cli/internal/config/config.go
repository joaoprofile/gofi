package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

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

	// DefaultModel is what the init wizard preselects. It is pinned rather than
	// derived from the head of Models(), so adding a newer release offers it
	// without changing what a project gets when the user just presses enter.
	DefaultModel = ModelOpus48

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

	// UI surface frameworks. The field is free-form — a project may name one
	// that is not here — and these are the ones gofi recognises on its own.
	FrameworkReact       = "react"
	FrameworkReactNative = "react-native"
	FrameworkAngular     = "angular"
	FrameworkVue         = "vue"
	FrameworkSvelte      = "svelte"
	FrameworkAstro       = "astro"

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

// Model is one entry of the model picker: the ID recorded in .gofi.yaml, the
// name a picker shows, and a short note on what the family is for. The note is
// carried only by the newest member of each family — repeating it down the list
// would say nothing about the choice between two Opus releases.
type Model struct {
	ID    string
	Label string
	Note  string
}

// Models returns the canonical Claude models, ordered by family (flagship →
// light) and, within a family, newest → oldest.
//
// Single source of truth: the `gofi init` wizard offers exactly this list and
// seeds the pick into .gofi.yaml as `ai.models`, which is the list the gofi-ai
// extension's /model picker reads. Adding a model is one entry here (plus a
// Model* const above) and it appears in every picker at once.
func Models() []Model {
	return []Model{
		{ModelFable5, "Fable 5", ""},
		{ModelOpus5, "Opus 5", "most capable"},
		{ModelOpus48, "Opus 4.8", ""},
		{ModelOpus47, "Opus 4.7", ""},
		{ModelSonnet5, "Sonnet 5", "fast & sharp"},
		{ModelSonnet46, "Sonnet 4.6", ""},
		{ModelHaiku45, "Haiku 4.5", "fastest"},
	}
}

// AllModels returns just the IDs from Models, for the callers that validate or
// serialise them.
func AllModels() []string {
	models := Models()
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.ID)
	}
	return out
}

type GofiConfig struct {
	Version  int         `yaml:"version"`
	Project  Project     `yaml:"project"`
	Backend  *Backend    `yaml:"backend,omitempty"`
	Frontend *UISurface  `yaml:"frontend,omitempty"`
	Mobile   *UISurface  `yaml:"mobile,omitempty"`
	// Surfaces holds UI surfaces beyond the web front end and the mobile app —
	// a back office, an admin console — keyed by the name the project gave
	// them. Absence means there are none.
	Surfaces map[string]*UISurface `yaml:"surfaces,omitempty"`
	Ops      *Ops        `yaml:"ops,omitempty"`
	AI       AI          `yaml:"ai"`
	Agents   []string    `yaml:"agents"`
	Sources  Sources     `yaml:"sources"`
	Git      Git         `yaml:"git"`
	Graph    *Graph      `yaml:"graph,omitempty"`
	Training Training    `yaml:"training,omitempty"`
	Test     TestSection `yaml:"test"`
	Hsec     HsecConfig  `yaml:"hsec"`
	Sonar    SonarConfig `yaml:"sonar"`
}

// NamedSurface pairs a UI surface with the name it answers to.
type NamedSurface struct {
	Name    string
	Surface *UISurface
}

// Key is where the surface sits in the document, so a message can point at a
// line the user can find.
func (n NamedSurface) Key() string {
	if n.Name == "frontend" || n.Name == "mobile" {
		return n.Name
	}
	return "surfaces." + n.Name
}

// UISurfaces returns every declared UI surface — frontend and mobile first, then
// the extra ones in name order. Callers that treat all surfaces alike (graph
// scopes, sonar sources) walk this instead of naming two blocks and silently
// skipping the rest.
func (c *GofiConfig) UISurfaces() []NamedSurface {
	out := make([]NamedSurface, 0, 2+len(c.Surfaces))
	if c.Frontend != nil {
		out = append(out, NamedSurface{"frontend", c.Frontend})
	}
	if c.Mobile != nil {
		out = append(out, NamedSurface{"mobile", c.Mobile})
	}
	names := make([]string, 0, len(c.Surfaces))
	for name := range c.Surfaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if s := c.Surfaces[name]; s != nil {
			out = append(out, NamedSurface{name, s})
		}
	}
	return out
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

// Graph configures the code graph kept under .gofi/graph/. The whole block is
// optional, and its absence means every default: the graph is what the agents
// read before they open a file, so a project has one unless it says otherwise.
type Graph struct {
	// Enabled turns the graph off. nil (yaml absence) defaults to true.
	Enabled *bool `yaml:"enabled,omitempty"`
	// Hooks keeps the graph in step with the code through git hooks. nil
	// (yaml absence) defaults to true.
	Hooks *bool `yaml:"hooks,omitempty"`
	// Deep resolves calls with the type-checker instead of the syntax tree:
	// exact, at the cost of the project having to compile. Written even when
	// false: it is the setting that decides whether the agents may read an
	// absent edge as an absent call, and a key nobody sees is a key nobody
	// weighs.
	Deep bool `yaml:"deep"`
	// Exclude are directory glob patterns left out of the scan.
	Exclude []string `yaml:"exclude,omitempty"`
}

// MarshalYAML writes the block with the one comment a reader needs. Everything
// else here is self-describing; `deep` is not, and it is the setting that
// decides how much a laudo may conclude from the graph.
func (g Graph) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	// The comment goes on the key, not on the value: go-yaml renders a value's
	// head comment below the line it belongs to.
	add := func(key, comment string, val *yaml.Node) {
		k := &yaml.Node{Kind: yaml.ScalarNode, Value: key, HeadComment: comment}
		node.Content = append(node.Content, k, val)
	}
	boolean := func(b bool) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}
	}

	if g.Enabled != nil {
		add("enabled", "false desliga o grafo e, com ele, os hooks que o mantêm em dia.", boolean(*g.Enabled))
	}
	if g.Hooks != nil {
		add("hooks", "", boolean(*g.Hooks))
	}
	add("deep", "fast (false) lê só a sintaxe: rápido, roda em código que não compila, e\n"+
		"não inventa aresta quando a chamada é ambígua — ela é contada, não ligada.\n"+
		"deep (true) resolve pelo type-checker: chamada exata e implementação de\n"+
		"interface visíveis, ao custo de o projeto precisar compilar.\n"+
		"Só em deep a ausência de aresta prova ausência de uso.\n"+
		"Vale para `gofi update` e `gofi graph build`. Os hooks de git reconstroem\n"+
		"sempre em fast (--fast): o commit não espera o type-checker. Deep também\n"+
		"só muda a varredura Go — superfície de UI é sintática de todo jeito.\n"+
		"Com false, quem pede o deep é o agent, quando precisa provar ausência.",
		boolean(g.Deep))

	if len(g.Exclude) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range g.Exclude {
			seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: e})
		}
		add("exclude", "pastas fora da varredura, além das ignoradas por padrão.", seq)
	}
	return node, nil
}

// On reports whether the project keeps a graph.
func (g *Graph) On() bool { return g == nil || g.Enabled == nil || *g.Enabled }

// HooksOn reports whether gofi maintains the git hooks that rebuild the graph.
// Turning the graph off turns the hooks off with it: a hook rebuilding
// something the project does not want is worse than no hook.
func (g *Graph) HooksOn() bool {
	return g.On() && (g == nil || g.Hooks == nil || *g.Hooks)
}

// UseDeep reports whether builds use the type-checker.
func (g *Graph) UseDeep() bool { return g != nil && g.Deep }

// Excludes returns the scan exclusions.
func (g *Graph) Excludes() []string {
	if g == nil {
		return nil
	}
	return g.Exclude
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
	// Shapes the old material told projects to write are folded into the
	// current ones before anything reads the document, because a file the
	// parser rejects takes down every gofi command — including the update that
	// exists to repair it.
	data, _, err = normalizeLegacy(data)
	if err != nil {
		return nil, fmt.Errorf("normalize %s: %w", path, err)
	}
	var cfg GofiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := migrate(&cfg, data); err != nil {
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	// Blocks that came after the first releases are required by Validate, so a
	// project created before them would fail to load — including under the very
	// `gofi update` that rewrites the file. Seeding first makes validation judge
	// the effective config instead of the age of the file.
	Backfill(&cfg)
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
	if cfg.Backend != nil && cfg.Backend.Path == "" {
		cfg.Backend.Path = DefaultSourceRoot
	}

	var lg legacyConfig
	if err := yaml.Unmarshal(data, &lg); err != nil {
		return err
	}

	if cfg.Version < CurrentVersion {
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
	}

	// `ui:` is folded whatever version the file declares. The current schema has
	// no such key, so a config that says v2 and still carries one would lose
	// every surface in it the next time the file is written.
	if lg.UI != nil {
		switch {
		case lg.UI.Web != nil || lg.UI.Mobile != nil:
			if lg.UI.Web != nil && cfg.Frontend == nil {
				cfg.Frontend = lg.UI.Web
			}
			if lg.UI.Mobile != nil && cfg.Mobile == nil {
				cfg.Mobile = lg.UI.Mobile
			}
		case lg.UI.Framework != "":
			// Single-surface form: the framework decides which block it
			// becomes, so a lone react-native app lands in mobile:, not
			// frontend:.
			surface := lg.UI.UISurface
			if isMobileFramework(surface.Framework) && cfg.Mobile == nil {
				cfg.Mobile = &surface
			} else if cfg.Frontend == nil {
				cfg.Frontend = &surface
			}
		}
		// Anything else under `ui:` is a surface the project declared and the
		// two named blocks cannot hold. It keeps its name under surfaces:.
		for name, surface := range extraSurfaces(data) {
			if cfg.Surfaces == nil {
				cfg.Surfaces = map[string]*UISurface{}
			}
			if _, taken := cfg.Surfaces[name]; !taken {
				cfg.Surfaces[name] = surface
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
	backup(path)
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

// BackupDir holds the previous versions of .gofi.yaml, relative to the project
// root. It sits under .gofi/ because projects already ignore that tree, so a
// rewrite never turns up as an untracked file in the user's git status.
const BackupDir = ".gofi/backup"

// keptBackups is how many previous versions survive: enough to walk back from a
// rewrite the user did not want, few enough that the folder stays readable.
const keptBackups = 5

// backup copies the config as it stands before Save overwrites it. Best effort
// and on every write, not only the migrations — a project that cannot be backed
// up is still a project that has to be able to save.
func backup(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		// Nothing on disk yet: this is `gofi init`, not a rewrite.
		return
	}
	dir := filepath.Join(filepath.Dir(path), BackupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	// Milliseconds, so two rewrites in the same second do not land on the same
	// name. The format still sorts oldest-first as plain text, which is what
	// the pruning below relies on.
	stamp := time.Now().UTC().Format("20060102-150405.000")
	if os.WriteFile(filepath.Join(dir, FileName+"."+stamp), data, 0o644) != nil {
		return
	}
	// The stamp sorts lexicographically, so the oldest come first.
	old, err := filepath.Glob(filepath.Join(dir, FileName+".*"))
	if err != nil || len(old) <= keptBackups {
		return
	}
	sort.Strings(old)
	for _, f := range old[:len(old)-keptBackups] {
		os.Remove(f)
	}
}

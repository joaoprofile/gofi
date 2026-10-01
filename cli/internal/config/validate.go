package config

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/host"
)

var (
	// slugRe accepts a leading lowercase letter, then optional lowercase
	// letters, digits and hyphens, ending with letter or digit. Single-char
	// slugs are allowed; trailing hyphen is rejected.
	slugRe = regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])?$`)
	// segRe accepts one path segment. Deliberately permissive: it names a
	// folder that may already exist in someone else's repository, where App,
	// my_app and v2 are all real.
	segRe      = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	sourceRe   = regexp.MustCompile(`^github\.com/[^/]+/[^@]+@[^@]+$`)
	validLangs = map[string]bool{
		LanguageGo:     true,
		LanguageRust:   true,
		LanguageJava:   true,
		LanguageCSharp: true,
		LanguagePython: true,
		LanguageNodeJS: true,
	}
	// validModels stays in lockstep with AllModels(): the map is built at
	// package load so adding a new Model* const + entry in AllModels() is
	// enough — no separate list to keep in sync.
	validModels = buildValidModels()
)

// ValidSurfacePath reports whether p can name where a surface's code lives.
// Accepted: "." — the workspace root itself, for a repository gofi is adopting
// rather than scaffolding — or a relative path of one or more segments, so a
// monorepo can say services/api. Rejected is anything that would take the path
// out of the workspace: absolute paths, "..", and Windows separators or drive
// letters, which would not survive being written into .gofi.yaml and read back
// on another machine.
func ValidSurfacePath(p string) bool {
	if p == "" {
		return false
	}
	if p == "." {
		return true
	}
	if strings.HasPrefix(p, "/") || strings.ContainsAny(p, `\:`) {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." || !segRe.MatchString(seg) {
			return false
		}
	}
	return true
}

func buildValidModels() map[string]bool {
	m := make(map[string]bool, len(AllModels()))
	for _, id := range AllModels() {
		m[id] = true
	}
	return m
}

func (c *GofiConfig) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("version: expected %d, got %d", CurrentVersion, c.Version)
	}
	if !slugRe.MatchString(c.Project.Name) {
		return fmt.Errorf("project.name: %q is not a valid slug", c.Project.Name)
	}
	if c.Project.Root == "" {
		return fmt.Errorf("project.root: required")
	}
	// A project must have at least one area: backend, frontend or mobile.
	if c.Backend == nil && len(c.UISurfaces()) == 0 {
		return fmt.Errorf("config: at least one of backend, frontend or mobile is required")
	}
	if c.Backend != nil {
		if !validLangs[c.Backend.Language] {
			return fmt.Errorf("backend.language: %q invalid (expected go|rust|java|csharp|python|nodejs)", c.Backend.Language)
		}
		if !ValidSurfacePath(c.Backend.Path) {
			return fmt.Errorf("backend.path: %q is not a valid path (e.g. src, services/api, or . for the root)", c.Backend.Path)
		}
	}
	for _, s := range c.UISurfaces() {
		if err := validateSurface(s.Key(), s.Surface); err != nil {
			return err
		}
	}
	if err := validateOps(c.Ops); err != nil {
		return err
	}
	if _, ok := host.Get(c.AI.Host); !ok {
		return fmt.Errorf("ai.host: %q invalid (expected one of %s)", c.AI.Host, strings.Join(host.IDs(), ", "))
	}
	// The model list is Claude's; on another host the model is the team's
	// choice in that host's own terms.
	if host.IsClaude(c.AI.Host) && !validModels[c.AI.Model] {
		return fmt.Errorf("ai.model: %q invalid", c.AI.Model)
	}
	for tier := range c.AI.Tiers {
		if !slices.Contains(host.Tiers, host.Tier(tier)) {
			return fmt.Errorf("ai.tiers: %q is not a tier (expected light, standard or deep)", tier)
		}
	}
	if c.AI.Intake != "" && !slices.Contains(IntakeModes, c.AI.Intake) {
		return fmt.Errorf("ai.intake: %q invalid (expected %s)", c.AI.Intake, strings.Join(IntakeModes, ", "))
	}
	if c.AI.Guard != "" && !slices.Contains(GuardModes, c.AI.Guard) {
		return fmt.Errorf("ai.guard: %q invalid (expected %s)", c.AI.Guard, strings.Join(GuardModes, ", "))
	}
	if !sourceRe.MatchString(c.Sources.Agents) {
		return fmt.Errorf("sources.agents: %q is not github.com/<org>/<repo>@<tag>", c.Sources.Agents)
	}
	for lang, url := range c.Sources.SDK {
		if !validLangs[lang] {
			return fmt.Errorf("sources.sdk: %q is not a supported language", lang)
		}
		if !sourceRe.MatchString(url) {
			return fmt.Errorf("sources.sdk.%s: %q is not github.com/<org>/<repo>@<tag>", lang, url)
		}
	}
	for ds, url := range c.Sources.UI {
		if strings.TrimSpace(ds) == "" {
			return fmt.Errorf("sources.ui: empty design system key")
		}
		if !sourceRe.MatchString(url) {
			return fmt.Errorf("sources.ui.%s: %q is not github.com/<org>/<repo>@<tag>", ds, url)
		}
	}
	if err := c.Test.Validate(); err != nil {
		return err
	}
	if err := c.Sonar.validate(); err != nil {
		return err
	}
	if err := c.Hsec.validate(); err != nil {
		return err
	}
	return nil
}

var (
	// CVE/GHSA/Go vuln ids (Trivy, Nancy) and numeric npm/yarn advisory ids.
	dependencyAdvisoryRe = regexp.MustCompile(`(?i)^(CVE-\d{4}-\d+|GHSA(-[0-9a-z]{4}){3}|GO-\d{4}-\d+|\d+)$`)
	gitDirPatternRe      = regexp.MustCompile(`(^|/)\.git(/|$)`)
	hsecSeverities       = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"}
	hsecOutputFormats    = []string{"text", "json", "sarif"}
)

// A dependency advisory is never a false positive — the vulnerable code is
// really there. It can be a risk accept (e.g. the vulnerable package of the
// module is not imported), which keeps it visible as accepted risk.
func validateSuppressions(key string, rules []HsecSuppression, allowAdvisory bool) error {
	for i, r := range rules {
		if strings.TrimSpace(r.Rule) == "" {
			return fmt.Errorf("%s[%d].rule: required (horusec rule_id, e.g. HS-LEAKS-25)", key, i)
		}
		if !allowAdvisory && dependencyAdvisoryRe.MatchString(strings.TrimSpace(r.Rule)) {
			return fmt.Errorf("%s[%d].rule: %q is a dependency advisory — update the dependency, or register it under hsec.risk_accepts if it does not apply", key, i, r.Rule)
		}
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("%s[%d].reason: required — say why %s is not blocking here", key, i, r.Rule)
		}
	}
	return nil
}

// validate checks the hsec block. A disabled block is always valid. Empty
// severity/output fall back to defaults at render time, so only a non-empty
// unknown value is rejected here — at edit time instead of scan time.
func (h *HsecConfig) validate() error {
	if !h.Enabled {
		return nil
	}
	if th := strings.ToUpper(strings.TrimSpace(h.SeverityThreshold)); th != "" && !slices.Contains(hsecSeverities, th) {
		return fmt.Errorf("hsec.severity_threshold: %q is not one of %s", h.SeverityThreshold, strings.Join(hsecSeverities, ", "))
	}
	if f := strings.TrimSpace(h.OutputFormat); f != "" && !slices.Contains(hsecOutputFormats, f) {
		return fmt.Errorf("hsec.output_format: %q is not one of %s", h.OutputFormat, strings.Join(hsecOutputFormats, ", "))
	}
	if h.TimeoutSeconds < 0 {
		return fmt.Errorf("hsec.timeout_seconds: must not be negative")
	}
	if h.EnableGitHistory {
		for _, p := range h.IgnorePaths {
			if gitDirPatternRe.MatchString(p) {
				return fmt.Errorf("hsec.enable_git_history: needs .git, but hsec.ignore_paths excludes it (%q) — GitLeaks would fail silently; disable one of the two", p)
			}
		}
	}
	switch h.DockerRuntime {
	case "", HsecDockerRuntimeHost:
	case HsecDockerRuntimeIsolated:
		if !h.UseDocker {
			return fmt.Errorf("hsec.docker_runtime: %q requires hsec.use_docker: true", h.DockerRuntime)
		}
	default:
		return fmt.Errorf("hsec.docker_runtime: %q is not one of %s, %s", h.DockerRuntime, HsecDockerRuntimeHost, HsecDockerRuntimeIsolated)
	}
	if err := validateSuppressions("hsec.false_positives", h.FalsePositives, false); err != nil {
		return err
	}
	if err := validateSuppressions("hsec.risk_accepts", h.RiskAccepts, true); err != nil {
		return err
	}
	if h.Manager == nil {
		return nil
	}
	u, err := url.Parse(strings.TrimSpace(h.Manager.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("hsec.manager.url: must be an absolute http(s) URL, got %q", h.Manager.URL)
	}
	if strings.TrimSpace(h.Manager.RepositoryName) == "" {
		return fmt.Errorf("hsec.manager.repository_name: required when hsec.manager is set")
	}
	if h.Manager.TimeoutSeconds < 0 {
		return fmt.Errorf("hsec.manager.timeout_seconds: must not be negative")
	}
	return nil
}

// validate checks the sonar block. A disabled block is always valid (the user
// is not using it). When enabled, a non-empty project key is required so the
// rendered sonar-project.properties identifies the project on the server.
func (s *SonarConfig) validate() error {
	if !s.Enabled {
		return nil
	}
	if strings.TrimSpace(s.ProjectKey) == "" {
		return fmt.Errorf("sonar.project_key: required when sonar.enabled is true")
	}
	return nil
}

// validateSurface checks one front-end surface (the top-level frontend: or
// mobile: block). A present surface needs a framework + path; everything
// else (brand, styling, state, forms, i18n, testing, ds, legacy) is free-form,
// so a project that brings its own design system and stack is as valid as one
// on the gofi presets. name is the block label for error messages.
func validateSurface(name string, s *UISurface) error {
	if s == nil {
		return nil
	}
	if s.Framework == "" {
		return fmt.Errorf("%s.framework: required", name)
	}
	if !ValidSurfacePath(s.Path) {
		return fmt.Errorf("%s.path: %q is not a valid path", name, s.Path)
	}
	return nil
}

// validateOps checks the platform block. All fields are optional (the wizard
// seeds only path); non-empty enum fields must be in their allowed set.
func validateOps(ops *Ops) error {
	if ops == nil {
		return nil
	}
	checks := []struct {
		field, val string
		allowed    map[string]bool
	}{
		{"cloud", ops.Cloud, map[string]bool{CloudOCI: true, CloudAWS: true, CloudGCP: true, CloudAzure: true}},
		{"iac", ops.IaC, map[string]bool{IaCTerraform: true, IaCOpenTofu: true, IaCPulumi: true}},
		{"cicd", ops.CICD, map[string]bool{CICDGitHubActions: true, CICDAzureDevOps: true, CICDGitLabCI: true, CICDOCIDevOps: true}},
		{"target", ops.Target, map[string]bool{TargetK8s: true, TargetOKE: true, TargetEKS: true, TargetGKE: true, TargetSwarm: true, TargetContainerInstances: true, TargetPaaS: true}},
		{"registry", ops.Registry, map[string]bool{RegistryOCIR: true, RegistryECR: true, RegistryGAR: true, RegistryACR: true}},
	}
	for _, c := range checks {
		if c.val != "" && !c.allowed[c.val] {
			return fmt.Errorf("ops.%s: %q invalid", c.field, c.val)
		}
	}
	if ops.Path != "" && !slugRe.MatchString(ops.Path) {
		return fmt.Errorf("ops.path: %q is not a valid slug", ops.Path)
	}
	return nil
}

func (t *TestSection) Validate() error {
	if t.Default == "" {
		return fmt.Errorf("test.default: required")
	}
	if _, ok := t.Tasks[t.Default]; !ok {
		return fmt.Errorf("test.default: %q not in test.tasks", t.Default)
	}
	for name, task := range t.Tasks {
		for _, need := range task.Needs {
			if _, ok := t.Tasks[need]; !ok {
				return fmt.Errorf("test.tasks.%s.needs: %q not in test.tasks", name, need)
			}
		}
	}
	if cycle := detectCycle(t.Tasks); cycle != "" {
		return fmt.Errorf("test.tasks: cycle detected: %s", cycle)
	}
	return nil
}

func detectCycle(tasks map[string]TestTask) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	for name := range tasks {
		color[name] = white
	}
	var path []string
	var visit func(string) string
	visit = func(name string) string {
		switch color[name] {
		case gray:
			for i, n := range path {
				if n == name {
					return strings.Join(append(path[i:], name), " -> ")
				}
			}
			return name
		case black:
			return ""
		}
		color[name] = gray
		path = append(path, name)
		for _, need := range tasks[name].Needs {
			if c := visit(need); c != "" {
				return c
			}
		}
		path = path[:len(path)-1]
		color[name] = black
		return ""
	}
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	for _, name := range names {
		if c := visit(name); c != "" {
			return c
		}
	}
	return ""
}

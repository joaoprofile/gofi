package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// slugRe accepts a leading lowercase letter, then optional lowercase
	// letters, digits and hyphens, ending with letter or digit. Single-char
	// slugs are allowed; trailing hyphen is rejected.
	slugRe     = regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])?$`)
	sourceRe   = regexp.MustCompile(`^github\.com/[^/]+/[^@]+@[^@]+$`)
	validHosts = map[string]bool{AIHostClaudeVSCode: true}
	validLangs = map[string]bool{
		LanguageGo:     true,
		LanguageRust:   true,
		LanguageJava:   true,
		LanguageCSharp: true,
		LanguagePython: true,
		LanguageNodeJS: true,
	}
	validModels = map[string]bool{
		ModelOpus48: true, ModelOpus47: true, ModelSonnet46: true, ModelHaiku45: true,
	}
	validAgents = map[string]bool{
		AgentPD: true, AgentSpec: true, AgentEng: true, AgentUI: true,
		AgentOps: true, AgentQA: true, AgentDoc: true, AgentStatus: true,
	}
	validDS = map[string]bool{DSWeb: true, DSMobile: true}
)

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
	if c.Backend == nil && c.Frontend == nil && c.Mobile == nil {
		return fmt.Errorf("config: at least one of backend, frontend or mobile is required")
	}
	if c.Backend != nil {
		if !validLangs[c.Backend.Language] {
			return fmt.Errorf("backend.language: %q invalid (expected go|rust|java|csharp|python|nodejs)", c.Backend.Language)
		}
		if c.Backend.Path == "" || !slugRe.MatchString(c.Backend.Path) {
			return fmt.Errorf("backend.path: %q is not a valid slug (e.g. src, services, backend)", c.Backend.Path)
		}
	}
	if err := validateSurface("frontend", c.Frontend); err != nil {
		return err
	}
	if err := validateSurface("mobile", c.Mobile); err != nil {
		return err
	}
	if err := validateOps(c.Ops); err != nil {
		return err
	}
	if !validHosts[c.AI.Host] {
		return fmt.Errorf("ai.host: %q invalid (expected claude-vscode)", c.AI.Host)
	}
	if !validModels[c.AI.Model] {
		return fmt.Errorf("ai.model: %q invalid", c.AI.Model)
	}
	if len(c.Agents) == 0 {
		return fmt.Errorf("agents: at least one agent required")
	}
	seen := map[string]bool{}
	for _, a := range c.Agents {
		if !validAgents[a] {
			return fmt.Errorf("agents: %q invalid", a)
		}
		if seen[a] {
			return fmt.Errorf("agents: %q duplicated", a)
		}
		seen[a] = true
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
		if !validDS[ds] {
			return fmt.Errorf("sources.ui: %q is not a supported design system (expected gofi-ui|gofi-ui-native)", ds)
		}
		if !sourceRe.MatchString(url) {
			return fmt.Errorf("sources.ui.%s: %q is not github.com/<org>/<repo>@<tag>", ds, url)
		}
	}
	if err := c.Training.Validate(); err != nil {
		return err
	}
	if err := c.Test.Validate(); err != nil {
		return err
	}
	if err := c.Sonar.validate(); err != nil {
		return err
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
// mobile: block). A present surface needs a framework + slug path;
// brand/styling/state/testing/ds are validated only when set so future presets
// don't break older configs. name is the block label for error messages.
func validateSurface(name string, s *UISurface) error {
	if s == nil {
		return nil
	}
	if s.Framework == "" {
		return fmt.Errorf("%s.framework: required", name)
	}
	if s.Path == "" || !slugRe.MatchString(s.Path) {
		return fmt.Errorf("%s.path: %q is not a valid slug", name, s.Path)
	}
	if s.Brand != "" && s.Brand != BrandBlue && s.Brand != BrandViolet && s.Brand != BrandGreen {
		return fmt.Errorf("%s.brand: %q invalid (expected blue|violet|green)", name, s.Brand)
	}
	if s.DS != "" && !validDS[s.DS] {
		return fmt.Errorf("%s.ds: %q invalid (expected gofi-ui|gofi-ui-native)", name, s.DS)
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

func (t *Training) Validate() error {
	scopes := map[string][]TrainingItem{
		"shared": t.Shared,
		"pd":     t.PD,
		"spec":   t.Spec,
		"eng":    t.Eng,
		"qa":     t.QA,
	}
	for name, items := range scopes {
		seen := map[string]bool{}
		for _, it := range items {
			if !slugRe.MatchString(it.Topic) {
				return fmt.Errorf("training.%s[].topic: %q is not a valid slug", name, it.Topic)
			}
			if seen[it.Topic] {
				return fmt.Errorf("training.%s[].topic: %q duplicated", name, it.Topic)
			}
			seen[it.Topic] = true
		}
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

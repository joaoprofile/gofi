package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/detect"
	"github.com/joaoprofile/gofi-cli/internal/gitops"
	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi-cli/internal/hsec"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/scaffold"
	"github.com/joaoprofile/gofi-cli/internal/sonar"
	"github.com/joaoprofile/gofi-cli/internal/toolchain"
	"github.com/joaoprofile/gofi-cli/internal/tui/spinner"
	"github.com/joaoprofile/gofi-cli/internal/tui/styles"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: i18n.T("cmd.init.short"),
		Long: `Run an interactive wizard that creates a new gofi project end-to-end.

The wizard asks for AI host, Claude model, project name, repository name, root path,
target language (go | rust), agents to activate and (optionally) the git remote URL.
After confirmation, gofi creates the project directory, runs git init, scaffolds the
language toolchain, installs the .claude/ structure with the selected agents and
writes .gofi.yaml as the source of truth.

Failures roll back the created directory.`,
		Example: `gofi init`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("gofi init requires an interactive terminal")
			}
			return runInit()
		},
	}
}

func runInit() error {
	res, err := wizard.Run(nil)
	if err != nil {
		if errors.Is(err, wizard.ErrCancelled) {
			fmt.Println("init cancelled.")
			return nil
		}
		return err
	}

	if err := checkRootIsUsable(res.Root); err != nil {
		return err
	}

	confirmSummary(res)

	// Track whether the workspace folder didn't exist before init so rollback
	// knows whether to nuke it. When initialising in-place (cwd or any other
	// pre-existing dir), we never delete the user's directory on failure —
	// only the artifacts we created get rolled back manually if needed.
	rootCreated := false
	if exists, _ := pathExists(res.Root); !exists {
		rootCreated = true
	}

	if err := executePipeline(res); err != nil {
		if rootCreated {
			_ = os.RemoveAll(res.Root)
		}
		return fmt.Errorf("init failed: %w", err)
	}

	printNextSteps(res)
	return nil
}

// checkRootIsUsable rejects the chosen workspace folder when it's already a
// gofi project, and rejects any non-empty pre-existing folder *unless* it is
// the current working directory — which is the documented "init in place"
// case (e.g. running `gofi init` inside an empty new repo with a stray
// README.md or .git already there).
func checkRootIsUsable(root string) error {
	yamlPath := filepath.Join(root, config.FileName)
	if _, err := os.Stat(yamlPath); err == nil {
		return fmt.Errorf("%s already contains a gofi project (%s present)", root, config.FileName)
	}
	exists, _ := pathExists(root)
	if !exists {
		return nil
	}
	empty, _ := dirIsEmpty(root)
	if empty {
		return nil
	}
	cwd, err := os.Getwd()
	if err == nil {
		if abs, absErr := filepath.Abs(root); absErr == nil && abs == cwd {
			return nil
		}
	}
	return fmt.Errorf("path %s already exists and is not empty", root)
}

// detectToolchain is the preflight entry point, indirected so tests can force a
// missing/present toolchain without depending on the host environment.
var detectToolchain = toolchain.Detect

func executePipeline(r *wizard.Result) error {
	hasBack := r.Has(wizard.EnvBack)
	goBackend := hasBack && r.Language == config.LanguageGo
	needNode := r.Has(wizard.EnvWeb) || r.Has(wizard.EnvMobile)

	pre := detectToolchain(toolchain.Needs{Go: goBackend, Node: needNode})
	renderPreflight(pre)

	data := scaffold.TemplateData{
		ProjectName: r.Name,
		Date:        time.Now().Format("2006-01-02"),
		AIHost:      r.AIHost,
		AIModel:     r.AIModel,
		Agents:      r.Agents,
	}
	if goBackend {
		data.Language = r.Language
		data.GoModule = r.GoModule
		data.SourceRoot = r.SourcePath
	}

	backendLang := ""
	if hasBack {
		backendLang = r.Language
	}
	cfg := buildConfig(r)
	sdkRef := cfg.Sources.SDK[backendLang]
	uiSurfaces := uiSurfacesFromResult(r)

	var skipped []string
	if hasBack && r.Language != config.LanguageGo {
		skipped = append(skipped, fmt.Sprintf("backend (%s) — scaffold not implemented yet (only Go today)", r.Language))
	}

	steps := []spinner.Step{
		{Name: "Create workspace folder", Fn: func() error {
			return os.MkdirAll(r.Root, 0o755)
		}},
		{Name: "Initialise git repository (no-op if existing)", Fn: func() error {
			return gitops.Init(r.Root)
		}},
	}
	if goBackend && pre.GoOK {
		steps = append(steps, spinner.Step{Name: "Scaffold Go (" + r.SourcePath + "/)", Fn: func() error {
			_, err := scaffold.InstallGo(r.Root, data)
			return err
		}})
	} else if goBackend && !pre.GoOK {
		skipped = append(skipped, "backend (Go) — Go toolchain not found; install Go and run scaffold later")
	}
	steps = append(steps,
		spinner.Step{Name: "Write .gofi.yaml", Fn: func() error {
			return config.Save(filepath.Join(r.Root, config.FileName), cfg)
		}},
		spinner.Step{Name: "Seed .gofi/ + .gitignore", Fn: func() error {
			if err := os.MkdirAll(filepath.Join(r.Root, ".gofi"), 0o755); err != nil {
				return err
			}
			if err := ensureGofiIgnored(r.Root); err != nil {
				return err
			}
			return ensureGitignore(r.Root, ".env")
		}},
		spinner.Step{Name: "Seed local .env", Fn: func() error {
			// Backend projects get a populated .env from
			// env/localstack/.env-example in the "Seed ops/localstack + .env"
			// step below; here we only seed an empty one for non-backend setups.
			if hasBack {
				return nil
			}
			return ensureEnvFile(r.Root)
		}},
		spinner.Step{Name: "Seed Horusec config", Fn: func() error {
			if !cfg.Hsec.Enabled {
				return nil
			}
			if _, err := hsec.WriteConfig(r.Root, cfg.Hsec); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write horusec-config.json: %v\n", err)
			}
			return nil
		}},
		spinner.Step{Name: "Seed Sonar config", Fn: func() error {
			if !cfg.Sonar.Enabled {
				return nil
			}
			if _, err := sonar.WriteConfig(r.Root, cfg.Sonar, backendLang); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write sonar-project.properties: %v\n", err)
			}
			return nil
		}},
		spinner.Step{Name: "Seed specs/ prd/ ops/", Fn: func() error {
			if err := seedDocDir(r.Root, "ops"); err != nil {
				return err
			}
			if r.CreateSpecsDir {
				if err := seedDocDir(r.Root, "specs"); err != nil {
					return err
				}
				if err := scaffold.SeedCorpusIndex(r.Root, "specs"); err != nil {
					return err
				}
			}
			if r.CreatePrdDir {
				if err := seedDocDir(r.Root, "prd"); err != nil {
					return err
				}
				if err := scaffold.SeedCorpusIndex(r.Root, "prd"); err != nil {
					return err
				}
			}
			return nil
		}},
		spinner.Step{Name: "Fetch agents + install .claude/", Fn: func() error {
			sha, err := installFromSource(r.Root, backendLang, uiSurfaces, r.AgentsRef, sdkRef, data, scaffold.InstallNew)
			if err != nil {
				return err
			}
			r.ClaudeSource = "fetch:" + sha
			if err := writeInstalledSha(r.Root, sha); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not record installed SHA: %v\n", err)
			}
			return nil
		}},
	)
	if r.InstitutionalRef != "" {
		steps = append(steps, spinner.Step{Name: "Pull institutional base", Fn: func() error {
			return seedInstitutionalFromRepo(r.Root, r.Name, r.InstitutionalRef)
		}})
	}
	if hasBack {
		steps = append(steps, spinner.Step{Name: "Seed ops/localstack + .env", Fn: func() error {
			return seedLocalstackEnv(r.Root, r.AgentsRef)
		}})
	}
	if goBackend && pre.GoOK {
		steps = append(steps, spinner.Step{Name: "Wire SDK into go.work", Fn: func() error {
			return scaffold.EnsureGoWorkSDK(r.Root, r.Language)
		}})
	}
	if r.GitRemote != "" {
		steps = append(steps, spinner.Step{Name: "Configure git remote", Fn: func() error {
			return gitops.AddRemote(r.Root, "origin", r.GitRemote)
		}})
	}
	// The GOFI AI extension is the editor-side half of the harness the steps
	// above just installed — the panel that talks to the `.claude/skills/` this
	// project now has. Best effort: a machine with no editor on PATH still gets
	// a working project, and `gofi install extensions` fixes it later.
	var extensionsNote string
	steps = append(steps, spinner.Step{Name: "Install the GOFI AI extension", Fn: func() error {
		extensionsNote = installExtensionsOnInit(context.Background())
		return nil
	}})
	// The graph is the map the agents read before they open a file, so a
	// project ships with one. Best effort like the extension above: a scaffold
	// that cannot be scanned yet is still a working scaffold.
	var graphNote, hooksNote string
	steps = append(steps, spinner.Step{Name: "Build the code graph", Fn: func() error {
		graphNote = buildGraphQuietly(context.Background(), cfg, r.Root)
		hooksNote = installGraphHooksQuietly(cfg, r.Root)
		return nil
	}})

	results := spinner.Run(steps)
	if spinner.AnyFailed(results) {
		for _, res := range results {
			if res.Err != nil {
				return fmt.Errorf("step %q failed: %w", res.Name, res.Err)
			}
		}
	}
	for _, note := range []string{extensionsNote, graphNote, hooksNote} {
		if note != "" {
			fmt.Println("  " + styles.Note(note))
		}
	}

	// Web/Mobile via official CLIs — streamed output, after the spinner steps.
	// Gated on the Node preflight; missing Node = skipped, not fatal.
	if r.Has(wizard.EnvWeb) {
		if pre.NodeOK {
			fmt.Println("\n" + styles.Header("▶ Creating web app (Vite) at "+r.WebPath+"/"))
			if err := scaffold.CreateViteApp(r.Root, r.WebPath, r.WebDS == config.DSWeb); err != nil {
				return fmt.Errorf("create web app: %w", err)
			}
		} else {
			skipped = append(skipped, "web — Node.js LTS not found; install it, then: npm create vite@latest "+r.WebPath)
		}
	}
	if r.Has(wizard.EnvMobile) {
		if pre.NodeOK {
			fmt.Println("\n" + styles.Header("▶ Creating mobile app (Expo) at "+r.MobilePath+"/"))
			if err := scaffold.CreateExpoApp(r.Root, r.MobilePath, r.MobileDS == config.DSMobile); err != nil {
				return fmt.Errorf("create mobile app: %w", err)
			}
		} else {
			skipped = append(skipped, "mobile — Node.js LTS not found; install it, then: npx create-expo-app "+r.MobilePath)
		}
	}

	r.Skipped = skipped
	return nil
}

// renderPreflight prints the toolchain detection result. No-op when nothing was
// required (back-only with Go present needs no Node check, etc.).
func renderPreflight(p toolchain.Preflight) {
	if len(p.Checks) == 0 {
		return
	}
	fmt.Println()
	fmt.Println("  " + styles.Header("Toolchain"))
	for _, c := range p.Checks {
		switch {
		case c.OK && !c.Warn:
			fmt.Println("    " + styles.Success("✓") + " " + c.Name + " " + c.Version)
		case c.OK && c.Warn:
			fmt.Println("    " + styles.Warn("!") + " " + c.Name + " " + c.Version + " — " + c.Hint)
		default:
			line := "    " + styles.Error("✗") + " " + c.Name
			if c.Hint != "" {
				line += " — " + c.Hint
			}
			fmt.Println(line)
		}
	}
}

// seedDocDir creates <projectRoot>/<name>/.gitkeep so the directory exists
// in git before any content is written. Used for specs/ and prd/.
func seedDocDir(projectRoot, name string) error {
	dir := filepath.Join(projectRoot, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	keep := filepath.Join(dir, ".gitkeep")
	if _, err := os.Stat(keep); err == nil {
		return nil
	}
	return os.WriteFile(keep, []byte{}, 0o644)
}

// ensureEnvFile creates an empty .env at projectRoot when missing so users
// have a place for local-only configuration right after init. Mode 0600
// since env files commonly hold secrets. Never overwrites an existing one.
func ensureEnvFile(projectRoot string) error {
	path := filepath.Join(projectRoot, ".env")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte{}, 0o600)
}

// gofiIgnoreRules keep .gofi/ out of git except for the graph.
//
// The graph is versioned on purpose: it is the map the agents read, so the team
// and CI should share the one the code actually produced instead of each
// rebuilding their own. Canonical ordering in the JSON is what makes that
// bearable to diff.
//
// Excluding a directory and re-including a child of it needs the `.gofi/*`
// spelling — a plain `.gofi/` stops git from ever descending, and the negation
// below would never be reached.
var gofiIgnoreRules = []string{
	".gofi/*",
	"!" + graph.OutDir + "/",
	external.ExtractorsDir + "/",
}

// ensureGofiIgnored writes those rules, upgrading a .gitignore written before
// the graph existed. Order matters, so the legacy line is rewritten in place
// rather than left to compete with the new ones.
func ensureGofiIgnored(projectRoot string) error {
	path := filepath.Join(projectRoot, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var kept []string
	for line := range strings.Lines(string(existing)) {
		if trimmed := strings.TrimSpace(line); trimmed == ".gofi/" || slices.Contains(gofiIgnoreRules, trimmed) {
			continue
		}
		kept = append(kept, strings.TrimRight(line, "\n"))
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	body := strings.Join(append(kept, gofiIgnoreRules...), "\n") + "\n"
	if body == string(existing) {
		return nil
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// ensureGitignore appends entry to <projectRoot>/.gitignore if not already
// present, creating the file when missing.
func ensureGitignore(projectRoot, entry string) error {
	path := filepath.Join(projectRoot, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(existing), "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	body := string(existing)
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += entry + "\n"
	return os.WriteFile(path, []byte(body), 0o644)
}

// defaultWebSurface / defaultMobileSurface are the gofi presets seeded into a
// brand new surface. Path and DS are filled by the caller from the wizard; the
// rest is a starting point the project is free to replace in .gofi.yaml — the
// gofi-ui skill reads whatever is there.
func defaultWebSurface(dir string) config.UISurface {
	return config.UISurface{
		Framework: detect.WebFramework(dir),
		Brand:     config.BrandBlue,
		Styling:   config.StylingTailwind,
		State:     config.StateTanstackQuery,
		Testing:   config.TestingVitest,
	}
}

func defaultMobileSurface() config.UISurface {
	return config.UISurface{
		Framework: config.FrameworkReactNative,
		Brand:     config.BrandBlue,
		Styling:   config.StylingStylesheet,
		State:     config.StateTanstackQuery,
		Testing:   config.TestingJest,
	}
}

func buildConfig(r *wizard.Result) *config.GofiConfig {
	proj := config.Project{Name: r.Name, Root: r.Root}
	testLang, testPath := "", ""
	var backend *config.Backend
	if r.Has(wizard.EnvBack) {
		backend = &config.Backend{Language: r.Language, Path: r.SourcePath}
		testLang, testPath = r.Language, r.SourcePath
	}

	var frontend, mobile *config.UISurface
	if r.Has(wizard.EnvWeb) {
		s := defaultWebSurface(filepath.Join(r.Root, r.WebPath))
		s.Path, s.DS = r.WebPath, r.WebDS
		frontend = &s
	}
	if r.Has(wizard.EnvMobile) {
		s := defaultMobileSurface()
		s.Path, s.DS = r.MobilePath, r.MobileDS
		mobile = &s
	}

	// Go SDK is the only git source the wizard configures (cloned into
	// .gofi/gofi-sdk-go/). Web/mobile design systems are npm packages.
	src := config.Sources{Agents: r.AgentsRef, Institutional: r.InstitutionalRef}
	if r.Has(wizard.EnvBack) && r.Language == config.LanguageGo {
		if v := r.SDKURLs[config.LanguageGo]; v != "" {
			src.SDK = map[string]string{config.LanguageGo: v}
		}
	}

	return &config.GofiConfig{
		Version:  config.CurrentVersion,
		Project:  proj,
		Backend:  backend,
		Frontend: frontend,
		Mobile:   mobile,
		Ops:      config.DefaultOps(),
		AI:       config.AI{Host: r.AIHost, Model: r.AIModel, Models: []string{r.AIModel}},
		Agents:   r.Agents,
		Sources:  src,
		Git:      config.Git{Remote: r.GitRemote},
		Test:     config.DefaultTestSection(testLang, testPath),
		Hsec:     config.DefaultHsec(),
		Sonar:    config.DefaultSonar(proj.Name, backend, frontend, mobile),
	}
}

// uiSurfacesFromResult lists the surfaces whose design-system docs should be
// installed into .claude/. Web/mobile always use their gofi design system.
func uiSurfacesFromResult(r *wizard.Result) []string {
	var s []string
	if r.Has(wizard.EnvWeb) {
		s = append(s, "web")
	}
	if r.Has(wizard.EnvMobile) {
		s = append(s, "mobile")
	}
	return s
}

// uiSurfacesFromConfig is the update-time equivalent of uiSurfacesFromResult.
func uiSurfacesFromConfig(cfg *config.GofiConfig) []string {
	var s []string
	if cfg.Frontend != nil && cfg.Frontend.DS != "" {
		s = append(s, "web")
	}
	if cfg.Mobile != nil && cfg.Mobile.DS != "" {
		s = append(s, "mobile")
	}
	return s
}

func confirmSummary(r *wizard.Result) {
	row := func(k, v string) string {
		return styles.Label(fmt.Sprintf("%-9s", k)) + " " + styles.Value(v)
	}
	dsLabel := func(ds string) string {
		if ds == "" {
			return "no design system"
		}
		return ds
	}
	lines := []string{styles.Header("Summary"), ""}
	lines = append(lines, row("name", r.Name))
	lines = append(lines, row("root", r.Root))
	lines = append(lines, row("surfaces", strings.Join(r.Environments, ", ")))
	if r.Has(wizard.EnvBack) {
		b := r.Language + " (" + r.SourcePath + "/)"
		if r.Language == config.LanguageGo {
			b += "  module=" + r.GoModule
		}
		lines = append(lines, row("backend", b))
	}
	if r.Has(wizard.EnvWeb) {
		lines = append(lines, row("web", "react ("+r.WebPath+"/)  "+dsLabel(r.WebDS)))
	}
	if r.Has(wizard.EnvMobile) {
		lines = append(lines, row("mobile", "expo ("+r.MobilePath+"/)  "+dsLabel(r.MobileDS)))
	}
	lines = append(lines, row("ops", "ops/"))
	lines = append(lines, row("AI", r.AIHost+" ("+r.AIModel+")"))
	lines = append(lines, row("agents", strings.Join(r.Agents, ", ")))
	lines = append(lines, row("skills", r.AgentsRef))
	if r.InstitutionalRef != "" {
		lines = append(lines, row("institutional", r.InstitutionalRef))
	} else {
		lines = append(lines, row("institutional", "manual (no repo)"))
	}
	if r.GitRemote != "" {
		lines = append(lines, row("remote", r.GitRemote))
	}
	fmt.Println()
	fmt.Println(styles.Panel(strings.Join(lines, "\n")))
}

func printNextSteps(r *wizard.Result) {
	fmt.Printf("\n  %s — project created at %s (.claude/ from %s)\n\n",
		styles.Success("✓ done"), r.Root, sourceLabel(r.ClaudeSource))
	if len(r.Skipped) > 0 {
		fmt.Println("  " + styles.Warn("Skipped — install the toolchain, then create later:"))
		for _, s := range r.Skipped {
			fmt.Println("    - " + s)
		}
		fmt.Println()
	}
	fmt.Println("  " + styles.Header("Next steps"))
	fmt.Printf("    cd %s\n", r.Root)
	fmt.Println("    git status                 # review the scaffolded files")
	fmt.Println("    gofi commit \"chore: gofi init\"")
	if r.GitRemote == "" {
		fmt.Println("    gofi remote add <url>      # configure a git remote")
	}
	fmt.Println("    gofi h                     # explore commands")
	for _, a := range r.Agents {
		fmt.Printf("    /%s\n", a)
	}
	fmt.Println()
}

func pathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func dirIsEmpty(p string) (bool, error) {
	entries, err := os.ReadDir(p)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func sourceLabel(s string) string {
	if s == "" {
		return "embedded"
	}
	return s
}

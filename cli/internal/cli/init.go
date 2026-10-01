package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/detect"
	"github.com/joaoprofile/gofi/cli/internal/gitops"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/hsec"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
	"github.com/joaoprofile/gofi/cli/internal/sonar"
	"github.com/joaoprofile/gofi/cli/internal/toolchain"
	"github.com/joaoprofile/gofi/cli/internal/tui/flow"
	"github.com/joaoprofile/gofi/cli/internal/tui/spinner"
	"github.com/joaoprofile/gofi/cli/internal/tui/styles"
	"github.com/joaoprofile/gofi/cli/internal/tui/wizard"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [name]",
		Short: i18n.T("cmd.init.short"),
		Long: `Create a new gofi project, or adopt an existing repository, through an
interactive dialogue.

With a name, the project is created in ./<name>; without one, in the current
folder. The dialogue asks for the surfaces (backend, web, mobile), the backend
language and module, the Claude model, the agents to activate and, optionally,
the git remote. Every question comes with a suggestion — enter keeps it, esc
goes back to the previous question, ctrl+c leaves without writing anything.

After the review, gofi creates the folder, runs git init, scaffolds each
surface, installs the .claude/ structure with the selected agents and writes
.gofi.yaml as the source of truth. Failures roll back a folder gofi created.`,
		Example: `gofi init my-project
gofi init`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return errors.New("gofi init requires an interactive terminal")
			}
			name := ""
			if len(args) == 1 {
				name = strings.TrimSpace(args[0])
				if !initNameRe.MatchString(name) {
					return errors.New(i18n.T("init.invalid_name", name))
				}
			}
			return runInit(name)
		},
	}
}

// initNameRe mirrors the wizard's project-name rule, so a bad name fails before
// the dialogue opens instead of on its first question.
var initNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]+$`)

func runInit(name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	meta := wizard.Meta{Version: Version}
	target := cwd
	if name != "" {
		target = filepath.Join(cwd, name)
		meta.Name, meta.Root = name, target
	}

	res, err := wizard.Run(nil, detect.Scan(target), meta)
	if err != nil {
		if errors.Is(err, wizard.ErrCancelled) {
			fmt.Println(styles.Note(i18n.T("init.cancelled")))
			return nil
		}
		return err
	}

	// The wizard may point the root somewhere other than the scanned folder, in
	// which case the pre-wizard scan describes the wrong tree.
	if res.Root != target {
		res.Detected = detect.Scan(res.Root)
	}

	if err := checkRootIsUsable(res.Root, res.Detected); err != nil {
		return err
	}

	fmt.Println()
	flow.Say(i18n.T("init.creating", res.Name, res.Root))

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
// README.md or .git already there) — or a folder holding a codebase gofi
// recognised, which is the adoption case: the content is the reason to init
// there, not an obstacle.
func checkRootIsUsable(root string, found detect.Result) error {
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
	if found.Any() {
		return nil
	}
	return fmt.Errorf("path %s already exists and is not empty", root)
}

// detectToolchain is the preflight entry point, indirected so tests can force a
// missing/present toolchain without depending on the host environment.
var detectToolchain = toolchain.Detect

// createViteApp and createExpoApp are indirected for the same reason: they
// shell out to npm/npx, so tests swap them to assert whether a surface was
// scaffolded at all.
var (
	createViteApp = scaffold.CreateViteApp
	createExpoApp = scaffold.CreateExpoApp
)

func executePipeline(r *wizard.Result) error {
	hasBack := r.Has(wizard.EnvBack)
	goBackend := hasBack && r.Language == config.LanguageGo
	adoptedBack := hasBack && r.Adopted(wizard.EnvBack)
	// Only a surface gofi is going to create needs Node: an adopted app is never
	// scaffolded, so demanding a toolchain for it would warn about nothing.
	needNode := (r.Has(wizard.EnvWeb) && !r.Adopted(wizard.EnvWeb)) ||
		(r.Has(wizard.EnvMobile) && !r.Adopted(wizard.EnvMobile))

	// The host decides the folder everything below is written to: fixed first,
	// so no path is resolved against another host's layout.
	h, ok := host.Get(r.AIHost)
	if !ok {
		return fmt.Errorf("ai host %q is not supported (expected one of %s)", r.AIHost, strings.Join(host.IDs(), ", "))
	}
	layout.SetHome(h.Home)
	scaffold.SetSkillModels(h, nil)

	onClaude := h.ID == host.ClaudeCode.ID
	pre := detectToolchain(toolchain.Needs{Go: goBackend, Node: needNode, Claude: onClaude})
	renderPreflight(pre)
	// The one toolchain init cannot work around on Claude Code: one older than
	// the minimum starts every session without the project's instructions.
	// Stopping here, before anything is written, leaves nothing half-made.
	if !pre.ClaudeOK {
		return fmt.Errorf("%s", i18n.T("init.claude_required", toolchain.MinClaudeCode))
	}

	data := scaffold.TemplateData{
		ProjectName: r.Name,
		Date:        time.Now().Format("2006-01-02"),
		AIHost:      r.AIHost,
		AIModel:     r.AIModel,
		Agents:      scaffold.Skills(),
	}
	if hasBack {
		data.Language = r.Language
		data.Module = r.Module
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
	// An adopted backend needs no scaffold whatever its language, so reporting a
	// missing one would describe a gap that is not there.
	if hasBack && !adoptedBack && !scaffold.HasBackendScaffold(r.Language) {
		skipped = append(skipped, i18n.T("init.skip.no_scaffold", r.Language))
	}

	steps := []spinner.Step{
		{Name: i18n.T("init.step.mkdir"), Fn: func() error {
			return os.MkdirAll(r.Root, 0o755)
		}},
		{Name: i18n.T("init.step.git"), Fn: func() error {
			return gitops.Init(r.Root)
		}},
	}
	switch {
	case !hasBack || !scaffold.HasBackendScaffold(r.Language):
		// Nothing to write — already reported in skipped above.
	case goBackend && !pre.GoOK:
		skipped = append(skipped, i18n.T("init.skip.no_go"))
	case goBackend && adoptedBack:
		// The tree is already there. Writing the scaffold over it would litter
		// someone's repository with a README, a main.go and empty folders it
		// never asked for; only go.work is added, because the SDK needs it.
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.adopt_go", r.SourcePath), Fn: func() error {
			return scaffold.EnsureGoWork(r.Root, r.SourcePath)
		}})
	case adoptedBack:
		// Same reasoning as Go, minus go.work: outside Go there is no workspace
		// file to reconcile, so adoption means leaving the tree exactly as is.
		skipped = append(skipped, i18n.T("init.skip.adopted_back", r.Language, r.SourcePath))
	default:
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.scaffold", r.Language, r.SourcePath), Fn: func() error {
			_, err := scaffold.InstallBackend(r.Language, r.Root, data)
			return err
		}})
	}
	steps = append(steps,
		spinner.Step{Name: i18n.T("init.step.yaml"), Fn: func() error {
			return config.Save(filepath.Join(r.Root, config.FileName), cfg)
		}},
		spinner.Step{Name: i18n.T("init.step.gofi_dir"), Fn: func() error {
			if err := os.MkdirAll(filepath.Join(r.Root, ".gofi"), 0o755); err != nil {
				return err
			}
			if err := ensureGofiIgnored(r.Root); err != nil {
				return err
			}
			return ensureGitignore(r.Root, ".env")
		}},
		spinner.Step{Name: i18n.T("init.step.env"), Fn: func() error {
			// Backend projects get a populated .env from
			// env/localstack/.env-example in the "Seed ops/localstack + .env"
			// step below; here we only seed an empty one for non-backend setups.
			if hasBack {
				return nil
			}
			return ensureEnvFile(r.Root)
		}},
		spinner.Step{Name: i18n.T("init.step.hsec"), Fn: func() error {
			if !cfg.Hsec.Enabled {
				return nil
			}
			if _, err := hsec.WriteConfig(r.Root, cfg.Hsec); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write horusec-config.json: %v\n", err)
			}
			return nil
		}},
		spinner.Step{Name: i18n.T("init.step.sonar"), Fn: func() error {
			if !cfg.Sonar.Enabled {
				return nil
			}
			if _, err := sonar.WriteConfig(r.Root, cfg.Sonar, backendLang); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write sonar-project.properties: %v\n", err)
			}
			return nil
		}},
		spinner.Step{Name: i18n.T("init.step.docs"), Fn: func() error {
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
		spinner.Step{Name: i18n.T("init.step.claude"), Fn: func() error {
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
		spinner.Step{Name: i18n.T("init.step.mcp"), Fn: func() error {
			h, _ := host.Get(r.AIHost)
			if _, err := h.RegisterMCP(r.Root); err != nil {
				return err
			}
			// The guard and the allowed read-only queries live in the host's
			// settings; only a host whose hook contract gofi verified gets them.
			if !h.Guard {
				return nil
			}
			_, err := installAgentSettings(r.Root, config.AI{Guard: config.GuardWarn})
			return err
		}},
	)
	if r.InstitutionalRef != "" {
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.institutional"), Fn: func() error {
			return seedInstitutionalFromRepo(r.Root, r.Name, r.InstitutionalRef)
		}})
	}
	if hasBack {
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.localstack"), Fn: func() error {
			return seedLocalstackEnv(r.Root, r.AgentsRef)
		}})
	}
	if goBackend && pre.GoOK {
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.gowork"), Fn: func() error {
			return scaffold.EnsureGoWorkSDK(r.Root, r.Language)
		}})
	}
	if r.GitRemote != "" {
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.remote"), Fn: func() error {
			return gitops.AddRemote(r.Root, "origin", r.GitRemote)
		}})
	}
	// The GOFI AI extension is the editor-side half of the harness the steps
	// above just installed — the panel that talks to the `.claude/skills/` this
	// project now has. Best effort: a machine with no editor on PATH still gets
	// a working project, and `gofi install extensions` fixes it later.
	var extensionsNote string
	if onClaude {
		// The extension's chat drives Claude Code; another host has its own.
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.extension"), Fn: func() error {
			extensionsNote = installExtensionsOnInit(context.Background())
			return nil
		}})
	}
	// The graph is the map the agents read before they open a file, so a
	// project ships with one. Best effort like the extension above: a scaffold
	// that cannot be scanned yet is still a working scaffold.
	var graphNote string
	steps = append(steps, spinner.Step{Name: i18n.T("init.step.graph"), Fn: func() error {
		graphNote = buildGraphQuietly(context.Background(), cfg, r.Root)
		return nil
	}})
	if wantsIndexHooks(cfg) {
		steps = append(steps, spinner.Step{Name: i18n.T("init.step.hooks"), Fn: func() error {
			_, err := installIndexHooks(r.Root)
			return err
		}})
	}

	results := spinner.Run(steps)
	if spinner.AnyFailed(results) {
		for _, res := range results {
			if res.Err != nil {
				return fmt.Errorf("step %q failed: %w", res.Name, res.Err)
			}
		}
	}
	for _, note := range []string{extensionsNote, graphNote} {
		if note != "" {
			fmt.Println(styles.Detail(styles.Note(note)))
		}
	}

	// Web/Mobile via official CLIs — streamed output, after the spinner steps.
	// Gated on the Node preflight; missing Node = skipped, not fatal.
	//
	// An adopted surface is never handed to a scaffolder: create-vite and
	// create-expo-app write into the folder they are given, and the design-system
	// starter that follows replaces the app entry files. Over a repository that
	// already has an app that is not a scaffold, it is data loss.
	if r.Has(wizard.EnvWeb) {
		switch {
		case r.Adopted(wizard.EnvWeb):
			fmt.Println("\n" + styles.Bullet(styles.Info, i18n.T("init.web_adopted", r.WebPath)))
			if r.WebDS == config.DSWeb {
				skipped = append(skipped, i18n.T("init.skip.web_ds", config.DSWeb))
			}
		case pre.NodeOK:
			fmt.Println("\n" + styles.Bullet(styles.Info, i18n.T("init.web", r.WebPath)))
			if err := createViteApp(r.Root, r.WebPath, r.WebDS == config.DSWeb); err != nil {
				return fmt.Errorf("create web app: %w", err)
			}
		default:
			skipped = append(skipped, i18n.T("init.skip.no_node_web", r.WebPath))
		}
	}
	if r.Has(wizard.EnvMobile) {
		switch {
		case r.Adopted(wizard.EnvMobile):
			fmt.Println("\n" + styles.Bullet(styles.Info, i18n.T("init.mobile_adopted", r.MobilePath)))
			if r.MobileDS == config.DSMobile {
				skipped = append(skipped, i18n.T("init.skip.mobile_ds", config.DSMobile))
			}
		case pre.NodeOK:
			fmt.Println("\n" + styles.Bullet(styles.Info, i18n.T("init.mobile", r.MobilePath)))
			if err := createExpoApp(r.Root, r.MobilePath, r.MobileDS == config.DSMobile); err != nil {
				return fmt.Errorf("create mobile app: %w", err)
			}
		default:
			skipped = append(skipped, i18n.T("init.skip.no_node_mobile", r.MobilePath))
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
	fmt.Println(styles.Bullet(styles.Info, i18n.T("init.toolchain")))
	for _, c := range p.Checks {
		switch {
		case c.OK && !c.Warn:
			fmt.Println("  " + styles.Success("✓") + " " + c.Name + " " + styles.Note(c.Version))
		case c.OK && c.Warn:
			fmt.Println("  " + styles.Warn("!") + " " + c.Name + " " + styles.Note(c.Version+" — "+c.Hint))
		default:
			line := "  " + styles.Error("✗") + " " + c.Name
			if c.Hint != "" {
				line += " " + styles.Note("— "+c.Hint)
			}
			fmt.Println(line)
		}
	}
	fmt.Println()
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

// ensureGofiIgnored writes layout.IgnoreRules, upgrading a .gitignore written
// by an older release. Order matters, so the legacy lines are taken out rather
// than left to compete with the new ones.
func ensureGofiIgnored(projectRoot string) error {
	path := filepath.Join(projectRoot, ".gitignore")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var kept []string
	for line := range strings.Lines(string(existing)) {
		if trimmed := strings.TrimSpace(line); slices.Contains(layout.LegacyIgnoreRules, trimmed) || slices.Contains(layout.IgnoreRules, trimmed) {
			continue
		}
		kept = append(kept, strings.TrimRight(line, "\n"))
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	body := strings.Join(append(kept, layout.IgnoreRules...), "\n") + "\n"
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
		Graph:    config.DefaultGraph(),
		AI:       config.AI{Host: r.AIHost, Model: r.AIModel, Models: []string{r.AIModel}},
		Sources:  src,
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

func printNextSteps(r *wizard.Result) {
	fmt.Println()
	fmt.Println(styles.Bullet(styles.Done, i18n.T("init.done", r.Root)))
	fmt.Println(styles.Detail(styles.Note(i18n.T("init.source", sourceLabel(r.ClaudeSource)))))

	if len(r.Skipped) > 0 {
		fmt.Println()
		fmt.Println(styles.Bullet(styles.Warning, i18n.T("init.skipped")))
		for i, s := range r.Skipped {
			prefix := "     "
			if i == 0 {
				prefix = "  ⎿  "
			}
			fmt.Println(styles.Note(prefix) + s)
		}
	}
	// Adopting a repository leaves the code unannotated, so the graph knows the
	// calls but not which context a symbol belongs to. That bridge is built by
	// the agents, and nothing else in the output says so.
	if r.Detected.Any() {
		fmt.Println()
		fmt.Println(styles.Bullet(styles.Info, i18n.T("init.adopted")))
		fmt.Println(styles.Detail(styles.Note(i18n.T("init.adopted.hint"))))
	}

	type step struct{ cmd, why string }
	steps := []step{
		{"cd " + r.Root, ""},
		{"git status", i18n.T("init.next.review")},
		{`git add -A && git commit -m "chore: gofi init"`, ""},
	}
	if r.GitRemote == "" {
		steps = append(steps, step{"git remote add origin <url>", i18n.T("init.next.remote")})
	}
	steps = append(steps, step{"gofi h", i18n.T("init.next.help")}, step{"/" + config.AgentPD, i18n.T("init.next.chat")})
	width := 0
	for _, s := range steps {
		width = max(width, len(s.cmd))
	}

	fmt.Println()
	fmt.Println(styles.Bullet(styles.Info, i18n.T("init.next")))
	for i, s := range steps {
		prefix := "     "
		if i == 0 {
			prefix = "  ⎿  "
		}
		line := styles.Note(prefix) + styles.Value(s.cmd)
		if s.why != "" {
			line += strings.Repeat(" ", width-len(s.cmd)+2) + styles.Note("# "+s.why)
		}
		fmt.Println(line)
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

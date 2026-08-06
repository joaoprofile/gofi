package config

// DefaultTestSection returns the test runner config seeded into a freshly
// created .gofi.yaml. Tasks differ per language; hooks are empty by default.
//
// sourceRoot is the value of Project.Path (the folder that holds the source
// code, e.g. "src", "services", "backend"). For Go, it is woven into the
// `go -C <root> test` invocations so users that pick a non-default folder
// still get working tasks.
func DefaultTestSection(language, sourceRoot string) TestSection {
	if sourceRoot == "" {
		sourceRoot = "src"
	}
	switch language {
	case LanguageGo:
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {
					Desc: "Unit tests",
					Run:  "go -C " + sourceRoot + " test ./...",
				},
				"cover": {
					Desc: "Coverage in text",
					Run:  "go -C " + sourceRoot + " test -cover ./...",
				},
				"cover-html": {
					Desc: "Coverage as coverage.html",
					Run:  "go -C " + sourceRoot + " test -coverprofile=coverage.out ./... && go -C " + sourceRoot + " tool cover -html=coverage.out -o coverage.html",
				},
				"sonar": {
					Desc:  "SonarQube/SonarCloud analysis (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "gofi sonar start",
				},
			},
		}
	case LanguageRust:
		manifest := sourceRoot + "/Cargo.toml"
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {
					Desc: "Unit tests",
					Run:  "cargo test --manifest-path " + manifest,
				},
				"cover": {
					Desc: "Coverage via cargo-tarpaulin (cargo install cargo-tarpaulin)",
					Run:  "cargo tarpaulin --manifest-path " + manifest + " --out Stdout",
				},
				"cover-html": {
					Desc: "Coverage as HTML",
					Run:  "cargo tarpaulin --manifest-path " + manifest + " --out Html",
				},
				"sonar": {
					Desc:  "SonarQube/SonarCloud analysis (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "gofi sonar start",
				},
			},
		}
	case LanguageJava:
		// jacoco:report emits HTML, XML and CSV in one pass, so there is no
		// separate cover-html task to write.
		mvn := "mvn -f " + sourceRoot + "/pom.xml"
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit":  {Desc: "Unit tests", Run: mvn + " test"},
				"cover": {Desc: "Coverage via JaCoCo (target/site/jacoco/)", Run: mvn + " test jacoco:report"},
				"sonar": {
					Desc:  "SonarQube/SonarCloud analysis (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "gofi sonar start",
				},
			},
		}
	case LanguageCSharp:
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit":  {Desc: "Unit tests", Run: "dotnet test " + sourceRoot},
				"cover": {Desc: "Coverage via coverlet", Run: `dotnet test ` + sourceRoot + ` --collect:"XPlat Code Coverage"`},
				"sonar": {
					Desc:  "SonarQube/SonarCloud analysis (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "gofi sonar start",
				},
			},
		}
	case LanguageNodeJS:
		npm := "npm --prefix " + sourceRoot
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {Desc: "Unit tests", Run: npm + " test"},
				// A second reporter is needed because naming one replaces the
				// default, and a coverage run that prints nothing looks hung.
				"cover": {
					Desc: "Coverage as lcov (node --experimental-test-coverage)",
					Run: "node --test --experimental-test-coverage" +
						" --test-reporter=spec --test-reporter-destination=stdout" +
						" --test-reporter=lcov --test-reporter-destination=" + sourceRoot + "/coverage/lcov.info " + sourceRoot,
				},
				"sonar": {
					Desc:  "SonarQube/SonarCloud analysis (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "gofi sonar start",
				},
			},
		}
	case LanguagePython:
		// gofi has no Python scaffold, so there is no layout to write a real
		// command against; the placeholder keeps the YAML schema-valid.
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {
					Desc: "TODO — no Python scaffold yet; wire your own runner",
					Run:  `echo "gofi test: configure the python test task in .gofi.yaml"`,
				},
			},
		}
	}
	// No backend language (front-only) or unknown: a valid placeholder so the
	// .gofi.yaml stays schema-valid; the dev wires real test tasks later.
	return TestSection{
		Default: "unit",
		Tasks: map[string]TestTask{
			"unit": {Desc: "Tests", Run: `echo "configure test tasks in .gofi.yaml"`},
		},
	}
}

// DefaultHsec returns the Horusec block seeded into a freshly created
// .gofi.yaml. Enabled by default; users can disable per project later via
// `gofi config`.
func DefaultHsec() HsecConfig {
	return HsecConfig{
		Enabled: true,
		IgnorePaths: []string{
			"**/.gofi/**",
			"**/.claude/**",
			"**/vendor/**",
			"**/node_modules/**",
			"**/dist/**",
			"**/build/**",
			"**/target/**",
			"**/.git/**",
			"**/*.min.js",
			"**/*_test.go",
		},
		SeverityThreshold:    "HIGH",
		ReturnErrorOnFinding: true,
		OutputFormat:         "json",
		TimeoutSeconds:       600,
	}
}

// DefaultSonar returns the Sonar block seeded into a freshly created
// .gofi.yaml. Enabled by default; users can disable per project later via
// `gofi config`.
//
// projectName seeds the project key/name. The backend and every UI surface
// scope sonar.sources to the project's own source folders, and language picks
// the coverage report path so `gofi sonar` ships meaningful coverage on day one.
// The exclusions keep analysis to first-party project code: tests, mocks,
// generated code, the vendored SDK under .gofi/, and build artefacts are all
// dropped so the dashboard reflects only the code the team owns.
func DefaultSonar(projectName string, backend *Backend, surfaces ...*UISurface) SonarConfig {
	var sources []string
	if backend != nil && backend.Path != "" {
		sources = append(sources, backend.Path)
	}
	for _, s := range surfaces {
		if s != nil && s.Path != "" {
			sources = append(sources, s.Path)
		}
	}
	if len(sources) == 0 {
		sources = []string{"."}
	}

	key := projectName
	if key == "" {
		key = "gofi-project"
	}

	cfg := SonarConfig{
		Enabled:     true,
		ProjectKey:  key,
		ProjectName: key,
		Sources:     sources,
		Exclusions: []string{
			// Tests — analysed code only, not the test suite.
			"**/*_test.go",
			"**/*.test.ts",
			"**/*.test.tsx",
			"**/*.test.js",
			"**/*.spec.ts",
			"**/*.spec.tsx",
			"**/*.spec.js",
			"**/testdata/**",
			"**/__tests__/**",
			// Mocks / fakes / stubs.
			"**/mocks/**",
			"**/mock_*.go",
			"**/*_mock.go",
			"**/*.mock.ts",
			// Generated code.
			"**/*.gen.go",
			"**/*.pb.go",
			"**/*_gen.go",
			"**/*.min.js",
			// SDK + tooling checkouts and editor/agent state.
			"**/.gofi/**",
			"**/.claude/**",
			"**/vendor/**",
			"**/node_modules/**",
			// Build artefacts.
			"**/dist/**",
			"**/build/**",
			"**/target/**",
			"**/coverage.out",
			"**/coverage.html",
		},
	}
	if backend != nil {
		cfg.CoverageReport = defaultCoverageReport(backend.Language)
	}
	return cfg
}

// defaultCoverageReport returns the conventional coverage artefact path for a
// language — the file the `cover` test task produces — or "" when gofi has no
// convention for it yet.
func defaultCoverageReport(language string) string {
	switch language {
	case LanguageGo:
		return "coverage.out"
	case LanguageRust:
		return "cobertura.xml"
	case LanguageJava:
		return "target/site/jacoco/jacoco.xml"
	case LanguageCSharp:
		return "coverage.cobertura.xml"
	case LanguageNodeJS:
		return "coverage/lcov.info"
	}
	return ""
}

// DefaultAgentsRef is the source pin used as the wizard default in `gofi init`
// when the user does not customise the URL. Points at the gofi monorepo, whose
// AI harness content (skills, knowledge, sdk docs, templates, memory) lives
// under ai/. Pinned to @main until tagged releases exist; users bump via
// `gofi update`.
const DefaultAgentsRef = "github.com/joaoprofile/gofi@main"

// DefaultSDKGoRef is the default Go SDK source: the dedicated gofi-sdk-go repo,
// fetched into .gofi/gofi-sdk-go/ as the toolchain checkout (go.work). The web
// and mobile design systems ship as npm packages (gofi-ui / gofi-ui-native),
// installed by the create step — not as git sources.
const DefaultSDKGoRef = "github.com/joaoprofile/gofi-sdk-go@main"

// DefaultSourceRoot is the historical source folder name. It is the back-compat
// fallback used when migrating older configs and when a config omits a path —
// it must stay "src" so migrated projects keep pointing at their on-disk folder.
const DefaultSourceRoot = "src"

// Default folder names used by `gofi init` when the wizard input is left blank.
// Each surface defaults to a folder named after itself; the user overrides only
// if they want a different layout.
const (
	DefaultBackendPath  = "backend"
	DefaultFrontendPath = "frontend"
	DefaultMobilePath   = "mobile"
)

// DefaultOps returns the platform block seeded into a freshly created
// .gofi.yaml. It ships the first-class gofi-ops stack (OCI + Terraform + OKE +
// GitHub Actions + OCIR) so the agent has a complete `ops:` block to read from
// day one. The wizard does not ask these — the user adjusts them afterwards via
// `gofi config` or by editing .gofi.yaml.
func DefaultOps() *Ops {
	return &Ops{
		Cloud:    CloudOCI,
		IaC:      IaCTerraform,
		Target:   TargetOKE,
		CICD:     CICDGitHubActions,
		Registry: RegistryOCIR,
		Path:     DefaultOpsPath,
	}
}

// DefaultGraph returns the graph block written into a .gofi.yaml. The values are
// what an absent block already meant; what changes is that the project can now
// see them. The scan mode in particular governs what the agents may conclude
// from the graph, and a default nobody reads is a default nobody chose.
func DefaultGraph() *Graph {
	on := true
	return &Graph{Enabled: &on, Hooks: &on, Deep: false}
}

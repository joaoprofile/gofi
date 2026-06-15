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
					Desc:  "SonarScanner (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "sonar-scanner",
				},
			},
		}
	case LanguageRust:
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {
					Desc: "Unit tests",
					Run:  "cargo test",
				},
				"cover": {
					Desc: "Coverage via cargo-tarpaulin (cargo install cargo-tarpaulin)",
					Run:  "cargo tarpaulin --out Stdout",
				},
				"cover-html": {
					Desc: "Coverage as HTML",
					Run:  "cargo tarpaulin --out Html",
				},
				"sonar": {
					Desc:  "SonarScanner (requires SONAR_TOKEN, SONAR_HOST_URL)",
					Needs: []string{"cover"},
					Run:   "sonar-scanner",
				},
			},
		}
	case LanguageJava, LanguageCSharp, LanguagePython, LanguageNodeJS:
		// Preview languages: minimal placeholder task so the YAML stays
		// valid. The real scaffold will arrive when the SDK lands in
		// gofi-agents/sdk/<lang>/.
		return TestSection{
			Default: "unit",
			Tasks: map[string]TestTask{
				"unit": {
					Desc: "TODO — language scaffold not implemented yet",
					Run:  `echo "gofi test: ` + language + ` scaffold not yet implemented; edit .gofi.yaml when ready"`,
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

// DefaultSourceRoot is the source folder name used when the wizard input is
// left blank (saved to project.path).
const DefaultSourceRoot = "src"

// DefaultSources returns the source pin used for a brand new project.
func DefaultSources() Sources {
	return Sources{
		Agents: DefaultAgentsRef,
		SDK: map[string]string{
			LanguageGo: DefaultSDKGoRef,
		},
	}
}

// DefaultOps returns the platform block seeded into a freshly created
// .gofi.yaml. The wizard does not ask cloud/iac/etc — only the ops/ folder is
// created and the path recorded; the dev (or the infra spec) fills the rest.
func DefaultOps() *Ops {
	return &Ops{Path: DefaultOpsPath}
}

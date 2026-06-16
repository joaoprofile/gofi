package cli

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/sonar"
)

func newSonarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sonar",
		Short: "Run SonarQube/SonarCloud analysis against this project",
		Long: `Run the sonar-scanner static analysis against the current project.

Configuration lives under the sonar: block in .gofi.yaml. gofi renders that block
into .gofi/sonar-project.properties before each run, then invokes the sonar-scanner
binary against it. The rendered properties scope analysis to first-party project
code only — tests, mocks, generated code, the vendored SDK under .gofi/ and build
artefacts are excluded.

The server URL and token are read from the environment (SONAR_HOST_URL and
SONAR_TOKEN, typically exported from .env) and never stored in .gofi.yaml.

Without a subcommand, sonar runs the full scan (alias of 'gofi sonar start').`,
		Example: `gofi sonar
gofi sonar start
gofi sonar config
gofi sonar install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSonarStart()
		},
	}
	cmd.AddCommand(newSonarStartCmd(), newSonarConfigCmd(), newSonarInstallCmd())
	return cmd
}

func newSonarStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "start",
		Short:   "Run the analysis",
		Long:    `Render .gofi/sonar-project.properties from the sonar: block and invoke 'sonar-scanner' against the project.`,
		Example: `gofi sonar start`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSonarStart()
		},
	}
}

func newSonarConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "config",
		Short:   "Render and print sonar-project.properties without scanning",
		Long:    `Render .gofi/sonar-project.properties from the sonar: block and print it, without invoking sonar-scanner. Useful to inspect the exclusions and sources gofi derives.`,
		Example: `gofi sonar config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSonarConfig()
		},
	}
}

func newSonarInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "install",
		Short:   "Print instructions to install sonar-scanner",
		Long:    `Print platform-appropriate instructions for installing the sonar-scanner CLI. gofi does not install it automatically because there is no single official installer across platforms.`,
		Example: `gofi sonar install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSonarInstall()
		},
	}
}

func runSonarStart() error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	if !cfg.Sonar.Enabled {
		return errors.New("sonar is disabled in .gofi.yaml; set sonar.enabled to true to run")
	}
	if !sonar.IsInstalled() {
		return errors.New("sonar-scanner is not installed on PATH; run `gofi sonar install` for instructions")
	}
	configPath, err := sonar.WriteConfig(root, cfg.Sonar, backendLanguage(cfg))
	if err != nil {
		return fmt.Errorf("write sonar-project.properties: %w", err)
	}
	if missing := sonar.MissingEnv(cfg.Sonar.HostURL); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "warning: %s not set in the environment; sonar-scanner may fail to authenticate. Export them (e.g. from .env) before running.\n", strings.Join(missing, ", "))
	}
	fmt.Printf("Running sonar-scanner against %s …\n\n", root)
	if err := sonar.Run(root, configPath, os.Stdout, os.Stderr, os.Stdin); err != nil {
		return err
	}
	fmt.Println("\nAnalysis complete. Review the report on your SonarQube/SonarCloud dashboard.")
	return nil
}

func runSonarConfig() error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	configPath, err := sonar.WriteConfig(root, cfg.Sonar, backendLanguage(cfg))
	if err != nil {
		return fmt.Errorf("write sonar-project.properties: %w", err)
	}
	body, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n\n%s", configPath, body)
	return nil
}

func runSonarInstall() error {
	fmt.Println("Install the sonar-scanner CLI, then run `gofi sonar`:")
	fmt.Println()
	switch runtime.GOOS {
	case "darwin":
		fmt.Println("  brew install sonar-scanner")
	case "linux":
		fmt.Println("  # Homebrew on Linux:")
		fmt.Println("  brew install sonar-scanner")
		fmt.Println("  # or via npm:")
		fmt.Println("  npm install -g sonarqube-scanner")
		fmt.Println("  # or download the official CLI:")
		fmt.Println("  https://docs.sonarsource.com/sonarqube-server/latest/analyzing-source-code/scanners/sonarscanner/")
	case "windows":
		fmt.Println("  scoop install sonar-scanner")
		fmt.Println("  # or:")
		fmt.Println("  choco install sonarscanner-msbuild-net46")
		fmt.Println("  # or download the official CLI:")
		fmt.Println("  https://docs.sonarsource.com/sonarqube-server/latest/analyzing-source-code/scanners/sonarscanner/")
	default:
		fmt.Println("  https://docs.sonarsource.com/sonarqube-server/latest/analyzing-source-code/scanners/sonarscanner/")
	}
	fmt.Println()
	fmt.Println("Then export SONAR_HOST_URL and SONAR_TOKEN (e.g. in .env) before running `gofi sonar`.")
	return nil
}

// backendLanguage returns the project's backend language, or "" for a
// front-only project. Used to pick the sonar coverage report property.
func backendLanguage(cfg *config.GofiConfig) string {
	if cfg.Backend == nil {
		return ""
	}
	return cfg.Backend.Language
}

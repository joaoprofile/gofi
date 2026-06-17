package cli

import (
	"errors"
	"fmt"
	"io"
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
	// The rendered file documents the effective config (and is what
	// `gofi sonar config` prints); the scan itself passes the same properties
	// as -D flags so it works regardless of which sonar-scanner is installed.
	if _, err := sonar.WriteConfig(root, cfg.Sonar, backendLanguage(cfg)); err != nil {
		return fmt.Errorf("write sonar-project.properties: %w", err)
	}
	scannerArgs, err := sonar.ScannerArgs(cfg.Sonar, backendLanguage(cfg))
	if err != nil {
		return fmt.Errorf("build sonar-scanner arguments: %w", err)
	}
	warnSonarEnv(os.Stderr, cfg.Sonar.HostURL)
	fmt.Printf("Running sonar-scanner against %s …\n\n", root)
	if err := sonar.Run(root, scannerArgs, os.Stdout, os.Stderr, os.Stdin); err != nil {
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
	fmt.Println("Then set SONAR_HOST_URL and SONAR_TOKEN — gofi auto-loads a .env from the project root, so adding them there is enough.")
	return nil
}

// warnSonarEnv prints actionable guidance when the SonarQube server URL or
// token are not configured. hostURLFromConfig is the optional host pinned in
// the sonar: block, which removes the need for SONAR_HOST_URL.
func warnSonarEnv(w io.Writer, hostURLFromConfig string) {
	hostSet := strings.TrimSpace(hostURLFromConfig) != "" || strings.TrimSpace(os.Getenv("SONAR_HOST_URL")) != ""
	tokenSet := strings.TrimSpace(os.Getenv("SONAR_TOKEN")) != ""
	if hostSet && tokenSet {
		return
	}
	fmt.Fprintln(w, "warning: SonarQube is not fully configured:")
	if !hostSet {
		fmt.Fprintln(w, "  • SONAR_HOST_URL is not set — sonar-scanner defaults to SonarCloud")
		fmt.Fprintln(w, "    (https://sonarcloud.io), which additionally requires sonar.organization.")
		fmt.Fprintln(w, "    For a local/self-hosted SonarQube, export e.g. SONAR_HOST_URL=http://localhost:9000")
	}
	if !tokenSet {
		fmt.Fprintln(w, "  • SONAR_TOKEN is not set — generate one in your SonarQube under")
		fmt.Fprintln(w, "    My Account → Security → Generate Tokens, then export SONAR_TOKEN=<token>.")
	}
	fmt.Fprintln(w, "  Tip: put both in a .env at the project root — gofi loads it automatically (no `source .env` needed).")
}

// backendLanguage returns the project's backend language, or "" for a
// front-only project. Used to pick the sonar coverage report property.
func backendLanguage(cfg *config.GofiConfig) string {
	if cfg.Backend == nil {
		return ""
	}
	return cfg.Backend.Language
}

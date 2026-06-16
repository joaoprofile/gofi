// Package doctor runs environment checks and reports a status table for
// `gofi doctor`. Each check is independent and returns a Check value the
// CLI renders.
package doctor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// Status is the outcome of a single check.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusFail
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarn:
		return "warn"
	case StatusFail:
		return "fail"
	}
	return "?"
}

// Check is one row of the doctor report.
type Check struct {
	Name   string
	Status Status
	Detail string
	Hint   string
}

// Options tweaks how Run behaves; the zero value is fine for a real run.
type Options struct {
	// HTTPBaseURL overrides https://api.github.com (used in tests).
	HTTPBaseURL string
	// HTTPClient overrides the default 5s-timeout client (used in tests).
	HTTPClient *http.Client
	// Lookup mocks exec.LookPath (used in tests).
	Lookup func(string) (string, error)
}

// Run performs every check and returns the table. cfg is optional; when nil,
// the language-toolchain check is skipped.
func Run(cfg *config.GofiConfig, opts Options) []Check {
	if opts.Lookup == nil {
		opts.Lookup = exec.LookPath
	}
	if opts.HTTPBaseURL == "" {
		opts.HTTPBaseURL = "https://api.github.com"
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}

	checks := []Check{
		checkBinary(opts.Lookup, "git", true,
			"needed by Claude Code; the gofi CLI itself uses go-git"),
		checkBinary(opts.Lookup, "claude", true,
			"Claude Code CLI not found; install per https://docs.claude.com/en/docs/claude-code"),
	}
	projectRoot := ""
	hsecEnabled := false
	sonarEnabled := false
	if cfg != nil {
		projectRoot = cfg.Project.Root
		hsecEnabled = cfg.Hsec.Enabled
		sonarEnabled = cfg.Sonar.Enabled
		if cfg.Backend != nil && cfg.Backend.Language != "" {
			checks = append(checks, checkToolchain(opts.Lookup, cfg.Backend.Language))
		}
	}
	if hsecEnabled {
		checks = append(checks, checkBinary(opts.Lookup, "horusec", true,
			"hsec is enabled in .gofi.yaml; run `gofi hsec install` to install"))
	}
	if sonarEnabled {
		checks = append(checks, checkBinary(opts.Lookup, "sonar-scanner", true,
			"sonar is enabled in .gofi.yaml; run `gofi sonar install` for instructions"))
	}
	checks = append(checks, checkCache(projectRoot))
	checks = append(checks, checkGitHub(opts.HTTPClient, opts.HTTPBaseURL))
	return checks
}

func checkBinary(lookup func(string) (string, error), name string, warnOnMissing bool, hint string) Check {
	path, err := lookup(name)
	if err != nil {
		status := StatusFail
		if warnOnMissing {
			status = StatusWarn
		}
		return Check{
			Name:   name + " on PATH",
			Status: status,
			Detail: "not found",
			Hint:   hint,
		}
	}
	return Check{
		Name:   name + " on PATH",
		Status: StatusOK,
		Detail: path,
	}
}

func checkToolchain(lookup func(string) (string, error), language string) Check {
	switch language {
	case config.LanguageGo:
		return checkBinary(lookup, "go", false, "install Go from https://go.dev/dl/")
	case config.LanguageRust:
		return checkBinary(lookup, "cargo", false, "install Rust from https://rustup.rs/")
	}
	return Check{
		Name:   "toolchain",
		Status: StatusWarn,
		Detail: "unknown language: " + language,
	}
}

func checkCache(projectRoot string) Check {
	root := os.Getenv("GOFI_CACHE_DIR")
	if root == "" {
		if projectRoot == "" {
			return Check{
				Name:   "cache writable",
				Status: StatusOK,
				Detail: "(no project — cache check skipped)",
			}
		}
		root = filepath.Join(projectRoot, ".gofi", "cache")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Check{
			Name:   "cache writable",
			Status: StatusFail,
			Detail: err.Error(),
			Hint:   "set GOFI_CACHE_DIR to a writable path",
		}
	}
	probe := filepath.Join(root, ".gofi-doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return Check{
			Name:   "cache writable",
			Status: StatusFail,
			Detail: err.Error(),
			Hint:   "set GOFI_CACHE_DIR to a writable path",
		}
	}
	_ = os.Remove(probe)
	return Check{
		Name:   "cache writable",
		Status: StatusOK,
		Detail: root,
	}
}

func checkGitHub(client *http.Client, baseURL string) Check {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "HEAD", strings.TrimRight(baseURL, "/")+"/", nil)
	if err != nil {
		return Check{Name: "GitHub API reachable", Status: StatusFail, Detail: err.Error()}
	}
	req.Header.Set("User-Agent", "gofi-cli")
	resp, err := client.Do(req)
	if err != nil {
		return Check{
			Name:   "GitHub API reachable",
			Status: StatusFail,
			Detail: err.Error(),
			Hint:   "check your internet connection or proxy settings",
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return Check{
			Name:   "GitHub API reachable",
			Status: StatusWarn,
			Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
			Hint:   "GitHub may be having issues — try again later",
		}
	}
	return Check{
		Name:   "GitHub API reachable",
		Status: StatusOK,
		Detail: fmt.Sprintf("HTTP %d", resp.StatusCode),
	}
}

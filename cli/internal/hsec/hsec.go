// Package hsec wraps the Horusec SAST CLI. The gofi block `hsec:` in
// .gofi.yaml is rendered to a horusec-config.json under <project>/.gofi/
// every time `gofi hsec start` runs; the binary is then invoked against it.
package hsec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// ConfigFileName is the path (relative to projectRoot) where gofi writes the
// derived horusec-config.json before invoking horusec.
const ConfigFileName = ".gofi/horusec-config.json"

// OutputFileName is the JSON output written by horusec when run via gofi.
// Always JSON regardless of the user's chosen display format, so `gofi hsec
// list` can parse findings later.
const OutputFileName = ".gofi/horusec-output.json"

// horusecJSON mirrors the subset of horusec-config.json the CLI manipulates.
// Other fields are accepted by horusec but left at their defaults.
type horusecJSON struct {
	FilesOrPathsToIgnore            []string `json:"horusecCliFilesOrPathsToIgnore,omitempty"`
	SeveritiesToIgnore              []string `json:"horusecCliSeveritiesToIgnore,omitempty"`
	ReturnErrorIfFoundVulnerability bool     `json:"horusecCliReturnErrorIfFoundVulnerability"`
	PrintOutputType                 string   `json:"horusecCliPrintOutputType,omitempty"`
	JsonOutputFilepath              string   `json:"horusecCliJsonOutputFilepath,omitempty"`
	TimeoutInSecondsAnalysis        int      `json:"horusecCliTimeoutInSecondsAnalysis,omitempty"`
	EnableGitHistoryAnalysis        bool     `json:"horusecCliEnableGitHistoryAnalysis,omitempty"`
	EnableCommitAuthor              bool     `json:"horusecCliEnableCommitAuthor,omitempty"`
	EnableOwaspDependencyCheck      bool     `json:"horusecCliEnableOwaspDependencyCheck,omitempty"`
	HorusecApiUri                   string   `json:"horusecCliHorusecApiUri,omitempty"`
	RepositoryName                  string   `json:"horusecCliRepositoryName,omitempty"`
	TimeoutInSecondsRequest         int      `json:"horusecCliTimeoutInSecondsRequest,omitempty"`
}

// validSeverities are the levels horusec recognises, ordered from highest
// to lowest. Used to derive SeveritiesToIgnore from a threshold.
var validSeverities = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO"}

// BuildHorusecConfig derives a horusec-config.json from the user-friendly
// HsecConfig. The threshold is translated into a SeveritiesToIgnore list
// (everything strictly below the threshold is dropped), so users only think
// in terms of "show me HIGH and above".
func BuildHorusecConfig(c config.HsecConfig, outputJSONPath string) ([]byte, error) {
	if c.SeverityThreshold == "" {
		c.SeverityThreshold = "HIGH"
	}
	threshold := strings.ToUpper(c.SeverityThreshold)
	if !contains(validSeverities, threshold) {
		return nil, fmt.Errorf("invalid severity_threshold %q (expected one of %s)", c.SeverityThreshold, strings.Join(validSeverities, ", "))
	}

	ignoreSeverities := severitiesBelow(threshold)
	printType := c.OutputFormat
	if printType == "" {
		printType = "text"
	}
	returnError := c.ReturnErrorOnFinding
	if HasSuppressions(c) {
		printType = "json"
		returnError = false
	}

	hc := horusecJSON{
		FilesOrPathsToIgnore:            c.IgnorePaths,
		SeveritiesToIgnore:              ignoreSeverities,
		ReturnErrorIfFoundVulnerability: returnError,
		PrintOutputType:                 printType,
		JsonOutputFilepath:              outputJSONPath,
		TimeoutInSecondsAnalysis:        c.TimeoutSeconds,
		EnableGitHistoryAnalysis:        c.EnableGitHistory,
		EnableCommitAuthor:              c.EnableCommitAuthor,
		EnableOwaspDependencyCheck:      c.EnableOwaspDependencyCheck,
	}
	if m := c.Manager; m != nil {
		hc.HorusecApiUri = strings.TrimRight(strings.TrimSpace(m.URL), "/")
		hc.RepositoryName = m.RepositoryName
		hc.TimeoutInSecondsRequest = m.TimeoutSeconds
	}
	return json.MarshalIndent(hc, "", "  ")
}

// WriteConfig writes the derived horusec-config.json into <projectRoot>/.gofi/.
// Returns the absolute path of the written file.
func WriteConfig(projectRoot string, c config.HsecConfig) (string, error) {
	cfgPath := filepath.Join(projectRoot, ConfigFileName)
	outPath := filepath.Join(projectRoot, OutputFileName)
	body, err := BuildHorusecConfig(c, outPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

// IsInstalled reports whether the horusec binary is on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("horusec")
	return err == nil
}

// AuthTokenEnv holds the Manager repository token. It is gofi's own name on
// purpose: horusec reads horusecAuthEnv implicitly, so exporting that one
// would make every local scan publish.
const (
	AuthTokenEnv   = "HORUSEC_REPOSITORY_AUTHORIZATION"
	horusecAuthEnv = "HORUSEC_CLI_REPOSITORY_AUTHORIZATION"
)

// ErrPublishFailed marks a scan that ran but whose analysis never reached the
// Manager. horusec itself exits 0 in that case.
var ErrPublishFailed = errors.New("scan completed but publishing to the Horusec Manager failed; see the horusec log above")

// RunOptions drives one horusec invocation. Publish is honoured only together
// with a non-empty AuthToken, which reaches horusec through the child
// environment — never argv (visible in ps) nor the rendered config (tracked
// by git in some projects).
type RunOptions struct {
	ProjectRoot string
	ConfigPath  string
	Publish     bool
	AuthToken   string
	// FalsePositiveHashes are sent to horusec (-F) so the Manager records
	// the matching findings as false positives instead of vulnerabilities.
	FalsePositiveHashes []string
	// RiskAcceptHashes are sent (-R) so the Manager records them as accepted risk.
	RiskAcceptHashes []string
	Stdout           io.Writer
	Stderr           io.Writer
	Stdin            io.Reader
}

// Run invokes `horusec start` for the project — the local binary, or the
// official image against the isolated daemon — streaming its output. A
// non-zero horusec exit (findings at or above the threshold) is returned as
// is. When publishing, a send failure logged by horusec is returned as
// ErrPublishFailed unless the scan already failed on its own.
func Run(ctx context.Context, cfg config.HsecConfig, opts RunOptions) (err error) {
	if !isIsolated(cfg) && !IsInstalled() {
		return errors.New("horusec is not installed; run `gofi hsec install`")
	}
	if err := os.Remove(filepath.Join(opts.ProjectRoot, OutputFileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous horusec output: %w", err)
	}
	defer func() {
		if rmErr := removeWorkDir(opts.ProjectRoot); rmErr != nil && err == nil {
			err = rmErr
		}
	}()

	name, args := command(cfg, opts)
	cmd := exec.CommandContext(ctx, name, args...)
	// Interrupt instead of kill: horusec (and docker run, which proxies the
	// signal) then removes its work dir and tool containers on its own.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = interruptGrace
	cmd.Dir = opts.ProjectRoot
	cmd.Env = buildEnv(os.Environ(), opts)
	cmd.Stdin = opts.Stdin

	det := &publishFailureDetector{}
	if opts.Publish {
		cmd.Stdout = io.MultiWriter(opts.Stdout, det)
		cmd.Stderr = io.MultiWriter(opts.Stderr, det)
	} else {
		cmd.Stdout = opts.Stdout
		cmd.Stderr = opts.Stderr
	}
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return ErrInterrupted
	}
	det.flush()
	if runErr != nil {
		return runErr
	}
	if opts.Publish && det.failed() {
		return ErrPublishFailed
	}
	return nil
}

// ErrInterrupted marks a scan stopped by the user (Ctrl+C / SIGTERM).
var ErrInterrupted = errors.New("scan interrupted")

// interruptGrace is how long an interrupted horusec gets to clean up before
// it is killed.
const interruptGrace = 60 * time.Second

// WorkDirName is the copy of the project horusec analyses. It removes it on
// a clean exit; a killed run leaves it behind.
const WorkDirName = ".horusec"

func removeWorkDir(projectRoot string) error {
	dir := filepath.Join(projectRoot, WorkDirName)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove leftover %s (run `gofi hsec prune` if it is owned by root): %w", dir, err)
	}
	return nil
}

func buildArgs(cfg config.HsecConfig, opts RunOptions) []string {
	args := []string{"start", "--config-file-path", opts.ConfigPath, "-p", opts.ProjectRoot}
	if !cfg.UseDocker {
		args = append(args, "--disable-docker")
	}
	if isIsolated(cfg) {
		args = append(args, "-P", opts.ProjectRoot)
	}
	if len(opts.FalsePositiveHashes) > 0 {
		args = append(args, "-F", strings.Join(opts.FalsePositiveHashes, ","))
	}
	if len(opts.RiskAcceptHashes) > 0 {
		args = append(args, "-R", strings.Join(opts.RiskAcceptHashes, ","))
	}
	return args
}

func buildEnv(base []string, opts RunOptions) []string {
	env := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if strings.HasPrefix(kv, horusecAuthEnv+"=") || strings.HasPrefix(kv, AuthTokenEnv+"=") {
			continue
		}
		env = append(env, kv)
	}
	if opts.Publish && opts.AuthToken != "" {
		env = append(env, horusecAuthEnv+"="+opts.AuthToken)
	}
	return env
}

// publishFailureDetector watches horusec output for the error lines it logs
// when POST /api/analysis fails. Coupled to horusec v2 log format — the only
// signal available, since the exit code and the JSON output stay clean.
type publishFailureDetector struct {
	mu      sync.Mutex
	partial []byte
	hit     bool
}

func (d *publishFailureDetector) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.partial = append(d.partial, p...)
	for {
		i := bytes.IndexByte(d.partial, '\n')
		if i < 0 {
			break
		}
		d.scan(string(d.partial[:i]))
		d.partial = d.partial[i+1:]
	}
	return len(p), nil
}

func (d *publishFailureDetector) flush() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.partial) > 0 {
		d.scan(string(d.partial))
		d.partial = nil
	}
}

func (d *publishFailureDetector) scan(line string) {
	if !strings.Contains(line, "level=error") {
		return
	}
	if strings.Contains(line, "{ERROR_HTTP}") || strings.Contains(line, "/api/analysis") || strings.Contains(line, "sending analysis") {
		d.hit = true
	}
}

func (d *publishFailureDetector) failed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.hit
}

// ResolveAuthToken reads the Manager repository token from the environment.
func ResolveAuthToken() (string, error) {
	tok := strings.TrimSpace(os.Getenv(AuthTokenEnv))
	if tok == "" {
		return "", fmt.Errorf("%s is not set; export the Horusec repository token to publish", AuthTokenEnv)
	}
	return tok, nil
}

// CheckManager probes GET {baseURL}/api/health and fails unless it answers 2xx.
func CheckManager(baseURL string, timeout time.Duration) error {
	healthURL := strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/health"
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(healthURL)
	if err != nil {
		return fmt.Errorf("manager healthcheck failed at %s: %w", healthURL, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("manager healthcheck failed at %s: HTTP %d", healthURL, resp.StatusCode)
	}
	return nil
}

// DockerAvailable reports whether the Docker daemon answers `docker info`.
func DockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil
}

// ErrNoOutput and ErrAnalysisIncomplete stop a partial or missing analysis
// from reading as a clean one.
var (
	ErrNoOutput           = errors.New("horusec produced no output file; the analysis did not complete")
	ErrAnalysisIncomplete = errors.New("horusec analysis did not complete")
)

// VerifyAnalysis checks the status horusec wrote. A missing output is an error
// only when required (gofi needs the JSON to decide the verdict).
func VerifyAnalysis(projectRoot string, required bool) error {
	body, err := os.ReadFile(filepath.Join(projectRoot, OutputFileName))
	if errors.Is(err, os.ErrNotExist) {
		if required {
			return ErrNoOutput
		}
		return nil
	}
	if err != nil {
		return err
	}
	var doc struct {
		Status string `json:"status"`
		Errors string `json:"errors"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("parse horusec output: %w", err)
	}
	if doc.Status == "success" {
		return nil
	}
	detail := doc.Errors
	if len(detail) > 500 {
		detail = detail[:500] + " …"
	}
	return fmt.Errorf("%w (status %q): %s", ErrAnalysisIncomplete, doc.Status, detail)
}

// Finding is the simplified row gofi shows in `gofi hsec list`. It maps to
// a single entry in horusec's JSON output.
type Finding struct {
	ID       string
	RuleID   string
	Severity string
	File     string
	Line     string
	Code     string
	Details  string
	Hash     string
}

// ParseFindings reads the horusec-output.json (written by `Run`) and returns
// a flat list of findings. Returns (nil, nil) when the file is absent so the
// caller can decide between "no findings yet" and "no scan ran yet".
func ParseFindings(projectRoot string) ([]Finding, error) {
	path := filepath.Join(projectRoot, OutputFileName)
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var doc struct {
		AnalysisVulnerabilities []struct {
			Vulnerabilities struct {
				VulnerabilityID string `json:"vulnerabilityID"`
				RuleID          string `json:"rule_id"`
				Severity        string `json:"severity"`
				File            string `json:"file"`
				Line            string `json:"line"`
				Code            string `json:"code"`
				Details         string `json:"details"`
				VulnHash        string `json:"vulnHash"`
			} `json:"vulnerabilities"`
		} `json:"analysisVulnerabilities"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse horusec output: %w", err)
	}
	out := make([]Finding, 0, len(doc.AnalysisVulnerabilities))
	for _, e := range doc.AnalysisVulnerabilities {
		v := e.Vulnerabilities
		out = append(out, Finding{
			ID:       v.VulnerabilityID,
			RuleID:   v.RuleID,
			Severity: v.Severity,
			File:     v.File,
			Line:     v.Line,
			Code:     v.Code,
			Details:  v.Details,
			Hash:     v.VulnHash,
		})
	}
	return out, nil
}

// InstallScript runs the official horusec install script (Linux/macOS).
// Windows is unsupported by the script — caller must surface alternatives.
func InstallScript(stdout, stderr io.Writer) error {
	if runtime.GOOS == "windows" {
		return errors.New("automatic install via the official script is POSIX-only; install via winget/scoop/brew or download from https://github.com/ZupIT/horusec/releases")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		return errors.New("bash is required to run the horusec install script")
	}
	url := "https://raw.githubusercontent.com/ZupIT/horusec/main/deployments/scripts/install.sh"
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`set -e; curl -fsSL %q | bash -s latest`, url))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// severitiesBelow returns every severity strictly below threshold, ready to
// drop into HorusecCliSeveritiesToIgnore. INFO is always ignored.
func severitiesBelow(threshold string) []string {
	thIdx := indexOf(validSeverities, threshold)
	if thIdx < 0 {
		return []string{"INFO"}
	}
	out := make([]string, 0)
	for i := thIdx + 1; i < len(validSeverities); i++ {
		out = append(out, validSeverities[i])
	}
	return out
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// Package hsec wraps the Horusec SAST CLI. The gofi block `hsec:` in
// .gofi.yaml is rendered to a horusec-config.json under <project>/.gofi/
// every time `gofi hsec start` runs; the binary is then invoked against it.
package hsec

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

	hc := horusecJSON{
		FilesOrPathsToIgnore:            c.IgnorePaths,
		SeveritiesToIgnore:              ignoreSeverities,
		ReturnErrorIfFoundVulnerability: c.ReturnErrorOnFinding,
		PrintOutputType:                 printType,
		JsonOutputFilepath:              outputJSONPath,
		TimeoutInSecondsAnalysis:        c.TimeoutSeconds,
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

// Run invokes `horusec start -c <config> -p <projectRoot>`, streaming its
// stdout/stderr. Returns the underlying exit error; callers should surface
// non-zero exits to the user (horusec exits non-zero by design when
// vulnerabilities are found and ReturnErrorOnFinding is true).
func Run(projectRoot string, configPath string, stdout, stderr io.Writer, stdin io.Reader) error {
	if !IsInstalled() {
		return errors.New("horusec is not installed; run `gofi hsec install`")
	}
	cmd := exec.Command("horusec", "start", "-c", configPath, "-p", projectRoot, "--disable-docker")
	cmd.Dir = projectRoot
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	return cmd.Run()
}

// Finding is the simplified row gofi shows in `gofi hsec list`. It maps to
// a single entry in horusec's JSON output.
type Finding struct {
	ID       string
	Severity string
	File     string
	Line     string
	Details  string
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
				Severity        string `json:"severity"`
				File            string `json:"file"`
				Line            string `json:"line"`
				Details         string `json:"details"`
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
			Severity: v.Severity,
			File:     v.File,
			Line:     v.Line,
			Details:  v.Details,
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

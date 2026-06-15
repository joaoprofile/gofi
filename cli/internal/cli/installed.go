package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// installedFile records the resolved SHA of the last successful install or
// update of the gofi-agents repo for this project. From v2.4 it lives at
// <projectRoot>/.gofi/installed.yaml (was previously .claude/.installed.yaml).
const installedFileName = "installed.yaml"

// installedRecord tracks the resolved SHA of every external source pinned in
// this project. Agents is the gofi-agents commit; SDK is keyed by language and
// records the gofi-sdk-<lang> commit currently extracted at .gofi/gofi-sdk-<lang>/.
// SDK entries let `gofi update` and `gofi init` skip re-extracting the SDK
// when the live checkout already matches the resolved ref.
type installedRecord struct {
	Agents string            `yaml:"agents,omitempty"`
	SDK    map[string]string `yaml:"sdk,omitempty"`
}

func installedPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".gofi", installedFileName)
}

func loadInstalled(projectRoot string) installedRecord {
	var rec installedRecord
	b, err := os.ReadFile(installedPath(projectRoot))
	if err != nil {
		return rec
	}
	_ = yaml.Unmarshal(b, &rec)
	return rec
}

func saveInstalled(projectRoot string, rec installedRecord) error {
	b, err := yaml.Marshal(&rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(installedPath(projectRoot)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(installedPath(projectRoot), b, 0o644)
}

func readInstalledSha(projectRoot string) string {
	return loadInstalled(projectRoot).Agents
}

func writeInstalledSha(projectRoot, agentsSha string) error {
	rec := loadInstalled(projectRoot)
	rec.Agents = agentsSha
	return saveInstalled(projectRoot, rec)
}

func readInstalledSDKSha(projectRoot, language string) string {
	return loadInstalled(projectRoot).SDK[language]
}

func writeInstalledSDKSha(projectRoot, language, sdkSha string) error {
	rec := loadInstalled(projectRoot)
	if rec.SDK == nil {
		rec.SDK = map[string]string{}
	}
	rec.SDK[language] = sdkSha
	return saveInstalled(projectRoot, rec)
}

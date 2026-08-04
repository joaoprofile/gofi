package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// initBlocks are the blocks a current `gofi init` writes. Backfill seeds them in
// memory and MissingBlocks reports which the file still lacks; keeping the names
// in one place is what stops the repair and the report from disagreeing.
//
// `graph:` is not among them: its absence already means every default, so a
// config written by the current CLI does not carry it either.
var initBlocks = []string{"hsec", "sonar", "test", "ops"}

// Backfill seeds the blocks a project scaffolded by an older CLI never got, and
// reports which ones it seeded. Blocks already in the file are left exactly as
// the team wrote them: this closes gaps, it does not reconcile choices.
//
// Load calls it before validating, because these blocks are required and a
// project created before they existed would otherwise be unable to run any gofi
// command at all — including the update that would have repaired it.
//
// Absence is inferred from one identifying field per block, because an absent
// block and a zero-valued one are indistinguishable once unmarshalled — a
// `hsec:` the team disabled on purpose reads the same as one never written, and
// only a field the defaults always fill (a severity threshold, a project key)
// tells the two apart.
func Backfill(cfg *GofiConfig) []string {
	var seeded []string

	var language, sourceRoot string
	if cfg.Backend != nil {
		language, sourceRoot = cfg.Backend.Language, cfg.Backend.Path
	}

	if cfg.Test.Default == "" && len(cfg.Test.Tasks) == 0 {
		cfg.Test = DefaultTestSection(language, sourceRoot)
		seeded = append(seeded, "test")
	}
	if cfg.Hsec.SeverityThreshold == "" {
		cfg.Hsec = DefaultHsec()
		seeded = append(seeded, "hsec")
	}
	if cfg.Sonar.ProjectKey == "" {
		surfaces := make([]*UISurface, 0, 2)
		for _, s := range cfg.UISurfaces() {
			surfaces = append(surfaces, s.Surface)
		}
		cfg.Sonar = DefaultSonar(cfg.Project.Name, cfg.Backend, surfaces...)
		seeded = append(seeded, "sonar")
	}
	if cfg.Ops == nil {
		cfg.Ops = DefaultOps()
		seeded = append(seeded, "ops")
	}
	// The picker reads this list; leaving it empty makes the panel fall back to
	// its built-ins. The active model is the one choice the project has already
	// made, so it is the honest seed — the rest arrive commented out from
	// AI.MarshalYAML.
	if len(cfg.AI.Models) == 0 && cfg.AI.Model != "" {
		cfg.AI.Models = []string{cfg.AI.Model}
		seeded = append(seeded, "ai.models")
	}

	return seeded
}

// MissingBlocks reports which of the blocks a current `gofi init` writes are
// absent from the file on disk, plus "ai.models" when the model list was never
// pinned. Load backfills in memory, so the parsed struct cannot tell an absent
// block from a written one — only the raw document can, and it is the raw
// document the user has to end up with.
//
// A file that is missing or unreadable reports nothing: that is a different
// problem, and one the caller already has to handle.
func MissingBlocks(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var missing []string
	for _, key := range initBlocks {
		if _, ok := doc[key]; !ok {
			missing = append(missing, key)
		}
	}
	if ai, ok := doc["ai"].(map[string]any); ok {
		if _, has := ai["models"]; !has {
			missing = append(missing, "ai.models")
		}
	}
	return missing
}

// FileVersion reports the schema version recorded in the file on disk. Load
// migrates in memory and leaves cfg.Version already current, so it is the only
// way to tell whether the file itself still has to be rewritten. A file that is
// missing, unreadable or has no version reads as 0.
func FileVersion(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var doc struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return 0
	}
	return doc.Version
}

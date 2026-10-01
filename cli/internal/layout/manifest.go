package layout

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ManifestSchema versions the manifest file.
const ManifestSchema = "gofi-index-manifest/v1"

// Manifest records the last build of each target on this machine. It answers
// what a status check cannot read off the index itself: when it was built, by
// which gofi, and with which options — the options a freshness check has to
// repeat to compare like with like.
type Manifest struct {
	Schema string `json:"schema"`
	Code   *Build `json:"code,omitempty"`
	Docs   *Build `json:"docs,omitempty"`
}

// Build is one target's last build.
type Build struct {
	At   time.Time `json:"at"`
	Tool string    `json:"tool"`
	// Fingerprint identifies the sources the build read, for a target whose
	// freshness is checked by comparing it. Empty for one checked another way.
	Fingerprint string        `json:"fingerprint,omitempty"`
	Options     *BuildOptions `json:"options,omitempty"`
	Counts      Counts        `json:"counts"`
}

// BuildOptions are the scan options a code build ran with.
type BuildOptions struct {
	WithTests bool     `json:"tests,omitempty"`
	Exclude   []string `json:"exclude,omitempty"`
	MaxFileKB int      `json:"max_file_kb,omitempty"`
}

// Counts sizes what a build produced. Each target fills the fields it has.
type Counts struct {
	Scopes   int `json:"scopes,omitempty"`
	Docs     int `json:"docs,omitempty"`
	Sections int `json:"sections,omitempty"`
	Files    int `json:"files,omitempty"`
	Nodes    int `json:"nodes"`
	Edges    int `json:"edges"`
}

// LoadManifest reads the manifest of a project. A project never built on this
// machine has none, and gets an empty one rather than an error.
func LoadManifest(root string) (*Manifest, error) {
	b, err := os.ReadFile(filepath.Join(root, ManifestFile))
	if errors.Is(err, fs.ErrNotExist) {
		return &Manifest{Schema: ManifestSchema}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Record stores one target's build and writes the manifest.
func Record(root, target string, build Build) error {
	m, err := LoadManifest(root)
	if err != nil {
		// A manifest that cannot be read is only a record; a fresh one loses
		// nothing a rebuild would not restore.
		m = &Manifest{}
	}
	m.Schema = ManifestSchema
	switch target {
	case TargetCode:
		m.Code = &build
	case TargetDocs:
		m.Docs = &build
	}
	path := filepath.Join(root, ManifestFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

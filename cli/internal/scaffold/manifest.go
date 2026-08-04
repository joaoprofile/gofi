package scaffold

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ManifestFile records the sha256 of every file the CLI wrote into the project,
// keyed by a slash-separated path relative to the project root. It is the
// baseline an update compares against: a file still matching its recorded hash
// is the CLI's to refresh, one that drifted carries the team's edits and is
// never overwritten.
const ManifestFile = ".gofi/installed-files.yaml"

// Manifest maps a project-relative path to the sha256 of the content installed
// there.
type Manifest map[string]string

// LoadManifest reads the recorded hashes. A project installed before the
// manifest existed simply yields an empty one — see preserver.write for how an
// unrecorded file is treated.
func LoadManifest(projectRoot string) Manifest {
	m := Manifest{}
	b, err := os.ReadFile(filepath.Join(projectRoot, ManifestFile))
	if err != nil {
		return m
	}
	_ = yaml.Unmarshal(b, &m)
	return m
}

// merge folds the entries into the manifest on disk. Installs happen in several
// passes (agents, SDK, each UI surface) and each one only knows its own files,
// so a pass must not drop what the others recorded.
func (m Manifest) merge(projectRoot string) error {
	if len(m) == 0 {
		return nil
	}
	merged := LoadManifest(projectRoot)
	for k, v := range m {
		merged[k] = v
	}
	b, err := yaml.Marshal(merged)
	if err != nil {
		return err
	}
	path := filepath.Join(projectRoot, ManifestFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// managedTrees are the .claude entries an update refreshes from upstream.
// memory/, institutional/ and knowledge/ are absent on purpose: those are
// preserved wholesale by the install itself.
var managedTrees = []string{"CLAUDE.md", "skills", "templates", "scripts", "sdk"}

// tunedTrees are the managed trees a project is expected to adjust — the
// per-language and per-surface docs, where design tokens and house rules live.
// A file there that the CLI has no record of writing is assumed to be the
// team's, because guessing wrong deletes work the CLI cannot bring back.
var tunedTrees = []string{"sdk"}

// PreservedFiles lists the managed files an update would keep instead of
// overwriting, so the plan can say so before anything is written.
func PreservedFiles(projectRoot string) []string {
	baseline := LoadManifest(projectRoot)
	claude := filepath.Join(projectRoot, ".claude")
	var kept []string
	for _, tree := range managedTrees {
		root := filepath.Join(claude, tree)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel := relSlash(projectRoot, p)
			raw, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if recorded, tracked := baseline[rel]; tracked {
				if recorded != hashBytes(raw) {
					kept = append(kept, rel)
				}
				return nil
			}
			if underAny(rel, tunedTrees) {
				kept = append(kept, rel)
			}
			return nil
		})
	}
	sort.Strings(kept)
	return kept
}

// preserver decides whether a managed file may be overwritten and records what
// it wrote, so the next update can tell the CLI's own output apart from the
// team's edits.
type preserver struct {
	root  string
	base  Manifest
	next  Manifest
	guard bool
	kept  []string
}

func newPreserver(projectRoot string, mode InstallMode) *preserver {
	p := &preserver{root: projectRoot, next: Manifest{}, guard: mode == InstallUpdate}
	if p.guard {
		p.base = LoadManifest(projectRoot)
	}
	return p
}

// write puts content at target unless the file already there carries changes
// the CLI did not make. Reports whether it wrote.
func (p *preserver) write(target string, content []byte) (bool, error) {
	rel := relSlash(p.root, target)
	existing, err := os.ReadFile(target)
	if err == nil {
		switch {
		case bytes.Equal(existing, content):
			p.record(rel, content)
			return false, nil
		case p.guard && p.keeps(rel, existing):
			p.kept = append(p.kept, rel)
			return false, nil
		case p.base[rel] != hashBytes(existing):
			// About to replace content the CLI cannot reproduce from upstream.
			p.backup(rel, existing)
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return false, err
	}
	p.record(rel, content)
	return true, nil
}

// keeps reports whether the file on disk belongs to the team. A recorded file
// that still matches its hash is untouched and may be refreshed; one that
// drifted is theirs. Without a record the answer depends on where it sits —
// see tunedTrees.
func (p *preserver) keeps(rel string, existing []byte) bool {
	recorded, tracked := p.base[rel]
	if !tracked {
		return underAny(rel, tunedTrees)
	}
	return recorded != hashBytes(existing)
}

func (p *preserver) record(rel string, content []byte) {
	if p.next == nil {
		return
	}
	p.next[rel] = hashBytes(content)
}

// backup keeps the replaced content under .gofi/backup/ so an overwrite the
// user did not want stays recoverable. Best effort: failing to save a copy must
// not fail the install.
func (p *preserver) backup(rel string, content []byte) {
	if p.root == "" {
		return
	}
	target := filepath.Join(p.root, ".gofi", "backup", runStamp(), filepath.FromSlash(rel))
	if os.MkdirAll(filepath.Dir(target), 0o755) != nil {
		return
	}
	_ = os.WriteFile(target, content, 0o644)
}

// save persists what this pass installed. Errors are the caller's to report;
// a missing manifest only costs the next update its baseline.
func (p *preserver) save() error {
	if p.root == "" {
		return nil
	}
	return p.next.merge(p.root)
}

// runStamp names the backup folder of the current run, so every file a single
// command replaces lands together.
var runStamp = sync.OnceValue(func() string {
	return time.Now().UTC().Format("20060102-150405")
})

func relSlash(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func underAny(rel string, trees []string) bool {
	for _, t := range trees {
		if strings.HasPrefix(rel, ".claude/"+t+"/") {
			return true
		}
	}
	return false
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

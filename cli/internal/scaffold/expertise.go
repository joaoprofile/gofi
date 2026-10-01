package scaffold

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/expertise"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// expertiseDir is the name of the packs' tree, both in the source (ai/) and
// under the project's agents folder.
const expertiseDir = "expertise"

// walkExpertise invokes fn for every file of every pack the source ships, rel
// relative to the expertise tree.
func walkExpertise(agentsFS fs.FS, srcRoot string, fn func(rel string, content []byte) error) error {
	dir := path.Join(srcRoot, "ai", expertiseDir)
	if !dirExistsInFS(agentsFS, dir) {
		return nil
	}
	return fs.WalkDir(agentsFS, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Base(p) == GitkeepName {
			return err
		}
		content, err := fs.ReadFile(agentsFS, p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		return fn(strings.TrimPrefix(p, dir+"/"), content)
	})
}

// installExpertise writes the source's packs into the project's expertise
// area through the preserver: a file the team edited is kept on update.
func installExpertise(agentsFS fs.FS, srcRoot, projectRoot string, p *preserver) ([]string, error) {
	var created []string
	err := walkExpertise(agentsFS, srcRoot, func(rel string, content []byte) error {
		target := layout.Expertise().Abs(projectRoot, rel)
		written, err := p.write(target, content)
		if written {
			created = append(created, target)
		}
		return err
	})
	return created, err
}

// InstallExpertiseContent refreshes the project's expertise packs from the
// source. Like the skills, they are gofi's: an update rewrites them, keeps a
// file the team edited (InstallUpdate) or puts upstream back with a backup
// (InstallReset).
func InstallExpertiseContent(agentsFS fs.FS, srcRoot, projectRoot string, mode InstallMode) (created []string, err error) {
	if err := os.MkdirAll(layout.Expertise().Abs(projectRoot), 0o755); err != nil {
		return nil, err
	}
	p := newPreserver(projectRoot, mode)
	defer func() {
		if saveErr := p.save(); saveErr != nil && err == nil {
			err = fmt.Errorf("record installed files: %w", saveErr)
		}
	}()
	return installExpertise(agentsFS, srcRoot, projectRoot, p)
}

// PlanExpertiseUpdate lists what InstallExpertiseContent would create or
// change, writing nothing.
func PlanExpertiseUpdate(agentsFS fs.FS, srcRoot, projectRoot string) ([]Change, error) {
	var changes []Change
	p := newPreserver(projectRoot, InstallUpdate)
	add := planAdder(projectRoot, p, &changes)
	err := walkExpertise(agentsFS, srcRoot, func(rel string, content []byte) error {
		return add(filepath.Join(expertiseDir, filepath.FromSlash(rel)), content)
	})
	return changes, err
}

// seededCopy is one file older versions of gofi seeded into knowledge/.
type seededCopy struct {
	rel     string // project-relative
	content []byte
	// untouched says the manifest recorded it and it still matches.
	untouched bool
	// replaced says the file that replaced it is installed.
	replaced bool
}

// seededCopies lists the seeded copies still in knowledge/, in path order.
func seededCopies(projectRoot string) []seededCopy {
	base := LoadManifest(projectRoot)
	olds := make([]string, 0, len(expertise.Moved))
	for old := range expertise.Moved {
		olds = append(olds, old)
	}
	sort.Strings(olds)
	var out []seededCopy
	for _, old := range olds {
		path := layout.Knowledge().Abs(projectRoot, old)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		rel := relSlash(projectRoot, path)
		recorded, ok := base[rel]
		_, err = os.Stat(filepath.Join(projectRoot, layout.Home(), filepath.FromSlash(expertise.Moved[old])))
		out = append(out, seededCopy{rel: rel, content: b, untouched: ok && recorded == hashBytes(b), replaced: err == nil})
	}
	return out
}

// PlanSeededKnowledge splits the seeded copies still in knowledge/ into those
// an expertise update retires — untouched since gofi wrote them — and those it
// keeps: edited, so the team's learning now, or with no record, because
// guessing wrong deletes work the CLI cannot bring back.
func PlanSeededKnowledge(projectRoot string) (retire, kept []string) {
	for _, c := range seededCopies(projectRoot) {
		if c.untouched {
			retire = append(retire, c.rel)
		} else {
			kept = append(kept, c.rel)
		}
	}
	return retire, kept
}

// RetireSeededKnowledge removes the copies PlanSeededKnowledge retires, each
// backed up first and only once the file that replaced it is installed.
func RetireSeededKnowledge(projectRoot string) ([]string, error) {
	p := newPreserver(projectRoot, InstallReset)
	var removed []string
	for _, c := range seededCopies(projectRoot) {
		if !c.untouched || !c.replaced {
			continue
		}
		p.backup(c.rel, c.content)
		if err := os.Remove(filepath.Join(projectRoot, filepath.FromSlash(c.rel))); err != nil {
			return removed, err
		}
		removed = append(removed, c.rel)
	}
	return removed, nil
}

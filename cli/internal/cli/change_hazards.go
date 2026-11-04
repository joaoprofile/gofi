package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// A changeHazard is something about the project's changes the person must
// hear before they are lost to it: a change deleted without being archived,
// one in a folder gofi does not read, one away from its context's folder.
type changeHazard struct {
	Kind   string // lost | unseen | misplaced
	Change string
	Dir    string // where it is, or was
	Commit string // lost: the commit that deleted it; "" when not committed yet
	Target string // misplaced: where it belongs
}

const (
	hazardLost      = "lost"
	hazardUnseen    = "unseen"
	hazardMisplaced = "misplaced"
)

// changeHazards reads the open changes, the folders under specs/ and git's
// history of them for what can be lost without anyone noticing.
func changeHazards(root string, changes []contract.Change) []changeHazard {
	open := map[string]bool{}
	for _, c := range changes {
		open[c.ID] = true
	}
	archived := archivedChanges(root)
	var out []changeHazard
	for _, d := range deletedProposals(root) {
		id := path.Base(path.Dir(d.path))
		if open[id] || archived[id] {
			continue
		}
		out = append(out, changeHazard{Kind: hazardLost, Change: id, Dir: path.Dir(d.path), Commit: d.commit})
	}
	for _, dir := range proposalDirs(root) {
		id := path.Base(dir)
		if found, ok := contract.FindChange(root, id); ok && found == dir {
			continue
		}
		out = append(out, changeHazard{Kind: hazardUnseen, Change: id, Dir: dir})
	}
	for _, c := range changes {
		dir, ok := contract.FindChange(root, c.ID)
		if !ok || len(c.Contexts) != 1 {
			continue
		}
		if want := contract.NewChangeDir(c.ID, c.Contexts); dir != want {
			out = append(out, changeHazard{Kind: hazardMisplaced, Change: c.ID, Dir: dir, Target: want})
		}
	}
	return out
}

// archivedChanges are the ids in every changes/archive/<date>-<id>/ folder.
func archivedChanges(root string) map[string]bool {
	ids := map[string]bool{}
	_ = filepath.WalkDir(layout.Specs().Abs(root), func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || filepath.Base(filepath.Dir(p)) != contract.ArchiveDir {
			return nil
		}
		if name := d.Name(); len(name) > 11 && name[10] == '-' {
			ids[name[11:]] = true
		}
		return nil
	})
	return ids
}

// proposalDirs are the folders under specs/ holding a proposal of a change,
// the archive aside, relative to the root.
func proposalDirs(root string) []string {
	var dirs []string
	_ = filepath.WalkDir(layout.Specs().Abs(root), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && d.Name() == contract.ArchiveDir {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "proposal.md" || !isChangeProposal(p) {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(p))
		if err == nil {
			dirs = append(dirs, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(dirs)
	return dirs
}

var reTipoMudanca = regexp.MustCompile(`(?m)^tipo:\s*mudan[cç]a\s*$`)

func isChangeProposal(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	head, _, _ := strings.Cut(string(b), "\n---")
	return reTipoMudanca.MatchString(head)
}

type deletedProposal struct{ path, commit string }

var reCommit = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// deletedProposals are the proposals of changes git saw deleted: on disk and
// not committed yet, or by a commit — the latest one for each path.
func deletedProposals(root string) []deletedProposal {
	specs := layout.Specs().Path()
	isProposal := func(p string) bool {
		return strings.HasSuffix(p, "/proposal.md") && strings.Contains(p, "/"+contract.ChangesDir+"/") && !strings.Contains(p, "/"+contract.ArchiveDir+"/")
	}
	seen := map[string]bool{}
	var out []deletedProposal
	if b, err := gitOut(root, "ls-files", "--deleted", "--", specs); err == nil {
		for _, p := range strings.Fields(string(b)) {
			if isProposal(p) && !seen[p] {
				seen[p] = true
				out = append(out, deletedProposal{path: p})
			}
		}
	}
	b, err := gitOut(root, "log", "--diff-filter=D", "--name-only", "--format=%h", "--", specs)
	if err != nil {
		return out
	}
	commit := ""
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
		case reCommit.MatchString(line):
			commit = line
		case isProposal(line) && !seen[line]:
			seen[line] = true
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(line))); err == nil {
				continue
			}
			out = append(out, deletedProposal{path: line, commit: commit})
		}
	}
	return out
}

func gitOut(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.quotePath=false"}, args...)...)
	cmd.Dir = dir
	return cmd.Output()
}

// printChangeHazards says each hazard with what restores or fixes it.
func printChangeHazards(out io.Writer, hs []changeHazard) {
	if len(hs) == 0 {
		return
	}
	fmt.Fprintln(out)
	for _, h := range hs {
		switch h.Kind {
		case hazardLost:
			if h.Commit == "" {
				fmt.Fprintln(out, "  ! "+i18n.T("change.hazard.lost.disk", h.Change, h.Dir, h.Dir))
			} else {
				fmt.Fprintln(out, "  ! "+i18n.T("change.hazard.lost.commit", h.Change, h.Commit, h.Commit, h.Dir))
			}
		case hazardUnseen:
			fmt.Fprintln(out, "  ! "+i18n.T("change.hazard.unseen", h.Dir))
		case hazardMisplaced:
			fmt.Fprintln(out, "  ! "+i18n.T("change.hazard.misplaced", h.Change, h.Dir, h.Target))
		}
	}
}

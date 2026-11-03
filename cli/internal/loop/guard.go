package loop

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

// skipDirs are folders no source of the project lives in.
var skipDirs = map[string]bool{".git": true, ".gofi": true, ".claude": true, "node_modules": true, "vendor": true}

// guarded are the source files of a tree that a role must not change: the
// tests, for a role that implements; the code, for the role that writes the
// tests; all of them, for a role that writes no code — the plan, the triage,
// the delivery. Who writes the code never writes the test that judges it
// (0002, D12, D17). Nil when the role's stage guards nothing or the pack
// says nothing of the language's files.
func (r *Runner) guarded(rl role.Role) func(string) bool {
	if r.IsTest == nil || r.IsSource == nil {
		return nil
	}
	switch rl.Contract.Stage {
	case "implement":
		// The repair is not guarded: it fixes a test that does not compile,
		// without changing what it proves.
		return r.IsTest
	case "test":
		return func(p string) bool { return r.IsSource(p) && !r.IsTest(p) }
	case "plan", "triage", "deliver", "learn":
		return r.IsSource
	}
	return nil
}

// snapshot reads the files of a tree that keep says are guarded.
func snapshot(dir string, keep func(string) bool) map[string][]byte {
	out := map[string][]byte{}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if keep(rel) {
			if b, err := os.ReadFile(p); err == nil {
				out[rel] = b
			}
		}
		return nil
	})
	return out
}

// restore puts back what a turn changed among the guarded files — content
// as it was, files it made removed — and reports which.
func restore(dir string, before map[string][]byte, keep func(string) bool) []string {
	after := snapshot(dir, keep)
	var changed []string
	for rel, b := range after {
		if old, ok := before[rel]; !ok || !bytes.Equal(old, b) {
			changed = append(changed, rel)
		}
	}
	for rel := range before {
		if _, ok := after[rel]; !ok {
			changed = append(changed, rel)
		}
	}
	sort.Strings(changed)
	for _, rel := range changed {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if old, ok := before[rel]; ok {
			_ = os.MkdirAll(filepath.Dir(abs), 0o755)
			_ = os.WriteFile(abs, old, 0o644)
		} else {
			_ = os.Remove(abs)
		}
	}
	return changed
}

func joinFiles(files []string) string { return strings.Join(files, ", ") }

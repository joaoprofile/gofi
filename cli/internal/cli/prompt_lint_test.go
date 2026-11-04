package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const aiDir = "../../../ai"

var (
	reInlineGofi = regexp.MustCompile("`(gofi [^`]+)`")
	reFenceGofi  = regexp.MustCompile(`(?m)^\s*(?:\$ )?(gofi [^\n#|]+)`)
	reSlash      = regexp.MustCompile("(?:^|[\\s(`'\"])(/gofi-[a-z-]+)")
	reWord       = regexp.MustCompile(`^[a-z][a-z-]*$`)
)

// markdowns are the prompt layer gofi installs: every Markdown under ai/,
// the SDK's generated API reference aside.
func markdowns(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(aiDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "api" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(aiDir, p)
			out[filepath.ToSlash(rel)] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// The skills told agents to run gofi graph explain and /gofi-full long after
// both were gone: an agent following them failed, or improvised. Every gofi
// command the prompt layer names is one this CLI has.
func TestEveryGofiCommandTheSkillsNameExists(t *testing.T) {
	root := NewRoot()
	bad := map[string][]string{}
	for file, text := range markdowns(t) {
		var cites []string
		for _, m := range reInlineGofi.FindAllStringSubmatch(text, -1) {
			cites = append(cites, m[1])
		}
		for _, block := range strings.Split(text, "```")[1:] {
			for _, m := range reFenceGofi.FindAllStringSubmatch(block, -1) {
				cites = append(cites, m[1])
			}
		}
		for _, cite := range cites {
			var words []string
			for _, w := range strings.Fields(cite)[1:] {
				if !reWord.MatchString(w) {
					break
				}
				words = append(words, w)
			}
			if len(words) == 0 {
				continue
			}
			cmd, rest, err := root.Find(words)
			if err != nil || cmd == root {
				bad[file] = append(bad[file], cite)
				continue
			}
			// A word left over where the command takes subcommands, and none
			// matched, is a subcommand that does not exist.
			if len(rest) > 0 && len(cmd.Commands()) > 0 && cmd.Args == nil && !cmd.Runnable() {
				bad[file] = append(bad[file], cite)
			}
		}
	}
	report(t, bad, "names a gofi command this CLI does not have")
}

// The internal roles are dispatched by the loop, never invoked by a slash:
// the prompt layer names as /gofi-<name> only the entry skills.
func TestOnlyEntrySkillsAreNamedAsSlashCommands(t *testing.T) {
	entry := map[string]bool{}
	dirs, err := os.ReadDir(filepath.Join(aiDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(aiDir, "skills", d.Name(), "contract.yaml"))
		if err != nil || !strings.Contains(string(b), "visibility: internal") {
			entry["/"+d.Name()] = true
		}
	}
	bad := map[string][]string{}
	for file, text := range markdowns(t) {
		for _, m := range reSlash.FindAllStringSubmatch(text, -1) {
			if !entry[m[1]] {
				bad[file] = append(bad[file], m[1])
			}
		}
	}
	report(t, bad, "names as a slash command what is not an entry skill")
}

func report(t *testing.T, bad map[string][]string, what string) {
	t.Helper()
	files := make([]string, 0, len(bad))
	for f := range bad {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		seen := map[string]bool{}
		var uniq []string
		for _, c := range bad[f] {
			if !seen[c] {
				seen[c] = true
				uniq = append(uniq, c)
			}
		}
		t.Errorf("ai/%s %s: %s", f, what, strings.Join(uniq, "; "))
	}
}

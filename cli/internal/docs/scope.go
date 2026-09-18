package docs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Scope limits a build to what one commit touched. A nil *Scope is a full
// build. The pre-commit hook uses it so a commit carries the index of the
// documents its author changed and nothing else: rebuilding every INDEX.md on
// every commit rewrote contexts nobody touched and left them dirty in every
// developer's tree.
type Scope struct {
	// Dirs are the specs/ and prd/ folders holding a staged document.
	Dirs map[string]bool
	// Documents is true when a document of any corpus is staged (JSON index).
	Documents bool
	// Knowledge is true when a file of the knowledge areas is staged.
	Knowledge bool
	// Anything is true when the commit stages any file (code index).
	Anything bool
}

// StagedScope derives the scope from the paths a commit stages, relative to
// the project root. Deleted and renamed paths count too: the folder that lost
// a document needs its index rewritten as much as the one that gained it.
func StagedScope(paths []string) *Scope {
	s := &Scope{Dirs: map[string]bool{}}
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		s.Anything = true
		if !strings.HasSuffix(p, ".md") || path.Base(p) == IndexMarkdown {
			continue
		}
		if strings.HasPrefix(p, ".claude/knowledge/") || strings.HasPrefix(p, ".claude/sdk/") {
			s.Knowledge = true
			continue
		}
		corpus := corpusOf(p)
		if corpus == "" || strings.Contains(p, "/diagrams/") {
			continue
		}
		s.Documents = true
		if corpus == "specs" || corpus == "prd" {
			s.Dirs[path.Dir(p)] = true
		}
	}
	return s
}

// idle reports a scope with nothing to index.
func (s *Scope) idle(withCode bool) bool {
	return !s.Documents && !s.Knowledge && len(s.Dirs) == 0 && !(withCode && s.Anything)
}

func (s *Scope) touchesCorpus(corpus string) bool {
	for d := range s.Dirs {
		if strings.HasPrefix(d, corpus+"/") || d == corpus {
			return true
		}
	}
	return false
}

// writeIfChanged writes only when the content differs, so an unchanged index
// is not rewritten (and does not show up as modified). Paths actually written
// are appended to written.
func writeIfChanged(path string, data []byte, written *[]string) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if written != nil {
		*written = append(*written, path)
	}
	return nil
}

var rootFooterRe = regexp.MustCompile(`^_\d+ documentos em \d+ contextos, \d+ pastas\._$`)

type rootTable struct {
	head   []string
	rows   map[string]string
	order  []string
	footer string
}

// parseRootIndex splits a corpus router into its fixed head, one row per
// context and the footer. ok is false for anything that is not a router this
// tool wrote, which is then left for a full build.
func parseRootIndex(content string) (t rootTable, ok bool) {
	if !strings.Contains(content, generatedMark) {
		return t, false
	}
	t.rows = map[string]string{}
	inTable := false
	for _, line := range strings.Split(strings.TrimRight(content, "\n"), "\n") {
		switch {
		case !inTable && strings.HasPrefix(line, "|---"):
			t.head = append(t.head, line)
			inTable = true
		case !inTable:
			t.head = append(t.head, line)
		case strings.HasPrefix(line, "| "):
			ctx := rowContext(line)
			t.rows[ctx] = line
			t.order = append(t.order, ctx)
		case rootFooterRe.MatchString(line):
			t.footer = line
		}
	}
	return t, inTable
}

func rowContext(row string) string {
	parts := strings.Split(row, "|")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

var rowPlaceRe = regexp.MustCompile("`[^`]+`")

// mergeRootIndex keeps every row of the current router, in its current order,
// except those of the affected contexts: a replaced row keeps its position, a
// removed context loses its row, and a new one is inserted before the first
// row that sorts after it — no other line moves. The footer is recounted from
// the rows kept, so it stays consistent with the table it closes.
func mergeRootIndex(current, fresh string, affected map[string]bool) (string, error) {
	cur, ok := parseRootIndex(current)
	if !ok {
		return "", errors.New("current router is not a generated index")
	}
	next, ok := parseRootIndex(fresh)
	if !ok {
		return "", errors.New("fresh router is not a generated index")
	}
	rows := map[string]string{}
	var ctxs []string
	for _, ctx := range cur.order {
		row := cur.rows[ctx]
		if affected[ctx] {
			fresh, ok := next.rows[ctx]
			if !ok {
				continue
			}
			row = fresh
		}
		rows[ctx] = row
		ctxs = append(ctxs, ctx)
	}
	var added []string
	for ctx := range affected {
		if _, kept := rows[ctx]; kept {
			continue
		}
		if row, ok := next.rows[ctx]; ok {
			rows[ctx] = row
			added = append(added, ctx)
		}
	}
	sort.Strings(added)
	for _, ctx := range added {
		i := len(ctxs)
		for j, c := range ctxs {
			if c > ctx {
				i = j
				break
			}
		}
		ctxs = append(ctxs[:i], append([]string{ctx}, ctxs[i:]...)...)
	}

	docsTotal, places := 0, map[string]bool{}
	var b strings.Builder
	for _, line := range cur.head {
		b.WriteString(line + "\n")
	}
	for _, ctx := range ctxs {
		row := rows[ctx]
		b.WriteString(row + "\n")
		parts := strings.Split(row, "|")
		if len(parts) > 4 {
			var n int
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%d", &n)
			docsTotal += n
			for _, p := range rowPlaceRe.FindAllString(parts[4], -1) {
				places[p] = true
			}
		}
	}
	fmt.Fprintf(&b, "\n_%d documentos em %d contextos, %d pastas._\n", docsTotal, len(ctxs), len(places))
	return b.String(), nil
}

// affectedContexts are the contexts whose router row a scoped build rewrites:
// those with a document in a staged folder now, and those whose current row
// points at one (a context that moved out or disappeared).
func affectedContexts(current string, entries map[string][]indexEntry, scope *Scope) map[string]bool {
	out := map[string]bool{}
	for ctx, group := range entries {
		for _, e := range group {
			if scope.Dirs[e.dir] {
				out[ctx] = true
			}
		}
	}
	if cur, ok := parseRootIndex(current); ok {
		for ctx, row := range cur.rows {
			parts := strings.Split(row, "|")
			if len(parts) < 5 {
				continue
			}
			for _, p := range rowPlaceRe.FindAllString(parts[4], -1) {
				dir := strings.TrimSuffix(strings.Trim(p, "`"), "/"+IndexMarkdown)
				if scope.Dirs[dir] {
					out[ctx] = true
				}
			}
		}
	}
	return out
}

package retrieval

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// TextQuery is a search for exact text rather than for meaning: an error
// message, a config key, a SQL fragment — what a ranked search cannot promise
// to find and grep can.
type TextQuery struct {
	Pattern    string
	Regex      bool // Pattern is a regular expression; otherwise it is literal
	IgnoreCase bool
	Areas      []string // none means every area
	Limit      int      // matches returned; the rest are only counted
}

// TextMatch is one line that matched, and the unit it sits in — the symbol or
// section a reader would open to understand it. That is what grep cannot say.
type TextMatch struct {
	Area string `json:"area"`
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
	// In is the innermost symbol or section holding the line, when the index
	// has one there.
	In *Hit `json:"in,omitempty"`
}

// TextResult is every match found, the first Limit of them in full.
type TextResult struct {
	Matches []TextMatch `json:"matches"`
	Total   int         `json:"total"` // matching lines, returned or not
	Files   int         `json:"files"` // files with at least one match
}

// Size limits. A text search reads whole files, so the ones no reader greps —
// bundles, dumps, lockfiles — are left out rather than dominating the output.
const (
	maxGrepFileBytes = 1 << 20
	maxGrepLineRunes = 240
)

// ErrEmptyPattern is returned for a search with nothing to look for.
var ErrEmptyPattern = errors.New("empty pattern")

// Text searches the project's files for a pattern, line by line.
//
// The files are the ones the project is made of: what git tracks or would
// track, the documents the index covers even where git ignores them, and the
// code trees the graph reads from outside git — the vendored SDK. Build output
// and dependencies never enter, because git already ignores them.
func (e *Engine) Text(q TextQuery) (*TextResult, error) {
	if strings.TrimSpace(q.Pattern) == "" {
		return nil, ErrEmptyPattern
	}
	expr := q.Pattern
	if !q.Regex {
		expr = regexp.QuoteMeta(expr)
	}
	if q.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, a := range q.Areas {
		allowed[a] = true
	}
	var files []string
	for _, f := range e.searchableFiles() {
		if len(allowed) == 0 || allowed[areaOfFile(f)] {
			files = append(files, f)
		}
	}

	perFile := make([][]TextMatch, len(files))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.NumCPU() {
		wg.Go(func() {
			for i := range next {
				perFile[i] = e.grepFile(files[i], re)
			}
		})
	}
	for i := range files {
		next <- i
	}
	close(next)
	wg.Wait()

	res := &TextResult{Matches: []TextMatch{}}
	spans := e.spansByPath()
	for _, ms := range perFile {
		if len(ms) == 0 {
			continue
		}
		res.Files++
		res.Total += len(ms)
		for _, m := range ms {
			if q.Limit > 0 && len(res.Matches) >= q.Limit {
				break
			}
			m.In = innermost(spans[m.Path], m.Line)
			res.Matches = append(res.Matches, m)
		}
	}
	return res, nil
}

func (e *Engine) grepFile(rel string, re *regexp.Regexp) []TextMatch {
	full := filepath.Join(e.root, filepath.FromSlash(rel))
	fi, err := os.Stat(full)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxGrepFileBytes {
		return nil
	}
	b, err := os.ReadFile(full)
	if err != nil || bytes.IndexByte(b[:min(len(b), 512)], 0) >= 0 {
		return nil
	}
	var out []TextMatch
	area := areaOfFile(rel)
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), maxGrepFileBytes)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if re.MatchString(line) {
			out = append(out, TextMatch{Area: area, Path: rel, Line: n, Text: clip(strings.TrimSpace(line))})
		}
	}
	return out
}

// searchableFiles lists the project's files, project-relative and slash
// separated, sorted so the same search always reads in the same order.
func (e *Engine) searchableFiles() []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		if f != "" && !seen[f] && !strings.HasPrefix(f, ".git/") {
			seen[f] = true
			out = append(out, f)
		}
	}
	listed := gitFiles(e.root)
	if listed == nil {
		listed = walkFiles(e.root, e.root)
	}
	for _, f := range listed {
		add(f)
	}
	// Documents the index covers but git ignores: .claude/ is often local.
	for _, u := range e.units {
		if u.hit.Kind != KindSymbol {
			add(u.hit.Path)
		}
	}
	// Code trees the graph reads from outside git, the vendored SDK above all:
	// the text a call into the SDK dead-ends at is text an agent must reach.
	if e.ws != nil {
		for _, s := range e.ws.Index.Scopes {
			dir := filepath.Join(e.root, filepath.FromSlash(s.Root))
			if s.Root == "" || s.Root == "." || !strings.HasPrefix(s.Root, ".gofi/") {
				continue
			}
			for _, f := range walkFiles(e.root, dir) {
				add(f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// gitFiles asks git for the tracked files plus the untracked ones it does not
// ignore. nil means this is not a repository.
func gitFiles(root string) []string {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range strings.Split(string(b), "\x00") {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// skipDirs are never searched when there is no git to say what is ignored.
var skipDirs = map[string]bool{".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true}

func walkFiles(root, dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] || (d.Name() == ".gofi" && p != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

// areaOfFile files a path under a document area, or under code when it is in
// none: the code area is everything the project holds outside its documents.
func areaOfFile(p string) string {
	if a := docs.AreaOf(p); a != "" {
		return a
	}
	return AreaCode
}

// spansByPath groups the index's units by file, for finding what holds a line.
func (e *Engine) spansByPath() map[string][]Hit {
	out := map[string][]Hit{}
	for _, u := range e.units {
		out[u.hit.Path] = append(out[u.hit.Path], u.hit)
	}
	return out
}

// innermost is the smallest unit whose lines include line: a method rather
// than its type, a subsection rather than its chapter.
func innermost(spans []Hit, line int) *Hit {
	var best *Hit
	for i := range spans {
		s := &spans[i]
		if line < s.Start || line > max(s.End, s.Start) {
			continue
		}
		if best == nil || s.End-s.Start < best.End-best.Start {
			best = s
		}
	}
	if best == nil {
		return nil
	}
	h := *best
	h.Score = 0
	return &h
}

func clip(s string) string {
	if r := []rune(s); len(r) > maxGrepLineRunes {
		return string(r[:maxGrepLineRunes]) + "…"
	}
	return s
}

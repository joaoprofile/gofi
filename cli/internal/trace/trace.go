// Package trace links a contract's requirements to the code that realizes
// them and the tests that prove them (0002, D10): the code says which
// requirement a symbol implements — gofi:implements REQ-03 — and a test which
// scenario it tests — gofi:tests REQ-03.1. A requirement no symbol implements,
// or a scenario no test tests, is a gap: the change is not done, whatever the
// audit thinks of the rest.
package trace

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// The directives, as the roles write them in a comment of any language.
const (
	Implements = "implements"
	Tests      = "tests"
)

// Mark is one directive found in the code.
type Mark struct {
	Kind    string `json:"kind"` // implements | tests
	Context string `json:"context,omitempty"`
	ID      string `json:"id"` // REQ-03, REQ-03.1
	File    string `json:"file"`
	Line    int    `json:"line"`
}

var (
	reDirective = regexp.MustCompile(`gofi:(implements|tests)\b([^\n]*)`)
	reID        = regexp.MustCompile(`(?:\b([a-z0-9][a-z0-9_-]*)/)?\b(REQ-\d+(?:\.\d+)?)\b`)
)

// skipped are folders no source of the project is in.
var skipped = map[string]bool{".git": true, "node_modules": true, "vendor": true, ".gofi": true, "dist": true, "build": true}

// Scan reads the directives of the source files under src, a folder of the
// project root: a mark's context is the one it names (pontos/REQ-03), else
// the one the pack says the file's code is in. A gofi:tests counts only in a
// test file — production code cannot vouch for itself.
func Scan(root, src string, lp *pack.Pack) ([]Mark, error) {
	if lp == nil {
		return nil, nil
	}
	base := filepath.Join(root, filepath.FromSlash(src))
	var out []Mark
	err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != base && (skipped[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		rel = filepath.ToSlash(rel)
		if !lp.IsSource(rel) {
			return nil
		}
		marks, err := scanFile(p, rel, lp)
		if err != nil {
			return err
		}
		for i := range marks {
			marks[i].File = filepath.ToSlash(filepath.Join(src, rel))
		}
		out = append(out, marks...)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

func scanFile(path, rel string, lp *pack.Pack) ([]Mark, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	test := lp.IsTest(rel)
	ctx := lp.ContextOf(rel)
	var out []Mark
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		for _, m := range reDirective.FindAllStringSubmatch(sc.Text(), -1) {
			if m[1] == Tests && !test {
				continue
			}
			for _, id := range reID.FindAllStringSubmatch(m[2], -1) {
				c := id[1]
				if c == "" {
					c = ctx
				}
				out = append(out, Mark{Kind: m[1], Context: c, ID: id[2], Line: n})
			}
		}
	}
	return out, sc.Err()
}

// Row is one line of the matrix: a requirement, with the symbols that
// implement it, or one of its scenarios, with the tests that test it.
type Row struct {
	Context  string `json:"context"`
	ID       string `json:"id"`
	Title    string `json:"title"`
	Scenario bool   `json:"scenario,omitempty"`
	// Doc and Line are where the contract says it.
	Doc   string `json:"doc"`
	Line  int    `json:"line"`
	Marks []Mark `json:"marks"`
}

// Gap reports whether nothing in the code answers the row.
func (r Row) Gap() bool { return len(r.Marks) == 0 }

// Need is the directive the row asks for.
func (r Row) Need() string {
	if r.Scenario {
		return Tests
	}
	return Implements
}

// Source is one contract document whose requirements are traced.
type Source struct {
	Context      string
	Doc          string
	Requirements []docs.Requirement
}

// Matrix lines up the requirements of the sources with the marks: each
// requirement with what implements it — a mark on it or on one of its
// scenarios —, each scenario with what tests it.
func Matrix(sources []Source, marks []Mark) []Row {
	var rows []Row
	for _, src := range sources {
		for _, r := range src.Requirements {
			req := Row{Context: src.Context, ID: r.ID, Title: r.Title, Doc: src.Doc, Line: r.Line, Marks: []Mark{}}
			for _, m := range marks {
				if m.Kind == Implements && m.Context == src.Context && (m.ID == r.ID || docs.Parent(m.ID) == r.ID) {
					req.Marks = append(req.Marks, m)
				}
			}
			rows = append(rows, req)
			for _, s := range r.Scenarios {
				row := Row{Context: src.Context, ID: s.ID, Title: s.Title, Scenario: true, Doc: src.Doc, Line: s.Line, Marks: []Mark{}}
				for _, m := range marks {
					if m.Kind == Tests && m.Context == src.Context && m.ID == s.ID {
						row.Marks = append(row.Marks, m)
					}
				}
				rows = append(rows, row)
			}
		}
	}
	for i := range rows {
		slices.SortFunc(rows[i].Marks, func(a, b Mark) int {
			if a.File != b.File {
				return strings.Compare(a.File, b.File)
			}
			return a.Line - b.Line
		})
	}
	return rows
}

// Gaps are the rows nothing in the code answers.
func Gaps(rows []Row) []Row {
	var out []Row
	for _, r := range rows {
		if r.Gap() {
			out = append(out, r)
		}
	}
	return out
}

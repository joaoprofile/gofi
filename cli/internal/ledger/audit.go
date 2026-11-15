package ledger

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The ledger says that a read was expensive. This says why, by opening the
// document the agent read and checking the three things the retrieval
// protocol depends on: front matter with keywords, so the document is found
// without being opened; "## " headings, so a section can be read instead of
// the file; and an index of the corpus, so discovery starts from a cheap
// listing. A document missing those cannot be read cheaply, and no prompting
// fixes that — so each finding carries the request that does: one file, one
// thing to do, handed to the agent only when the person asks for it. Nothing
// here rewrites a document: the agent owns the project's templates, and it is
// the one that has to live with the result.

// corpus is a body of documents the retrieval protocol governs.
type corpus struct {
	area  layout.Area
	label string // what a document of it is called: i18n key
	index string // its index, relative to the project; empty when it has none
}

func corpora() []corpus {
	return []corpus{
		{layout.Specs(), "ledger.corpus.spec", layout.Specs().Path("INDEX.md")},
		{layout.PRD(), "ledger.corpus.prd", layout.PRD().Path("INDEX.md")},
		{layout.Contexts(), "ledger.corpus.context", ""},
		{layout.Institutional(), "ledger.corpus.institutional", ""},
	}
}

func corpusOf(rel string) (corpus, bool) {
	for _, c := range corpora() {
		if strings.HasPrefix(rel, c.area.Path()+"/") {
			return c, true
		}
	}
	return corpus{}, false
}

const (
	smallDoc     = 120 // lines under which reading a document whole costs little
	oversizedDoc = 700 // lines over which even a well-sectioned one is worth splitting
	maxScan      = 6   // findings a scan surfaces at once: a wall of them is noise
	maxScanFiles = 400 // files walked per corpus
)

// doc is what a retrieval decision depends on in a document.
type doc struct {
	lines       int
	frontMatter bool
	keywords    []string
	headings    int
}

var reHeading = regexp.MustCompile(`^##\s+\S`)

// inspect reads a document's structure; false when it cannot be read.
func inspect(root, rel string) (doc, bool) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return doc{}, false
	}
	defer f.Close()
	var d doc
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inFront := false
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		d.lines++
		switch {
		case d.lines == 1 && line == "---":
			inFront = true
		case inFront && line == "---":
			inFront, d.frontMatter = false, true
		case inFront:
			if v, ok := strings.CutPrefix(line, "keywords:"); ok {
				d.keywords = keywords(v)
			}
		case reHeading.MatchString(line):
			d.headings++
		}
	}
	return d, sc.Err() == nil
}

// keywords reads "keywords: [a, b]" and a bare comma-separated list alike.
func keywords(value string) []string {
	var out []string
	for _, k := range strings.Split(strings.Trim(strings.TrimSpace(value), "[]"), ",") {
		if k = strings.Trim(strings.TrimSpace(k), `"'`); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// Review checks the documents the conversation read against the protocol:
// what is costing tokens right now. Short documents are left alone —
// structure would not have saved anything.
func Review(root string, rows []Row) []Finding {
	if root == "" {
		return nil
	}
	type read struct {
		n      int
		scoped bool
	}
	reads, order := map[string]*read{}, []string{}
	for _, row := range rows {
		if row.Name != "Read" || row.Target == "" {
			continue
		}
		if _, ok := corpusOf(row.Target); !ok {
			continue
		}
		if reads[row.Target] == nil {
			reads[row.Target] = &read{scoped: true}
			order = append(order, row.Target)
		}
		reads[row.Target].n++
		reads[row.Target].scoped = reads[row.Target].scoped && row.Scoped
	}

	var out []Finding
	seen := map[string]bool{}
	for _, rel := range order {
		c, _ := corpusOf(rel)
		d, ok := inspect(root, rel)
		if !ok || d.lines < smallDoc {
			continue
		}
		seen[c.area.Path()] = true
		r := reads[rel]
		switch {
		case !d.frontMatter || len(d.keywords) == 0:
			out = append(out, noFrontMatter(rel, d, c, "ledger.audit.front_matter_read"))
		case d.headings == 0:
			out = append(out, noHeadings(rel, d, c, "ledger.audit.headings_read"))
		case d.lines > oversizedDoc && !r.scoped:
			out = append(out, Finding{Level: Info, Text: i18n.T("ledger.audit.oversized", rel, d.lines, d.headings),
				Fix: &Fix{i18n.T("ledger.fix.split"), i18n.T("ledger.prompt.split", i18n.T(c.label), rel, d.lines, d.headings)}})
		case r.n > 1:
			out = append(out, Finding{Level: Info, Text: i18n.T("ledger.audit.reopened", rel, r.n),
				Fix: &Fix{i18n.T("ledger.fix.keywords"), i18n.T("ledger.prompt.keywords", i18n.T(c.label), rel, strings.Join(d.keywords, ", "))}})
		}
	}
	for _, c := range corpora() {
		if seen[c.area.Path()] && c.index != "" && !exists(root, c.index) {
			out = append(out, noIndex(c))
		}
	}
	return out
}

// Scan walks the corpora for documents that cannot be read cheaply, before
// any is opened: the same problems Review finds once they cost, found while
// they are still free. skip are the texts already reported, so a document is
// not named twice.
func Scan(root string, skip []Finding) []Finding {
	if root == "" {
		return nil
	}
	reported := map[string]bool{}
	for _, f := range skip {
		if f.Fix != nil {
			reported[f.Fix.Prompt] = true
		}
	}
	var out []Finding
	add := func(f Finding) bool {
		if !reported[f.Fix.Prompt] {
			reported[f.Fix.Prompt] = true
			out = append(out, f)
		}
		return len(out) < maxScan
	}
	for _, c := range corpora() {
		docs := markdown(root, c.area.Path())
		if len(docs) == 0 {
			continue
		}
		if c.index != "" && !exists(root, c.index) && !add(noIndex(c)) {
			return out
		}
		for _, rel := range docs {
			d, ok := inspect(root, rel)
			if !ok || d.lines < smallDoc {
				continue
			}
			var f Finding
			switch {
			case !d.frontMatter || len(d.keywords) == 0:
				f = noFrontMatter(rel, d, c, "ledger.audit.front_matter")
			case d.headings == 0:
				f = noHeadings(rel, d, c, "ledger.audit.headings")
			default:
				continue
			}
			if !add(f) {
				return out
			}
		}
	}
	return out
}

func noFrontMatter(rel string, d doc, c corpus, key string) Finding {
	missing := i18n.T("ledger.audit.no_front_matter")
	if d.frontMatter {
		missing = i18n.T("ledger.audit.no_keywords")
	}
	return Finding{Level: Warn, Text: i18n.T(key, rel, d.lines, missing),
		Fix: &Fix{i18n.T("ledger.fix.front_matter"), i18n.T("ledger.prompt.front_matter", i18n.T(c.label), rel, protocol(), layout.Templates().Path(), d.lines, missing)}}
}

func noHeadings(rel string, d doc, c corpus, key string) Finding {
	return Finding{Level: Warn, Text: i18n.T(key, rel, d.lines),
		Fix: &Fix{i18n.T("ledger.fix.headings"), i18n.T("ledger.prompt.headings", i18n.T(c.label), rel, d.lines, layout.Templates().Path())}}
}

func noIndex(c corpus) Finding {
	return Finding{Level: Warn, Text: i18n.T("ledger.audit.no_index", c.index, c.area.Path()),
		Fix: &Fix{i18n.T("ledger.fix.index"), i18n.T("ledger.prompt.index", c.index, c.area.Path())}}
}

// protocol is where the project keeps the retrieval protocol the fixes follow.
func protocol() string {
	return layout.Expertise().Path("harness-protocols", "rag-retrieval.md")
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// markdown lists the documents of a corpus, relative to the project, its
// index left out. A huge tree is cut short rather than stalling whoever asks.
func markdown(root, prefix string) []string {
	var out []string
	base := filepath.Join(root, filepath.FromSlash(prefix))
	_ = filepath.WalkDir(base, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if strings.HasPrefix(e.Name(), ".") && path != base {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(e.Name(), ".md") || e.Name() == "INDEX.md" {
			return nil
		}
		if rel, err := filepath.Rel(root, path); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		if len(out) >= maxScanFiles {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(out)
	return out
}

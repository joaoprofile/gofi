package docs

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

var (
	reDocPath     = regexp.MustCompile(`(?:specs|prd)/[a-z0-9_]+/[a-z0-9_.-]+\.md`)
	reWikilink    = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	reCreateTable = regexp.MustCompile(`(?i)CREATE TABLE (?:IF NOT EXISTS )?"?([a-z_][a-z0-9_]*)`)
	// The directive a package carries to say which context it belongs to. It is
	// a DECLARED edge between code and documents — stronger than inferring one
	// from a table name, and the only edge available to a context that owns no
	// table of its own.
	reContextTag = regexp.MustCompile(`(?m)^\s*(?://|#|--)\s*gofi:context\s+([a-z0-9_-]+)`)
)

// Builder walks a project's corpora and writes the index and the graph.
type Builder struct {
	Root string
	// Language selects which .claude/sdk/<lang>/ folders the knowledge index
	// covers. Empty indexes only the cross-agent layer.
	Language string
	// WithCode links entities to the source files that mention them. It shells
	// out to git grep once per entity, so it is the slow half of a build; the
	// git hook path turns it off.
	WithCode bool
}

// Build writes every derived artifact: the JSON the tool reads, and the
// markdown a person and an agent read.
//
// One command on purpose. They all go stale on the same edit, and four separate
// commands means three of them are forgotten — leaving an index that lies,
// which is worse than no index at all: without one an agent knows it does not
// know, with a stale one it points confidently at the wrong place.
func (b *Builder) Build() (*Index, *Graph, []Drift, error) {
	docs, bodies, err := b.scan()
	if err != nil {
		return nil, nil, nil, err
	}
	idx := &Index{Schema: IndexSchema, Docs: docs}
	g := b.graph(docs, bodies)

	out := filepath.Join(b.Root, OutDir)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, nil, nil, err
	}
	if err := writeJSON(filepath.Join(out, IndexFile), idx); err != nil {
		return nil, nil, nil, err
	}
	if err := writeJSON(filepath.Join(out, GraphFile), g); err != nil {
		return nil, nil, nil, err
	}

	var drift []Drift
	for _, corpus := range []string{"specs", "prd"} {
		d, err := WriteIndexes(b.Root, corpus)
		if err != nil {
			return nil, nil, nil, err
		}
		drift = append(drift, d...)
	}
	if err := WriteKnowledgeIndex(b.Root, b.Language); err != nil {
		return nil, nil, nil, err
	}
	return idx, g, drift, nil
}

// walkCorpus visits every indexable document of one corpus.
//
// Generated indexes and extracted diagrams are skipped: INDEX.md is derived
// from these very documents, so indexing it would make the index describe
// itself, and diagrams/ holds PlantUML lifted out of the specs.
func walkCorpus(root, corpus string, visit func(path string, fm Frontmatter, body []string)) error {
	base := filepath.Join(root, corpus)
	if _, err := os.Stat(base); err != nil {
		return nil
	}
	sep := string(os.PathSeparator)
	return filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		if d.Name() == IndexMarkdown || strings.Contains(path, sep+"diagrams"+sep) {
			return nil
		}
		fm, body, err := ParseFile(path)
		if err != nil || len(fm) == 0 {
			return nil
		}
		visit(path, fm, body)
		return nil
	})
}

// scan reads every document once, returning the index entries and the raw
// bodies the graph needs for cited paths and wikilinks.
func (b *Builder) scan() ([]Doc, map[string][]string, error) {
	var docs []Doc
	bodies := map[string][]string{}
	for _, corpus := range Corpora {
		err := walkCorpus(b.Root, corpus, func(path string, fm Frontmatter, body []string) {
			rel := relPath(b.Root, path)
			docs = append(docs, Doc{
				Path:     rel,
				Context:  fm.Get("contexto"),
				Kind:     kindOf(fm, corpus),
				Status:   fm.Get("status"),
				Title:    titleOf(body),
				Facets:   facetsOf(fm),
				Sections: sectionsOf(body),
			})
			bodies[rel] = body
		})
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, bodies, nil
}

func kindOf(fm Frontmatter, corpus string) string {
	if k := fm.Get("tipo"); k != "" {
		return k
	}
	if strings.Contains(corpus, "memory") {
		return "memoria"
	}
	return ""
}

func titleOf(body []string) string {
	for _, l := range body {
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(l, "# "))
		}
	}
	return ""
}

// FacetFields are the controlled-vocabulary fields, in the order they are
// concatenated into the searchable blob.
var FacetFields = []string{"entidades", "operacoes", "marketplaces", "keywords"}

func facetsOf(fm Frontmatter) string {
	var terms []string
	for _, f := range FacetFields {
		terms = append(terms, fm.List(f)...)
	}
	return strings.Join(terms, " ")
}

// sectionsOf records every ## and ### with the line range it spans.
//
// Headings are the best retrieval surface a spec has: they are written in
// natural language, which is the language the question arrives in. Keywords are
// labels; headings are sentences.
func sectionsOf(body []string) []Section {
	var marks []int
	for i, l := range body {
		if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
			marks = append(marks, i)
		}
	}
	secs := make([]Section, 0, len(marks))
	for n, i := range marks {
		end := len(body)
		if n+1 < len(marks) {
			end = marks[n+1]
		}
		secs = append(secs, Section{
			Heading: strings.TrimSpace(strings.TrimLeft(body[i], "# ")),
			Start:   i + 1,
			End:     end + 1,
		})
	}
	return secs
}

// graph materializes the links that already exist in the corpus but were only
// ever implicit.
func (b *Builder) graph(docs []Doc, bodies map[string][]string) *Graph {
	g := &Graph{Schema: GraphSchema, Nodes: map[string]Node{}}
	byStem := map[string]string{}
	contexts := map[string]bool{}
	for _, d := range docs {
		byStem[stemOf(d.Path)] = d.Path
		if d.Context != "" {
			contexts[d.Context] = true
		}
	}

	seen := map[Edge]bool{}
	add := func(from, to, kind string) {
		if from == "" || to == "" || from == to {
			return
		}
		e := Edge{from, to, kind}
		if !seen[e] {
			seen[e] = true
			g.Edges = append(g.Edges, e)
		}
	}
	resolve := func(target string) string {
		t := strings.TrimSpace(target)
		t = strings.Trim(t, "`")
		if i := strings.Index(t, "|"); i >= 0 {
			t = strings.TrimSpace(t[:i])
		}
		if contexts[t] {
			return PrefixCtx + t
		}
		if p, ok := byStem[t]; ok {
			return p
		}
		base := stemOf(t)
		if p, ok := byStem[base]; ok {
			return p
		}
		if contexts[base] {
			return PrefixCtx + base
		}
		return ""
	}

	entities := map[string]bool{}
	for _, d := range docs {
		fm, _, err := ParseFile(filepath.Join(b.Root, filepath.FromSlash(d.Path)))
		if err != nil {
			continue
		}
		g.Nodes[d.Path] = Node{
			Kind: NodeDoc, Context: d.Context, Corpus: corpusOf(d.Path),
			Status: d.Status, Version: fm.Get("versao"),
		}
		if d.Context != "" {
			id := PrefixCtx + d.Context
			if _, ok := g.Nodes[id]; !ok {
				g.Nodes[id] = Node{Kind: NodeContext}
			}
			add(d.Path, id, EdgeBelongs)
		}
		for _, field := range []string{"prd", "spec", "prds", "specs"} {
			for _, target := range valuesOf(fm, field) {
				add(d.Path, resolve(target), EdgeDeclares)
			}
		}
		body := strings.Join(bodies[d.Path], "\n")
		for _, m := range uniq(reDocPath.FindAllString(body, -1)) {
			add(d.Path, resolve(m), EdgeCites)
		}
		for _, m := range reWikilink.FindAllStringSubmatch(body, -1) {
			add(d.Path, resolve(m[1]), EdgeWikilink)
		}
		for _, e := range fm.List("entidades") {
			id := PrefixEntity + e
			if _, ok := g.Nodes[id]; !ok {
				g.Nodes[id] = Node{Kind: NodeEntity}
			}
			entities[e] = true
			add(d.Path, id, EdgeTouches)
		}
	}

	if b.WithCode {
		names := make([]string, 0, len(entities))
		for e := range entities {
			names = append(names, e)
		}
		sort.Strings(names)
		byEntity, tagged := b.codeIndex(names)
		// Iterated over the sorted names, not over the map: Go randomizes map
		// order, and these are the bulk of the edges. The graph is committed,
		// so an unordered walk here means two builds of an unchanged corpus
		// produce two different files — the diff would be the whole artifact,
		// every time.
		for _, e := range names {
			for _, f := range byEntity[e] {
				id := PrefixCode + f
				if _, ok := g.Nodes[id]; !ok {
					g.Nodes[id] = Node{Kind: NodeCode}
				}
				add(PrefixEntity+e, id, EdgeImplements)
			}
		}
		// A file that names its context is tied to it directly, with no table in
		// between. Inference by table name cannot reach a context that owns no
		// table, and cannot tell a package that IMPLEMENTS a context from one
		// that merely mentions one of its tables.
		files := make([]string, 0, len(tagged))
		for f := range tagged {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			ctx := tagged[f]
			id := PrefixCode + f
			ctxID := PrefixCtx + ctx
			if _, ok := g.Nodes[id]; !ok {
				g.Nodes[id] = Node{Kind: NodeCode, Context: ctx}
			} else {
				n := g.Nodes[id]
				n.Context = ctx
				g.Nodes[id] = n
			}
			if _, ok := g.Nodes[ctxID]; !ok {
				g.Nodes[ctxID] = Node{Kind: NodeContext}
			}
			add(id, ctxID, EdgeBelongs)
		}
	}
	return g
}

// codeIndex maps each entity to the source files that mention it.
//
// One pass: list the tracked files once, read each once, and test every entity
// against the contents. Two shapes were tried first and are worth naming.
// One 'git grep -l' per entity re-scans the whole tree once per table, and
// parallelizing it only hides that. A single 'git grep -oF' with every pattern
// looks ideal — it prints "path:term", the whole mapping at once — but -o emits
// one line per OCCURRENCE, so a common name like 'product' floods the output
// and it ends up slower than the naive version.
//
// The corpus itself and .gofi/ are excluded: the first is documentation, not
// implementation, and the second holds the code graph, which names every symbol
// in the project and would tie every entity to it.
func (b *Builder) codeIndex(entities []string) (map[string][]string, map[string]string) {
	files := b.trackedFiles()
	if len(files) == 0 {
		return nil, nil
	}
	wanted := make(map[string]bool, len(entities))
	for _, e := range entities {
		wanted[e] = true
	}

	type hit struct{ entity, file, context string }
	hits := make(chan hit, 256)
	var wg sync.WaitGroup
	work := make(chan string)

	workers := min(8, max(2, runtime.NumCPU()))
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range work {
				content, err := os.ReadFile(filepath.Join(b.Root, filepath.FromSlash(f)))
				if err != nil || len(content) > maxScanBytes || isBinary(content) {
					continue
				}
				// One pass per file, not one per entity: split the bytes into
				// identifier-shaped tokens and look each up. Testing all N
				// entities with Contains re-scans the same file N times, which
				// is what made this the slow half of the build.
				for tok := range identifiers(content) {
					if wanted[tok] {
						hits <- hit{entity: tok, file: f}
					}
				}
				if m := reContextTag.FindSubmatch(content); m != nil {
					hits <- hit{file: f, context: string(m[1])}
				}
			}
		}()
	}
	go func() {
		for _, f := range files {
			work <- f
		}
		close(work)
		wg.Wait()
		close(hits)
	}()

	res := map[string][]string{}
	tagged := map[string]string{}
	for h := range hits {
		if h.context != "" {
			tagged[h.file] = h.context
			continue
		}
		res[h.entity] = append(res[h.entity], h.file)
	}
	for _, v := range res {
		sort.Strings(v)
	}
	return res, tagged
}

// maxScanBytes skips generated bundles and vendored blobs: a table name found
// in a 5 MB minified asset is not an implementation of it.
const maxScanBytes = 1 << 20

func isBinary(b []byte) bool {
	if len(b) > 512 {
		b = b[:512]
	}
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// identifiers yields the distinct snake_case tokens in a file. Entity names are
// table names, so anything that is not identifier-shaped cannot be one.
func identifiers(b []byte) map[string]bool {
	out := make(map[string]bool, 256)
	start := -1
	for i := 0; i <= len(b); i++ {
		var c byte
		if i < len(b) {
			c = b[i]
		}
		isWord := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || c == '_'
		switch {
		case isWord && start < 0:
			start = i
		case !isWord && start >= 0:
			if i-start > 2 {
				out[strings.ToLower(string(b[start:i]))] = true
			}
			start = -1
		}
	}
	return out
}

// trackedFiles lists the source files to scan, minus the corpus and derived
// artifacts.
//
// git ls-files is preferred because it already honours .gitignore, which keeps
// build output and vendored trees out. Outside a repository it returns nothing,
// so the walk is the fallback: silently scanning no code at all would report a
// context as having no implementation when it has one.
func (b *Builder) trackedFiles() []string {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = b.Root
	if out, err := cmd.Output(); err == nil {
		var files []string
		for _, f := range strings.Split(string(out), "\x00") {
			if f != "" && !skipPath(f) {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			return files
		}
	}
	return b.walkFiles()
}

func (b *Builder) walkFiles() []string {
	var files []string
	_ = filepath.WalkDir(b.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel := relPath(b.Root, path)
		if d.IsDir() {
			for _, skip := range vendored {
				if d.Name() == skip {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !skipPath(rel) {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func skipPath(p string) bool {
	for _, pre := range []string{".claude/", ".gofi/", "specs/", "prd/"} {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// valuesOf reads a field that may hold either a single path or a list.
func valuesOf(fm Frontmatter, field string) []string {
	if l := fm.List(field); len(l) > 0 {
		return l
	}
	if v := fm.Get(field); v != "" {
		return []string{v}
	}
	return nil
}

func corpusOf(path string) string {
	for _, c := range Corpora {
		if strings.HasPrefix(path, c+"/") {
			return c
		}
	}
	return ""
}

func stemOf(path string) string {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

func relPath(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func writeJSON(path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Load reads the index and graph written by a previous build.
func Load(root string) (*Index, *Graph, error) {
	var idx Index
	var g Graph
	if err := readJSON(filepath.Join(root, OutDir, IndexFile), &idx); err != nil {
		return nil, nil, err
	}
	if err := readJSON(filepath.Join(root, OutDir, GraphFile), &g); err != nil {
		return &idx, nil, err
	}
	return &idx, &g, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

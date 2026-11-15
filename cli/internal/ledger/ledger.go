// Package ledger keeps what a conversation read and what that cost, and says
// where the reading could have been cheaper.
//
// Two questions, both answered from what the engine already streams and from
// the project on disk — nothing here asks a model anything:
//
//  1. What is the conversation costing? The tokens as the engine counts them,
//     new input apart from cache reads, cache writes and output: they bill at
//     very different rates, and the split is the whole story.
//
//  2. Is the project's retrieval working? Every Read, Grep and Glob, and every
//     query of the gofi index, is recorded with how much text it brought into
//     the context. A gofi project has a retrieval protocol — front matter
//     first, then the headings, then the one section — and, for code, the
//     graph: `gofi show <symbol>` answers "where is it, who calls it" in a few
//     lines, which the same question costs many times over when asked by
//     grepping the tree and opening what matched. An agent that opens a long
//     spec whole, or hunts a symbol with Grep while the graph sits on disk,
//     is spending context the protocol exists to save: that shows here as a
//     finding, not as a surprise on the invoice.
//
// audit.go is the other half: the documents themselves, checked for what the
// protocol depends on, each finding with the instruction that fixes it.
package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

const (
	// charsPerToken turns the size of a tool's output into tokens, roughly:
	// the engine does not meter it apart. The headline counts are its own.
	charsPerToken = 4
	// heavyCall is a single call worth naming, in tokens.
	heavyCall = 4000
	// smallRead is a whole-file read not worth complaining about.
	smallRead = 500
	// lowCacheRate is the share of the input read from the cache under which
	// the prefix is taken to be churning.
	lowCacheRate = 0.5
	// cacheWarmup is how many turns pass before the cache is judged: the
	// first ones write it, and the rate is meant to look poor meanwhile.
	cacheWarmup = 3
	// minCodeSearches is how many code searches a conversation has before it
	// is old enough to judge how it searches.
	minCodeSearches = 3
)

// Level says what a finding asks of the person.
type Level int

const (
	Warn Level = iota // costing tokens now: worth changing
	Info              // worth knowing
	Good              // what is working
)

// Finding is one thing the ledger noticed. Fix, when there is one, is a
// bounded instruction the person can hand the agent to repair it.
type Finding struct {
	Level Level
	Text  string
	Fix   *Fix
}

// Fix is the repair of a finding: what it is called and the request that
// makes the agent do it — one file, one acceptance criterion.
type Fix struct {
	Label  string
	Prompt string
}

// Tokens are the engine's counts, each kind apart.
type Tokens struct {
	Input, Output, CacheRead, CacheWrite int
}

// Kinds of retrieval: what question a call was asking.
const (
	kindGraph = "graph" // answered by the index
	kindDoc   = "doc"   // a document of the project's corpora
	kindCode  = "code"  // the source tree
	kindWeb   = "web"
)

// Row is one retrieval call and what it brought into the context.
type Row struct {
	ID     string
	Name   string // the tool, or "gofi" for a query of the index
	Target string // the file, the pattern or the query
	Kind   string
	// Scoped says the call narrowed itself: a Read of a section rather than
	// of the file, a Grep under a path rather than over the tree.
	Scoped bool
	Tokens int
	Failed bool
	// cited are the files an index answer named: what a grep and a read
	// would have opened instead.
	cited []string
}

// graphRow is the name of a row that came from a query of the index.
const graphRow = "gofi"

// Ledger is the account of one conversation.
type Ledger struct {
	root    string
	tokens  Tokens
	turns   int
	pending map[string]*Row
	rows    []*Row
}

// New opens the account of a conversation in the project at root.
func New(root string) *Ledger {
	return &Ledger{root: root, pending: map[string]*Row{}}
}

// Usage adds what one message read and wrote. Only messages are added: the
// engine's total at the end of a turn already holds them.
func (l *Ledger) Usage(input, output, cacheRead, cacheWrite int) {
	l.tokens.Input += input
	l.tokens.Output += output
	l.tokens.CacheRead += cacheRead
	l.tokens.CacheWrite += cacheWrite
}

// Turn counts a turn that ended.
func (l *Ledger) Turn() { l.turns++ }

// retrievalTools pull text into the context: the ones retrieval is about.
var retrievalTools = map[string]bool{"Read": true, "Grep": true, "Glob": true, "WebFetch": true, "NotebookRead": true}

var (
	// graphQuery is a gofi command that answers from the index. `gofi index`
	// maintains it rather than consults it, so it does not count. Matched
	// anywhere in the line: the agent chains and pipes.
	graphQuery = regexp.MustCompile(`\bgofi\s+((?:find|show|path|impact|trace)\b[^\n|;&]*)`)
	// citedFile is how the index says where something lives: path.ext:line.
	citedFile = regexp.MustCompile(`([A-Za-z0-9_@.\-/\\]+\.[A-Za-z0-9]+):\d+`)
)

// Call opens a row when the agent reaches for a file or asks the index.
func (l *Ledger) Call(id, name string, input json.RawMessage) {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	str := func(key string) string { s, _ := in[key].(string); return s }

	// A query of the index arrives as a shell command or as a call to gofi's
	// MCP server, not as a retrieval tool — and it is the one search that is
	// meant to happen, so it is measured beside the reads it replaces.
	query := ""
	switch {
	case name == "Bash":
		if m := graphQuery.FindStringSubmatch(str("command")); m != nil {
			query = strings.TrimSpace(m[1])
		}
	case strings.HasPrefix(name, guard.MCPPrefix):
		switch tool := strings.TrimPrefix(name, guard.MCPPrefix); tool {
		case "find":
			query = strings.TrimSpace("find " + str("query"))
		case "show":
			query = strings.TrimSpace("show " + str("ref"))
		case "path":
			query = strings.TrimSpace("path " + str("from") + " " + str("to"))
		case "impact":
			var targets []string
			if list, ok := in["targets"].([]any); ok {
				for _, t := range list {
					if s, ok := t.(string); ok {
						targets = append(targets, s)
					}
				}
			}
			query = strings.TrimSpace("impact " + strings.Join(targets, " "))
		}
	}
	if query != "" {
		l.pending[id] = &Row{ID: id, Name: graphRow, Target: query, Kind: kindGraph, Scoped: true}
		return
	}
	if !retrievalTools[name] {
		return
	}
	keys := []string{"file_path", "path", "pattern", "url", "notebook_path"}
	if name == "Grep" {
		keys = []string{"path", "glob", "pattern"}
	}
	target := ""
	for _, k := range keys {
		if v := str(k); v != "" {
			target = v
			break
		}
	}
	target = l.relative(target)
	row := &Row{ID: id, Name: name, Target: target, Kind: kindCode, Scoped: true}
	switch {
	case name == "WebFetch":
		row.Kind = kindWeb
	case strings.HasPrefix(target, layout.IndexDir+"/"):
		row.Kind = kindGraph
	case inCorpus(target):
		row.Kind = kindDoc
	}
	switch name {
	case "Read", "NotebookRead":
		_, offset := in["offset"].(float64)
		_, limit := in["limit"].(float64)
		row.Scoped = offset || limit
	case "Grep":
		// A grep under a path or a glob is aimed; one over the tree sweeps.
		row.Scoped = str("path") != "" || str("glob") != ""
	}
	l.pending[id] = row
}

// Result closes the row with what the call brought into the context.
func (l *Ledger) Result(id, text string, failed bool) {
	row := l.pending[id]
	if row == nil {
		return
	}
	delete(l.pending, id)
	row.Tokens, row.Failed = approx(len(text)), failed
	if row.Name == graphRow && !failed {
		row.cited = cited(text)
	}
	l.rows = append(l.rows, row)
}

// relative strips the project's root, so a finding names a path of the project.
func (l *Ledger) relative(target string) string {
	if l.root == "" || !filepath.IsAbs(target) {
		return filepath.ToSlash(target)
	}
	if rel, err := filepath.Rel(l.root, target); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(target)
}

// inCorpus reports a path under the documents the retrieval protocol governs.
func inCorpus(path string) bool {
	for _, prefix := range []string{layout.Specs().Path(), layout.PRD().Path(), layout.Home()} {
		if strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func approx(chars int) int { return (chars + charsPerToken/2) / charsPerToken }

const (
	maxCitedText  = 64 << 10 // an answer longer than this was too broad a query already
	maxCitedFiles = 40
)

// cited are the distinct files an index answer named, in order.
func cited(text string) []string {
	if len(text) > maxCitedText {
		text = text[:maxCitedText]
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range citedFile.FindAllStringSubmatch(text, -1) {
		if file := filepath.ToSlash(m[1]); !seen[file] {
			seen[file] = true
			if out = append(out, file); len(out) == maxCitedFiles {
				break
			}
		}
	}
	return out
}

// Report is the account as it stands.
type Report struct {
	Tokens Tokens
	// Billed is the input of every kind; CacheRate the share of it read from
	// the cache, negative while nothing was read.
	Billed    int
	CacheRate float64
	Turns     int
	// The retrieval calls: how many, how they split, and what they brought.
	Calls, Graph, Code, Docs    int
	Retrieved                   int
	CodeTokens, GraphTokens     int
	AvoidedTokens, AvoidedFiles int
	// Heaviest are the calls that brought the most, largest first.
	Heaviest []Row
	Findings []Finding
}

// Report reads the account: the numbers, and what they say is worth changing.
// It stats the files the index cited and the documents that were read — the
// disk, and nothing else.
func (l *Ledger) Report() Report {
	r := Report{Tokens: l.tokens, Turns: l.turns, CacheRate: -1, Calls: len(l.rows)}
	r.Billed = l.tokens.Input + l.tokens.CacheRead + l.tokens.CacheWrite
	if r.Billed > 0 {
		r.CacheRate = float64(l.tokens.CacheRead) / float64(r.Billed)
	}
	for _, row := range l.rows {
		r.Retrieved += row.Tokens
		switch row.Kind {
		case kindGraph:
			r.Graph++
			r.GraphTokens += row.Tokens
		case kindCode:
			r.Code++
			r.CodeTokens += row.Tokens
		case kindDoc:
			r.Docs++
		}
	}
	r.AvoidedTokens, r.AvoidedFiles = l.avoided()
	heaviest := make([]Row, 0, len(l.rows))
	for _, row := range l.rows {
		heaviest = append(heaviest, *row)
	}
	sort.SliceStable(heaviest, func(i, j int) bool { return heaviest[i].Tokens > heaviest[j].Tokens })
	r.Heaviest = heaviest[:min(len(heaviest), 5)]
	r.Findings = append(l.findings(r), Review(l.root, l.Rows())...)
	return r
}

// Rows are the retrieval calls that ended, in order.
func (l *Ledger) Rows() []Row {
	out := make([]Row, 0, len(l.rows))
	for _, row := range l.rows {
		out = append(out, *row)
	}
	return out
}

// avoided is what the index answered without anything being opened: the size
// of the files its answers named, off disk. A file the conversation went on
// to read anyway was not avoided, and one named twice counts once. It is an
// upper bound on the alternative — a grep sometimes answers from the matched
// line alone.
func (l *Ledger) avoided() (tokens, files int) {
	read := map[string]bool{}
	for _, row := range l.rows {
		if row.Name == "Read" && row.Target != "" {
			read[row.Target] = true
		}
	}
	prefixes := scopePrefixes(l.root)
	counted := map[string]bool{}
	for _, row := range l.rows {
		for _, file := range row.cited {
			for _, prefix := range prefixes {
				rel := prefix + file
				st, err := os.Stat(filepath.Join(l.root, filepath.FromSlash(rel)))
				if err != nil || st.IsDir() {
					continue
				}
				if !read[rel] && !counted[rel] {
					counted[rel] = true
					tokens += approx(int(st.Size()))
				}
				break
			}
		}
	}
	return tokens, len(counted)
}

// scopePrefixes are where the graph's paths are rooted. The graph records a
// file relative to the scope that was scanned — backend, frontend, a vendored
// SDK —, never to the project: against the project's root alone, nothing
// would be found in exactly the repositories the graph helps most.
func scopePrefixes(root string) []string {
	prefixes := []string{""}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(layout.CodeDir), "gofi_graph_index.json"))
	if err != nil {
		return prefixes // one scope: the graph is rooted at the project
	}
	var index struct {
		Scopes []struct {
			Root string `json:"root"`
		} `json:"scopes"`
	}
	if json.Unmarshal(data, &index) != nil {
		return prefixes
	}
	for _, s := range index.Scopes {
		if r := strings.TrimRight(filepath.ToSlash(s.Root), "/"); r != "" && r != "." {
			prefixes = append(prefixes, r+"/")
		}
	}
	return prefixes
}

// GraphAvailable reports whether the project has a code graph built: without
// one there is nothing to prefer over a Grep, and saying otherwise would be
// advice the project cannot follow.
func GraphAvailable(root string) bool {
	dir := filepath.Join(root, filepath.FromSlash(layout.CodeDir))
	for _, name := range []string{"gofi_graph.json", "gofi_graph_index.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// findings turns the numbers into things worth changing. Each names the call
// and says what to do instead; every rule fires on a measured quantity.
func (l *Ledger) findings(r Report) []Finding {
	var out []Finding
	add := func(level Level, key string, args ...any) {
		out = append(out, Finding{Level: level, Text: i18n.T(key, args...)})
	}

	// 1. A document of the corpora read whole: the breach of the protocol
	// that costs the most, since those are long by design.
	whole := map[*Row]bool{}
	for _, row := range l.rows {
		if row.Name == "Read" && !row.Scoped && inCorpus(row.Target) && row.Tokens > smallRead {
			if whole[row] = true; len(whole) <= 5 {
				add(Warn, "ledger.whole_read", row.Target, count(row.Tokens))
			}
		}
	}

	// 2. The same file read more than once: the second read adds nothing,
	// the content was in the context already.
	type again struct{ n, tokens int }
	reads, order := map[string]*again{}, []string{}
	for _, row := range l.rows {
		if row.Name != "Read" || row.Target == "" {
			continue
		}
		if reads[row.Target] == nil {
			reads[row.Target] = &again{}
			order = append(order, row.Target)
		}
		reads[row.Target].n++
		reads[row.Target].tokens += row.Tokens
	}
	for _, target := range order {
		if a := reads[target]; a.n > 1 {
			add(Warn, "ledger.reread", target, a.n, count(a.tokens))
		}
	}

	// 3. A single call that filled the context on its own.
	for _, row := range l.rows {
		if row.Tokens >= heavyCall && !whole[row] {
			add(Info, "ledger.heavy", strings.TrimSpace(row.Name+" "+row.Target), count(row.Tokens))
		}
	}

	// 4. The cache. A low share read from it means the stable prefix keeps
	// changing and context that should be nearly free is billed again — once
	// the cache has had the turns to establish itself.
	if r.CacheRate >= 0 && l.turns > 0 {
		percent := fmt.Sprintf("%d%%", int(r.CacheRate*100+0.5))
		switch {
		case r.CacheRate >= lowCacheRate:
			add(Good, "ledger.cache_good", percent)
		case l.turns < cacheWarmup:
			add(Info, "ledger.cache_warming", percent)
		default:
			add(Warn, "ledger.cache_low", percent, l.turns)
		}
	}

	// 5. Code hunted through the tree with the graph on disk.
	if GraphAvailable(l.root) && r.Code >= minCodeSearches {
		switch {
		case r.Graph == 0:
			add(Warn, "ledger.no_graph", r.Code, count(r.CodeTokens))
		case r.Code > r.Graph:
			add(Info, "ledger.few_graph", r.Graph, r.Graph+r.Code, r.Code, count(r.CodeTokens))
		}
	}

	// 6. What is working, in the one currency that settles the argument.
	switch {
	case r.AvoidedFiles > 0:
		add(Good, "ledger.avoided", r.AvoidedFiles, count(r.AvoidedTokens), count(r.GraphTokens))
	case r.Graph > 0 && r.Graph >= r.Code:
		add(Good, "ledger.by_graph", r.Graph, r.Graph+r.Code, count(r.GraphTokens))
	}
	scoped := 0
	for _, row := range l.rows {
		if row.Scoped {
			scoped++
		}
	}
	switch {
	case len(l.rows) > 0 && scoped == len(l.rows):
		add(Good, "ledger.all_scoped", len(l.rows), count(r.Retrieved))
	case scoped > 0:
		add(Info, "ledger.some_scoped", scoped, len(l.rows))
	}
	return out
}

// count is a number of tokens at a glance: 950, 42k, 1.2M.
func count(n int) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 10_000:
		return fmt.Sprintf("%dk", (n+500)/1000)
	case n >= 1_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e3), ".0") + "k"
	}
	return fmt.Sprint(n)
}

// Count is a number of tokens at a glance, for whoever shows the account.
func Count(n int) string { return count(n) }

// Package scan reads a repository — gofi's or not — and says what it holds
// that a gofi project is made of: the code and its contexts, the documents
// that read like a PRD or a spec, the API contracts, the acceptance tests,
// the tables, the tests. It writes nothing and asks no model: it is what
// `gofi scan` prints, and what `gofi init` proposes the contexts from.
package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/detect"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// Schema names the format of a scan's JSON.
const Schema = "gofi-scan/v1"

// The kinds a document is read as.
const (
	KindPRD       = "prd"       // product: problem, goals, personas, scope
	KindSpec      = "spec"      // requirements, rules, acceptance criteria
	KindDecision  = "decision"  // an architecture decision record
	KindAPI       = "api"       // an API contract written as prose: endpoints, payloads, statuses
	KindOps       = "ops"       // deploy, runbook, incidents
	KindReadme    = "readme"    // a README
	KindChangelog = "changelog" // a CHANGELOG
	KindOther     = "other"     // nothing reads clearly
)

// Report is what a scan found.
type Report struct {
	Schema string `json:"schema"`
	Root   string `json:"root"`
	Name   string `json:"name,omitempty"`
	// Gofi is what of gofi the repository already has.
	Gofi      Gofi           `json:"gofi"`
	Backend   *Surface       `json:"backend,omitempty"`
	Web       *Surface       `json:"web,omitempty"`
	Mobile    *Surface       `json:"mobile,omitempty"`
	Contexts  []Context      `json:"contexts"`
	Documents []Document     `json:"documents"`
	Features  []Feature      `json:"features"`
	APIs      []API          `json:"apis"`
	Tables    []Table        `json:"tables"`
	Tests     int            `json:"tests"`
	Sources   int            `json:"sources"`
	Skipped   map[string]int `json:"skipped,omitempty"`
}

// Surface is a part of the code gofi recognises: the backend, a web or a
// mobile app.
type Surface struct {
	Path      string `json:"path"`
	Language  string `json:"language,omitempty"`
	Framework string `json:"framework,omitempty"`
	Module    string `json:"module,omitempty"`
	Marker    string `json:"marker,omitempty"`
}

// Gofi is what of a gofi project a repository has.
type Gofi struct {
	Config bool `json:"config"` // .gofi.yaml
	Agents bool `json:"agents"` // AGENTS.md
	// ClaudeRoot is a CLAUDE.md at the root: the team's, and it hides
	// AGENTS.md from Claude Code. ClaudeDir is the .claude/CLAUDE.md older
	// releases installed, which gofi update agents turns into AGENTS.md.
	ClaudeRoot bool `json:"claude_md_root"`
	ClaudeDir  bool `json:"claude_md_dir"`
	Skills     bool `json:"skills"` // .claude/skills/ or .agents/skills/
	Specs      int  `json:"specs"`  // documents under specs/
	PRDs       int  `json:"prds"`   // documents under prd/
	Graph      bool `json:"graph"`  // .gofi/index/code/
	Contract   int  `json:"living_specs"`
}

// Context is a context the code's folders name, as the language's pack lays
// them out, with what points at it.
type Context struct {
	Name string `json:"name"`
	// Dir is the folder of the context's code, from the repository root.
	Dir       string   `json:"dir"`
	Sources   int      `json:"sources"`
	Tests     int      `json:"tests"`
	Tables    []string `json:"tables,omitempty"`
	Documents []string `json:"documents,omitempty"`
}

// Document is a document of the repository and what it reads as.
type Document struct {
	Path     string   `json:"path"`
	Title    string   `json:"title,omitempty"`
	Kind     string   `json:"kind"`
	Evidence []string `json:"evidence,omitempty"`
	Lines    int      `json:"lines"`
}

// Feature is a Gherkin file: acceptance scenarios already written.
type Feature struct {
	Path      string `json:"path"`
	Title     string `json:"title,omitempty"`
	Scenarios int    `json:"scenarios"`
}

// API is a contract of an API: OpenAPI, protobuf or GraphQL.
type API struct {
	Path       string `json:"path"`
	Format     string `json:"format"` // openapi | proto | graphql
	Title      string `json:"title,omitempty"`
	Operations int    `json:"operations"`
}

// Table is a table the schema or the migrations create.
type Table struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
}

// Limits keep a scan of a large repository quick.
const (
	maxFiles    = 50000
	maxDocBytes = 64 << 10
	maxDocs     = 2000
)

// Scan reads the repository at root.
func Scan(root string) (*Report, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	found := detect.Scan(abs)
	r := &Report{Schema: Schema, Root: abs, Name: found.Name, Contexts: []Context{}, Documents: []Document{},
		Features: []Feature{}, APIs: []API{}, Tables: []Table{}, Skipped: map[string]int{}}
	for _, s := range []struct {
		in  detect.Surface
		out **Surface
	}{{found.Backend, &r.Backend}, {found.Web, &r.Web}, {found.Mobile, &r.Mobile}} {
		if s.in.Found() {
			*s.out = &Surface{Path: s.in.Path, Language: s.in.Language, Framework: s.in.Framework, Module: s.in.Module, Marker: s.in.Marker}
		}
	}
	r.Gofi = gofiOf(abs)

	var lp *pack.Pack
	if r.Backend != nil {
		if p, err := pack.Load(r.Backend.Language); err == nil && p != nil {
			lp = p.Bind(r.Backend.Module)
		}
	}
	contexts := map[string]*Context{}
	tables := map[string]*Table{}
	seen := 0
	_ = filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(abs, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel != "." && (detect.IgnoredDir(d.Name()) || strings.HasPrefix(d.Name(), ".") && d.Name() != ".github") {
				return filepath.SkipDir
			}
			return nil
		}
		if seen++; seen > maxFiles {
			r.Skipped["files"]++
			return nil
		}
		name := strings.ToLower(d.Name())
		ext := strings.ToLower(filepath.Ext(name))
		switch {
		case ext == ".md" || ext == ".mdx" || ext == ".adoc" || ext == ".rst":
			if inGofiCorpus(rel) {
				return nil
			}
			if len(r.Documents) >= maxDocs {
				r.Skipped["documents"]++
				return nil
			}
			if doc, ok := readDocument(p, rel); ok {
				r.Documents = append(r.Documents, doc)
			}
		case ext == ".feature":
			if f, ok := readFeature(p, rel); ok {
				r.Features = append(r.Features, f)
			}
		case ext == ".proto" || ext == ".graphql" || ext == ".gql" || isOpenAPIName(name):
			if a, ok := readAPI(p, rel, ext); ok {
				r.APIs = append(r.APIs, a)
			}
		case ext == ".sql" || name == "schema.prisma":
			for _, t := range readTables(p) {
				if tables[t] == nil {
					tables[t] = &Table{Name: t}
				}
				tables[t].Files = appendOnce(tables[t].Files, rel)
			}
		}
		// The backend's code: its sources, its tests, and the context its
		// folders name.
		if r.Backend != nil && lp != nil {
			in := r.Backend.Path == "." || strings.HasPrefix(rel, r.Backend.Path+"/")
			if !in {
				return nil
			}
			inner := strings.TrimPrefix(rel, r.Backend.Path+"/")
			test, source := lp.IsTest(inner), lp.IsSource(inner)
			if !test && !source {
				return nil
			}
			if test {
				r.Tests++
			} else {
				r.Sources++
			}
			if ctx := lp.ContextOf(inner); ctx != "" && !strings.Contains(ctx, "*") {
				c := contexts[ctx]
				if c == nil {
					c = &Context{Name: ctx, Dir: contextDir(r.Backend.Path, inner, ctx)}
					contexts[ctx] = c
				}
				if test {
					c.Tests++
				} else {
					c.Sources++
				}
			}
		}
		return nil
	})

	for _, t := range tables {
		sort.Strings(t.Files)
		r.Tables = append(r.Tables, *t)
	}
	sort.Slice(r.Tables, func(i, j int) bool { return r.Tables[i].Name < r.Tables[j].Name })
	for _, c := range contexts {
		// A table and a document point at a context when they name it.
		for _, t := range r.Tables {
			if names(t.Name, c.Name) {
				c.Tables = append(c.Tables, t.Name)
			}
		}
		for _, d := range r.Documents {
			if names(d.Path, c.Name) || names(d.Title, c.Name) {
				c.Documents = append(c.Documents, d.Path)
			}
		}
		r.Contexts = append(r.Contexts, *c)
	}
	sort.Slice(r.Contexts, func(i, j int) bool {
		if r.Contexts[i].Sources != r.Contexts[j].Sources {
			return r.Contexts[i].Sources > r.Contexts[j].Sources
		}
		return r.Contexts[i].Name < r.Contexts[j].Name
	})
	if len(r.Skipped) == 0 {
		r.Skipped = nil
	}
	return r, nil
}

// Of returns the documents of a kind.
func (r *Report) Of(kind string) []Document {
	var out []Document
	for _, d := range r.Documents {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// gofiOf is what of gofi the repository already has.
func gofiOf(root string) Gofi {
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		return err == nil
	}
	count := func(dir string, match func(string) bool) int {
		n := 0
		_ = filepath.WalkDir(filepath.Join(root, dir), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && match(filepath.ToSlash(p)) {
				n++
			}
			return nil
		})
		return n
	}
	isMD := func(p string) bool { return strings.HasSuffix(p, ".md") && !strings.HasSuffix(p, "/INDEX.md") }
	return Gofi{
		Config:     exists(config.FileName),
		Agents:     exists("AGENTS.md"),
		ClaudeRoot: exists("CLAUDE.md"),
		ClaudeDir:  exists(".claude/CLAUDE.md"),
		Skills:     exists(".claude/skills") || exists(".agents/skills"),
		Specs:      count("specs", isMD),
		PRDs:       count("prd", isMD),
		Graph:      exists(".gofi/index/code"),
		Contract:   count("specs", func(p string) bool { return strings.HasSuffix(p, "/spec.md") && !strings.Contains(p, "/changes/") }),
	}
}

// inGofiCorpus is a document gofi already reads as its own: the contract,
// the PRDs, the agents' instructions.
func inGofiCorpus(rel string) bool {
	for _, p := range []string{"specs/", "prd/"} {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return rel == "AGENTS.md" || rel == "CLAUDE.md"
}

// signals are what a document of a kind says, in Portuguese and English: the
// strong ones say the kind — a persona, an acceptance criterion, the
// consequences of a decision —, the weak ones only add to it. A heading that
// says one counts twice what the body does.
var signals = map[string]struct{ strong, weak []string }{
	KindPRD: {
		strong: []string{"persona", "user stor", "história de usuário", "historia de usuario", "público-alvo", "target audience",
			"proposta de valor", "value proposition", "visão do produto", "product vision", "problem statement", "jornada do usuário", "user journey", "prd"},
		weak: []string{"problema", "objetivo", "goal", "métrica", "metric", "kpi", "escopo", "scope", "stakeholder", "roadmap", "fora do escopo", "out of scope"},
	},
	KindSpec: {
		strong: []string{"requisito", "requirement", "critério de aceite", "critérios de aceite", "critérios de aceitação", "acceptance criteria",
			"regra de negócio", "regras de negócio", "business rule", "caso de uso", "use case", "rf-", "rnf-", "rn-", "req-"},
		weak: []string{"cenário", "scenario", "dado que", "given ", "especificação", "specification", "deve ", "must ", "shall "},
	},
	KindDecision: {
		strong: []string{"consequências", "consequences", "alternativas consideradas", "alternatives considered", "decision record", "registro de decisão"},
		weak:   []string{"status", "contexto", "context", "decisão", "decision", "alternativas", "alternatives"},
	},
	KindOps: {
		strong: []string{"runbook", "incidente", "incident", "rollback", "on-call", "plantão", "postmortem", "post-mortem"},
		weak:   []string{"deploy", "monitoramento", "monitoring", "alerta", "alert", "manutenção", "maintenance"},
	},
	KindAPI: {
		strong: []string{"contrato da api", "api contract", "endpoint", "get /", "post /", "put /", "patch /", "delete /", "status code", "código de status"},
		weak:   []string{"request", "response", "payload", "4xx", "5xx", "content-type", "envelope", "header"},
	},
}

var reHeading = regexp.MustCompile(`^#{1,6}\s+(.+)$|^=+\s+(.+)$`)

// readDocument reads the start of a document and what it reads as.
func readDocument(p, rel string) (Document, bool) {
	f, err := os.Open(p)
	if err != nil {
		return Document{}, false
	}
	defer f.Close()
	d := Document{Path: rel, Kind: KindOther}
	var heads, body strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	read := 0
	for sc.Scan() {
		line := sc.Text()
		d.Lines++
		if read > maxDocBytes {
			continue
		}
		read += len(line) + 1
		if m := reHeading.FindStringSubmatch(line); m != nil {
			h := strings.TrimSpace(m[1] + m[2])
			if d.Title == "" {
				d.Title = h
			}
			heads.WriteString(strings.ToLower(h) + "\n")
			continue
		}
		body.WriteString(strings.ToLower(line) + "\n")
	}
	base := strings.ToLower(filepath.Base(rel))
	switch {
	case strings.HasPrefix(base, "readme"):
		d.Kind = KindReadme
		return d, true
	case strings.HasPrefix(base, "changelog") || strings.HasPrefix(base, "history"):
		d.Kind = KindChangelog
		return d, true
	}
	lower := strings.ToLower(rel)
	best, bestScore := KindOther, 0
	for _, kind := range []string{KindPRD, KindSpec, KindDecision, KindOps, KindAPI} {
		score, strong := 0, false
		var hit []string
		count := func(words []string, isStrong bool) {
			for _, s := range words {
				switch {
				case strings.Contains(heads.String(), s):
					score += 2
				case strings.Contains(body.String(), s):
					score++
				default:
					continue
				}
				strong = strong || isStrong
				hit = append(hit, strings.TrimSpace(s))
			}
		}
		count(signals[kind].strong, true)
		count(signals[kind].weak, false)
		// An ADR says so by where it is, more than by its words.
		if kind == KindDecision && (strings.Contains(lower, "/adr") || strings.HasPrefix(lower, "adr") || strings.Contains(lower, "/decisions/")) {
			score, strong = score+4, true
			hit = append([]string{"pasta de decisões"}, hit...)
		}
		// Weak words alone — a goal, a scope — say no kind.
		if strong && score > bestScore {
			best, bestScore, d.Evidence = kind, score, hit
		}
	}
	// A word or two is no evidence: what says nothing clearly stays other.
	if bestScore >= 4 {
		d.Kind = best
		if len(d.Evidence) > 5 {
			d.Evidence = d.Evidence[:5]
		}
	} else {
		d.Evidence = nil
	}
	return d, true
}

var (
	reFeature  = regexp.MustCompile(`(?i)^\s*(feature|funcionalidade|característica)\s*:\s*(.*)$`)
	reScenario = regexp.MustCompile(`(?i)^\s*(scenario|scenario outline|scenario template|example|cenário|cenario|esquema do cenário|exemplo)\s*:`)
)

func readFeature(p, rel string) (Feature, bool) {
	data, err := os.ReadFile(p)
	if err != nil {
		return Feature{}, false
	}
	f := Feature{Path: rel}
	for _, line := range strings.Split(string(data), "\n") {
		if m := reFeature.FindStringSubmatch(line); m != nil && f.Title == "" {
			f.Title = strings.TrimSpace(m[2])
		}
		if reScenario.MatchString(line) {
			f.Scenarios++
		}
	}
	return f, true
}

func isOpenAPIName(name string) bool {
	ext := filepath.Ext(name)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return false
	}
	return strings.Contains(name, "openapi") || strings.Contains(name, "swagger") || strings.HasPrefix(name, "api.")
}

var (
	reOpenAPI   = regexp.MustCompile(`(?m)^\s*"?(openapi|swagger)"?\s*:`)
	reTitle     = regexp.MustCompile(`(?m)^\s*"?title"?\s*:\s*"?([^"\n,]+)`)
	reOperation = regexp.MustCompile(`(?mi)^\s*"?(get|post|put|patch|delete)"?\s*:`)
	reRPC       = regexp.MustCompile(`(?m)^\s*rpc\s+\w+`)
	reGraphQL   = regexp.MustCompile(`(?m)^\s*(type|extend type)\s+(Query|Mutation|Subscription)\b`)
	reGQLField  = regexp.MustCompile(`(?m)^\s+\w+\s*(\(|:)`)
)

func readAPI(p, rel, ext string) (API, bool) {
	data, err := os.ReadFile(p)
	if err != nil || len(data) > 4<<20 {
		return API{}, false
	}
	text := string(data)
	switch ext {
	case ".proto":
		return API{Path: rel, Format: "proto", Operations: len(reRPC.FindAllString(text, -1))}, true
	case ".graphql", ".gql":
		if !reGraphQL.MatchString(text) {
			return API{}, false
		}
		return API{Path: rel, Format: "graphql", Operations: len(reGQLField.FindAllString(text, -1))}, true
	}
	if !reOpenAPI.MatchString(text) {
		return API{}, false
	}
	a := API{Path: rel, Format: "openapi", Operations: len(reOperation.FindAllString(text, -1))}
	if m := reTitle.FindStringSubmatch(text); m != nil {
		a.Title = strings.TrimSpace(m[1])
	}
	return a, true
}

var (
	reCreateTable = regexp.MustCompile("(?i)create\\s+table\\s+(?:if\\s+not\\s+exists\\s+)?[\"`\\[]?([\\w.]+?)[\"`\\]]?\\s*\\(")
	rePrismaModel = regexp.MustCompile(`(?m)^model\s+(\w+)\s*\{`)
)

func readTables(p string) []string {
	data, err := os.ReadFile(p)
	if err != nil || len(data) > 8<<20 {
		return nil
	}
	var out []string
	re := reCreateTable
	if strings.HasSuffix(p, ".prisma") {
		re = rePrismaModel
	}
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		name := strings.ToLower(m[1])
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		out = appendOnce(out, name)
	}
	return out
}

// names reports whether text names a context: the name, or its singular,
// as a word of it.
func names(text, ctx string) bool {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127) })
	stem := strings.TrimSuffix(ctx, "s")
	return slices.ContainsFunc(words, func(w string) bool {
		return w == ctx || w == stem || strings.TrimSuffix(w, "s") == stem
	})
}

// contextDir is the folder of a context's code: the file's path up to the
// folder that names the context, from the repository root.
func contextDir(backend, inner, ctx string) string {
	segs := strings.Split(inner, "/")
	for i, s := range segs[:len(segs)-1] {
		if s == ctx {
			dir := strings.Join(segs[:i+1], "/") + "/"
			if backend != "." {
				dir = backend + "/" + dir
			}
			return dir
		}
	}
	return ""
}

func appendOnce(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

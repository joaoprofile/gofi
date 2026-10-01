package cli

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/intake"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/retrieval"
)

func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: i18n.T("cmd.mcp.short"),
		Long: `Serve the index to an AI agent over the Model Context Protocol.

The host — Claude Code, or any client that speaks MCP — starts 'gofi mcp' for
the session and talks to it over stdin and stdout; no port is opened and
nothing leaves the machine. The agent sees gofi's queries as tools of its own:

  find          where is it — documents and code, ranked; or exact text
  show          what a reference is — symbol, document, section, context, table
  path          how one symbol reaches another
  index_status  whether the index still matches the project
  index         rebuild the index after an edit (writes derived files only)

The answers are the ones the terminal commands print, from the same core. The
indexes stay loaded between calls and are reopened when they change.

'gofi install mcp' registers the server where the project's host reads it.`,
		Example: `gofi install mcp
gofi mcp`,
		Args: cobra.NoArgs,
		RunE: runMCP,
	}
	return cmd
}

func runMCP(cmd *cobra.Command, _ []string) error {
	root, err := graphProjectRoot()
	if err != nil {
		return err
	}
	language := ""
	if cfg, _, err := loadProjectConfig(); err == nil {
		language = backendLanguage(cfg)
	}
	return newMCPServer(root, language).Run(cmd.Context(), &mcp.StdioTransport{})
}

// mcpInstructions reach the agent when it connects: when to use the tools, in
// the few lines a host shows every session.
const mcpInstructions = `gofi indexes this project: its documents (specs, PRDs, memory, knowledge, SDK docs) and its code graph.
Ask it before Grep, Glob or reading whole files: find says where something is and which lines to read; show describes a symbol (with its callers), a document, a section, a context or a table; path says how A reaches B.
For exact text — an error message, a config key, SQL — use find with text or regex: each line comes with the symbol or section that holds it.
After editing code, run index (target code) before querying what you changed.
Given a task in free text, call intake first: it says what the request asks for, the context it is about, the phases — which role, in order, at what tier — the expertise to read, and what to ask the person.`

// gofiMCP serves one project. Calls are serialized: they are fast, and one of
// them rebuilds the index the others read.
type gofiMCP struct {
	root, language string

	mu     sync.Mutex
	engine *retrieval.Engine
	stamp  string
}

func newMCPServer(root, language string) *mcp.Server {
	g := &gofiMCP{root: root, language: language}
	s := mcp.NewServer(&mcp.Implementation{Name: host.ServerName, Title: "gofi", Version: Version},
		&mcp.ServerOptions{Instructions: mcpInstructions})
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: new(false)}

	mcp.AddTool(s, &mcp.Tool{
		Name: "find", Title: "Find in documents and code", Annotations: readOnly,
		Description: `Where is it? Searches the project's documents (specs, PRDs, memory, knowledge, SDK docs, institutional base) and code symbols together, ranked, and names the lines to read. Use it before Grep, Glob or opening files. With text or regex it finds exact text line by line in every project file instead, and names the symbol or section holding each line.`,
	}, g.find)
	mcp.AddTool(s, &mcp.Tool{
		Name: "show", Title: "Show a reference", Annotations: readOnly,
		Description: `What is it? Describes one reference from the index: a symbol (Bill, service.Bill, sym:...) with where it is, what it calls and what calls it; a document path (outline with line ranges, links in and out); a section (path.md#heading or #L42); a context (ctx:name); a table (table:name). An ambiguous name returns the candidates.`,
	}, g.show)
	mcp.AddTool(s, &mcp.Tool{
		Name: "path", Title: "Path between symbols", Annotations: readOnly,
		Description: `How does A reach B? Every hop between two symbols in the code graph, with where in the code each happens. In a fast graph an ambiguous call has no edge: no path is not proof there is none.`,
	}, g.path)
	mcp.AddTool(s, &mcp.Tool{
		Name: "intake", Title: "Plan a request", Annotations: readOnly,
		Description: `How should this request be carried out? Reads a task in free text against the index and the roles' contracts: the intent and the artifact it ends with, the context and what it already has, the phases in order — each role (follow its .claude/skills/<role>/SKILL.md or the host's equivalent) at its tier, with why — the expertise packs to read, the open questions with options from the index, and the assembled request for the first phase. Call it before starting a task. Rules only: settle an open question from the conversation, or ask the person, then call again with answers (the question ids) or proceed.`,
	}, g.intake)
	mcp.AddTool(s, &mcp.Tool{
		Name: "index_status", Title: "Index status", Annotations: readOnly,
		Description: `Whether the index still matches the project: when each target was built, and for each code scope whether a source file changed since. Builds nothing.`,
	}, g.status)
	mcp.AddTool(s, &mcp.Tool{
		Name: "index", Title: "Rebuild the index",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true, DestructiveHint: new(false), OpenWorldHint: new(false)},
		Description: `Rebuild the index after editing: target "code" (incremental — only changed trees), "docs" (also regenerates the specs/PRD INDEX.md files), or empty for both. Writes derived files only. Queries refresh the document index by themselves; run this for code you just changed.`,
	}, g.index)
	return s
}

type findArgs struct {
	Query      string   `json:"query" jsonschema:"the question in words, or the exact text when text or regex is set"`
	In         []string `json:"in,omitempty" jsonschema:"limit to areas: specs, prd, memory, institutional, knowledge, sdk, skills, code"`
	Limit      int      `json:"limit,omitempty" jsonschema:"how many answers (default 5; 30 lines for a text search)"`
	Text       bool     `json:"text,omitempty" jsonschema:"search for this exact text, line by line, instead of ranking"`
	Regex      bool     `json:"regex,omitempty" jsonschema:"the query is a regular expression (implies text)"`
	IgnoreCase bool     `json:"ignore_case,omitempty" jsonschema:"ignore case in a text search"`
}

type showArgs struct {
	Ref   string `json:"ref" jsonschema:"what to describe: a symbol, a document path, path.md#section, ctx:name or table:name"`
	Limit int    `json:"limit,omitempty" jsonschema:"how many neighbours or members to list (default 12)"`
}

type pathArgs struct {
	From string `json:"from" jsonschema:"the symbol the path starts at"`
	To   string `json:"to" jsonschema:"the symbol the path should reach"`
}

type intakeArgs struct {
	Request string            `json:"request" jsonschema:"the task, in the person's words"`
	Answers map[string]string `json:"answers,omitempty" jsonschema:"answers to the previous call's questions, by question id (context: a context name, or novo)"`
	Proceed bool              `json:"proceed,omitempty" jsonschema:"go on without answering: each open point takes its default, written down as an assumption"`
}

type statusArgs struct{}

type indexArgs struct {
	Target string `json:"target,omitempty" jsonschema:"code, docs, or empty for both"`
	Full   bool   `json:"full,omitempty" jsonschema:"rebuild every code tree, even the unchanged ones"`
}

func (g *gofiMCP) find(_ context.Context, _ *mcp.CallToolRequest, a findArgs) (*mcp.CallToolResult, any, error) {
	for _, area := range a.In {
		if !retrieval.ValidArea(area) {
			return nil, nil, fmt.Errorf("in %q: use %s", area, strings.Join(retrieval.Areas, ", "))
		}
	}
	return g.answer(func(e *retrieval.Engine, out *bytes.Buffer) error {
		if a.Text || a.Regex {
			limit := a.Limit
			if limit <= 0 {
				limit = textLimit
			}
			res, err := e.Text(retrieval.TextQuery{Pattern: a.Query, Regex: a.Regex, IgnoreCase: a.IgnoreCase, Areas: a.In, Limit: limit})
			if err != nil {
				return err
			}
			printTextMatches(out, res)
			return nil
		}
		limit := a.Limit
		if limit <= 0 {
			limit = 5
		}
		printHits(out, e.Find(retrieval.Query{Text: a.Query, Areas: a.In, Limit: limit}), e.Missing)
		return nil
	})
}

// intake plans a request with the rules only: the agent calling it is a model
// already, and settles what they leave open from the conversation — or asks.
func (g *gofiMCP) intake(_ context.Context, _ *mcp.CallToolRequest, a intakeArgs) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(a.Request) == "" {
		return nil, nil, fmt.Errorf("request: the task, in the person's words")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	res, err := intake.Run(g.root, a.Request, intake.Options{Language: g.language, Answers: a.Answers, Proceed: a.Proceed})
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	printIntake(&out, res)
	if len(res.Plan) > 0 || res.Direct != "" {
		out.WriteString("\n" + res.Brief)
	}
	return textResult(out.String()), nil, nil
}

func (g *gofiMCP) show(_ context.Context, _ *mcp.CallToolRequest, a showArgs) (*mcp.CallToolResult, any, error) {
	return g.answer(func(e *retrieval.Engine, out *bytes.Buffer) error {
		limit := a.Limit
		if limit <= 0 {
			limit = 12
		}
		view, err := e.Show(a.Ref, limit)
		return renderView(out, view, err)
	})
}

func (g *gofiMCP) path(_ context.Context, _ *mcp.CallToolRequest, a pathArgs) (*mcp.CallToolResult, any, error) {
	return g.answer(func(e *retrieval.Engine, out *bytes.Buffer) error {
		view, err := e.Path(a.From, a.To)
		return renderView(out, view, err)
	})
}

func (g *gofiMCP) status(_ context.Context, _ *mcp.CallToolRequest, _ statusArgs) (*mcp.CallToolResult, any, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	cfg, err := config.Load(filepath.Join(g.root, config.FileName))
	if err != nil {
		return nil, nil, err
	}
	st, err := indexStatus(cfg, g.root)
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	printIndexStatus(&out, st)
	return textResult(out.String()), nil, nil
}

func (g *gofiMCP) index(ctx context.Context, _ *mcp.CallToolRequest, a indexArgs) (*mcp.CallToolResult, any, error) {
	if a.Target != "" && a.Target != layout.TargetCode && a.Target != layout.TargetDocs {
		return nil, nil, fmt.Errorf("target %q: use %s, %s or leave it empty", a.Target, layout.TargetCode, layout.TargetDocs)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	// The command finds its project from the working folder; the server's is
	// the project it serves, which is not always where the process was
	// started. It runs there — under the lock, so no other call sees the move.
	if cwd, err := os.Getwd(); err == nil && filepath.Clean(cwd) != filepath.Clean(g.root) {
		if err := os.Chdir(g.root); err != nil {
			return nil, nil, err
		}
		defer os.Chdir(cwd) //nolint:errcheck // back where the process was
	}
	// The terminal command itself, so the agent's rebuild is the one a person
	// would run: same defaults, same migration of an old layout, same output.
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	args := []string{"--no-html"}
	if a.Target != "" {
		args = append([]string{a.Target}, args...)
	}
	if a.Full {
		args = append(args, "--full")
	}
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("%w\n%s", err, out.String())
	}
	g.engine = nil // whatever was loaded is now stale
	return textResult(out.String()), nil, nil
}

// answer runs a query against a current engine and returns its text.
func (g *gofiMCP) answer(query func(*retrieval.Engine, *bytes.Buffer) error) (*mcp.CallToolResult, any, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, err := g.current()
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	if err := query(e, &out); err != nil {
		return nil, nil, err
	}
	return textResult(out.String()), nil, nil
}

// current returns the engine, reopened when an index changed on disk since it
// was loaded — a rebuild from a terminal, a git checkout, a document edited
// between two calls. The check is a few stats; opening is what it saves.
func (g *gofiMCP) current() (*retrieval.Engine, error) {
	if err := ensureDocsIndex(g.root, g.language); err != nil {
		return nil, err
	}
	stamp := indexStamp(g.root)
	if g.engine != nil && stamp == g.stamp {
		return g.engine, nil
	}
	e, err := retrieval.Open(g.root, g.language)
	if err != nil {
		return nil, err
	}
	g.engine, g.stamp = e, stamp
	return e, nil
}

// indexStamp identifies the indexes on disk by the size and time of every file
// the engine reads.
func indexStamp(root string) string {
	var b strings.Builder
	stamp := func(p string) {
		if fi, err := os.Stat(p); err == nil {
			fmt.Fprintf(&b, "%s:%d:%d;", p, fi.Size(), fi.ModTime().UnixNano())
		}
	}
	stamp(filepath.Join(root, filepath.FromSlash(docs.OutDir), docs.IndexFile))
	stamp(filepath.Join(root, filepath.FromSlash(docs.OutDir), docs.GraphFile))
	_ = filepath.WalkDir(filepath.Join(root, filepath.FromSlash(layout.CodeDir)), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".json") {
			stamp(p)
		}
		return nil
	})
	return b.String()
}

func textResult(text string) *mcp.CallToolResult {
	if strings.TrimSpace(text) == "" {
		text = "(no output)"
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

// projectHost is the host the current project runs on, Claude Code when there
// is no project or it does not say.
func projectHost() host.Host {
	if cfg, _, err := loadProjectConfig(); err == nil {
		if h, ok := host.Get(cfg.AI.Host); ok {
			return h
		}
	}
	return host.ClaudeCode
}

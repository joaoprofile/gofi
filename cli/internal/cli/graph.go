package cli

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/githooks"
	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
	"github.com/joaoprofile/gofi-cli/internal/graph/query"
	"github.com/joaoprofile/gofi-cli/internal/graph/workspace"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

func newGraphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: i18n.T("cmd.graph.short"),
		Long: `Build and query a code graph of this project.

The graph is extracted from the source itself — packages, types, functions and
the calls between them — never guessed. It is written to .gofi/graph/ as three
files: gofi_graph_report.md, the compact map an agent reads before scanning the
repository; gofi_graph.json, the source of truth; and gofi_graph.html, an
offline visualization.

gofi init builds it once. After that 'gofi graph build --update' keeps it in
step with the code, which is cheap enough to run from a git hook.`,
		Example: `gofi graph build
gofi graph build --deep
gofi graph explain Server.Start
gofi graph open`,
	}
	cmd.AddCommand(newGraphBuildCmd(), newGraphExplainCmd(), newGraphOpenCmd(),
		newGraphInstallCmd(), newGraphHooksCmd())
	return cmd
}

func newGraphHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: i18n.T("cmd.graph.hooks.short"),
		Long: `Install the git hooks that keep the graph in step with the code.

gofi writes a marked block into pre-commit, post-checkout and post-merge and
leaves the rest of each file alone, so a repository already using husky or
lefthook keeps working. The block runs 'gofi graph build --update', which is a
hash comparison when nothing changed.

The graph is tracked by git, so pre-commit rebuilds and stages it: the map ships
in the same commit as the code it describes. post-checkout and post-merge are
the repair pass, for when a merge resolved the generated file line by line.

With --remove the block is taken out and any hook that gofi created is deleted.
Without arguments, lists the hooks that currently carry the block.`,
		Example: `gofi graph hooks
gofi graph hooks --install
gofi graph hooks --remove`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphHooks(cmd)
		},
	}
	cmd.Flags().Bool("install", false, i18n.T("cmd.graph.flag.hooks_install"))
	cmd.Flags().Bool("remove", false, i18n.T("cmd.graph.flag.remove"))
	return cmd
}

func runGraphHooks(cmd *cobra.Command) error {
	root, err := graphProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	if remove, _ := cmd.Flags().GetBool("remove"); remove {
		results, err := githooks.Uninstall(root)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Fprintln(out, i18n.T("graph.hooks.none"))
			return nil
		}
		for _, r := range results {
			fmt.Fprintf(out, "  %-14s %s\n", r.Hook, r.Action)
		}
		return nil
	}

	if install, _ := cmd.Flags().GetBool("install"); install {
		results, err := githooks.Install(root, graphHookBodies())
		if err != nil {
			return err
		}
		for _, r := range results {
			fmt.Fprintf(out, "  %-14s %s  %s\n", r.Hook, r.Action, relativeTo(root, r.Path))
		}
		return nil
	}

	installed := githooks.Installed(root)
	if len(installed) == 0 {
		fmt.Fprintln(out, i18n.T("graph.hooks.none"))
		return nil
	}
	fmt.Fprintln(out, i18n.T("graph.hooks.list"))
	for _, hook := range installed {
		fmt.Fprintf(out, "  %s\n", hook)
	}
	return nil
}

func newGraphInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [language]",
		Short: i18n.T("cmd.graph.install.short"),
		Long: `Install the extractor for a language gofi does not read natively.

An extractor is a separate executable that speaks gofi's line protocol. It is
installed into .gofi/graph/extractors/ — inside the project, not globally — so
two projects can pin different versions and neither is affected by whatever
happens to be on PATH.

Give --from a local build or an https URL, and --sha256 to pin what you expect
to get. The digest of whatever was installed is always printed, so it can be
recorded and required on the next machine.

Without arguments, lists the extractors this project can already use.`,
		Example: `gofi graph install
gofi graph install java --from ./gofi-graph-java
gofi graph install rust --from https://example.com/gofi-graph-rust --sha256 6f1b...`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphInstall(cmd, args)
		},
	}
	cmd.Flags().String("from", "", i18n.T("cmd.graph.flag.from"))
	cmd.Flags().String("sha256", "", i18n.T("cmd.graph.flag.sha256"))
	cmd.Flags().Bool("remove", false, i18n.T("cmd.graph.flag.remove"))
	return cmd
}

func newGraphBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [path]",
		Short: i18n.T("cmd.graph.build.short"),
		Long: `Scan the project and write the graph to .gofi/graph/.

Which folders get scanned comes from .gofi.yaml, never from convention: the
backend under backend.path, the front-end and mobile trees under their own
path, and the vendored SDK. Each is a scope with its own graph, because each is
read by its own extractor. Giving a path scans that path instead, and is the
only way to override the declared layout.

Two modes. The default (fast) reads syntax only: it is quick, works on code that
does not compile, and refuses to guess when a call is ambiguous — those are
counted and reported rather than turned into a wrong edge. With --deep the Go
type-checker resolves every call exactly and interface implementations become
visible, at the cost of needing the project to build. Only in deep does an
absent edge prove an absent call, which is what makes it worth asking for
before concluding that nothing else uses a symbol.

The mode a project wants lives in .gofi.yaml (graph.deep). --deep asks for it on
one run; --fast refuses it on one run, and is what the git hooks pass so a
commit never waits for the type-checker. Deep only changes the Go scan: the
TypeScript extractor is syntactic and records itself as fast either way.

With --update the scan is skipped entirely when no .go file changed, which is
what makes this safe to run on every commit.

Go is extracted natively. Any other language is delegated to an installed
extractor ('gofi graph install <lang>') and written to .gofi/graph/<lang>/, so a
polyglot repository keeps one graph per language.`,
		Example: `gofi graph build
gofi graph build --deep
gofi graph build --update
gofi graph build --lang java
gofi graph build ./backend --open`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphBuild(cmd, args)
		},
	}
	f := cmd.Flags()
	f.String("lang", "", i18n.T("cmd.graph.flag.lang"))
	f.Duration("timeout", 10*time.Minute, i18n.T("cmd.graph.flag.timeout"))
	f.Bool("deep", false, i18n.T("cmd.graph.flag.deep"))
	f.Bool("fast", false, i18n.T("cmd.graph.flag.fast"))
	cmd.MarkFlagsMutuallyExclusive("deep", "fast")
	f.Bool("update", false, i18n.T("cmd.graph.flag.update"))
	f.Bool("open", false, i18n.T("cmd.graph.flag.open"))
	f.Bool("tests", false, i18n.T("cmd.graph.flag.tests"))
	f.Bool("no-html", false, i18n.T("cmd.graph.flag.no_html"))
	f.StringSlice("exclude", nil, i18n.T("cmd.graph.flag.exclude"))
	f.Int("max-file-kb", 2048, i18n.T("cmd.graph.flag.max_file_kb"))
	f.BoolP("verbose", "v", false, i18n.T("cmd.graph.flag.verbose"))
	return cmd
}

func newGraphExplainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <symbol>",
		Short: i18n.T("cmd.graph.explain.short"),
		Long: `Describe a symbol using the graph instead of opening files.

Prints where it is declared, its signature and documentation, what it calls,
what calls it, and the types it touches. Names may be given in short form:
'NewServer', 'api.NewServer' and the full node ID all resolve.

With --to, the output becomes the path between two symbols instead — every edge
from one to the other, which answers "how does A end up reaching B".`,
		Example: `gofi graph explain Server.Start
gofi graph explain api.NewServer
gofi graph explain handler.Login --to store.User`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphExplain(cmd, args)
		},
	}
	cmd.Flags().String("to", "", i18n.T("cmd.graph.flag.to"))
	cmd.Flags().Int("limit", 12, i18n.T("cmd.graph.flag.limit"))
	cmd.Flags().String("lang", "", i18n.T("cmd.graph.flag.lang"))
	return cmd
}

func newGraphOpenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "open",
		Short:   i18n.T("cmd.graph.open.short"),
		Long:    `Open .gofi/graph/gofi_graph.html in the default browser. The page is self-contained: no server and no network.`,
		Example: `gofi graph open`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := graphProjectRoot()
			if err != nil {
				return err
			}
			lang, _ := cmd.Flags().GetString("lang")
			return graph.Open(root, lang)
		},
	}
	cmd.Flags().String("lang", "", i18n.T("cmd.graph.flag.lang"))
	return cmd
}

func runGraphBuild(cmd *cobra.Command, args []string) error {
	f := cmd.Flags()
	verbose, _ := f.GetBool("verbose")
	deep, _ := f.GetBool("deep")
	fast, _ := f.GetBool("fast")
	update, _ := f.GetBool("update")
	open, _ := f.GetBool("open")
	tests, _ := f.GetBool("tests")
	noHTML, _ := f.GetBool("no-html")
	exclude, _ := f.GetStringSlice("exclude")
	maxKB, _ := f.GetInt("max-file-kb")
	lang, _ := f.GetString("lang")
	timeout, _ := f.GetDuration("timeout")

	// The scan root defaults to the project root, not the current directory, so
	// the graph is the same no matter where in the tree the command is run.
	found, err := findProjectRoot()
	if err != nil {
		return err
	}
	// findProjectRoot only returns a directory that has a .gofi.yaml, so a load
	// failure here is a broken file, not an absent one. Carrying on would graph
	// a tree the project never declared — silently, and into a map the agents
	// then read as the truth.
	cfg, err := config.Load(filepath.Join(found, config.FileName))
	if err != nil {
		return err
	}
	root := declaredRoot(cfg, found)

	var log *slog.Logger
	if verbose {
		log = slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	// Without an explicit path the command graphs the project as configured: the
	// backend, front-end and mobile folders declared in .gofi.yaml, plus the
	// vendored SDK. Scanning the raw root instead would fold every one of them
	// into a single graph, which is the layout no gofi project has.
	//
	// An explicit path is the developer overriding that layout, and is the only
	// thing that does.
	if len(args) == 0 && graphEnabled(cfg) {
		opt := graphOptions(cfg, root)
		// Another language reads the same declared sources through its own
		// extractor. The vendored SDK and the key that dates it belong to the
		// backend toolchain, so they do not carry over.
		if lang != "" && lang != opt.Language {
			opt.Language, opt.SDKKey = lang, ""
		}
		// opt.Deep arrives carrying what the project declared. --fast overrides
		// it so the git hooks, which run on every commit, never pay for the
		// type-checker in a project that asked for deep everywhere else.
		opt.Deep = deep || (opt.Deep && !fast)
		opt.WithTests = tests
		opt.NoHTML = noHTML
		opt.MaxFileKB = maxKB
		opt.Timeout = timeout
		opt.Update = update
		opt.Exclude = append(opt.Exclude, exclude...)
		opt.Log = log
		if err := buildWorkspace(cmd, opt, root); err != nil {
			return err
		}
		// The graph belongs in git, and a project scaffolded before it existed
		// ignores all of .gofi/ — writing it there would be writing it nowhere.
		// Only after a build that produced something, so a failed scan leaves
		// the repository as it found it.
		if err := ensureGofiIgnored(root); err != nil {
			return err
		}
		if open {
			return graph.Open(root, opt.Language)
		}
		return nil
	}

	scanRoot := root
	if len(args) > 0 {
		scanRoot = args[0]
	}

	opt := graph.BuildOptions{
		Root:      scanRoot,
		Language:  lang,
		Deep:      deep,
		WithTests: tests,
		Exclude:   exclude,
		MaxFileKB: maxKB,
		NoHTML:    noHTML,
		Update:    update,
		Timeout:   timeout,
	}
	opt.Log = log
	// Out is left empty on purpose: the default layout resolves against the scan
	// root, so graphing a subdirectory writes under that subdirectory and never
	// overwrites the project graph.
	res, err := graph.Build(cmd.Context(), opt)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if res.Unchanged {
		fmt.Fprintln(out, i18n.T("graph.build.unchanged", res.Files))
	} else {
		s := res.Graph.Stats
		fmt.Fprintln(out, i18n.T("graph.build.done",
			s.Packages, s.Nodes-s.Packages, s.Edges, s.DurationMS, res.Graph.Mode))
		if !deep && s.Ambiguous > 0 {
			fmt.Fprintln(out, i18n.T("graph.build.ambiguous", s.Ambiguous))
		}
		// An extractor's own diagnostics explain the gaps in its graph, so they
		// are shown rather than swallowed.
		for _, d := range res.Diagnostics {
			fmt.Fprintln(out, "  "+d.String())
		}
		if extra := res.DiagCount - len(res.Diagnostics); extra > 0 {
			fmt.Fprintln(out, i18n.T("graph.build.more_diags", extra))
		}
		fmt.Fprintln(out, i18n.T("graph.build.written",
			relativeTo(root, res.Dir), strings.Join(res.Outputs, ", ")))
	}

	if open {
		return graph.Open(root, lang)
	}
	return nil
}

func runGraphExplain(cmd *cobra.Command, args []string) error {
	root, err := graphProjectRoot()
	if err != nil {
		return err
	}
	lang, _ := cmd.Flags().GetString("lang")
	g, err := explainScope(root, lang, args[0])
	if err != nil {
		return err
	}
	limit, _ := cmd.Flags().GetInt("limit")
	to, _ := cmd.Flags().GetString("to")

	if to != "" {
		fmt.Fprint(cmd.OutOrStdout(), query.Path(g, args[0], to))
		return nil
	}
	// Several words mean the user is searching rather than naming a node, so
	// fall back to the wider search that shows each match with its neighbours.
	if len(args) > 1 {
		fmt.Fprint(cmd.OutOrStdout(), query.Query(g, strings.Join(args, " "), limit, 5))
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), query.Explain(g, args[0], limit))
	return nil
}

// graphProjectRoot is the project the graph commands operate on: the folder
// .gofi.yaml declares, not whichever directory the command was run from.
//
// Unlike the build, reading a graph tolerates a configuration it cannot parse —
// the map on disk is still there, and refusing to show it would help nobody.
func graphProjectRoot() (string, error) {
	found, err := findProjectRoot()
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(filepath.Join(found, config.FileName))
	if err != nil {
		return found, nil
	}
	return declaredRoot(cfg, found), nil
}

// scopeLabel names a scope by what it scanned and what that is written in. The
// folder is the one thing a developer cannot check by eye — which directory
// .gofi.yaml pointed the scan at — and a graph of the wrong tree looks
// perfectly healthy otherwise.
func scopeLabel(sc workspace.Scope, root string) string {
	s := fmt.Sprintf("%s (%s, %s", sc.Name, relativeTo(root, sc.Root), sc.Language)
	if sc.Framework != "" {
		s += " + " + sc.Framework
	}
	return s + ")"
}

// buildWorkspace builds every scope and reports them one line each. A scope
// that failed does not hide the ones that worked: a broken SDK checkout still
// leaves the project's own graph usable.
func buildWorkspace(cmd *cobra.Command, opt workspace.Options, root string) error {
	res, buildErr := workspace.Build(cmd.Context(), opt)
	out := cmd.OutOrStdout()

	for _, s := range res.Built() {
		g := s.Result.Graph
		label := scopeLabel(s.Scope, root)
		switch {
		case s.Skipped || s.Result.Unchanged:
			fmt.Fprintln(out, i18n.T("graph.build.scope_unchanged", label, s.Result.Files))
		default:
			st := g.Stats
			fmt.Fprintln(out, i18n.T("graph.build.scope_done", label,
				st.Packages, st.Nodes-st.Packages, st.Edges, st.DurationMS, g.Mode))
			if !opt.Deep && st.Ambiguous > 0 {
				fmt.Fprintln(out, "  "+i18n.T("graph.build.ambiguous", st.Ambiguous))
			}
			for _, d := range s.Result.Diagnostics {
				fmt.Fprintln(out, "  "+d.String())
			}
			if extra := s.Result.DiagCount - len(s.Result.Diagnostics); extra > 0 {
				fmt.Fprintln(out, "  "+i18n.T("graph.build.more_diags", extra))
			}
		}
	}
	if len(res.Built()) > 0 {
		fmt.Fprintln(out, i18n.T("graph.build.written",
			relativeTo(root, graph.Dir(root, opt.Language)), workspace.IndexFile))
	}
	return buildErr
}

// explainScope picks the graph that knows a symbol. A project calls into the
// vendored SDK constantly, and "not found" would be the wrong answer for a
// symbol that is right there in the next scope over.
func explainScope(root, lang, term string) (*model.Graph, error) {
	w := workspace.Load(root, lang)
	var first *model.Graph
	for _, s := range w.Index.Scopes {
		g, err := w.Graph(s.Name)
		if err != nil {
			continue
		}
		if len(query.Resolve(g, term)) > 0 {
			return g, nil
		}
		if first == nil {
			first = g
		}
	}
	if first != nil {
		return first, nil
	}
	// Nothing loaded at all: graph.Load is what says how to fix that.
	return graph.Load(root, lang)
}

func runGraphInstall(cmd *cobra.Command, args []string) error {
	root, err := graphProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	if len(args) == 0 {
		langs := external.Installed(root)
		if len(langs) == 0 {
			fmt.Fprintln(out, i18n.T("graph.install.none"))
			return nil
		}
		fmt.Fprintln(out, i18n.T("graph.install.list"))
		for _, lang := range langs {
			spec, err := external.Find(root, lang)
			if err != nil {
				continue
			}
			fmt.Fprintf(out, "  %-12s %s\n", lang, relativeTo(root, spec.Path))
		}
		return nil
	}

	lang := args[0]
	if remove, _ := cmd.Flags().GetBool("remove"); remove {
		if err := external.Uninstall(root, lang); err != nil {
			return err
		}
		fmt.Fprintln(out, i18n.T("graph.install.removed", lang))
		return nil
	}

	from, _ := cmd.Flags().GetString("from")
	sum, _ := cmd.Flags().GetString("sha256")
	res, err := external.Install(cmd.Context(), external.InstallOptions{
		ProjectRoot: root,
		Language:    lang,
		From:        from,
		SHA256:      sum,
	})
	if err != nil {
		return err
	}
	// An extractor is a downloaded binary, and a binary in a commit is a
	// mistake nobody notices until the repository has doubled in size. The
	// graph directory around it is versioned, so the exclusion only holds as
	// part of the whole rule set, in order.
	if err := ensureGofiIgnored(root); err != nil {
		return err
	}
	fmt.Fprintln(out, i18n.T("graph.install.done", lang, relativeTo(root, res.Path)))
	fmt.Fprintln(out, i18n.T("graph.install.sha256", res.SHA256))
	fmt.Fprintln(out, i18n.T("graph.install.next", lang))
	return nil
}

// relativeTo shortens an absolute output path for display, falling back to the
// absolute form when it is not under the project.
func relativeTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

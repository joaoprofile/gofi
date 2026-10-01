package cli

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/expertise"
	"github.com/joaoprofile/gofi/cli/internal/githooks"
	"github.com/joaoprofile/gofi/cli/internal/graph"
	"github.com/joaoprofile/gofi/cli/internal/graph/extract/external"
	"github.com/joaoprofile/gofi/cli/internal/graph/workspace"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/retrieval"
	"github.com/joaoprofile/gofi/cli/internal/sdkdoc"
	"os"
)

// Index targets: the code graph and the document index. With no target both
// are built — they go stale on the same edits, and an agent reads both.
const (
	indexCode = "code"
	indexDocs = "docs"
)

func newIndexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [code|docs]",
		Short: i18n.T("cmd.index.short"),
		Long: `Build what gofi find, show and path answer from: the code graph and the
document index. Without a target both are built.

code  reads the sources .gofi.yaml declares — the backend, each front-end
      surface, the vendored SDK — one graph per tree. Incremental: an unchanged
      tree is skipped, --full rebuilds it anyway. The default mode (fast) reads
      syntax only and never guesses an ambiguous call; --deep asks the Go
      type-checker, which is exact but needs the project to compile, and is the
      only mode in which a missing edge proves a missing call. --lang reads
      the backend tree with another language's extractor, as one more scope.

docs  indexes specs, PRDs and memory, and the reference libraries — knowledge,
      SDK docs and boilerplates, the institutional base, skill references —
      section by section with the lines each spans, and links each table a
      document declares to the code that touches it.

Both are derived and must follow every edit: a stale index is worse than none,
because it points confidently at the wrong place.`,
		Example: `gofi index
gofi index code --deep
gofi index docs
gofi index status`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{indexCode, indexDocs},
		RunE:      runIndex,
	}
	f := cmd.Flags()
	f.Bool("deep", false, i18n.T("cmd.index.flag.deep"))
	f.Bool("fast", false, i18n.T("cmd.index.flag.fast"))
	cmd.MarkFlagsMutuallyExclusive("deep", "fast")
	f.Bool("full", false, i18n.T("cmd.index.flag.full"))
	f.String("lang", "", i18n.T("cmd.index.flag.lang"))
	f.Bool("tests", false, i18n.T("cmd.index.flag.tests"))
	f.StringSlice("exclude", nil, i18n.T("cmd.index.flag.exclude"))
	f.Int("max-file-kb", 2048, i18n.T("cmd.index.flag.max_file_kb"))
	f.Duration("timeout", 10*time.Minute, i18n.T("cmd.index.flag.timeout"))
	f.Bool("no-html", false, i18n.T("cmd.index.flag.no_html"))
	f.BoolP("verbose", "v", false, i18n.T("cmd.index.flag.verbose"))
	cmd.AddCommand(newIndexStatusCmd(), newIndexHooksCmd(), newIndexCheckCmd(), newIndexMigrateCmd(), newIndexInstallCmd(), newIndexOpenCmd())
	return cmd
}

func runIndex(cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) == 1 {
		target = args[0]
		if target != indexCode && target != indexDocs {
			return fmt.Errorf("index %q: use %s or %s", target, indexCode, indexDocs)
		}
	}
	found, err := findProjectRoot()
	if err != nil {
		return err
	}
	// findProjectRoot only returns a folder with a .gofi.yaml, so a load failure
	// is a broken file, not an absent one. Indexing anyway would map a tree the
	// project never declared, and the agents would read it as the truth.
	cfg, err := config.Load(filepath.Join(found, config.FileName))
	if err != nil {
		return err
	}
	root := declaredRoot(cfg, found)
	out := cmd.OutOrStdout()

	if note := removeLegacyHooks(root); note != "" {
		fmt.Fprintln(out, note)
	}
	if err := migrateLayout(root, backendLanguage(cfg), out); err != nil {
		return err
	}
	if target != indexDocs {
		if err := indexCodeGraph(cmd, cfg, root); err != nil {
			return err
		}
	}
	if target != indexCode {
		if err := indexDocuments(cmd, cfg, root); err != nil {
			return err
		}
	}
	return nil
}

func indexCodeGraph(cmd *cobra.Command, cfg *config.GofiConfig, root string) error {
	if !graphEnabled(cfg) {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("index.code.disabled"))
		return nil
	}
	f := cmd.Flags()
	deep, _ := f.GetBool("deep")
	fast, _ := f.GetBool("fast")
	full, _ := f.GetBool("full")
	lang, _ := f.GetString("lang")
	tests, _ := f.GetBool("tests")
	exclude, _ := f.GetStringSlice("exclude")
	maxKB, _ := f.GetInt("max-file-kb")
	timeout, _ := f.GetDuration("timeout")
	noHTML, _ := f.GetBool("no-html")
	verbose, _ := f.GetBool("verbose")

	opt := graphOptions(cfg, root)
	// Another language reads the backend tree as one more scope of the project,
	// filed in the same index as the others, so find, show and path reach it
	// with no flag. Only that scope is built; the index keeps the rest.
	if lang != "" && lang != opt.Language {
		if !workspace.CanExtract(root, lang) {
			return fmt.Errorf("%s", i18n.T("index.code.no_extractor", lang))
		}
		dir := opt.SrcDir
		if dir == "" {
			dir = "."
		}
		opt.SkipProject, opt.SkipSDK = true, true
		opt.Surfaces = []workspace.Surface{{Name: lang, Dir: dir, Language: lang}}
	}
	// opt.Deep arrives as the project declared it; --deep and --fast override
	// it for one run.
	opt.Deep = deep || (opt.Deep && !fast)
	opt.Update = !full
	opt.WithTests, opt.NoHTML, opt.MaxFileKB, opt.Timeout = tests, noHTML, maxKB, timeout
	opt.Exclude = append(opt.Exclude, exclude...)
	if verbose {
		opt.Log = slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	res, err := buildWorkspace(cmd, opt, root)
	if res != nil {
		if rerr := recordCodeBuild(root, opt, res.Index); rerr != nil && err == nil {
			err = rerr
		}
	}
	if err != nil {
		return err
	}
	// The graph belongs in git, and a project scaffolded before it existed
	// ignores all of .gofi/ — writing it there would be writing it nowhere.
	return ensureGofiIgnored(root)
}

func indexDocuments(cmd *cobra.Command, cfg *config.GofiConfig, root string) error {
	b := &docs.Builder{Root: root, WithCode: true, Language: backendLanguage(cfg)}
	idx, g, drift, err := b.Build()
	if err != nil {
		return err
	}
	sections := 0
	for _, d := range idx.Docs {
		sections += len(d.Sections)
	}
	if err := recordDocsBuild(root, b); err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, i18n.T("index.docs.done", len(idx.Docs), sections, len(g.Nodes), len(g.Edges), docs.OutDir))
	for _, d := range drift {
		fmt.Fprintln(out, "  "+i18n.T("index.docs.drift", d))
	}
	return nil
}

// removeLegacyHooks takes out the git hook block older releases installed. It
// ran `gofi graph build` and `gofi docs build`, which no longer exist: left in
// place it would fail on every commit — silently, by design — and the indexes
// would quietly stop following the code.
func removeLegacyHooks(root string) string {
	if len(githooks.Legacy(root)) == 0 {
		return ""
	}
	results, err := githooks.UninstallLegacy(root)
	if err != nil || len(results) == 0 {
		return ""
	}
	return i18n.T("index.hooks.legacy_removed", len(results))
}

// --- check ----------------------------------------------------------------

func newIndexCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: i18n.T("cmd.index.check.short"),
		Long: `Check the document corpus and measure the search. For CI: exits non-zero
when the corpus has format errors.

Format: a missing required field, an unknown field, a facet term that is not in
the lexicon, an expertise pack whose contract does not load, SDK conventions
citing an API the pinned SDK checkout does not have, a team correction whose
overrides: points at a rule that no longer exists (errors); a context memory grown past a consolidated state
(warning). The lexicon check keeps the vocabulary from rotting into a tag cloud:
a new term enters the lexicon before it enters a document. Nothing is
rewritten.

Search: when .claude/eval/golden.md exists, every question in it is asked and
the answer is scored — is the document in the first results, and does it point
at the right section. Run it before and after a change to the corpus; a change
that lowers the score is a change to revert.

Orphans: specs and PRDs nothing references. Context memory and the reference
libraries are left out unless --all: they are reached by searching, not cited.`,
		Example: `gofi index check
gofi index check --lexicon
gofi index check --all`,
		Args: cobra.NoArgs,
		RunE: runIndexCheck,
	}
	cmd.Flags().Bool("lexicon", false, i18n.T("cmd.index.flag.lexicon"))
	cmd.Flags().Bool("all", false, i18n.T("cmd.index.flag.all"))
	return cmd
}

func runIndexCheck(cmd *cobra.Command, _ []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()

	findings := docs.Validate(root)
	// A pack whose contract does not load routes nothing: as much an error as
	// a document missing its frontmatter.
	_, problems := expertise.Load(root)
	for _, pr := range problems {
		findings = append(findings, docs.Finding{Path: pr.Path, Message: pr.Message, Error: true})
	}
	findings = append(findings, sdkDrift(root)...)
	findings = append(findings, docs.CheckOverrides(root)...)
	errs := 0
	for _, f := range findings {
		kind := i18n.T("index.check.warn")
		if f.Error {
			kind, errs = i18n.T("index.check.error"), errs+1
		}
		fmt.Fprintf(out, "  %s %s: %s\n", kind, f.Path, f.Message)
	}
	if full, _ := cmd.Flags().GetBool("lexicon"); full {
		if unused := docs.ValidateLexicon(root); len(unused) > 0 {
			fmt.Fprintln(out, "\n  "+i18n.T("index.check.unused_terms", len(unused)))
			for _, u := range unused {
				fmt.Fprintf(out, "      %s\n", u)
			}
		}
		if tables := docs.UndocumentedTables(root); len(tables) > 0 {
			fmt.Fprintln(out, "\n  "+i18n.T("index.check.unclaimed_tables", len(tables)))
			for _, t := range tables {
				fmt.Fprintf(out, "      %s\n", t)
			}
		}
	}
	fmt.Fprintln(out, "\n  "+i18n.T("index.check.format", errs, len(findings)-errs))

	if _, g, _ := docs.Load(root); g != nil {
		all, _ := cmd.Flags().GetBool("all")
		list, total := (&docs.Explorer{Graph: g}).Orphans(all)
		fmt.Fprintln(out, "  "+i18n.T("index.check.orphans", len(list), total))
		for _, o := range list {
			fmt.Fprintf(out, "      %s\n", o)
		}
	}

	if questions, err := docs.ReadGolden(root); err == nil && len(questions) > 0 {
		language := ""
		if cfg, _, err := loadProjectConfig(); err == nil {
			language = backendLanguage(cfg)
		}
		engine, err := retrieval.Open(root, language)
		if err != nil {
			return err
		}
		ev := engine.Evaluate(questions)
		pct := func(n int) int { return 100 * n / max(ev.Questions, 1) }
		fmt.Fprintln(out, "  "+i18n.T("index.check.search", ev.Questions, pct(ev.DocTop1), pct(ev.DocTop3), pct(ev.SectionTop3), ev.MRR))
		for _, q := range ev.Misses {
			fmt.Fprintf(out, "      %s %s → %s\n", q.ID, q.Ask, q.Document)
		}
	}

	if errs > 0 {
		return fmt.Errorf("%s", i18n.T("index.check.failed", errs))
	}
	return nil
}

// --- migrate --------------------------------------------------------------

func newIndexMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [step]",
		Short: i18n.T("cmd.index.migrate.short"),
		Long: `Bring a document corpus written in an older format up to the current one.

Steps, in order, each idempotent:

  stamp     write the format marker every later step keys off
  lexicon   derive the controlled vocabulary from the project itself —
            entities from the schema, operations from the keywords already
            shared between contexts. Nothing is invented
  facets    replace the free tag cloud with the controlled axes
  diagrams  lift PlantUML out of the specs into .puml files

Without a step, all four run in order. Without --apply nothing is written: read
the sample first. Marketplaces and synonyms are hand-kept and never
overwritten. Run 'gofi index check' before and after — a migration that lowers
the search score is a migration to revert.`,
		Example: `gofi index migrate
gofi index migrate facets
gofi index migrate --apply`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			apply, _ := cmd.Flags().GetBool("apply")
			m := &docs.Migrator{Root: root, Apply: apply}
			steps := map[string]func() (*docs.MigrateResult, error){
				"stamp": m.Stamp, "lexicon": m.Lexicon, "facets": m.Facets, "diagrams": m.Diagrams,
			}
			order := []string{"stamp", "lexicon", "facets", "diagrams"}
			if len(args) == 1 {
				if _, ok := steps[args[0]]; !ok {
					return fmt.Errorf("%s", i18n.T("index.migrate.unknown", args[0], strings.Join(order, ", ")))
				}
				order = []string{args[0]}
			}
			out := cmd.OutOrStdout()
			for _, name := range order {
				res, err := steps[name]()
				if err != nil {
					return err
				}
				fmt.Fprintln(out, "  "+i18n.T("index.migrate.step", res.Step, res.Changed, res.Skipped))
				for _, n := range res.Notes {
					fmt.Fprintf(out, "            %s\n", n)
				}
				for _, sm := range res.Samples {
					fmt.Fprintf(out, "\n    %s\n", sm)
				}
			}
			if !apply {
				fmt.Fprintln(out, "\n  "+i18n.T("index.migrate.dry"))
			}
			return nil
		},
	}
	cmd.Flags().Bool("apply", false, i18n.T("cmd.index.flag.apply"))
	return cmd
}

// --- install and open -----------------------------------------------------

func newIndexInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [language]",
		Short: i18n.T("cmd.index.install.short"),
		Long: `Install the extractor that graphs a language gofi does not read natively.

Go is built in. Any other language is read by an external extractor — a
binary that writes the same graph format — installed per project under
.gofi/extractors/, never committed. Without arguments, lists the
installed extractors. The download is verified against --sha256 when given.`,
		Example: `gofi index install
gofi index install java --from ./gofi-graph-java
gofi index install rust --from https://example.com/gofi-graph-rust --sha256 6f1b...
gofi index install java --remove`,
		Args: cobra.MaximumNArgs(1),
		RunE: runIndexInstall,
	}
	cmd.Flags().String("from", "", i18n.T("cmd.index.flag.from"))
	cmd.Flags().String("sha256", "", i18n.T("cmd.index.flag.sha256"))
	cmd.Flags().Bool("remove", false, i18n.T("cmd.index.flag.remove"))
	return cmd
}

func runIndexInstall(cmd *cobra.Command, args []string) error {
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
	res, err := external.Install(cmd.Context(), external.InstallOptions{ProjectRoot: root, Language: lang, From: from, SHA256: sum})
	if err != nil {
		return err
	}
	// An extractor is a downloaded binary, and a binary in a commit is a
	// mistake nobody notices until the repository has doubled in size.
	if err := ensureGofiIgnored(root); err != nil {
		return err
	}
	fmt.Fprintln(out, i18n.T("graph.install.done", lang, relativeTo(root, res.Path)))
	fmt.Fprintln(out, i18n.T("graph.install.sha256", res.SHA256))
	fmt.Fprintln(out, i18n.T("graph.install.next", lang))
	return nil
}

func newIndexOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [scope]",
		Short: i18n.T("cmd.index.open.short"),
		Long: `Open the visualization of one code graph in the default browser. Without a
scope, the first the index lists — the backend, when there is one. The page is
self-contained: no server and no network.`,
		Example: `gofi index open
gofi index open frontend`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := graphProjectRoot()
			if err != nil {
				return err
			}
			ix, err := workspace.LoadIndex(root)
			if err != nil || len(ix.Scopes) == 0 {
				return fmt.Errorf("%s", i18n.T("index.open.none"))
			}
			scope := ix.Scopes[0]
			if len(args) == 1 {
				var ok bool
				if scope, ok = ix.Scope(args[0]); !ok {
					names := make([]string, 0, len(ix.Scopes))
					for _, s := range ix.Scopes {
						names = append(names, s.Name)
					}
					return fmt.Errorf("%s", i18n.T("index.open.unknown", args[0], strings.Join(names, ", ")))
				}
			}
			return graph.Open(filepath.Join(graph.Dir(root), filepath.FromSlash(scope.Dir)))
		},
	}
}

// --- shared ---------------------------------------------------------------

// graphProjectRoot is the project the index commands operate on: the folder
// .gofi.yaml declares, not whichever directory the command was run from.
// Reading tolerates a configuration it cannot parse — the index on disk is
// still there, and refusing to show it would help nobody.
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

// scopeLabel names a scope by what it scanned and what that is written in: the
// folder is the one thing a developer cannot check by eye, and a graph of the
// wrong tree looks perfectly healthy otherwise.
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
func buildWorkspace(cmd *cobra.Command, opt workspace.Options, root string) (*workspace.Result, error) {
	res, buildErr := workspace.Build(cmd.Context(), opt)
	out := cmd.OutOrStdout()
	for _, s := range res.Built() {
		g := s.Result.Graph
		label := scopeLabel(s.Scope, root)
		if s.Skipped || s.Result.Unchanged {
			fmt.Fprintln(out, i18n.T("graph.build.scope_unchanged", label, s.Result.Files))
			continue
		}
		st := g.Stats
		fmt.Fprintln(out, i18n.T("graph.build.scope_done", label, st.Packages, st.Nodes-st.Packages, st.Edges, st.DurationMS, g.Mode))
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
	if len(res.Built()) > 0 {
		fmt.Fprintln(out, i18n.T("graph.build.written", relativeTo(root, graph.Dir(root)), workspace.IndexFile))
	}
	return res, buildErr
}

// relativeTo shortens an absolute path for display, falling back to the
// absolute form when it is not under the project.
func relativeTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

// sdkDrift reports SDK conventions and boilerplates that cite an API the
// project's SDK checkout does not have. The reference is generated and cannot
// drift; what people write about the SDK can, and this is where it shows.
// Without a checkout there is nothing to compare against, and nothing is said.
func sdkDrift(root string) []docs.Finding {
	checkout := filepath.Join(root, ".gofi", "gofi-sdk-"+config.LanguageGo)
	if fi, err := os.Stat(checkout); err != nil || !fi.IsDir() {
		return nil
	}
	exports, err := sdkdoc.ReadExports(checkout)
	if err != nil {
		return nil
	}
	sdk := layout.SDK().Path(config.LanguageGo)
	var out []docs.Finding
	for _, d := range exports.CheckDocs(root, sdk+"/knowledge", sdk+"/boilerplates") {
		out = append(out, docs.Finding{Path: d.Path, Error: true,
			Message: "cites SDK API the pinned checkout does not have: " + strings.Join(d.Stale, ", ")})
	}
	return out
}

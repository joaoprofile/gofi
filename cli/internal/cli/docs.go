package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/docs"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: i18n.T("cmd.docs.short"),
		Long: `Build and query a retrieval index over this project's own documents.

Where 'gofi graph' maps the code, 'gofi docs' maps what was written about it —
specs, PRDs and per-context memory. The index is written to .gofi/docs/ as
index.json, the sections of every document with the line range each covers, and
graph.json, the links between documents.

Those links are not new: frontmatter cross-references, document paths cited in
prose, [[wikilinks]] and shared entities were always there, just never
materialized. Building the graph only writes them down.

The two graphs meet at the entity. A schema table name is the one symbol that
exists in both the documents and the code, so 'docs explain --entity' answers
"which spec owns this table" and "which code implements it" as one question.`,
		Example: `gofi docs build
gofi docs validate
gofi docs migrate --apply
gofi docs eval`,
	}
	cmd.AddCommand(newDocsBuildCmd(), newDocsValidateCmd(), newDocsMigrateCmd(),
		newDocsEvalCmd(), newDocsHooksCmd())
	return cmd
}

func newDocsBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: i18n.T("cmd.docs.build.short"),
		Long: `Scan the corpora and (re)build the index and the document graph.

Both are derived: every edit to a document makes them a little stale, and a
stale index is worse than none — without an index an agent knows it does not
know, while a stale one points confidently at the wrong place. Cheap enough to
run from a git hook.

--with-code additionally links each entity to the source files that mention it,
which costs a pass over the tracked files. Without it the build only reads the
documents.

--staged is the pre-commit mode: it indexes only what the commit stages. With
no staged document it touches nothing; otherwise it rewrites only the INDEX.md
of the staged folders and only their contexts' rows in the router, writes a
file only when its content changes, and stages exactly what it rewrote.`,
		Example: `gofi docs build
gofi docs build --with-code
gofi docs build --staged --with-code`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			withCode, _ := cmd.Flags().GetBool("with-code")
			// The corpus does not depend on the config — the language only
			// picks which source extensions the code index scans — so a broken
			// .gofi.yaml degrades the build instead of stopping it. But it is
			// said out loud: the validation message is the one thing that
			// explains why, and swallowing it leaves the user with a build that
			// quietly indexed no code.
			cfg, _, cfgErr := loadProjectConfig()
			if cfgErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  aviso: %v\n  seguindo sem a linguagem do backend\n", cfgErr)
			}
			b := &docs.Builder{Root: root, WithCode: withCode, Language: backendLanguage(cfg)}
			staged, _ := cmd.Flags().GetBool("staged")
			if staged {
				paths, err := stagedPaths(root)
				if err != nil {
					return err
				}
				b.Scope = docs.StagedScope(paths)
			}
			idx, g, drift, err := b.Build()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if staged {
				if err := stageFiles(root, b.Written); err != nil {
					return err
				}
				if idx == nil {
					fmt.Fprintln(out, "  nada no stage para indexar")
					return nil
				}
			}
			sections := 0
			for _, d := range idx.Docs {
				sections += len(d.Sections)
			}
			fmt.Fprintf(out, "  %d documents, %d sections\n", len(idx.Docs), sections)
			fmt.Fprintf(out, "  %d nodes, %d edges\n", len(g.Nodes), len(g.Edges))
			for _, d := range drift {
				fmt.Fprintf(out, "  drift: %s\n", d)
			}
			return nil
		},
	}
	cmd.Flags().Bool("with-code", false, i18n.T("cmd.docs.flag.withcode"))
	cmd.Flags().Bool("staged", false, "index only what the commit stages, and stage what it rewrote (pre-commit mode)")
	return cmd
}

// stagedPaths lists the paths the next commit stages, relative to root.
// --no-renames reports a move as a deletion plus an addition, so the folder a
// document left is re-indexed as well as the one it entered.
func stagedPaths(root string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--no-renames", "--relative", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list staged files: %w", err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// stageFiles adds (or records the removal of) exactly the Markdown indexes a
// scoped build rewrote, so they ship in the commit that made them change. The
// JSON under docs.OutDir is left to the hook's own `git add`: forcing it here
// would start tracking it in a project that chose to ignore it.
func stageFiles(root string, files []string) error {
	out := filepath.Join(root, docs.OutDir) + string(filepath.Separator)
	var md []string
	for _, f := range files {
		if !strings.HasPrefix(f, out) {
			md = append(md, f)
		}
	}
	if len(md) == 0 {
		return nil
	}
	args := append([]string{"add", "-A", "--"}, md...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stage rebuilt indexes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func newDocsValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: i18n.T("cmd.docs.validate.short"),
		Long: `Check the corpus against the format and the project's own lexicon.

Errors: a missing required field, an unknown field, a facet term that is not in
the lexicon. Warnings: a context memory that has grown past the point of being
a consolidated state.

The lexicon check is what keeps the vocabulary from rotting back into a tag
cloud. Left unchecked, terms used by exactly one document accumulate until the
index stops telling anything apart — a new term has to enter the lexicon before
it enters a document.

Nothing is ever rewritten here. In particular no field is renamed: spec/specs
and prd/prds are different fields — singular names the primary document, plural
lists them all — and they coexist in the same file, so "normalizing" one into
the other silently destroys the list.

Exits non-zero when there are errors, so it can gate a commit.`,
		Example: `gofi docs validate
gofi docs validate --lexicon`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			findings := docs.Validate(root)
			errors := 0
			for _, f := range findings {
				kind := "AVISO"
				if f.Error {
					kind, errors = "ERRO ", errors+1
				}
				fmt.Fprintf(out, "  %s %s: %s\n", kind, f.Path, f.Message)
			}
			if full, _ := cmd.Flags().GetBool("lexicon"); full {
				if unused := docs.ValidateLexicon(root); len(unused) > 0 {
					fmt.Fprintf(out, "\n  %d termos no léxico que documento nenhum usa:\n", len(unused))
					for _, u := range unused {
						fmt.Fprintf(out, "       %s\n", u)
					}
				}
				if tables := docs.UndocumentedTables(root); len(tables) > 0 {
					fmt.Fprintf(out, "\n  %d tabelas do schema que documento nenhum reivindica:\n", len(tables))
					for _, t := range tables {
						fmt.Fprintf(out, "       %s\n", t)
					}
				}
			}
			fmt.Fprintf(out, "\n  %d erros | %d avisos\n", errors, len(findings)-errors)
			if errors > 0 {
				return fmt.Errorf("%d erros de formato no corpus", errors)
			}
			return nil
		},
	}
	cmd.Flags().Bool("lexicon", false, i18n.T("cmd.docs.flag.lexicon"))
	return cmd
}

func newDocsMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [step]",
		Short: i18n.T("cmd.docs.migrate.short"),
		Long: `Bring a corpus written in an older format up to the current one.

Steps, in order, each idempotent so running twice changes nothing:

  stamp     write the format marker every later step keys off
  lexicon   derive the controlled vocabulary from the project itself —
            entities from the schema, operations from the keywords already
            shared between contexts. Nothing is invented
  facets    replace the free tag cloud with the controlled axes
  diagrams  lift PlantUML out of the specs into .puml files

Without a step name, all four run in order. Without --apply nothing is written:
read the sample first, since a detected entity should be the table the document
OWNS, not every word it happens to mention.

Marketplaces and synonyms are never overwritten — they are hand-kept, and the
synonym bridge is what makes a question in one language find a heading named in
another.

Run 'gofi docs eval' before and after. A migration that lowers recall or raises
cost is a migration to revert, which is why --apply wants a clean tree.`,
		Example: `gofi docs migrate
gofi docs migrate facets
gofi docs migrate --apply`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			apply, _ := cmd.Flags().GetBool("apply")
			m := &docs.Migrator{Root: root, Apply: apply}
			steps := map[string]func() (*docs.MigrateResult, error){
				"stamp": m.Stamp, "lexicon": m.Lexicon,
				"facets": m.Facets, "diagrams": m.Diagrams,
			}
			order := []string{"stamp", "lexicon", "facets", "diagrams"}
			if len(args) == 1 {
				if _, ok := steps[args[0]]; !ok {
					return fmt.Errorf("passo %q não existe; use um de: %s",
						args[0], strings.Join(order, ", "))
				}
				order = []string{args[0]}
			}
			out := cmd.OutOrStdout()
			for _, name := range order {
				res, err := steps[name]()
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "  %-9s %d alterados, %d já no formato\n",
					res.Step, res.Changed, res.Skipped)
				for _, n := range res.Notes {
					fmt.Fprintf(out, "            %s\n", n)
				}
				for _, sm := range res.Samples {
					fmt.Fprintf(out, "\n    %s\n", sm)
				}
			}
			if !apply {
				fmt.Fprintln(out, "\n  (nada escrito — repita com --apply)")
			}
			return nil
		},
	}
	cmd.Flags().Bool("apply", false, i18n.T("cmd.docs.flag.apply"))
	return cmd
}

func newDocsEvalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: i18n.T("cmd.docs.eval.short"),
		Long: `Measure the index against the project's own golden set.

The questions live in ` + "`" + `.claude/eval/golden.md` + "`" + `: what someone asks, and where the
answer actually is. Write them in the language people ask in, and do not reuse
the document's own keywords — a question written with the exact keyword only
measures itself.

Run it before and after any change to the corpus or the lexicon. A change that
lowers recall or raises cost is a change to revert; without a measurement it is
just a change everyone hopes was an improvement.

The score is lexical, so read it as the index's discriminating power and as a
comparison between two runs — not as how often an agent succeeds.`,
		Example: `gofi docs eval`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			idx, _, err := docs.Load(root)
			if idx == nil {
				return fmt.Errorf("sem índice em %s — rode 'gofi docs build' antes", docs.OutDir)
			}
			res, err := docs.Evaluate(root, idx, docs.NewSearcher(root, idx))
			if err != nil {
				return fmt.Errorf("sem conjunto-ouro em %s: %w", docs.GoldenFile, err)
			}
			fmt.Fprint(cmd.OutOrStdout(), res.Format())
			return nil
		},
	}
	return cmd
}

func newDocsHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: i18n.T("cmd.docs.hooks.short"),
		Long: `Install the git hooks that keep the index in step with the documents.

gofi writes one marked block into pre-commit, post-checkout and post-merge, and
leaves the rest of each file alone, so a repository already using husky or
lefthook keeps working. That block rebuilds every derived artifact — the code
graph and the document index — because they go stale on the same edits and
splitting them across two blocks would have each installer delete the other's.

The files are tracked, so pre-commit rebuilds and stages them: the map ships in
the same commit as the documents it describes. The document pass is scoped to
what the commit stages (docs build --staged), so a commit never rewrites the
index of a context it did not touch. post-checkout and post-merge rebuild only
the code graph and leave the tracked indexes alone.

Without the hooks the index depends on somebody remembering, and a stale index
is worse than none: without one an agent knows it does not know, with a stale
one it points confidently at the wrong place.

With --remove the block is taken out. Without arguments, lists the hooks that
currently carry it.`,
		Example: `gofi docs hooks
gofi docs hooks --install
gofi docs hooks --remove`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraphHooks(cmd)
		},
	}
	cmd.Flags().Bool("install", false, i18n.T("cmd.graph.flag.hooks_install"))
	cmd.Flags().Bool("remove", false, i18n.T("cmd.graph.flag.remove"))
	return cmd
}

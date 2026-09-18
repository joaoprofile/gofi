package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/docs"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

func newFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find [question]",
		Short: i18n.T("cmd.find.short"),
		Long: `Ask this project's documents a question, instead of loading their index.

Prints the documents that answer it, each with the section that matched and the
lines it spans, so the next read is those lines and not the whole file.

Matching runs over the section headings as well as the frontmatter facets:
headings are written in natural language, which is the language the question
arrives in, while facets are labels. With .claude/lexicon/sinonimos.md the terms
are bridged across languages, which is what lets a question in one language find
a heading named in another.

The flags navigate instead of searching. --entity is the one that crosses over
into the code: a schema table is the only symbol that exists both in the
documents and in the source, so it answers which spec owns a table and which
code implements it as one question.

Nothing found usually means the wrong words, not a missing document. Try the
technical term, and record the pair that was missing in the lexicon.`,
		Example: `gofi find "ordem de delete chave estrangeira"
gofi find --entity core_company
gofi find --context pricing
gofi find --links sdd-churn.md
gofi find --orphans`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFind(cmd, args)
		},
	}
	cmd.Flags().IntP("limit", "n", 5, i18n.T("cmd.docs.flag.limit"))
	cmd.Flags().String("entity", "", i18n.T("cmd.docs.flag.entity"))
	cmd.Flags().String("context", "", i18n.T("cmd.docs.flag.context"))
	cmd.Flags().String("links", "", i18n.T("cmd.docs.flag.links"))
	cmd.Flags().Bool("orphans", false, i18n.T("cmd.docs.flag.orphans"))
	cmd.Flags().Bool("hubs", false, i18n.T("cmd.docs.flag.hubs"))
	cmd.Flags().Bool("all", false, i18n.T("cmd.docs.flag.all"))
	cmd.Flags().Bool("json", false, i18n.T("cmd.docs.flag.json"))
	return cmd
}

func runFind(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	entity, _ := cmd.Flags().GetString("entity")
	context, _ := cmd.Flags().GetString("context")
	links, _ := cmd.Flags().GetString("links")
	orphans, _ := cmd.Flags().GetBool("orphans")
	hubs, _ := cmd.Flags().GetBool("hubs")
	navigating := entity != "" || context != "" || links != "" || orphans || hubs

	if !navigating {
		if len(args) == 0 {
			return cmd.Help()
		}
		idx, _, err := docs.Load(root)
		if idx == nil {
			return fmt.Errorf("sem índice em %s — rode 'gofi docs build' antes", docs.OutDir)
		}
		_ = err
		limit, _ := cmd.Flags().GetInt("limit")
		out := cmd.OutOrStdout()
		results := docs.NewSearcher(root, idx).Search(strings.Join(args, " "), limit)
		if len(results) == 0 {
			fmt.Fprintln(out, "  nada encontrado — tente o termo técnico, ou registre o par")
			fmt.Fprintf(out, "  que faltou em %s/sinonimos.md\n", docs.LexiconDir)
			return nil
		}
		for _, r := range results {
			fmt.Fprintf(out, "  [%2d] %s  (%s)\n", r.Score, r.Doc.Path, r.Doc.Context)
			if r.Section != nil {
				fmt.Fprintf(out, "       §%s  L%d-%d\n", r.Section.Heading, r.Section.Start, r.Section.End)
			}
		}
		return nil
	}
	return runNavigate(cmd, root, entity, context, links, orphans, hubs)
}

func runNavigate(cmd *cobra.Command, root, entity, context, links string,
	orphans, hubs bool) error {
	_, g, err := docs.Load(root)
	if g == nil {
		return fmt.Errorf("sem grafo em %s — rode 'gofi docs build' antes: %w", docs.OutDir, err)
	}
	e := &docs.Explorer{Graph: g}
	out := cmd.OutOrStdout()
	limit, _ := cmd.Flags().GetInt("limit")
	asJSON, _ := cmd.Flags().GetBool("json")

	switch {
	case entity != "":
		documents, code, ok := e.Entity(entity)
		if !ok {
			near := e.EntitiesLike(entity)
			fmt.Fprintf(out, "  %q não está no grafo.", entity)
			if len(near) > 0 {
				fmt.Fprintf(out, " Perto: %s", strings.Join(near, ", "))
			}
			fmt.Fprintln(out)
			return nil
		}
		fmt.Fprintf(out, "  %s\n\n  documentado em %d:\n", entity, len(documents))
		for _, d := range documents {
			fmt.Fprintf(out, "       %s\n", d)
		}
		fmt.Fprintf(out, "\n  implementado em %d:\n", len(code))
		for i, c := range code {
			if i == limit {
				fmt.Fprintf(out, "       … +%d\n", len(code)-limit)
				break
			}
			fmt.Fprintf(out, "       %s\n", c)
		}
		return nil

	case context != "":
		tree, ok := e.Tree(context)
		if !ok {
			fmt.Fprintf(out, "  contextos: %s\n", strings.Join(e.Contexts(), ", "))
			return nil
		}
		if asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(tree)
		}
		renderTree(out, tree)
		return nil

	case links != "":
		id := e.Resolve(links)
		if id == "" {
			return fmt.Errorf("%q não está no grafo de documentos", links)
		}
		n := e.Explain(id)
		fmt.Fprintf(out, "  %s   [%s]\n", n.ID, n.Kind)
		if len(n.Incoming) > 0 {
			fmt.Fprintf(out, "\n  <- apontam para cá (%d):\n", len(n.Incoming))
			for i, l := range n.Incoming {
				if i == limit {
					break
				}
				fmt.Fprintf(out, "       [%s] %s\n", l.Kind, l.Node)
			}
		}
		if len(n.Outgoing) > 0 {
			fmt.Fprintf(out, "\n  -> aponta para (%d):\n", len(n.Outgoing))
			for i, l := range n.Outgoing {
				if i == limit {
					break
				}
				fmt.Fprintf(out, "       [%s] %s\n", l.Kind, l.Node)
			}
		}
		if len(n.Code) > 0 {
			shown := n.Code
			if len(shown) > 6 {
				shown = shown[:6]
			}
			fmt.Fprintf(out, "\n  -> código (%d): %s\n", len(n.Code), strings.Join(shown, ", "))
		}
		return nil

	case orphans:
		all, _ := cmd.Flags().GetBool("all")
		list, total := e.Orphans(all)
		note := " (memória de contexto fora: é ponto de entrada)"
		if all {
			note = ""
		}
		fmt.Fprintf(out, "  %d de %d documentos que ninguém referencia%s:\n\n", len(list), total, note)
		for _, o := range list {
			fmt.Fprintf(out, "       %s\n", o)
		}
		return nil

	case hubs:
		fmt.Fprint(out, "  mais referenciados:\n\n")
		for _, h := range e.Hubs(limit) {
			fmt.Fprintf(out, "    %3d  %s\n", h.Count, h.Path)
		}
		return nil
	}
	return cmd.Help()
}

// renderTree draws the neighbourhood of a context.
//
// The shape carries the meaning: a flat list answers "what is filed here", a
// tree answers "what is this context made of and what does it touch" — which is
// the question someone actually opens it with.
func renderTree(out io.Writer, t *docs.ContextTree) {
	fmt.Fprintf(out, "%s\n", t.Name)

	var blocks []func(last bool)
	for _, sec := range []struct {
		label string
		list  []docs.DocRef
	}{{"specs", t.Specs}, {"prd", t.PRDs}, {"memória", t.Memory}} {
		if len(sec.list) == 0 {
			continue
		}
		blocks = append(blocks, func(last bool) {
			fmt.Fprintf(out, "%s %s (%d)\n", tee(last), sec.label, len(sec.list))
			pad := cont(last)
			for i, d := range sec.list {
				dLast := i == len(sec.list)-1
				fmt.Fprintf(out, "%s%s %s  v%s %s\n", pad, tee(dLast),
					shortName(d.Path), d.Version, d.Status)
				inner := pad + cont(dLast)
				if len(d.Entities) > 0 {
					fmt.Fprintf(out, "%s%s tabelas: %s\n", inner, tee(len(d.Cites) == 0),
						strings.Join(d.Entities, ", "))
				}
				if len(d.Cites) > 0 {
					fmt.Fprintf(out, "%s%s cita: %s\n", inner, tee(true),
						strings.Join(shortAll(d.Cites), ", "))
				}
			}
		})
	}
	if len(t.Packages) > 0 {
		blocks = append(blocks, func(last bool) {
			total := 0
			for _, p := range t.Packages {
				total += p.Files
			}
			fmt.Fprintf(out, "%s código (%s em %s)\n", tee(last),
				plural(total, "arquivo", "arquivos"), plural(len(t.Packages), "pacote", "pacotes"))
			pad := cont(last)
			for i, pk := range t.Packages {
				fmt.Fprintf(out, "%s%s %s — %s\n", pad, tee(i == len(t.Packages)-1),
					pk.Dir, plural(pk.Files, "arquivo", "arquivos"))
			}
		})
	}
	if len(t.Entities) > 0 {
		blocks = append(blocks, func(last bool) {
			fmt.Fprintf(out, "%s tabelas (%d)\n", tee(last), len(t.Entities))
			pad := cont(last)
			for i, en := range t.Entities {
				extra := fmt.Sprintf("%s de código", plural(en.CodeFiles, "arquivo", "arquivos"))
				if len(en.Shared) > 0 {
					extra += "; também em " + strings.Join(en.Shared, ", ")
				}
				fmt.Fprintf(out, "%s%s %s — %s\n", pad, tee(i == len(t.Entities)-1), en.Name, extra)
			}
		})
	}
	if len(t.Neighbors) > 0 {
		blocks = append(blocks, func(last bool) {
			fmt.Fprintf(out, "%s contextos vizinhos (%d)\n", tee(last), len(t.Neighbors))
			pad := cont(last)
			for i, n := range t.Neighbors {
				var why string
				switch {
				case len(n.Entities) > 0:
					why = strings.Join(n.Entities, ", ")
					if n.Common > 0 {
						why += fmt.Sprintf(" (+%d comuns)", n.Common)
					}
				case n.Common > 0:
					why = fmt.Sprintf("só %s comum a quase todo contexto",
						plural(n.Common, "tabela", "tabelas"))
				default:
					why = plural(n.Cites, "citação", "citações")
				}
				fmt.Fprintf(out, "%s%s %s — %s\n", pad, tee(i == len(t.Neighbors)-1), n.Name, why)
			}
		})
	}
	if len(t.Unlinked) > 0 {
		blocks = append(blocks, func(last bool) {
			fmt.Fprintf(out, "%s menções sem link (%d)\n", tee(last), len(t.Unlinked))
			pad := cont(last)
			for i, u := range t.Unlinked {
				fmt.Fprintf(out, "%s%s %s ~ %s — %s\n", pad, tee(i == len(t.Unlinked)-1),
					shortName(u.A), shortName(u.B), strings.Join(u.Shared, ", "))
			}
		})
	}
	if len(t.Gaps) > 0 {
		blocks = append(blocks, func(last bool) {
			fmt.Fprintf(out, "%s lacunas (%d)\n", tee(last), len(t.Gaps))
			pad := cont(last)
			for i, g := range t.Gaps {
				fmt.Fprintf(out, "%s%s %s — %s\n", pad, tee(i == len(t.Gaps)-1), g.What, g.Why)
			}
		})
	}
	for i, b := range blocks {
		b(i == len(blocks)-1)
	}
}

func tee(last bool) string {
	if last {
		return "└──"
	}
	return "├──"
}

func cont(last bool) string {
	if last {
		return "    "
	}
	return "│   "
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

func shortName(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func shortAll(in []string) []string {
	out := make([]string, len(in))
	for i, p := range in {
		out[i] = shortName(p)
	}
	return out
}

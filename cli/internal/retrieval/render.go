package retrieval

import (
	"fmt"
	"io"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// RenderContext draws what is filed under a context as a tree.
func RenderContext(t *docs.ContextTree) string {
	var b strings.Builder
	renderContext(&b, t)
	return b.String()
}

// renderTree draws the neighbourhood of a context.
//
// The shape carries the meaning: a flat list answers "what is filed here", a
// tree answers "what is this context made of and what does it touch" — which is
// the question someone actually opens it with.
func renderContext(out io.Writer, t *docs.ContextTree) {
	fmt.Fprintf(out, "%s\n", t.Name)

	var blocks []func(last bool)
	for _, sec := range []struct {
		label string
		list  []docs.DocRef
	}{{"specs", t.Specs}, {"prd", t.PRDs}, {"memória", t.Memory}, {"referências", t.References}} {
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

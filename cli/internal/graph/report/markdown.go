// Package report produces the graph's readable outputs: the Markdown report the
// assistant reads first, and the HTML visualization for humans.
package report

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/analyze"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// These limits keep the report deliberately small: it exists to be read in full
// inside a context window, not to be exhaustive. The complete detail lives in
// gofi_graph.json and is served by the query commands.
const (
	maxPackages    = 40
	maxHubs        = 20
	maxCommunities = 12
	maxCycles      = 8
	maxSurprises   = 12
)

// stack names what the scanned tree is written in — the language, and the
// framework the project declared for it. It leads the report because a reader
// arriving at a front-end graph needs to know which code base it describes
// before any of the numbers mean anything.
func stack(g *model.Graph) string {
	if g.Language == "" {
		return "linguagem nao declarada"
	}
	if g.Framework == "" {
		return g.Language
	}
	return g.Language + " + " + g.Framework
}

// Markdown builds the gofi_graph_report.md.
func Markdown(g *model.Graph) string {
	var b strings.Builder
	short := func(id string) string { return model.ShortID(id, g.Module) }
	// A graph from an external extractor lives in its own directory, so every
	// command shown here has to carry the language or it queries the Go graph.
	lang := ""
	if g.Language != "" && g.Language != "go" {
		lang = " --lang " + g.Language
	}

	fmt.Fprintf(&b, "# Mapa do codigo — %s\n\n", g.Module)
	fmt.Fprintf(&b, "Gerado por gofi graph (%s, modo `%s`).\n", stack(g), g.Mode)
	b.WriteString("Fonte de verdade: `gofi_graph.json`. Este relatorio e o resumo navegavel dele.\n\n")
	b.WriteString("> Leia este arquivo **antes** de varrer o repositorio. Ele diz onde olhar.\n")
	fmt.Fprintf(&b, "> Para detalhe de qualquer simbolo use `gofi graph explain <no>%s` em vez de abrir arquivos.\n\n", lang)

	// ---- summary ----
	s := g.Stats
	b.WriteString("## Resumo\n\n")
	fmt.Fprintf(&b, "%d pacotes, %d tipos, %d funcoes, %d metodos em %d arquivos (%d linhas).\n",
		s.Packages, s.Types, s.Funcs, s.Methods, s.Files, s.LOC)
	fmt.Fprintf(&b, "Grafo com %d nos e %d arestas, das quais %d sao chamadas.\n",
		s.Nodes, s.Edges, s.CallEdges)
	if g.Mode == "deep" {
		fmt.Fprintf(&b, "Modo deep: chamadas resolvidas pelo type-checker (confianca 1.00). "+
			"%d chamadas nao resolvidas.\n", s.Unresolved)
	} else {
		fmt.Fprintf(&b, "Modo fast: chamadas por heuristica sintatica. "+
			"%d ambiguas e %d nao resolvidas — rode `--deep` para precisao total.\n",
			s.Ambiguous, s.Unresolved)
	}

	// ---- packages ----
	b.WriteString("## Pacotes\n\n")
	b.WriteString("Instabilidade I = saida/(entrada+saida). Perto de 0 = base estavel, ")
	b.WriteString("muita gente depende dele. Perto de 1 = camada de borda.\n\n")
	b.WriteString("| pacote | simbolos | linhas | dependem dele | ele depende de | I |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")

	var pkgs []*model.Node
	for _, n := range g.Nodes {
		if n.Kind == model.KindPackage && !n.External {
			pkgs = append(pkgs, n)
		}
	}
	symbolCount := map[string]int{}
	for _, e := range g.Edges {
		if e.Rel == model.RelContains {
			if from := g.Get(e.From); from != nil && from.Kind == model.KindPackage {
				symbolCount[e.From]++
			}
		}
	}
	slices.SortFunc(pkgs, func(a, b *model.Node) int {
		return cmp.Or(
			cmp.Compare(b.InDeg, a.InDeg),
			cmp.Compare(a.ID, b.ID),
		)
	})
	for i, p := range pkgs {
		if i >= maxPackages {
			fmt.Fprintf(&b, "\n_(+%d pacotes omitidos)_\n", len(pkgs)-maxPackages)
			break
		}
		instability := 0.0
		if p.InDeg+p.OutDeg > 0 {
			instability = float64(p.OutDeg) / float64(p.InDeg+p.OutDeg)
		}
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %.2f |\n",
			short(p.ID), symbolCount[p.ID], p.Lines, p.InDeg, p.OutDeg, instability)
	}
	b.WriteString("\n")

	// ---- hubs ----
	b.WriteString("## Pontos centrais\n\n")
	b.WriteString("Simbolos com maior acoplamento. Mexer aqui tem alcance largo.\n\n")
	for _, n := range analyze.Hubs(g, maxHubs, false) {
		fmt.Fprintf(&b, "- `%s` — %s, `%s:%d`, entrada %d / saida %d",
			short(n.ID), n.Kind, n.File, n.Line, n.InDeg, n.OutDeg)
		if n.Doc != "" {
			fmt.Fprintf(&b, "\n  %s", n.Doc)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// ---- communities ----
	if len(g.Communities) > 0 {
		b.WriteString("## Comunidades\n\n")
		b.WriteString("Grupos de simbolos que se chamam e se referenciam entre si. ")
		b.WriteString("Nao seguem a arvore de pastas: e o acoplamento real.\n\n")
		for i, c := range g.Communities {
			if i >= maxCommunities {
				fmt.Fprintf(&b, "_(+%d comunidades menores omitidas)_\n", len(g.Communities)-maxCommunities)
				break
			}
			if c.Size < 2 {
				continue
			}
			var pkgNames []string
			for _, p := range c.Packages {
				pkgNames = append(pkgNames, "`"+short(model.PkgID(p))+"`")
			}
			fmt.Fprintf(&b, "**C%d · %s** — %d simbolos em %s\n", c.ID, c.Label, c.Size, strings.Join(pkgNames, ", "))
			var topNames []string
			for _, t := range c.Top {
				topNames = append(topNames, "`"+short(t)+"`")
			}
			if len(topNames) > 0 {
				fmt.Fprintf(&b, "  nucleo: %s\n", strings.Join(topNames, ", "))
			}
			b.WriteString("\n")
		}
	}

	// ---- cycles ----
	if len(g.Cycles) > 0 {
		b.WriteString("## Ciclos de chamada\n\n")
		b.WriteString("Funcoes que se chamam em circulo (recursao mutua). ")
		b.WriteString("Costumam ser o ponto mais dificil de refatorar.\n\n")
		for i, c := range g.Cycles {
			if i >= maxCycles {
				fmt.Fprintf(&b, "_(+%d ciclos omitidos)_\n", len(g.Cycles)-maxCycles)
				break
			}
			var names []string
			for _, id := range c {
				names = append(names, "`"+short(id)+"`")
			}
			fmt.Fprintf(&b, "- %d nos: %s\n", len(c), strings.Join(names, " ↔ "))
		}
		b.WriteString("\n")
	}

	// ---- surprises ----
	if len(g.Surprises) > 0 {
		b.WriteString("## Conexoes inesperadas\n\n")
		b.WriteString("Ligacoes que atravessam comunidades por um caminho quase unico. ")
		b.WriteString("Ou sao integracao legitima, ou sao vazamento de camada.\n\n")
		for i, e := range g.Surprises {
			if i >= maxSurprises {
				fmt.Fprintf(&b, "_(+%d omitidas)_\n", len(g.Surprises)-maxSurprises)
				break
			}
			fmt.Fprintf(&b, "- `%s` --%s--> `%s`  (`%s:%d`)\n",
				short(e.From), e.Rel, short(e.To), e.File, e.Line)
		}
		b.WriteString("\n")
	}

	// ---- how to query ----
	b.WriteString("## Como consultar sem abrir arquivos\n\n")
	b.WriteString("```sh\n")
	fmt.Fprintf(&b, "gofi graph explain <no>%s          # tudo sobre um simbolo: origem, vizinhos, doc\n", lang)
	fmt.Fprintf(&b, "gofi graph explain <A> --to <B>%s  # como A alcanca B, aresta por aresta\n", lang)
	fmt.Fprintf(&b, "gofi graph open%s                  # abre a visualizacao HTML do grafo\n", lang)
	b.WriteString("```\n\n")
	b.WriteString("Os nomes aceitam forma curta: `NewServer`, `api.NewServer` ou o ID completo funcionam.\n")

	return b.String()
}

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scan"
)

// StatusAdopted is a context gofi adopted from existing code: it has code,
// and no contract yet.
const StatusAdopted = "adotado"

func newContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: i18n.T("cmd.context.short"),
		Long: `The project's contexts. A context is born with its memory: from a PRD, a
spec — or, in a repository gofi adopted, from the code it already has.`,
		Example: `gofi context adopt
gofi context adopt --all`,
	}
	adopt := &cobra.Command{
		Use:   "adopt [<context>...]",
		Short: i18n.T("cmd.context.adopt.short"),
		Long: `Adopt the contexts the code's folders name, as gofi scan finds them: each
one gets its memory (.claude/memory/contexts/<context>.md) with status
adotado — code that exists, no contract yet —, the folder of its code, the
tables and documents that name it, and what is missing: the PRD, the living
spec, the code tied to the contract.

A context that already has its memory is left as it is. With no context named
and no --all, the candidates are listed. gofi init asks which to adopt when it
adopts a repository.`,
		Example: `gofi context adopt
gofi context adopt pedidos estoque
gofi context adopt --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			r, err := scan.Scan(root)
			if err != nil {
				return err
			}
			all, _ := cmd.Flags().GetBool("all")
			out := cmd.OutOrStdout()
			if len(args) == 0 && !all {
				printCandidates(out, r.Contexts)
				return nil
			}
			chosen, err := chooseContexts(r.Contexts, args, all)
			if err != nil {
				return err
			}
			written, kept, err := adoptContexts(root, r, chosen)
			reportAdoption(out, written, kept)
			return err
		},
	}
	adopt.Flags().Bool("all", false, i18n.T("cmd.context.adopt.flag.all"))
	cmd.AddCommand(adopt)
	return cmd
}

// chooseContexts are the candidates named, or all of them.
func chooseContexts(cands []scan.Context, names []string, all bool) ([]scan.Context, error) {
	if all {
		return cands, nil
	}
	var out []scan.Context
	for _, n := range names {
		i := slices.IndexFunc(cands, func(c scan.Context) bool { return c.Name == n })
		if i < 0 {
			var known []string
			for _, c := range cands {
				known = append(known, c.Name)
			}
			return nil, fmt.Errorf("%s", i18n.T("context.unknown", n, strings.Join(known, ", ")))
		}
		out = append(out, cands[i])
	}
	return out, nil
}

func printCandidates(w io.Writer, cands []scan.Context) {
	if len(cands) == 0 {
		fmt.Fprintln(w, i18n.T("context.none"))
		return
	}
	fmt.Fprintln(w, i18n.T("context.candidates", len(cands)))
	for i, c := range cands {
		line := fmt.Sprintf("  %2d. %-20s %s · %s", i+1, c.Name, c.Dir, i18n.T("scan.context.line", c.Sources, c.Tests))
		if len(c.Tables) > 0 {
			line += " · " + i18n.T("scan.context.tables", strings.Join(c.Tables, ", "))
		}
		if len(c.Documents) > 0 {
			line += " · " + i18n.T("scan.context.docs", len(c.Documents))
		}
		fmt.Fprintln(w, line)
	}
}

func reportAdoption(w io.Writer, written, kept []string) {
	for _, n := range written {
		fmt.Fprintf(w, "  %s\n", i18n.T("context.adopted", n))
	}
	for _, n := range kept {
		fmt.Fprintf(w, "  %s\n", i18n.T("context.kept", n))
	}
	if len(written) > 0 {
		fmt.Fprintf(w, "\n%s\n", i18n.T("context.adopted.next"))
	}
}

// adoptContexts writes the memory of each context chosen that has none yet,
// and says which it wrote and which it left as they were.
func adoptContexts(root string, r *scan.Report, chosen []scan.Context) (written, kept []string, err error) {
	for _, c := range chosen {
		if _, statErr := os.Stat(layout.Contexts().Abs(root, c.Name+".md")); statErr == nil {
			kept = append(kept, c.Name)
			continue
		}
		if _, werr := writeMemory(root, c.Name, []byte(adoptedMemory(root, c, r))); werr != nil {
			err = errors.Join(err, fmt.Errorf("%s: %w", c.Name, werr))
			continue
		}
		written = append(written, c.Name)
	}
	return written, kept, err
}

// adoptedMemory is the memory of a context adopted from its code: what
// exists — the code, and the living spec or the PRD when the context has
// them —, and what the contract still lacks.
func adoptedMemory(root string, c scan.Context, r *scan.Report) string {
	spec, prd := "n/a", "n/a"
	if p := filepath.ToSlash(filepath.Join("specs", c.Name, "spec.md")); exists(filepath.Join(root, p)) {
		spec = p
	}
	if p := filepath.ToSlash(filepath.Join("prd", c.Name, "prd-"+c.Name+".md")); exists(filepath.Join(root, p)) {
		prd = p
	}
	// The status is the furthest phase the context has reached.
	status := StatusAdopted
	switch {
	case spec != "n/a":
		status = "spec"
	case prd != "n/a":
		status = "prd"
	}
	keywords := []string{c.Name}
	add := func(w string) {
		w = strings.Trim(strings.ToLower(strings.ReplaceAll(w, "_", "-")), "-")
		if len(w) > 2 && !slices.Contains(keywords, w) && len(keywords) < 14 {
			keywords = append(keywords, w)
		}
	}
	for _, t := range c.Tables {
		add(t)
	}
	for _, d := range c.Documents {
		for _, w := range strings.FieldsFunc(strings.TrimSuffix(filepath.Base(d), filepath.Ext(d)), func(r rune) bool { return r == '-' || r == '_' || r == ' ' }) {
			add(w)
		}
	}
	var b strings.Builder
	version := func(p string) string {
		if p == "n/a" {
			return "n/a"
		}
		return "\"1.0\""
	}
	fmt.Fprintf(&b, "---\ncontexto: %s\nservicos: []\nstatus: %s\nversao: \"1.0\"\nversao_prd: %s\nversao_spec: %s\nprd: %s\nspec: %s\ndiretorio: %s\nkeywords: [%s]\natualizado: %s\n---\n",
		c.Name, status, version(prd), version(spec), prd, spec, c.Dir, strings.Join(keywords, ", "), time.Now().Format("2006-01-02"))
	fmt.Fprintf(&b, "# Contexto: %s\n\n## Estado atual\n\n", c.Name)
	fmt.Fprintf(&b, "Código existente, adotado pelo gofi sem contrato ainda: %d arquivo(s) de código e %d de teste em `%s`.", c.Sources, c.Tests, c.Dir)
	if len(c.Tables) > 0 {
		fmt.Fprintf(&b, " Tabelas: %s.", "`"+strings.Join(c.Tables, "`, `")+"`")
	}
	b.WriteString("\n")
	if len(c.Documents) > 0 {
		b.WriteString("\nDocumentos que o citam — a matéria-prima do PRD e da spec:\n\n")
		for _, d := range c.Documents {
			kind := ""
			for _, doc := range r.Documents {
				if doc.Path == d && doc.Kind != scan.KindOther {
					kind = " (" + doc.Kind + ")"
				}
			}
			fmt.Fprintf(&b, "- `%s`%s\n", d, kind)
		}
	}
	var missing []string
	if prd == "n/a" {
		missing = append(missing, "o PRD (`/gofi-pd`)")
	}
	if spec == "n/a" {
		missing = append(missing, "a spec viva (`/gofi-spec`)")
	}
	missing = append(missing, "o código ligado ao contrato (`gofi trace "+c.Name+"`)")
	fmt.Fprintf(&b, "\nFalta: %s.\n", strings.Join(missing, ", "))
	b.WriteString("\n## Histórico de versões\n| Versão | Data | Mudança |\n|--------|------|---------|\n")
	fmt.Fprintf(&b, "| v1.0 | %s | Contexto adotado do código existente. |\n", time.Now().Format("2006-01-02"))
	return b.String()
}

// askAdoption is gofi init's question, when it adopts a repository: which of
// the contexts the code's folders name become gofi's. Enter adopts them all;
// numbers pick some; n adopts none.
func askAdoption(in io.Reader, out io.Writer, root string) {
	r, err := scan.Scan(root)
	if err != nil || len(r.Contexts) == 0 {
		return
	}
	fmt.Fprintln(out)
	printCandidates(out, r.Contexts)
	fmt.Fprint(out, "\n"+i18n.T("context.ask"))
	line, _ := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	var chosen []scan.Context
	switch line {
	case "", "s", "sim", "y", "yes", "todos", "all":
		chosen = r.Contexts
	case "n", "nao", "não", "no", "nenhum", "none":
		fmt.Fprintln(out, i18n.T("context.ask.none"))
		return
	default:
		for _, f := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' }) {
			if n, err := strconv.Atoi(f); err == nil && n >= 1 && n <= len(r.Contexts) {
				chosen = append(chosen, r.Contexts[n-1])
			}
		}
	}
	written, kept, err := adoptContexts(root, r, chosen)
	reportAdoption(out, written, kept)
	if err != nil {
		fmt.Fprintln(out, err)
	}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

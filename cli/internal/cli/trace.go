package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/trace"
)

func newTraceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trace <change|context>",
		Short: i18n.T("cmd.trace.short"),
		Long: `Line up a contract's requirements with the code that implements them and the
tests that test them: the requirements a change adds or modifies — or, for a
context, its living spec's — each with the symbols marked gofi:implements
REQ-NN, each scenario with the tests marked gofi:tests REQ-NN.n.

A requirement nothing implements, or a scenario no test tests, is a gap, and
the command fails: a change with a gap is not done. A gofi:tests mark counts
only in a test file of the backend's language.`,
		Example: `gofi trace expor-pontos
gofi trace pontos
gofi trace expor-pontos --json`,
		Args: cobra.ExactArgs(1),
		RunE: runTrace,
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.flag.json"))
	cmd.Flags().String("report", "", "where to write the gaps, as SARIF")
	_ = cmd.Flags().MarkHidden("report")
	return cmd
}

func runTrace(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	report, _ := cmd.Flags().GetString("report")
	if report != "" {
		// A report of an earlier check never stands for this one.
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(report)))
	}
	all, err := contract.Docs(root)
	if err != nil {
		return err
	}
	sources, err := traceSources(all, args[0])
	if err != nil {
		return err
	}
	lp, err := pack.Load(backendLanguage(cfg))
	if err != nil {
		return err
	}
	src := ""
	if cfg.Backend != nil {
		src = cfg.Backend.Path
	}
	marks, err := trace.Scan(root, src, lp)
	if err != nil {
		return err
	}
	rows := trace.Matrix(sources, marks)
	gaps := trace.Gaps(rows)
	if report != "" {
		var results []sarifResult
		for _, g := range gaps {
			results = append(results, sarifResult{Rule: "gofi:" + g.Need() + " " + g.Context + "/" + g.ID, URI: g.Doc, Line: g.Line, Message: gapMessage(g)})
		}
		if err := writeSARIF(filepath.Join(root, filepath.FromSlash(report)), "gofi-trace", results); err != nil {
			return err
		}
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		if err := writeJSON(out, map[string]any{"schema": "gofi.trace/v1", "ref": args[0], "rows": rows, "gaps": len(gaps)}); err != nil {
			return err
		}
	} else {
		printTrace(out, args[0], rows, len(gaps))
	}
	if len(gaps) > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("trace.gaps"))}
	}
	return nil
}

// traceSources are what a reference traces: the requirements a change adds
// or modifies, by context, or a context's living spec.
func traceSources(all []contract.Doc, ref string) ([]trace.Source, error) {
	var out []trace.Source
	for _, d := range all {
		if d.Kind != contract.KindDelta || d.Change != ref {
			continue
		}
		src := trace.Source{Context: d.Context, Doc: d.Path}
		for _, s := range contract.DeltaSections(d) {
			if s.Op == contract.OpAdded || s.Op == contract.OpModified {
				src.Requirements = append(src.Requirements, s.Requirements...)
			}
		}
		out = append(out, src)
	}
	if len(out) > 0 {
		return out, nil
	}
	for _, d := range all {
		if d.Kind == contract.KindSpec && d.Change == "" && d.Context == ref {
			out = append(out, trace.Source{Context: d.Context, Doc: d.Path, Requirements: contract.Requirements(d)})
		}
	}
	if len(out) == 0 {
		return nil, errors.New(i18n.T("trace.unknown", ref))
	}
	return out, nil
}

func gapMessage(r trace.Row) string {
	if r.Scenario {
		return fmt.Sprintf("o cenário %s/%s (%s) não tem teste: escreva-o com gofi:tests %s num arquivo de teste", r.Context, r.ID, r.Title, r.ID)
	}
	return fmt.Sprintf("o requisito %s/%s (%s) não tem quem o implemente: marque o símbolo que o realiza com gofi:implements %s", r.Context, r.ID, r.Title, r.ID)
}

func printTrace(out io.Writer, ref string, rows []trace.Row, gaps int) {
	fmt.Fprintf(out, "rastreio de %s\n", ref)
	ctx := ""
	for _, r := range rows {
		if r.Context != ctx {
			ctx = r.Context
			fmt.Fprintf(out, "\n  %s\n", ctx)
		}
		indent, verb, none := "  ", "implementa", "nada o implementa"
		if r.Scenario {
			indent, verb, none = "      ", "testa", "nenhum teste"
		}
		title := r.ID + " " + r.Title
		if r.Gap() {
			fmt.Fprintf(out, "  %s%-48s — %s\n", indent, clipText(title, 48), none)
			continue
		}
		var at []string
		for _, m := range r.Marks {
			at = append(at, fmt.Sprintf("%s:%d", m.File, m.Line))
		}
		fmt.Fprintf(out, "  %s%-48s %s: %s\n", indent, clipText(title, 48), verb, strings.Join(at, ", "))
	}
	fmt.Fprintf(out, "\n  %d de %d sem resposta no código\n", gaps, len(rows))
}

func clipText(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

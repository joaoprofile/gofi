package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/trace"
)

// ReportFile is where a run's delivery report is kept, in its folder;
// TraceFile its trace matrix.
const (
	ReportFile = "report.md"
	TraceFile  = "trace.json"
)

func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report <run>",
		Short: i18n.T("cmd.report.short"),
		Long: `Show what a run delivered: the outcome, the trace of every requirement to its
code and its tests, what the code graph says changed and what depends on it,
the audit, and what each turn cost. Written when an accepted run ends; for a
run that did not get there, assembled from its state, without the trace.`,
		Example: `gofi report r-20261103-141753`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			if data, err := os.ReadFile(filepath.Join(loop.Dir(root, args[0]), ReportFile)); err == nil {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			s, err := loop.LoadState(root, args[0])
			if err != nil {
				return err
			}
			agents, _ := loop.Agents(root, s.Run)
			_, err = fmt.Fprint(cmd.OutOrStdout(), deliveryReport(s, agents, nil))
			return err
		},
	}
}

// writeReport keeps the delivery report of a run in its folder.
func writeReport(root string, s *loop.State, rows []trace.Row) (string, error) {
	agents, _ := loop.Agents(root, s.Run)
	// The matrix, as the panel draws it: what the report's table is made of.
	if rows != nil {
		if data, err := json.MarshalIndent(map[string]any{"schema": "gofi.trace/v1", "ref": s.Change, "rows": rows}, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(loop.Dir(root, s.Run), TraceFile), append(data, '\n'), 0o644)
		}
	}
	p := filepath.Join(loop.Dir(root, s.Run), ReportFile)
	return p, os.WriteFile(p, []byte(deliveryReport(s, agents, rows)), 0o644)
}

// lastAudit are the roles of the run's last audit.
func lastAudit(agents []loop.AgentState) []string {
	iter := 0
	for _, a := range agents {
		if a.Node == "audit" && a.Iter > iter {
			iter = a.Iter
		}
	}
	var out []string
	for _, a := range agents {
		if a.Node == "audit" && a.Iter == iter {
			out = append(out, a.Role)
		}
	}
	return out
}

// deliveryReport is what a run delivered, for the person who reviews it:
// nothing in it is a model's word — the state, the trace, the graph's
// diff, the agents' records.
func deliveryReport(s *loop.State, agents []loop.AgentState, rows []trace.Row) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Entrega — %s (%s)\n\n", s.Change, s.Context)
	b.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Execução | `%s` |\n", s.Run)
	outcome := s.Outcome
	if s.Verdict != "" {
		outcome += " · veredicto " + s.Verdict
	}
	fmt.Fprintf(&b, "| Resultado | %s · %d voltas de retrabalho |\n", outcome, s.Rounds)
	fmt.Fprintf(&b, "| Custo | US$ %.2f · %d turnos |\n", s.CostUSD, len(agents))
	switch {
	case s.Kept != "":
		fmt.Fprintf(&b, "| Trabalho guardado | branch `%s` |\n", s.Kept)
	case s.Branch != "":
		fmt.Fprintf(&b, "| Branch | `%s` → `%s` (%s) |\n", s.Branch, s.Base, s.Landing)
	case s.Base != "" && s.Landing == landed:
		fmt.Fprintf(&b, "| Commit | em `%s` |\n", s.Base)
	case s.Base != "" && s.Outcome == "accepted":
		fmt.Fprintf(&b, "| Commit | **não feito** — o trabalho está na árvore, sem commit em `%s` |\n", s.Base)
	}
	if s.Proposal != "" {
		fmt.Fprintf(&b, "| Proposta de emenda | `%s` |\n", s.Proposal)
	}

	b.WriteString("\n## Rastreio\n\n")
	if rows == nil {
		b.WriteString("Sem rastreio: a execução não chegou à entrega — `gofi trace " + s.Change + "`.\n")
	} else {
		b.WriteString("| Requisito / cenário | Implementa / testa |\n|---|---|\n")
		for _, r := range rows {
			name := r.ID + " — " + r.Title
			if r.Scenario {
				name = "  ↳ " + name
			}
			var at []string
			for _, m := range r.Marks {
				at = append(at, fmt.Sprintf("`%s:%d`", m.File, m.Line))
			}
			if len(at) == 0 {
				at = []string{"**lacuna**"}
			}
			fmt.Fprintf(&b, "| %s | %s |\n", name, strings.Join(at, ", "))
		}
	}

	if d := s.Diff; d != nil {
		b.WriteString("\n## O que mudou no código\n\n")
		for _, t := range d.Touched {
			where := t.File
			if t.Layer != "" {
				where += ", " + t.Layer
			}
			fmt.Fprintf(&b, "- `%s` (%s)\n", t.Symbol, where)
		}
		for _, r := range d.Removed {
			fmt.Fprintf(&b, "- removido: `%s`\n", r)
		}
		if d.Dependents > 0 {
			fmt.Fprintf(&b, "\n%d símbolos dependem do que mudou", d.Dependents)
			if len(d.Reach) > 0 {
				fmt.Fprintf(&b, ", nos contextos %s", strings.Join(d.Reach, ", "))
			}
			b.WriteString(".\n")
		}
	}

	b.WriteString("\n## Auditoria\n\n")
	if lenses := lastAudit(agents); len(lenses) > 0 {
		fmt.Fprintf(&b, "Lentes da última auditoria: %s.\n", strings.Join(lenses, ", "))
	}
	fmt.Fprintf(&b, "Achados resolvidos ao longo da execução: %d.", len(s.Resolved))
	if len(s.Open) > 0 {
		b.WriteString(" Em aberto:\n\n")
		for _, f := range s.Open {
			fmt.Fprintf(&b, "- [%s] %s — %s\n", f.Severity, f.Rule, f.Message)
		}
	} else {
		b.WriteString(" Nenhum em aberto.\n")
	}

	if len(s.Notes) > 0 {
		b.WriteString("\n## Ressalvas do contrato\n\nA auditoria achou no contrato o que não impede esta entrega — o código cumpre o que ele diz — e cabe numa próxima mudança:\n\n")
		for _, f := range s.Notes {
			fmt.Fprintf(&b, "- [%s] %s — %s\n", f.Severity, f.Rule, f.Message)
		}
	}
	if len(agents) > 0 {
		b.WriteString("\n## Turnos\n\n| Agente | Papel | Tier | US$ | s |\n|---|---|---|---|---|\n")
		for _, a := range agents {
			fmt.Fprintf(&b, "| %s | %s | %s | %.2f | %.0f |\n", a.ID, a.Role, a.Tier, a.CostUSD, a.Seconds)
		}
	}
	return b.String()
}

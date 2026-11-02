package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// DefaultBrief is the turn an agent node sends: the role's skill, invoked on
// the change, with what the run knows that the role needs now — the sensor
// output that failed, the findings still open — and where to write a
// decision it cannot find. Assembled from the state, never from what a model
// said before: each turn runs in a fresh session and reads the rest on disk.
func DefaultBrief(rl role.Role, s *State, elicitFile string) string {
	var b strings.Builder
	// An internal role is no slash command: the agent follows it.
	if rl.Contract.Internal() {
		fmt.Fprintf(&b, "Siga o papel descrito em %s.\n\n", layout.RoleSkill(rl.Name, true))
	} else {
		fmt.Fprintf(&b, "/%s ", rl.Name)
	}
	fmt.Fprintf(&b, "Trabalhe na mudança aprovada `%s` do contexto %s — `gofi change list`, os deltas e o design em `specs/changes/%s/`. Iteração %d da execução %s.\n\n",
		s.Change, s.Context, s.Change, s.Iter, s.Run)
	if s.Sensor != nil && len(s.Sensor.Failed) > 0 {
		fmt.Fprintf(&b, "**Os sensores falharam em %s.** Corrija antes de qualquer outra coisa:\n\n", strings.Join(s.Sensor.Failed, ", "))
		if len(s.Sensor.Findings) > 0 {
			for _, f := range s.Sensor.Findings {
				where := f.File
				if f.Line > 0 {
					where = fmt.Sprintf("%s:%d", f.File, f.Line)
				}
				if where == "" {
					where = f.Rule
				}
				fmt.Fprintf(&b, "- `%s` %s\n", where, clip(f.Message, 1200))
			}
			b.WriteString("\n")
		} else {
			fmt.Fprintf(&b, "```\n%s\n```\n\n", clip(s.Sensor.Output, 4000))
		}
	}
	if len(s.Open) > 0 {
		b.WriteString("**Achados abertos da auditoria** — corrija todos, sem ampliar o escopo:\n\n")
		for _, f := range s.Open {
			fmt.Fprintf(&b, "- [%s] %s — %s (`%s`)\n", f.Severity, f.Rule, f.Message, f.Fingerprint)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Faltou uma decisão que a mudança não cobre? Não suponha nem pergunte na conversa: grave a pergunta em `%s` (formato gofi.elicit/v1: questions com id, text, options, recommended, why) e encerre o turno.\n\n", elicitFile)
	// A role with a grant has no shell: it is told what to use instead, so a
	// turn is not spent on commands that are refused.
	if len(rl.Contract.Tools) > 0 && !slices.Contains(rl.Contract.Tools, "shell") {
		b.WriteString("Você não tem shell neste turno. Para compilar, analisar e testar use a ferramenta `verify` do gofi; para achar e entender código e documentos, `find`, `show` e `path`. O índice, o loop atualiza depois do seu turno.\n\n")
	}
	fmt.Fprintf(&b, "Se o seu papel audita: grave o veredicto com a ferramenta `verdict_write` do gofi (context %s, run %s, iter %d) — o mesmo que `gofi verdict write %s --run %s --iter %d`.\n", s.Context, s.Run, s.Iter, s.Context, s.Run, s.Iter)
	return b.String()
}

// clip keeps the end of a long output, where a build or a test says what broke.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

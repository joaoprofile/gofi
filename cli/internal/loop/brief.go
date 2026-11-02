package loop

import (
	"fmt"
	"strings"
)

// DefaultBrief is the turn an agent node sends: the role's skill, invoked on
// the change, with what the run knows that the role needs now — the sensor
// output that failed, the findings still open — and where to write a
// decision it cannot find. Assembled from the state, never from what a model
// said before: each turn runs in a fresh session and reads the rest on disk.
func DefaultBrief(n Node, s *State, elicitFile string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/%s Implemente e verifique a mudança aprovada `%s` do contexto %s — `gofi change list`, os deltas e o design em `specs/changes/%s/`. Iteração %d da execução %s.\n\n",
		n.Role, s.Change, s.Context, s.Change, s.Iter, s.Run)
	if s.Sensor != nil && len(s.Sensor.Failed) > 0 {
		fmt.Fprintf(&b, "**Os sensores falharam em %s.** Corrija antes de qualquer outra coisa:\n\n```\n%s\n```\n\n",
			strings.Join(s.Sensor.Failed, ", "), clip(s.Sensor.Output, 4000))
	}
	if len(s.Open) > 0 {
		b.WriteString("**Achados abertos da auditoria** — corrija todos, sem ampliar o escopo:\n\n")
		for _, f := range s.Open {
			fmt.Fprintf(&b, "- [%s] %s — %s (`%s`)\n", f.Severity, f.Rule, f.Message, f.Fingerprint)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Faltou uma decisão que a mudança não cobre? Não suponha nem pergunte na conversa: grave a pergunta em `%s` (formato gofi.elicit/v1: questions com id, text, options, recommended, why) e encerre o turno.\n\n", elicitFile)
	fmt.Fprintf(&b, "Se o seu papel audita: grave o veredicto com `gofi verdict write %s --run %s --iter %d`.\n", s.Context, s.Run, s.Iter)
	return b.String()
}

// clip keeps the end of a long output, where a build or a test says what broke.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…\n" + s[len(s)-max:]
}

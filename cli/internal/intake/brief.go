package intake

import (
	"fmt"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// doneWhen is the done criterion of each artifact, with the context's name.
var doneWhen = map[string]string{
	"prd":       "PRD em prd/%[1]s/prd-%[1]s.md, com frontmatter e keywords, pronto para a spec",
	"spec":      "spec em specs/%[1]s/sdd-%[1]s.md cobrindo a mudança, com frontmatter e keywords",
	"code":      "código de %[1]s implementado conforme a spec, com testes passando, e auditado",
	"ui":        "telas de %[1]s implementadas conforme a spec e o design system, e auditadas",
	"infra":     "IaC e pipeline de %[1]s aplicáveis conforme a spec de infra",
	"docs":      "documentação de contrato de %[1]s gerada a partir do código",
	"audit":     "relatório de auditoria de %[1]s contra a spec e os padrões",
	"status":    "panorama dos contextos respondido",
	"migration": "projeto migrado para a estrutura atual do gofi",
}

// later lists what a plan that ends at an artifact leaves out of scope.
var later = map[string]string{
	"prd":  "spec, código e auditoria — oferecidos ao concluir",
	"spec": "código e auditoria — oferecidos ao concluir",
}

// brief assembles the request the first phase receives (0001 D6): a template
// filled with checked data, never text a model wrote, so nothing is invented.
func brief(r *Result) string {
	ctxName := "<contexto>"
	if r.Context != nil {
		ctxName = r.Context.Name
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Pedido montado — %s · %s · %s\n\n", r.Intent, r.Artifact, ctxName)
	fmt.Fprintf(&b, "**Objetivo:** %s\n\n", strings.TrimSpace(r.Request))
	if r.Change != "" {
		fmt.Fprintf(&b, "**Mudança:** %s\n\n", r.Change)
	}
	if d, ok := doneWhen[r.Artifact]; ok {
		// Not every criterion names the context; one that does not must not
		// get it appended as a formatting error.
		if strings.Contains(d, "%[1]s") {
			d = fmt.Sprintf(d, ctxName)
		}
		b.WriteString("**Pronto quando:** " + d + "\n\n")
	}
	if r.Answer != "" {
		fmt.Fprintf(&b, "**Resposta:** `%s` — o índice responde sem papel.\n\n", r.Answer)
	}
	if len(r.Plan) > 0 {
		b.WriteString("**Plano:**\n\n")
		for i, ph := range r.Plan {
			fmt.Fprintf(&b, "%d. `/%s` — %s · %s (%s)\n", i+1, ph.Role, ph.Why, ph.Tier, ph.TierWhy)
		}
		b.WriteString("\n")
	}
	if r.Context != nil {
		b.WriteString("**Contexto:** " + r.Context.Name)
		if r.Context.New {
			b.WriteString(" (novo)")
		} else if len(r.Context.Has) > 0 {
			b.WriteString(" — tem " + strings.Join(r.Context.Has, ", "))
		}
		b.WriteString("\n\n")
	}
	b.WriteString("**Restrições:** as leis comuns do `AGENTS.md`")
	if r.Context != nil && strings.Contains(strings.Join(r.Context.Has, ","), "spec") {
		fmt.Fprintf(&b, "; a spec `specs/%[1]s/sdd-%[1]s.md` é a fonte da verdade", r.Context.Name)
	}
	b.WriteString(".\n\n")
	if r.Context != nil {
		fmt.Fprintf(&b, "**Memória:** grave `%s` com `gofi memory write %s` (o arquivo inteiro pela entrada padrão) — Edit e Write em `%s/` pedem aprovação que numa fase conduzida ninguém dá.\n\n", layout.Contexts().Path(r.Context.Name+".md"), r.Context.Name, layout.Home())
	}
	if l, ok := later[r.Artifact]; ok {
		b.WriteString("**Fora de escopo:** " + l + ".\n\n")
	}
	if len(r.Packs) > 0 {
		b.WriteString("**Especialidades:**\n\n")
		for _, p := range r.Packs {
			fmt.Fprintf(&b, "- `%s/` — %s\n", p.Dir, p.Why)
		}
		b.WriteString("\n")
	}
	if len(r.Assumptions) > 0 {
		b.WriteString("**Suposições:**\n\n")
		for _, a := range r.Assumptions {
			b.WriteString("- " + a + "\n")
		}
		b.WriteString("\n")
	}
	if len(r.Evidence) > 0 {
		b.WriteString("**Evidências** (leia só o trecho indicado):\n\n")
		for _, e := range r.Evidence {
			b.WriteString("- " + e.String() + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// PhaseTurn is what the conductor sends for phase i of the plan. Invoked as a
// skill, the phase runs on the model its role's contract names. asRole is for
// a phase the router raised above that: the conductor has set the raised
// tier's model on the session, and the agent is told to follow the role's
// SKILL.md instead of invoking the skill, whose own model would win.
func PhaseTurn(r *Result, i int, asRole bool) string {
	ph := r.Plan[i]
	var b strings.Builder
	if i > 0 {
		prev := r.Plan[i-1]
		ctx := "o contexto"
		if r.Context != nil {
			ctx = r.Context.Name
		}
		fmt.Fprintf(&b, "Fase %d de %d. A fase anterior (/%s) terminou; o estado está na memória de %s.\n\n", i+1, len(r.Plan), prev.Role, ctx)
		if doc := approvedDoc(r, i-1); doc != "" {
			fmt.Fprintf(&b, "A pessoa revisou e aprovou `%s` na parada de revisão: antes de começar, grave `status: aprovado` no frontmatter dele — só esse campo.\n\n", doc)
		}
	}
	b.WriteString(r.Brief)
	if asRole {
		return roleTurn(ph, b.String())
	}
	return "/" + ph.Role + " " + b.String()
}

// roleTurn has the agent follow a role's SKILL.md on the session's model, for
// a phase above its contract's tier.
func roleTurn(ph Phase, body string) string {
	return fmt.Sprintf("Siga o papel descrito em %s, no nível %s (%s).\n\n%s", layout.Skills().Path(ph.Role, "SKILL.md"), ph.Tier, ph.TierWhy, body)
}

// TurnFor is what to send for phase i: the turn the plan carries, or one
// built from the plan for a result that carries none.
func TurnFor(r *Result, i int, asRole bool) string {
	ph := r.Plan[i]
	if asRole && ph.RoleTurn != "" {
		return ph.RoleTurn
	}
	if !asRole && ph.Turn != "" {
		return ph.Turn
	}
	return PhaseTurn(r, i, asRole)
}

// Raised reports whether the router put phase i above its role's contract.
func Raised(r *Result, i int) bool {
	return r.Plan[i].Tier != r.Plan[i].Base
}

// ApprovalAfter reports whether the plan stops for review after phase i: a
// PRD or a spec is what the next phases build on (0001, D9).
func ApprovalAfter(r *Result, i int) bool {
	if i+1 >= len(r.Plan) {
		return false
	}
	p := r.Plan[i].Produces
	return p == "prd" || p == "spec"
}

// explainTurn is a question about the project as the conversation receives
// it: the question, and where to start reading — the sections the index
// ranked for it, with their lines. The agent reads those first, and asks the
// index for more before it searches the tree.
func explainTurn(r *Result) string {
	if len(r.Evidence) == 0 {
		return r.Request
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(r.Request))
	b.WriteString("\n\nComece por estes trechos do projeto — leia só as linhas indicadas; se precisar de mais, `gofi find` antes de varrer a árvore:\n")
	for _, e := range r.Evidence {
		b.WriteString("- " + e.String() + "\n")
	}
	return b.String()
}

// approvedDoc is the document the person approved at the review stop after
// phase i — the PRD or the spec it wrote —, or "" when there was no stop.
// Nothing else records the approval: the phase that runs after it does.
func approvedDoc(r *Result, i int) string {
	if !ApprovalAfter(r, i) || r.Context == nil {
		return ""
	}
	name := r.Context.Name
	switch r.Plan[i].Produces {
	case "prd":
		return layout.PRD().Path(name, "prd-"+name+".md")
	case "spec":
		return layout.Specs().Path(name, "sdd-"+name+".md")
	}
	return ""
}

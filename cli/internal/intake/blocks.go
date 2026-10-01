package intake

import (
	"fmt"
	"slices"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// blockNote is what an implementation is told to do when the spec leaves a
// decision open: stop and write it down, not guess and not ask in the
// conversation — the conversation is where asking costs the most.
const blockNote = "Faltou uma decisão que a spec não cobre? Não suponha nem pergunte na conversa: grave a pergunta em `%s` (formato %s: questions com id, text, options, recommended, why) e encerre a fase.\n\n"

// fillBlocks gives each implementation phase the file to write a decision it
// cannot find in the spec, and the phases that follow when it does: the spec
// records the person's answer, on a cheaper tier, and the implementation
// runs again on a complete spec. Decided with the plan; the conductor asks
// the question and splices the phases.
func fillBlocks(r *Result) {
	if r.Context == nil {
		return
	}
	for i := range r.Plan {
		ph := &r.Plan[i]
		if ph.Produces == "docs" {
			fillDocBlock(r, ph)
			continue
		}
		if !fixers[ph.Produces] || r.specRole == "" {
			continue
		}
		ph.ElicitFile = elicitFile(r, ph.Role)
		note := fmt.Sprintf(blockNote, ph.ElicitFile, ElicitSchema)
		again := *ph
		again.OnReject, again.OnBlock = nil, nil
		again.Why = "retoma com a spec completa"
		resume := "A spec foi completada com as decisões que faltavam; retome a implementação a partir dela.\n\n"
		again.Turn = prependBrief(ph.Turn, resume)
		if ph.RoleTurn != "" {
			again.RoleTurn = prependBrief(ph.RoleTurn, resume)
		}
		ph.Turn = prependBrief(ph.Turn, note)
		if ph.RoleTurn != "" {
			ph.RoleTurn = prependBrief(ph.RoleTurn, note)
		}
		ph.OnBlock = []Phase{recordPhase(r, ph.Role), again}
	}
}

// recordPhase writes the answers of a blocked implementation into the spec:
// decisions already taken, so no deeper than standard.
func recordPhase(r *Result, blocked string) Phase {
	tier := host.Standard
	if slices.Index(host.Tiers, r.specTier) < slices.Index(host.Tiers, tier) {
		tier = r.specTier
	}
	// It produces no spec to review: it writes down what the person just
	// answered, so the plan does not stop after it.
	ph := Phase{Role: r.specRole, Tier: tier, Base: r.specTier,
		Why:     "registra na spec as decisões que /" + blocked + " não encontrou",
		TierWhy: "só registra decisões já tomadas: nível " + string(tier)}
	skill := layout.Skills().Path(r.specRole, "SKILL.md")
	ph.Turn = fmt.Sprintf("Siga o papel descrito em %s em **modo registro**, no nível %s (%s).\n\n"+
		"/%s parou por decisões que a spec não cobre, e a pessoa respondeu — estão em **Decisões confirmadas**, no fim. "+
		"Registre cada uma na seção da spec a que pertence: só elas, sem mudar mais nada e sem subir a versão da spec; depois `gofi index docs`. Não implemente.\n\n%s",
		skill, ph.Tier, ph.TierWhy, blocked, r.Brief)
	ph.RoleTurn = ph.Turn
	return ph
}

// fillDocBlock gives the documentation phase the file for what it cannot
// read off the code: the question is asked, and the phase runs again with the
// answer at the end of its turn. Documentation changes no spec, so there is no
// record in between.
func fillDocBlock(r *Result, ph *Phase) {
	ph.ElicitFile = elicitFile(r, ph.Role)
	again := *ph
	again.OnBlock = nil
	again.Why = "retoma com as respostas"
	resume := "As respostas às suas perguntas estão em **Decisões confirmadas**, no fim; retome a documentação com elas.\n\n"
	again.Turn = prependBrief(ph.Turn, resume)
	if ph.RoleTurn != "" {
		again.RoleTurn = prependBrief(ph.RoleTurn, resume)
	}
	note := fmt.Sprintf(blockNote, ph.ElicitFile, ElicitSchema)
	ph.Turn = prependBrief(ph.Turn, note)
	if ph.RoleTurn != "" {
		ph.RoleTurn = prependBrief(ph.RoleTurn, note)
	}
	ph.OnBlock = []Phase{again}
}

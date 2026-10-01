package intake

import (
	"fmt"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/role"
)

// wideReach is how many coupled contexts make a change wide enough to need
// the deep tier: past it, a standard role that writes is raised. Calibrated
// by testdata/golden-intake.md.
const wideReach = 3

// route sets each phase's tier and says why (0001 D8). The contract's tier is
// the base; a request that names a tier wins over everything; a role that
// writes into a context coupled to many others is raised to deep.
func route(phases []Phase, roles map[string]role.Contract, ctx *Context, asked host.Tier) {
	for i := range phases {
		ph := &phases[i]
		c := roles[ph.Role]
		switch {
		case ph.Elicit:
			// Reading and asking needs no more than the contract's elicitation
			// tier, whatever the role it prepares needs.
			ph.Tier, ph.TierWhy = c.Elicit.Tier, "elicitação do contrato de "+ph.Role+": só lê e pergunta"
		case asked != "":
			ph.Tier, ph.TierWhy = asked, "pedido explícito"
		case c.Tier == host.Standard && !c.ReadsOnly && ctx != nil && len(ctx.Neighbors) >= wideReach:
			ph.Tier = host.Deep
			ph.TierWhy = fmt.Sprintf("altera %s, acoplado a %d contextos (%s)", ctx.Name, len(ctx.Neighbors), strings.Join(first(ctx.Neighbors, 3), ", "))
		default:
			ph.Tier, ph.TierWhy = c.Tier, "nível do contrato de "+ph.Role
		}
	}
}

func first(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

package intake

import (
	"fmt"

	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// Phase is one step of the plan: a role, the tier it runs at, and why it is
// in the plan and at that tier.
type Phase struct {
	Role     string    `json:"role"`
	Produces string    `json:"produces"`
	Tier     host.Tier `json:"tier"`
	// Base is the tier the role's contract declares — the one its installed
	// skill names a model for. A Tier above it is the router's raise.
	Base    host.Tier `json:"base_tier"`
	Why     string    `json:"why"`
	TierWhy string    `json:"tier_why"`

	// Turn is what a conductor sends to run the phase: the role's skill,
	// invoked with the assembled request. RoleTurn is the same work for a
	// phase the router raised, where the conductor sets Model and has the agent
	// follow the role's SKILL.md (see PhaseTurn). ReviewAfter says the plan
	// stops for review once the phase is done. Filled at the end of Run, so a
	// conductor — the editor, a script — runs a plan without rebuilding it.
	Turn        string `json:"turn,omitempty"`
	RoleTurn    string `json:"role_turn,omitempty"`
	Model       string `json:"model,omitempty"`
	ReviewAfter bool   `json:"review_after,omitempty"`
	// OnReject are the phases to run right after this audit when the QA fails
	// the delivery: the implementation one tier up, and the audit again. The
	// conductor reads the verdict at Result.VerdictFile (see Rejected).
	OnReject []Phase `json:"on_reject,omitempty"`

	// Elicit marks the cheaper phase the role's contract puts before it: it
	// walks Checklist and writes the open decisions to ElicitFile, relative
	// to the project root, for the person to answer before the role runs
	// (see Elicitation). It writes no document.
	// OnBlock are the phases to run when an implementation stops on a
	// decision the spec does not cover and writes it to ElicitFile: the spec
	// records the person's answer, and the implementation runs again.
	OnBlock []Phase `json:"on_block,omitempty"`

	Elicit     bool   `json:"elicit,omitempty"`
	Checklist  string `json:"checklist,omitempty"`
	ElicitFile string `json:"elicit_file,omitempty"`
}

// defaultArtifact is what a request ends with when it names no artifact: a
// change or a fix ends with code, a review with an audit.
var defaultArtifact = map[string]string{
	"create": "code", "change": "code", "fix": "code", "review": "audit",
	"document": "docs", "status": "status", "migrate": "migration", "deploy": "infra", "explain": "",
}

// planner builds plans from the roles' contracts — the playbooks of 0001 D7
// are what the contracts declare, not a table in the CLI.
type planner struct {
	roles     map[string]role.Contract
	producers map[string]string
}

func newPlanner(roles []role.Role) planner {
	p := planner{roles: map[string]role.Contract{}, producers: map[string]string{}}
	for _, r := range roles {
		p.roles[r.Name] = r.Contract
		if r.Contract.Produces != "" {
			p.producers[r.Contract.Produces] = r.Name
		}
	}
	return p
}

// plan returns the phases that take a request from the context's current
// state to the artifact it aims at.
func (p planner) plan(intent, artifact string, ctx *Context) ([]Phase, error) {
	target := p.producers[artifact]
	if target == "" {
		return nil, fmt.Errorf("no installed role produces %q — run `gofi update skills`", artifact)
	}
	var phases []Phase
	seen := map[string]bool{}
	var visit func(name, why string, depth int) error
	visit = func(name, why string, depth int) error {
		if seen[name] {
			return nil
		}
		if depth > len(p.roles) {
			return fmt.Errorf("contracts require each other in a cycle through %s", name)
		}
		c, ok := p.roles[name]
		if !ok {
			return fmt.Errorf("the plan needs %s, which is not installed", name)
		}
		for _, req := range c.Requires {
			if !req.Applies(intent) {
				continue
			}
			producer := p.producers[req.Artifact]
			if producer == "" {
				continue
			}
			switch {
			case !ctx.has(req.Artifact):
				if err := visit(producer, fmt.Sprintf("%s precisa de %s, e o contexto ainda não tem", name, req.Artifact), depth+1); err != nil {
					return err
				}
			case req.Revises(intent):
				if err := visit(producer, fmt.Sprintf("a mudança é registrada em %s antes de %s", req.Artifact, name), depth+1); err != nil {
					return err
				}
			}
		}
		seen[name] = true
		if e := c.Elicit; e != nil {
			phases = append(phases, Phase{Role: name, Tier: e.Tier, Base: c.Tier, Elicit: true, Checklist: e.Checklist,
				Why: "levanta as decisões que faltam antes de /" + name + " escrever"})
		}
		phases = append(phases, Phase{Role: name, Produces: c.Produces, Tier: c.Tier, Base: c.Tier, Why: why})
		for _, next := range c.Then {
			if err := visit(next, "sempre depois de "+name, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(target, "entrega "+artifact+", o alvo do pedido", 0); err != nil {
		return nil, err
	}
	return phases, nil
}

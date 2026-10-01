// Package role reads the contract of a role — a gofi skill — from the
// contract.yaml beside its SKILL.md.
//
// The open skills standard only takes string pairs in a skill's metadata, and
// a role's contract is structured, so it lives in a file of its own that no
// host loads: only gofi reads it. The contract says what the role delivers,
// what has to exist before it runs, who follows it and the tier it needs —
// never a model. The intake plans from contracts alone, so a team's skill with
// a contract is routed like gofi's own, with no list of skills in the CLI.
package role

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gofi-labs/gofi/cli/internal/host"
)

// ContractFile is the contract's name, beside the role's SKILL.md.
const ContractFile = "contract.yaml"

// Artifacts are what a role can produce, and what a request can aim at.
var Artifacts = []string{"prd", "spec", "code", "ui", "infra", "docs", "audit", "status", "migration"}

// Intents are the kinds of request the intake tells apart. A requirement's
// conditions name them.
var Intents = []string{"create", "change", "fix", "review", "document", "status", "migrate", "deploy", "explain"}

// Contract is what a role declares about itself.
type Contract struct {
	// Tier is the capability the role needs; the host turns it into a model.
	Tier host.Tier `yaml:"tier"`
	// Produces is the artifact the role delivers. A role that produces none
	// — a conductor such as gofi-full — is never planned as a phase.
	Produces string `yaml:"produces"`
	// Requires lists what has to exist before the role runs; the planner puts
	// the role that produces each missing one first.
	Requires []Requirement `yaml:"requires"`
	// Then lists the roles that always follow this one (the audit after the
	// code).
	Then []string `yaml:"then"`
	// Intents are verbs that point at this role, in the words people use
	// ("documentar", "auditar"). They extend the intake's lexicon.
	Intents []string `yaml:"intents"`
	// Needs are the data the role cannot start without; the intake asks for
	// what it cannot resolve. Known: context, change.
	Needs []string `yaml:"needs"`
	// ReadsOnly says the role changes no file.
	ReadsOnly bool `yaml:"reads_only"`
	// Elicit puts a cheaper phase before the role: it runs the role's
	// checklist against the request and what the project has, and writes
	// down only the decisions nothing answers. The person answers them there,
	// and the role writes its document at once instead of interviewing on
	// its own, more expensive tier.
	Elicit *Elicit `yaml:"elicit"`
}

// Elicit is how a role's open decisions are gathered before it runs.
type Elicit struct {
	// Tier runs the elicitation; below the role's own.
	Tier host.Tier `yaml:"tier"`
	// Checklist is the role's reference the elicitation walks, relative to
	// its skill folder.
	Checklist string `yaml:"checklist"`
}

// Requirement is one artifact a role depends on.
type Requirement struct {
	Artifact string `yaml:"artifact"`
	// OnlyOn limits the requirement to these intents: a spec needs a PRD when
	// a context is created, not every time it changes.
	OnlyOn []string `yaml:"only_on"`
	// ReviseOn makes the producer run again for these intents even when the
	// artifact exists: a change of behaviour is written in the spec first.
	ReviseOn []string `yaml:"revise_on"`
}

// Applies reports whether the requirement holds for an intent.
func (r Requirement) Applies(intent string) bool {
	return len(r.OnlyOn) == 0 || slices.Contains(r.OnlyOn, intent)
}

// Revises reports whether the artifact is produced again for an intent even
// when it exists.
func (r Requirement) Revises(intent string) bool {
	return slices.Contains(r.ReviseOn, intent)
}

// Parse reads and checks a contract. Unknown keys are errors: a misspelt key
// would otherwise be a declaration that silently does nothing.
func Parse(b []byte) (Contract, error) {
	var c Contract
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return c, fmt.Errorf("contract: %w", err)
	}
	if !slices.Contains(host.Tiers, c.Tier) {
		return c, fmt.Errorf("tier: %q is not a tier (expected light, standard or deep)", c.Tier)
	}
	if c.Produces != "" && !slices.Contains(Artifacts, c.Produces) {
		return c, fmt.Errorf("produces: %q is not an artifact (expected %s)", c.Produces, strings.Join(Artifacts, ", "))
	}
	if e := c.Elicit; e != nil {
		if !slices.Contains(host.Tiers, e.Tier) {
			return c, fmt.Errorf("elicit.tier: %q is not a tier", e.Tier)
		}
		if slices.Index(host.Tiers, e.Tier) >= slices.Index(host.Tiers, c.Tier) {
			return c, fmt.Errorf("elicit.tier: %s is not below the role's %s — the elicitation would cost what it saves", e.Tier, c.Tier)
		}
		if e.Checklist == "" {
			return c, errors.New("elicit.checklist: the reference the elicitation walks is required")
		}
	}
	for _, r := range c.Requires {
		if !slices.Contains(Artifacts, r.Artifact) {
			return c, fmt.Errorf("requires: %q is not an artifact", r.Artifact)
		}
		for _, i := range append(slices.Clone(r.OnlyOn), r.ReviseOn...) {
			if !slices.Contains(Intents, i) {
				return c, fmt.Errorf("requires %s: %q is not an intent (expected %s)", r.Artifact, i, strings.Join(Intents, ", "))
			}
		}
	}
	return c, nil
}

// Role is an installed role and its contract.
type Role struct {
	Name     string
	Contract Contract
}

// Load reads the contract of every role installed under skillsDir, sorted by
// name. A skill without a contract is left out — it runs when invoked, it is
// just not planned. A contract that does not parse is returned as a problem.
func Load(skillsDir string) ([]Role, []error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, nil
	}
	var roles []Role
	var problems []error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(skillsDir, e.Name(), ContractFile))
		if err != nil {
			continue
		}
		c, err := Parse(b)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		roles = append(roles, Role{Name: e.Name(), Contract: c})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
	return roles, problems
}

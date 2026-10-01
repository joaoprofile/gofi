// Package host knows the coding agents a gofi project can run on — Claude Code,
// Codex, Copilot — and what each one needs from gofi: the folder it discovers
// agent content in, where it reads MCP servers from, whether it can run the
// guard, and which of its models serves each capability tier.
//
// Everything above this package speaks tiers and hosts, never file names or
// model names of a vendor: a project changes host by changing one line, and
// gofi regenerates what the new host reads.
package host

import (
	"slices"
	"strings"
)

// Tier is a capability level, the only thing skills and routing ever name.
type Tier string

const (
	// Light reads and answers: lookups, status, documenting what exists.
	Light Tier = "light"
	// Standard writes with a clear scope: a localised change, a review.
	Standard Tier = "standard"
	// Deep decides and designs: PRDs, specs, changes across contexts.
	Deep Tier = "deep"
)

// Tiers lists the tiers, cheapest first.
var Tiers = []Tier{Light, Standard, Deep}

// Host is one coding agent and what gofi generates for it.
type Host struct {
	ID    string // the value of ai.host
	Label string
	// Home is the folder the host discovers skills in; the project's whole
	// agent content lives there. See layout.HomeFor.
	Home string
	// MCP is the project file the host reads MCP servers from.
	MCP MCPFormat
	// Guard reports whether gofi installs its guard for this host: only where
	// the host's hook contract is documented and verified.
	Guard bool
	// Chat reports whether gofi chat can drive this host.
	Chat bool
	// SkillModel reports whether the host's skill format has a model field,
	// so a skill can name the model its tier needs. Claude Code's does; the
	// open standard does not, and an unknown key there is an error.
	SkillModel bool
	// Models map each tier to the host's model. An empty map means gofi does
	// not presume the host's model names: the team sets ai.tiers, or the
	// session's model serves every tier.
	Models map[Tier]string
}

// The hosts gofi supports. Model names appear here and nowhere else.
var (
	ClaudeCode = Host{
		ID: "claude-code", Label: "Claude Code", Home: ".claude",
		MCP: MCPClaude, Guard: true, Chat: true, SkillModel: true,
		Models: map[Tier]string{Light: "haiku", Standard: "sonnet", Deep: "opus"},
	}
	Codex = Host{
		ID: "codex", Label: "Codex", Home: ".agents",
		MCP: MCPCodex,
	}
	Copilot = Host{
		ID: "copilot", Label: "GitHub Copilot", Home: ".agents",
		MCP: MCPCopilot,
	}
)

// All lists the supported hosts, in the order the wizard offers them.
var All = []Host{ClaudeCode, Codex, Copilot}

// legacy are ai.host values older releases wrote, and the host they mean.
var legacy = map[string]string{"claude-vscode": ClaudeCode.ID}

// Get returns the host for an ai.host value, old spellings included.
func Get(id string) (Host, bool) {
	if canon, ok := legacy[id]; ok {
		id = canon
	}
	i := slices.IndexFunc(All, func(h Host) bool { return h.ID == id })
	if i < 0 {
		return Host{}, false
	}
	return All[i], true
}

// IDs lists the ai.host values gofi accepts, old spellings included.
func IDs() []string {
	out := make([]string, 0, len(All)+len(legacy))
	for _, h := range All {
		out = append(out, h.ID)
	}
	for old := range legacy {
		out = append(out, old)
	}
	slices.Sort(out)
	return out
}

// IsClaude reports whether an ai.host value is Claude Code, under any
// spelling.
func IsClaude(id string) bool {
	h, ok := Get(id)
	return ok && h.ID == ClaudeCode.ID
}

// Model returns the model that serves a tier: the project's override first,
// then the host's own default. Empty means the session's model.
func (h Host) Model(t Tier, overrides map[string]string) string {
	if m := strings.TrimSpace(overrides[string(t)]); m != "" {
		return m
	}
	return h.Models[t]
}

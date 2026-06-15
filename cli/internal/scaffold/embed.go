package scaffold

import "embed"

// embeddedFS carries only the language scaffold templates (golang/, rust/).
// Agents, SDK content, prompts and templates are fetched from
// github.com/joaoprofile/gofi-agents at install time — there is no embedded
// fallback for that content.
//
//go:embed all:embedded
var embeddedFS embed.FS

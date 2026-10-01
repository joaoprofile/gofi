package scaffold

import "embed"

// embeddedFS carries the backend scaffold templates (golang/) and the starters.
// Agents, SDK content, prompts and templates are fetched from
// github.com/joaoprofile/gofi at install time — there is no embedded
// fallback for that content.
//
//go:embed all:embedded
var embeddedFS embed.FS

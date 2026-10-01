package guard

import "regexp"

// ReadOnlyRules are Claude Code permission rules for the gofi calls that only
// read the index. Allowing them ahead of time is what makes asking the index
// cheaper than grepping: a query that stops for a permission prompt loses to
// the tool that does not.
var ReadOnlyRules = []string{
	MCPPrefix + "find",
	MCPPrefix + "show",
	MCPPrefix + "path",
	MCPPrefix + "index_status",
	MCPPrefix + "intake",
	"Bash(gofi find *)",
	"Bash(gofi show *)",
	"Bash(gofi path *)",
	"Bash(gofi index status)",
	"Bash(gofi index status *)",
	"Bash(gofi intake *)",
}

// WriteRules are the gofi calls that write where Claude Code would stop an
// agent: a context's memory lives under .claude/, which it treats as
// sensitive whatever the rules say (see gofi memory write).
var WriteRules = []string{
	"Bash(gofi memory write *)",
}

// A read-only gofi command, optionally after a cd, and nothing else.
var reReadOnly = regexp.MustCompile(`^(\S*/)?gofi\s+(find|show|path|index\s+status)(\s|$)`)
var reCd = regexp.MustCompile(`^cd\s+\S+$`)

// ReadOnlyCommand reports whether every command in a shell line is a gofi
// query or a cd. Anything else in the chain — a redirect, a second command —
// makes it an ordinary shell call that must be approved as one.
func ReadOnlyCommand(cmd string) bool {
	if regexp.MustCompile(`[<>` + "`" + `$]`).MatchString(cmd) {
		return false
	}
	segs := segments(cmd)
	if len(segs) == 0 {
		return false
	}
	query := false
	for _, s := range segs {
		switch {
		case reReadOnly.MatchString(s):
			query = true
		case reCd.MatchString(s):
		default:
			return false
		}
	}
	return query
}

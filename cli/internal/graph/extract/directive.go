package extract

import (
	"go/ast"
	"strings"
)

// DirectiveTool is the tool name gofi answers to in a //tool:name directive.
const DirectiveTool = "gofi"

// ContextDirective tags a declaration with the gofi context it implements:
//
//	//gofi:context billing
//
// It is what ties a symbol in the graph back to specs/{context}/ and
// .claude/memory/contexts/{context}.md. Without it, an agent asked to work on a
// context has to guess which packages implement it from their names; with it,
// the graph answers directly.
//
// On a package clause the directive is the default for every symbol in that
// package, and a declaration can still override it with its own.
const ContextDirective = "context"

// contextOf returns the context named by a //gofi:context directive in the
// comment group, or "" when the group carries none.
func contextOf(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	for _, c := range doc.List {
		d, ok := ast.ParseDirective(c.Slash, c.Text)
		if !ok || d.Tool != DirectiveTool || d.Name != ContextDirective {
			continue
		}
		if name := normalizeContext(d.Args); name != "" {
			return name
		}
	}
	return ""
}

// normalizeContext reduces the directive argument to the slug used for
// directory names under specs/ and .claude/memory/contexts/. Anything that
// cannot be part of such a name ends the value, so a stray trailing comment
// never becomes part of the context.
func normalizeContext(args string) string {
	s := strings.ToLower(strings.TrimSpace(args))
	if i := strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}); i >= 0 {
		s = s[:i]
	}
	return s
}

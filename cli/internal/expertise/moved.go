package expertise

// Moved maps what older versions of gofi seeded into the project's knowledge/
// to the file that replaced it: knowledge-relative on the left, relative to the
// agents folder on the right. knowledge/ is the team's now; a seeded copy left
// there is a second, older version of a gofi section, and both would answer
// the same search.
var Moved = map[string]string{
	"shared/graph-retrieval-protocol.md":      "expertise/harness-protocols/graph-retrieval.md",
	"shared/rag-retrieval-protocol.md":        "expertise/harness-protocols/rag-retrieval.md",
	"shared/memory-protocol.md":               "expertise/harness-protocols/memory.md",
	"shared/learning-protocol.md":             "expertise/harness-protocols/learning.md",
	"shared/document-versioning.md":           "expertise/harness-protocols/document-versioning.md",
	"shared/llm-scoring-prompt.md":            "expertise/harness-protocols/llm-scoring-prompt.md",
	"shared/ddd-principles.md":                "expertise/ddd-architecture/ddd-principles.md",
	"shared/application-vs-domain-service.md": "expertise/ddd-architecture/application-vs-domain-service.md",
	"shared/clean-code.md":                    "expertise/ddd-architecture/clean-code.md",
	"shared/id-types.md":                      "expertise/ddd-architecture/id-types.md",
	"eng/impact-analysis-on-change.md":        "expertise/ddd-architecture/impact-analysis-on-change.md",
	"shared/event-driven-executor-pattern.md": "expertise/event-driven/executor-pattern.md",
	"shared/kafka-type-naming.md":             "expertise/event-driven/kafka-type-naming.md",
	"shared/diagram-conventions.md":           "expertise/diagramming/conventions.md",
	"ui/design-tokens.md":                     "expertise/ui-design/design-tokens.md",
	"ui/theming-dark-mode.md":                 "expertise/ui-design/theming-dark-mode.md",
	"ui/ux-principles.md":                     "expertise/ui-design/ux-principles.md",
	"eng/env-file-management.md":              "expertise/platform-delivery/env-file-management.md",
	"eng/rbac-helper.md":                      "sdk/go/knowledge/rbac.md",
}

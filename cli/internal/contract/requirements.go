package contract

import "github.com/joaoprofile/gofi/cli/internal/docs"

// The grammar of requirements is the corpus reader's (docs): the contract
// and the document graph read one, never two that drift.
type (
	Requirement = docs.Requirement
	Scenario    = docs.Scenario
	Step        = docs.Step
	Section     = docs.DeltaSection
)

// Delta operations (0003, S3.3).
const (
	OpAdded    = docs.OpAdded
	OpModified = docs.OpModified
	OpRemoved  = docs.OpRemoved
)

// Requirements reads the requirements of a document, with their scenarios.
func Requirements(d Doc) []Requirement { return docs.ParseRequirements(d.Body, d.Offset) }

// DeltaSections reads the ADICIONADO / MODIFICADO / REMOVIDO sections of a delta.
func DeltaSections(d Doc) []Section { return docs.ParseDelta(d.Body, d.Offset) }

// Parent is the requirement a scenario id belongs to: REQ-04 for REQ-04.1.
func Parent(scenarioID string) string { return docs.Parent(scenarioID) }

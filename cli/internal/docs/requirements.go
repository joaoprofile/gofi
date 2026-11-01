package docs

import (
	"regexp"
	"strings"
)

// The grammar of requirements and scenarios in a format-3 spec or delta
// (decision 0003, S3). It lives here, with the corpus reader, so the document
// graph and the contract read one grammar — never two that drift.

// Requirement is one `### Requisito: REQ-NN — título` block, with
// the scenarios under it. ID is empty when the heading did not carry a
// well-formed one: the parser keeps the block so the check can say so.
type Requirement struct {
	ID        string
	Title     string
	Line      int      // file line of the heading
	Text      []string // body lines before the first scenario, trimmed, blanks out
	Scenarios []Scenario
}

// Scenario is one `#### Cenário: REQ-NN.n — título` block.
type Scenario struct {
	ID    string
	Title string
	Line  int
	// Steps are the DADO / QUANDO / ENTÃO / E lines, keyword upper-cased and
	// without accents (DADO, QUANDO, ENTAO, E), in the order written.
	Steps []Step
	// Other are the body lines that are not a step, trimmed, blanks out.
	Other []string
}

// Step is one line of a scenario.
type Step struct {
	Keyword string
	Text    string
	Line    int
}

// Delta operations (0003, S3.3): the sections of a delta, by what they do to
// the living spec.
const (
	OpAdded    = "ADICIONADO"
	OpModified = "MODIFICADO"
	OpRemoved  = "REMOVIDO"
)

var (
	reRequirement = regexp.MustCompile(`^###\s+Requisito:\s*(.*)$`)
	reScenario    = regexp.MustCompile(`^####\s+Cen[áa]rio:\s*(.*)$`)
	reReqID       = regexp.MustCompile(`^REQ-\d{2,}$`)
	reScenarioID  = regexp.MustCompile(`^(REQ-\d{2,})\.\d+$`)
	reStep        = regexp.MustCompile(`(?i)^[-*]\s+(DADO|QUANDO|ENT[ÃA]O|E)\s+(.*)$`)
	reOp          = regexp.MustCompile(`(?i)^##\s+(ADICIONADOS?|MODIFICADOS?|REMOVIDOS?)\s*$`)
	reHeading2    = regexp.MustCompile(`^##\s`)
	reFence       = regexp.MustCompile("^\\s*(```|~~~)")
)

// IsFence reports whether a line opens or closes a fenced code block.
func IsFence(ln string) bool { return reFence.MatchString(ln) }

// RequirementHeading reads a `### Requisito:` line. ok is false for any other
// line; id is empty when the heading carries no well-formed REQ-NN, and title
// is then everything after the colon.
func RequirementHeading(ln string) (id, title string, ok bool) {
	m := reRequirement.FindStringSubmatch(ln)
	if m == nil {
		return "", "", false
	}
	id, title = splitHeading(m[1])
	if !reReqID.MatchString(id) {
		return "", strings.TrimSpace(m[1]), true
	}
	return id, title, true
}

// ScenarioHeading reads a `#### Cenário:` line, as RequirementHeading does.
func ScenarioHeading(ln string) (id, title string, ok bool) {
	m := reScenario.FindStringSubmatch(ln)
	if m == nil {
		return "", "", false
	}
	id, title = splitHeading(m[1])
	if !reScenarioID.MatchString(id) {
		return "", strings.TrimSpace(m[1]), true
	}
	return id, title, true
}

// DeltaOp reads a delta section heading (## ADICIONADO, a plural too) into
// its operation.
func DeltaOp(ln string) (op string, ok bool) {
	m := reOp.FindStringSubmatch(ln)
	if m == nil {
		return "", false
	}
	return strings.TrimSuffix(strings.ToUpper(m[1]), "S"), true
}

// splitHeading reads "REQ-04 — Título" (an em dash, an en dash or a hyphen
// between them, or nothing) into the id and the title.
func splitHeading(s string) (id, title string) {
	s = strings.TrimSpace(s)
	id, rest, _ := strings.Cut(s, " ")
	rest = strings.TrimSpace(rest)
	for _, sep := range []string{"—", "–", "-", ":"} {
		if t, ok := strings.CutPrefix(rest, sep); ok {
			rest = strings.TrimSpace(t)
			break
		}
	}
	return id, rest
}

// ParseRequirements reads the requirements in a body: every Requisito block,
// with its scenarios. offset is the file line the body starts at, so each
// block carries the line the reader sees.
func ParseRequirements(body []string, offset int) []Requirement {
	return parseRequirements(body, offset, 0, len(body))
}

// DeltaSection is a delta section and the requirements in it.
type DeltaSection struct {
	Op           string // OpAdded, OpModified, OpRemoved
	Line         int
	Requirements []Requirement
}

// ParseDelta reads the ADICIONADO / MODIFICADO / REMOVIDO sections of a
// delta, in the order written. A plural (ADICIONADOS) reads as the singular;
// a ## that is not an operation ends the one before it.
func ParseDelta(body []string, offset int) []DeltaSection {
	var out []DeltaSection
	var cur *DeltaSection
	start := 0
	flush := func(end int) {
		if cur != nil {
			cur.Requirements = parseRequirements(body, offset, start, end)
			out = append(out, *cur)
		}
	}
	fenced := false
	for i, ln := range body {
		if reFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if fenced || !reHeading2.MatchString(ln) {
			continue
		}
		flush(i)
		cur = nil
		if op, ok := DeltaOp(ln); ok {
			cur = &DeltaSection{Op: op, Line: offset + i}
			start = i + 1
		}
	}
	flush(len(body))
	return out
}

// parseRequirements reads the Requisito blocks in body lines [from, to).
// Inside a delta section the next ## ends the range before it is called.
func parseRequirements(body []string, offset, from, to int) []Requirement {
	var out []Requirement
	var req *Requirement
	var sc *Scenario
	closeScenario := func() {
		if sc != nil && req != nil {
			req.Scenarios = append(req.Scenarios, *sc)
		}
		sc = nil
	}
	closeRequirement := func() {
		closeScenario()
		if req != nil {
			out = append(out, *req)
		}
		req = nil
	}
	fenced := false
	for i := from; i < to; i++ {
		ln := body[i]
		line := offset + i
		if reFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if id, title, ok := RequirementHeading(ln); ok {
			closeRequirement()
			req = &Requirement{ID: id, Title: title, Line: line}
			continue
		}
		if id, title, ok := ScenarioHeading(ln); ok {
			closeScenario()
			sc = &Scenario{ID: id, Title: title, Line: line}
			continue
		}
		if reHeading2.MatchString(ln) || strings.HasPrefix(ln, "### ") {
			// Any other heading ends the block: what follows is not part of it.
			closeRequirement()
			continue
		}
		text := strings.TrimSpace(ln)
		if text == "" || req == nil {
			continue
		}
		if sc != nil {
			if m := reStep.FindStringSubmatch(text); m != nil {
				sc.Steps = append(sc.Steps, Step{Keyword: keyword(m[1]), Text: strings.TrimSpace(m[2]), Line: line})
			} else {
				sc.Other = append(sc.Other, text)
			}
			continue
		}
		req.Text = append(req.Text, text)
	}
	closeRequirement()
	return out
}

// keyword normalizes a step keyword: upper case, no accent.
func keyword(k string) string {
	k = strings.ToUpper(k)
	return strings.ReplaceAll(k, "Ã", "A")
}

// Parent is the requirement a scenario id belongs to: REQ-04 for REQ-04.1.
func Parent(scenarioID string) string {
	if m := reScenarioID.FindStringSubmatch(scenarioID); m != nil {
		return m[1]
	}
	return ""
}

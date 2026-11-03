// Package review is the semantic review of a change before the person
// approves it (0002, D17, gofi-spec-review): what gofi spec check cannot
// see — a requirement two readers would build differently, two that
// contradict each other, a scenario whose outcome no test can observe, what
// the discovery asked for and the change left out. Its findings go to the
// person, who corrects the change or approves it as it is; the review never
// edits the contract.
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Schema names the format of a review.
const Schema = "gofi-review/v1"

// Dir is where reviews are kept, under the project root: one file a change.
const Dir = ".gofi/reviews"

// Severities a review finding takes: a blocker cannot be implemented as
// written, a major forces the implementation to guess, a minor is worth
// fixing and stops nothing.
var Severities = []string{"blocker", "major", "minor"}

// Review is what the reviewer found in a change, as it was when reviewed.
type Review struct {
	Schema   string    `json:"schema"`
	Change   string    `json:"change"`
	Hash     string    `json:"hash"`
	Findings []Finding `json:"findings"`
}

// Finding is one thing the change says badly, or does not say.
type Finding struct {
	Severity string `json:"severity"`
	// Kind is what is wrong: ambiguidade, contradição, não testável,
	// lacuna, escopo.
	Kind string `json:"kind"`
	// Where is the document and the requirement or section: deltas/x.md#REQ-03.2.
	Where      string `json:"where"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// Example is the shape of a review, as the reviewer is told to write it.
var Example = fmt.Sprintf(`{"schema": %q, "findings": [{"severity": "major", "kind": "ambiguidade", "where": "deltas/<ctx>.md#REQ-01.2", "message": "...", "suggestion": "..."}]}`, Schema)

// Blocking are the findings that stop the approval: blockers and majors.
func (r *Review) Blocking() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Severity != "minor" {
			out = append(out, f)
		}
	}
	return out
}

// Parse reads what the reviewer wrote. Fields gofi does not know are
// ignored; a severity it does not know is an error — a finding read as
// nothing would let the change through.
func Parse(data []byte) (*Review, error) {
	var r Review
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("review is not valid JSON of %s: %w", Schema, err)
	}
	if r.Schema != "" && r.Schema != Schema {
		return nil, fmt.Errorf("schema %q: this gofi reads %s", r.Schema, Schema)
	}
	r.Schema = Schema
	if r.Findings == nil {
		r.Findings = []Finding{}
	}
	var problems []string
	for i := range r.Findings {
		f := &r.Findings[i]
		f.Severity = strings.ToLower(strings.TrimSpace(f.Severity))
		if !slices.Contains(Severities, f.Severity) {
			problems = append(problems, fmt.Sprintf("finding %d: severity %q — %s", i+1, f.Severity, strings.Join(Severities, ", ")))
		}
		if strings.TrimSpace(f.Message) == "" {
			problems = append(problems, fmt.Sprintf("finding %d: the message is required", i+1))
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	slices.SortStableFunc(r.Findings, func(a, b Finding) int {
		return slices.Index(Severities, a.Severity) - slices.Index(Severities, b.Severity)
	})
	return &r, nil
}

// Hash is what a review is of: the bodies of the change's documents, by
// path. A change edited after its review is reviewed again.
func Hash(bodies map[string][]string) string {
	paths := make([]string, 0, len(bodies))
	for p := range bodies {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	h := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(h, "%s\x00%s\x00", p, strings.TrimRight(strings.Join(bodies[p], "\n"), " \t\r\n"))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// Path is where a change's review is kept.
func Path(root, change string) string {
	return filepath.Join(root, filepath.FromSlash(Dir), change+".json")
}

// Load reads the review kept for a change; nil, with no error, when there
// is none.
func Load(root, change string) (*Review, error) {
	data, err := os.ReadFile(Path(root, change))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Save keeps a change's review.
func Save(root string, r *Review) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	p := Path(root, r.Change)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

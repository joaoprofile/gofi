// Package verdict is the audit's result as data (0002, D5): the findings, each
// with a severity, an owner and a fingerprint, instead of a status in a
// memory file. The conductor routes on the owner, tells progress from a
// finding that comes back by the fingerprint, and the person reads the same
// file the loop does.
package verdict

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// Schema names the format of a verdict file.
const Schema = "gofi-verdict/v1"

// Results, as the QA writes them in the context's memory too.
const (
	Approved = "aprovado"
	Rejected = "reprovado"
)

// Severities: the labels of the QA's report, lowercase. Only these exist.
var severities = map[string]int{"blocker": 0, "major": 1, "minor": 2, "suggestion": 3}

// Owners: whose work a finding sends the loop back to — the implementation,
// the change (its deltas or design), or the discovery behind it.
var owners = map[string]bool{"eng": true, "spec": true, "pd": true}

// Verdict is one audit of a context.
type Verdict struct {
	Schema   string    `json:"schema"`
	Context  string    `json:"contexto"`
	Change   string    `json:"mudanca,omitempty"`
	Run      string    `json:"run,omitempty"`
	Iter     int       `json:"iter,omitempty"`
	Result   string    `json:"result"`
	Findings []Finding `json:"findings"`
}

// Finding is one problem the audit found.
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	// Rule is what the finding is held against: a scenario (pedidos/REQ-04.1),
	// a principle, a rule of the pack's checklist.
	Rule     string `json:"rule"`
	Category string `json:"category,omitempty"`
	// Symbol is the code graph node the finding is about, when there is one:
	// it is what keeps a fingerprint stable when lines move.
	Symbol      string `json:"symbol,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Owner       string `json:"owner"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Message     string `json:"message"`
	Evidence    string `json:"evidence,omitempty"`
}

// Caveats counts the majors and minors still open — what the memory's
// ressalvas field carries.
func (v *Verdict) Caveats() int {
	n := 0
	for _, f := range v.Findings {
		if f.Severity == "major" || f.Severity == "minor" {
			n++
		}
	}
	return n
}

// Clean reports whether the audit accepts the delivery: approved with no
// caveat. Suggestions do not count.
func (v *Verdict) Clean() bool { return v.Result == Approved && v.Caveats() == 0 }

// Fingerprint identifies a finding across audits: the rule, the symbol (or
// the file, when the finding has no symbol) and the category — not the line,
// which moves with every edit above it.
func Fingerprint(f Finding) string {
	where := f.Symbol
	if where == "" {
		where = f.File
	}
	sum := sha1.Sum([]byte(strings.ToLower(f.Rule + "|" + where + "|" + f.Category)))
	return hex.EncodeToString(sum[:])[:12]
}

// Parse reads a verdict, checks it and fills in what gofi derives: the schema
// when missing, ids when missing, every fingerprint. Findings are sorted by
// severity, then id, so two writes of the same audit are the same bytes.
func Parse(data []byte, ctx string) (*Verdict, error) {
	var v Verdict
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("verdict is not valid JSON of %s: %w", Schema, err)
	}
	if v.Schema == "" {
		v.Schema = Schema
	}
	if v.Schema != Schema {
		return nil, fmt.Errorf("schema %q: this gofi writes %s", v.Schema, Schema)
	}
	if v.Context == "" {
		v.Context = ctx
	}
	if v.Context != ctx {
		return nil, fmt.Errorf("the verdict is about %q, not %q", v.Context, ctx)
	}
	if v.Findings == nil {
		v.Findings = []Finding{}
	}
	var problems []string
	blockers := 0
	for i := range v.Findings {
		f := &v.Findings[i]
		f.Severity = strings.ToLower(strings.TrimSpace(f.Severity))
		if _, ok := severities[f.Severity]; !ok {
			problems = append(problems, fmt.Sprintf("finding %d: severity %q — blocker, major, minor or suggestion", i+1, f.Severity))
		}
		if !owners[f.Owner] {
			problems = append(problems, fmt.Sprintf("finding %d: owner %q — eng, spec or pd", i+1, f.Owner))
		}
		if strings.TrimSpace(f.Rule) == "" || strings.TrimSpace(f.Message) == "" {
			problems = append(problems, fmt.Sprintf("finding %d: rule and message are required", i+1))
		}
		if f.ID == "" {
			f.ID = fmt.Sprintf("F-%d", i+1)
		}
		f.Fingerprint = Fingerprint(*f)
		if f.Severity == "blocker" {
			blockers++
		}
	}
	switch {
	case v.Result != Approved && v.Result != Rejected:
		problems = append(problems, fmt.Sprintf("result %q — %s or %s", v.Result, Approved, Rejected))
	case v.Result == Approved && blockers > 0:
		problems = append(problems, fmt.Sprintf("result %s with %d blocker(s): a blocker rejects the delivery", Approved, blockers))
	case v.Result == Rejected && blockers == 0 && v.Caveats() == 0:
		problems = append(problems, fmt.Sprintf("result %s with no blocker, major or minor: nothing rejects it", Rejected))
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "\n"))
	}
	sort.SliceStable(v.Findings, func(i, j int) bool {
		a, b := v.Findings[i], v.Findings[j]
		if severities[a.Severity] != severities[b.Severity] {
			return severities[a.Severity] < severities[b.Severity]
		}
		return a.ID < b.ID
	})
	return &v, nil
}

// Path is where a context's verdict lives.
func Path(root, ctx string) string {
	return filepath.Join(root, filepath.FromSlash(layout.VerdictDir), ctx+".json")
}

// Write stores a verdict, atomically.
func Write(root string, v *Verdict) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	p := Path(root, v.Context)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return p, os.Rename(tmp, p)
}

// Read loads a context's verdict; nil when there is none.
func Read(root, ctx string) (*Verdict, error) {
	data, err := os.ReadFile(Path(root, ctx))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var v Verdict
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

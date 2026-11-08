package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// FindingsFile keeps what each audit of a run found, kept as it was said: the
// context's verdict is rewritten by the next audit.
const FindingsFile = "findings.ndjson"

// Recorded is one finding of one audit.
type Recorded struct {
	Iter        int    `json:"iter"`
	Fingerprint string `json:"fingerprint"`
	Severity    string `json:"severity"`
	Owner       string `json:"owner"`
	Rule        string `json:"rule"`
	File        string `json:"file,omitempty"`
	Message     string `json:"message"`
}

func (r *Runner) keepFindings(s *State, v *verdict.Verdict) {
	if len(v.Findings) == 0 {
		return
	}
	var b strings.Builder
	for _, f := range v.Findings {
		line, err := json.Marshal(Recorded{Iter: s.Iter, Fingerprint: f.Fingerprint, Severity: f.Severity, Owner: f.Owner, Rule: f.Rule, File: f.File, Message: f.Message})
		if err == nil {
			b.Write(append(line, '\n'))
		}
	}
	file, err := os.OpenFile(filepath.Join(Dir(r.Root, s.Run), FindingsFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(b.String())
}

// ReadFindings is what every audit of a run found, in order; the last word on
// a finding is the last one recorded.
func ReadFindings(root, run string) []Recorded {
	data, err := os.ReadFile(filepath.Join(Dir(root, run), FindingsFile))
	if err != nil {
		return nil
	}
	var out []Recorded
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var f Recorded
		if json.Unmarshal([]byte(line), &f) == nil && f.Fingerprint != "" {
			out = append(out, f)
		}
	}
	return out
}

// Situation is how a finding of the run ended.
func Situation(s *State, f Recorded) string {
	has := func(list []Finding) bool {
		for _, x := range list {
			if x.Fingerprint == f.Fingerprint {
				return true
			}
		}
		return false
	}
	switch {
	case has(s.Open):
		return "em aberto"
	case has(s.Notes):
		return "ressalva do contrato"
	case f.Severity == "suggestion":
		return "sugestão"
	}
	return "resolvido"
}

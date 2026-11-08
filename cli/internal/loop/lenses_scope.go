package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/role"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

// lensFloorUSD is the least a lens is worth starting with. Lenses that ran to
// the end cost between US$ 0.18 and 0.32 (measured, a backend change on
// Sonnet); given US$ 0.16 each, five of them were cut before writing a verdict
// and what they spent bought nothing.
const lensFloorUSD = 0.35

// shortOfLenses reports that what is left of the budget would not carry n
// lenses to a verdict: the run stops at the node, resumable, having spent
// nothing on it. No budget, no check.
func (r *Runner) shortOfLenses(s *State, n int) (string, bool) {
	b := r.Policy.BudgetUSD
	if b <= 0 || n == 0 {
		return "", false
	}
	left := b - s.CostUSD
	if left >= lensFloorUSD*float64(n) {
		return "", false
	}
	return fmt.Sprintf("orçamento insuficiente para a auditoria: restam US$ %.2f para %d lentes, e cada uma precisa de US$ %.2f — nenhuma foi disparada; aumente loop.budget_usd e retome com gofi run --resume %s",
		max(left, 0), n, lensFloorUSD, s.Run), true
}

// AuditMark is what the last audit left for the next one: the lenses that
// found nothing, and the code as they read it.
type AuditMark struct {
	Clean []string          `json:"clean,omitempty"`
	Files map[string]string `json:"files,omitempty"`
	// Removed is what the change had removed, as one string.
	Removed string `json:"removed,omitempty"`
}

// scopeOf is the layers a lens is dispatched by; empty, a lens of the whole
// change, which anything the rework touched concerns.
func scopeOf(rl role.Role) []string {
	var out []string
	for _, t := range rl.Contract.Triggers {
		for _, p := range append(slices.Clone(t.All), t.Any...) {
			if l, ok := strings.CutPrefix(p, "diff.touches("); ok {
				out = append(out, strings.TrimSuffix(l, ")"))
			}
		}
	}
	return out
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// sinceAudit is what the code did after the last audit read it: whether
// anything changed, and the layers of what did. A symbol the graph places in
// no layer changes something without touching any layer, as for dispatch.
func (r *Runner) sinceAudit(s *State) (changed bool, layers map[string]bool) {
	layers = map[string]bool{}
	if strings.Join(s.Diff.Removed, ",") != s.Audited.Removed {
		changed = true
		for _, l := range role.Layers {
			layers[l] = true
		}
	}
	for _, t := range s.Diff.Touched {
		if t.File == "" {
			continue
		}
		if was, seen := s.Audited.Files[t.File]; seen && was == fileHash(filepath.Join(r.work(), filepath.FromSlash(t.File))) {
			continue
		}
		changed = true
		if t.Layer != "" {
			layers[t.Layer] = true
		}
	}
	return changed, layers
}

// affected keeps the lenses the rework concerns. A lens is spared only when
// the last audit of this run found nothing through it and nothing in its
// scope changed since; each one spared says why in the run's log.
func (r *Runner) affected(name string, roles []role.Role, s *State) (run []role.Role, kept []string) {
	if s.Audited == nil || s.Diff == nil {
		return roles, nil
	}
	changed, layers := r.sinceAudit(s)
	for _, rl := range roles {
		scope := scopeOf(rl)
		spared := slices.Contains(s.Audited.Clean, rl.Name) &&
			!slices.ContainsFunc(scope, func(l string) bool { return layers[l] }) &&
			(len(scope) > 0 || !changed)
		if !spared {
			run = append(run, rl)
			continue
		}
		kept = append(kept, rl.Name)
		r.emit(s, Event{Ev: "skip", Node: name, To: rl.Name, Why: "lente sem achados na última auditoria, e o retrabalho não tocou o escopo dela"})
	}
	return run, kept
}

// markAudit is what this audit leaves for the next.
func (r *Runner) markAudit(vs []*verdict.Verdict, kept []string, s *State) *AuditMark {
	m := &AuditMark{Clean: slices.Clone(kept), Files: map[string]string{}}
	for _, v := range vs {
		if v.Result == verdict.Approved && len(v.Findings) == 0 {
			m.Clean = append(m.Clean, v.Lens)
		}
	}
	if s.Diff != nil {
		m.Removed = strings.Join(s.Diff.Removed, ",")
		for _, t := range s.Diff.Touched {
			if t.File != "" {
				m.Files[t.File] = fileHash(filepath.Join(r.work(), filepath.FromSlash(t.File)))
			}
		}
	}
	return m
}

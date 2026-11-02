package verdict

import (
	"fmt"
	"slices"
	"sort"
)

// ownerRank orders the owners a merge keeps when lenses disagree: the
// contract's before the code's — fixing the code cannot fix a contract that
// is wrong, and whoever reads the contract decides.
var ownerRank = map[string]int{"pd": 0, "spec": 1, "eng": 2}

// Merge joins the verdicts of the lenses of one audit into the context's
// verdict, deterministically, with no model (0002, D17): a finding is one
// per fingerprint, at the highest severity any lens gave it; the delivery is
// rejected when any lens rejected it or a blocker is open. It reports the
// findings whose owner the lenses disagreed on, by fingerprint.
func Merge(ctx, run string, iter int, lenses []*Verdict) (*Verdict, []string) {
	out := &Verdict{Schema: Schema, Context: ctx, Run: run, Iter: iter, Result: Approved, Findings: []Finding{}}
	byPrint := map[string]int{}
	var conflicts []string
	for _, l := range lenses {
		out.Lenses = append(out.Lenses, l.Lens)
		if out.Change == "" {
			out.Change = l.Change
		}
		if l.Result == Rejected {
			out.Result = Rejected
		}
		for _, f := range l.Findings {
			f.Category = categoryOf(f, l.Lens)
			i, seen := byPrint[f.Fingerprint]
			if !seen {
				byPrint[f.Fingerprint] = len(out.Findings)
				out.Findings = append(out.Findings, f)
				continue
			}
			kept := &out.Findings[i]
			if kept.Owner != f.Owner {
				if !slices.Contains(conflicts, f.Fingerprint) {
					conflicts = append(conflicts, f.Fingerprint)
				}
				if ownerRank[f.Owner] < ownerRank[kept.Owner] {
					kept.Owner = f.Owner
				}
			}
			if severities[f.Severity] < severities[kept.Severity] {
				owner := kept.Owner
				*kept = f
				kept.Owner = owner
			}
		}
	}
	for _, f := range out.Findings {
		if f.Severity == "blocker" {
			out.Result = Rejected
		}
	}
	sort.SliceStable(out.Findings, func(i, j int) bool {
		a, b := out.Findings[i], out.Findings[j]
		if severities[a.Severity] != severities[b.Severity] {
			return severities[a.Severity] < severities[b.Severity]
		}
		return a.Fingerprint < b.Fingerprint
	})
	for i := range out.Findings {
		out.Findings[i].ID = fmt.Sprintf("F-%d", i+1)
	}
	sort.Strings(out.Lenses)
	sort.Strings(conflicts)
	return out, conflicts
}

// categoryOf keeps a finding's category, or names it after the lens that
// found it: the report says where each finding came from.
func categoryOf(f Finding, lens string) string {
	if f.Category != "" || lens == "" {
		return f.Category
	}
	return lens
}

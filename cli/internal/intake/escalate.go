package intake

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// VerdictRejected is the status the QA writes in the context's memory when it
// fails a delivery — the signal /gofi-full reopens the implementation on.
const VerdictRejected = "reprovado"

// fixers are the artifacts a failed audit sends back: what the QA reads was
// written by the phase that produced one of them.
var fixers = map[string]bool{"code": true, "ui": true, "infra": true}

// fillRetries gives each audit phase that follows an implementation the
// phases to run when the QA fails it (0001, D8): the implementation again, one
// tier up, then the audit again. Once — a second failure is the person's to
// look at, not another model's. Decided here, with the plan, so a conductor
// only reads the verdict and splices; the reasons are in the plan, not in it.
func fillRetries(r *Result) {
	if r.Context == nil {
		return
	}
	for i, ph := range r.Plan {
		if ph.Produces != "audit" {
			continue
		}
		fix := -1
		for j := i - 1; j >= 0; j-- {
			if fixers[r.Plan[j].Produces] {
				fix = j
				break
			}
		}
		if fix < 0 {
			continue
		}
		again := r.Plan[fix]
		again.Tier = up(again.Tier)
		again.Why = "refaz a entrega que /" + ph.Role + " reprovou"
		if again.Tier != r.Plan[fix].Tier {
			again.TierWhy = fmt.Sprintf("o QA reprovou a entrega no nível %s: refeita um nível acima", r.Plan[fix].Tier)
		} else {
			again.TierWhy = "o QA reprovou a entrega: refeita, já no nível mais alto"
		}
		again.Model, again.ReviewAfter, again.OnReject = "", false, nil
		lead := fmt.Sprintf("/%s reprovou a entrega: os achados estão no relatório e na memória de %s. Corrija-os todos, sem ampliar o escopo.\n\n", ph.Role, r.Context.Name)
		again.Turn = "/" + again.Role + " " + lead + r.Brief
		again.RoleTurn = ""
		if again.Tier != again.Base {
			again.RoleTurn = roleTurn(again, lead+r.Brief)
		}

		audit := ph
		audit.Why = "audita de novo depois da correção"
		audit.Model, audit.RoleTurn, audit.ReviewAfter, audit.OnReject = "", "", false, nil
		audit.Turn = "/" + audit.Role + " " + fmt.Sprintf("A correção de /%s terminou; audite de novo.\n\n", again.Role) + r.Brief

		r.Plan[i].OnReject = []Phase{again, audit}
		r.VerdictFile = layout.Contexts().Path(r.Context.Name + ".md")
	}
}

// up is the tier above t; deep is the highest.
func up(t host.Tier) host.Tier {
	switch t {
	case host.Light:
		return host.Standard
	default:
		return host.Deep
	}
}

// Verdict is what the context's memory says now: its bytes, to tell a verdict
// the phase wrote from one left by an earlier run. Nil when there is none.
func Verdict(root string, r *Result) []byte {
	if r.VerdictFile == "" {
		return nil
	}
	b, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(r.VerdictFile)))
	return b
}

// Rejected reports whether the audit that just ran failed the delivery: the
// context's memory changed since before the phase (before, from Verdict) and
// its status is now reprovado. An audit that wrote nothing rejects nothing —
// the conductor does not escalate on a verdict it cannot see.
func Rejected(root string, r *Result, before []byte) bool {
	now := Verdict(root, r)
	if now == nil || bytes.Equal(now, before) {
		return false
	}
	return Status(now) == VerdictRejected
}

// Status reads the status field of a document's frontmatter.
func Status(doc []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(doc))
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return ""
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return ""
		}
		if v, ok := strings.CutPrefix(line, "status:"); ok {
			return strings.ToLower(strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
	return ""
}

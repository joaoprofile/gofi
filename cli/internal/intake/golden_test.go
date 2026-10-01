package intake

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenCase is one row of testdata/golden-intake*.md.
type goldenCase struct {
	ID, Request, Context, Plan string
	Ask                        bool
}

func readGolden(t *testing.T, file string) []goldenCase {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	var out []goldenCase
	for _, line := range strings.Split(string(b), "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 5 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !strings.HasPrefix(cells[0], "I") && !strings.HasPrefix(cells[0], "H") || cells[0] == "#" {
			continue
		}
		out = append(out, goldenCase{cells[0], cells[1], cells[2], cells[3], cells[4] == "sim"})
	}
	if len(out) == 0 {
		t.Fatalf("%s has no case", file)
	}
	return out
}

// outcome renders a result in the golden's terms.
func outcome(r *Result) (ctx, plan string, ask bool) {
	switch {
	case r.Context == nil:
		ctx = "—"
	case r.Context.New:
		ctx = "novo"
	default:
		ctx = r.Context.Name
	}
	switch {
	case r.Direct != "":
		plan = "direto"
	case len(r.Plan) == 0:
		plan = "—"
	default:
		plan = strings.Join(roles(r), " → ")
	}
	return ctx, plan, r.Ask
}

// score runs a golden set and returns what matched and what did not.
func score(t *testing.T, root string, cases []goldenCase) (plans, contexts, asks int, misses []string) {
	for _, c := range cases {
		r := runIntake(t, root, c.Request, Options{})
		ctx, plan, ask := outcome(r)
		var why []string
		if plan == c.Plan {
			plans++
		} else {
			why = append(why, fmt.Sprintf("plano %q, esperado %q", plan, c.Plan))
		}
		if ctx == c.Context {
			contexts++
		} else {
			why = append(why, fmt.Sprintf("contexto %q, esperado %q", ctx, c.Context))
		}
		if ask == c.Ask {
			asks++
		} else if ask {
			why = append(why, "perguntou sem precisar")
		} else {
			why = append(why, "não perguntou e devia")
		}
		if len(why) > 0 {
			misses = append(misses, c.ID+" "+c.Request+": "+strings.Join(why, "; "))
		}
	}
	return plans, contexts, asks, misses
}

// The main golden set is a regression test: every case holds.
func TestGoldenIntake(t *testing.T) {
	cases := readGolden(t, "golden-intake.md")
	plans, contexts, asks, misses := score(t, project(t), cases)
	t.Logf("golden: plano %d/%d · contexto %d/%d · pergunta %d/%d", plans, len(cases), contexts, len(cases), asks, len(cases))
	for _, m := range misses {
		t.Error(m)
	}
}

// The holdout is measured, never tuned for: it reports, and fails no build.
func TestGoldenIntakeHoldout(t *testing.T) {
	if os.Getenv("GOFI_BENCH") == "" {
		t.Skip("set GOFI_BENCH=1 to measure the intake holdout")
	}
	cases := readGolden(t, "golden-intake-holdout.md")
	plans, contexts, asks, misses := score(t, project(t), cases)
	t.Logf("holdout: plano %d/%d · contexto %d/%d · pergunta %d/%d", plans, len(cases), contexts, len(cases), asks, len(cases))
	for _, m := range misses {
		t.Log("  " + m)
	}
}

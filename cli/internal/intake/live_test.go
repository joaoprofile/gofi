package intake

import (
	"context"
	"os"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/leancall"
)

// liveModel is the real light tier, through the lean call.
type liveModel struct{}

func (liveModel) Decide(ctx context.Context, system, prompt string, schema []byte) ([]byte, Spend, error) {
	out, sp, err := leancall.Claude{Executable: os.Getenv("GOFI_CLAUDE")}.Call(ctx, leancall.Request{
		System: system, Prompt: prompt, Schema: schema, Model: "haiku", MaxUSD: 0.05,
	})
	return out, Spend{USD: sp.USD, Millis: sp.Millis}, err
}

// The model, for real, on the points only it can settle. Opt-in: it spends.
func TestLiveModelSettlesOpenPoints(t *testing.T) {
	if os.Getenv("GOFI_BENCH_LIVE") == "" {
		t.Skip("set GOFI_BENCH_LIVE=1 to call the real light tier (about US$ 0.005 a case)")
	}
	root := project(t)
	cases := []struct {
		id, request, intent, context string
		ask                          bool
	}{
		{"L1", "rate limit estourando no envio do pricing", "fix", "pricing", false},
		{"L2", "o cancelamento de pedidos está duplicando notas", "fix", "order", false},
		{"L3", "altere as regras", "change", "", true},
	}
	var total float64
	hits := 0
	for _, c := range cases {
		r, err := Run(root, c.request, Options{Model: liveModel{}})
		if err != nil {
			t.Fatal(err)
		}
		ctx := ""
		if r.Context != nil {
			ctx = r.Context.Name
		}
		ok := r.Intent == c.intent && ctx == c.context && r.Ask == c.ask
		if ok {
			hits++
		}
		note, usd := "(modelo não consultado)", 0.0
		if r.Spent != nil {
			note, usd = r.Spent.Note, r.Spent.USD
		}
		total += usd
		t.Logf("%s ok=%v intent=%s context=%q ask=%v — %s", c.id, ok, r.Intent, ctx, r.Ask, note)
	}
	t.Logf("live: %d/%d · US$ %.4f no total", hits, len(cases), total)
}

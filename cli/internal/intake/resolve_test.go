package intake

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fakeModel answers with a fixed decision and records what it was asked.
type fakeModel struct {
	answer string
	err    error
	calls  int
	prompt string
	schema string
}

func (f *fakeModel) Decide(_ context.Context, _, prompt string, schema []byte) ([]byte, Spend, error) {
	f.calls++
	f.prompt, f.schema = prompt, string(schema)
	if f.err != nil {
		return nil, Spend{USD: 0.001}, f.err
	}
	return []byte(f.answer), Spend{USD: 0.0048, Millis: 5100}, nil
}

func decisionJSON(intent, artifact, ctx string, confident bool) string {
	b, _ := json.Marshal(decision{Intent: intent, Artifact: artifact, Context: ctx, Confident: confident, Reason: "as regras citadas estão em invoice"})
	return string(b)
}

// The model settles a context the rules left open among options, the
// decision is cached, and what it cost is on the result.
func TestModelSettlesAnAmbiguousContext(t *testing.T) {
	root := project(t)
	cache := t.TempDir()
	m := &fakeModel{answer: decisionJSON("change", "code", "invoice", true)}
	r, err := Run(root, "altere as regras", Options{Model: m, CacheDir: cache})
	if err != nil {
		t.Fatal(err)
	}
	if r.Context == nil || r.Context.Name != "invoice" || r.Ask {
		t.Fatalf("context=%+v ask=%v questions=%+v", r.Context, r.Ask, r.Questions)
	}
	if r.Spent == nil || r.Spent.USD != 0.0048 || !strings.Contains(r.Spent.Note, "contexto invoice") {
		t.Errorf("spent = %+v", r.Spent)
	}
	if !strings.Contains(r.Context.Why, "modelo light") {
		t.Errorf("the context does not say who chose it: %q", r.Context.Why)
	}
	// The model picked among the options it was given, and only those.
	for _, want := range []string{"invoice", "shipping", "integration"} {
		if !strings.Contains(m.schema, `"`+want+`"`) {
			t.Errorf("schema lacks option %s: %s", want, m.schema)
		}
	}
	// Same request, same index: the cache answers, at no cost.
	r, _ = Run(root, "altere as regras", Options{Model: m, CacheDir: cache})
	if m.calls != 1 || r.Spent == nil || !r.Spent.Cached || r.Context.Name != "invoice" {
		t.Errorf("calls=%d spent=%+v", m.calls, r.Spent)
	}
}

// A clear request, and a new context — a product decision — never reach
// the model.
func TestModelIsNotAskedWhatTheRulesSettle(t *testing.T) {
	root := project(t)
	m := &fakeModel{answer: decisionJSON("change", "code", "", true)}
	for _, q := range []string{
		"altere o fluxo de pricing, adicionando um rate limit",
		"crie um PRD de sincronização de pedidos",
	} {
		r, err := Run(root, q, Options{Model: m})
		if err != nil {
			t.Fatal(err)
		}
		if r.Spent != nil {
			t.Errorf("%q consulted the model: %+v", q, r.Spent)
		}
	}
	if m.calls != 0 {
		t.Errorf("the model was called %d time(s)", m.calls)
	}
}

// An answer outside the options, an unsure one or a failed call settles
// nothing: the question stands, and the result says why.
func TestAnUnusableAnswerLeavesTheQuestion(t *testing.T) {
	root := project(t)
	for name, m := range map[string]*fakeModel{
		"outside": {answer: decisionJSON("change", "code", "order-x", true)},
		"unsure":  {answer: decisionJSON("change", "code", "invoice", false)},
		"failed":  {err: errors.New("timeout")},
		"garbage": {answer: `{"intent": 3}`},
	} {
		r, err := Run(root, "altere as regras", Options{Model: m})
		if err != nil {
			t.Fatal(err)
		}
		if r.Context != nil || !r.Ask || r.Questions[0].ID != "context" {
			t.Errorf("%s: context=%+v questions=%+v", name, r.Context, r.Questions)
		}
		if r.Spent == nil || r.Spent.Note == "" {
			t.Errorf("%s: the result does not say what happened: %+v", name, r.Spent)
		}
	}
}

// With no verb the rules can read, the model settles the intent.
func TestModelSettlesAnIntentNoWordNamed(t *testing.T) {
	m := &fakeModel{answer: decisionJSON("fix", "code", "", true)}
	r, err := Run(project(t), "rate limit estourando no envio do pricing", Options{Model: m})
	if err != nil {
		t.Fatal(err)
	}
	if r.Intent != "fix" || m.calls != 1 {
		t.Fatalf("intent=%s calls=%d signals=%v", r.Intent, m.calls, r.Signals)
	}
	if want := []string{"gofi-eng", "gofi-qa"}; strings.Join(roles(r), ",") != strings.Join(want, ",") {
		t.Errorf("plan = %v, want %v", roles(r), want)
	}
}

// The person's answer settles the question; a bare "novo" takes the name the
// intake drew from the subject.
func TestAnswersSettleTheQuestions(t *testing.T) {
	root := project(t)
	r, err := Run(root, "altere as regras", Options{Answers: map[string]string{"context": "shipping"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Context == nil || r.Context.Name != "shipping" || r.Ask || r.Context.Why != "resposta da pessoa" {
		t.Errorf("context=%+v ask=%v", r.Context, r.Ask)
	}
	r, _ = Run(root, "crie um PRD de sincronização de pedidos", Options{Answers: map[string]string{"context": "novo"}})
	if r.Context == nil || !r.Context.New || !strings.Contains(r.Context.Name, "sincronizacao") || r.Ask {
		t.Errorf("context=%+v ask=%v", r.Context, r.Ask)
	}
	r, _ = Run(root, "altere o pricing", Options{Answers: map[string]string{"change": "limitar a 10 envios por minuto"}})
	if r.Ask || r.Change == "" || !strings.Contains(r.Brief, "10 envios por minuto") {
		t.Errorf("ask=%v change=%q", r.Ask, r.Change)
	}
}

// Going on without answering takes each default and writes it down: nothing
// is decided silently, and there is no third round.
func TestProceedWritesTheAssumptions(t *testing.T) {
	r, err := Run(project(t), "altere as regras", Options{Proceed: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Ask || r.Context == nil || len(r.Assumptions) == 0 {
		t.Fatalf("ask=%v context=%+v assumptions=%v", r.Ask, r.Context, r.Assumptions)
	}
	if !strings.Contains(r.Brief, "Suposições") || !strings.Contains(r.Brief, r.Context.Name) {
		t.Errorf("brief lacks the assumption:\n%s", r.Brief)
	}
}

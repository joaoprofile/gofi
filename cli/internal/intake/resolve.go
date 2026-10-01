package intake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/role"
)

// Model is a cheap, lean model the intake asks one closed question: a system
// prompt, a request and the JSON Schema the answer must match. The intake
// knows no host; the caller wires one (leancall for Claude Code).
type Model interface {
	Decide(ctx context.Context, system, prompt string, schema []byte) ([]byte, Spend, error)
}

// Spend is what one call cost, as the host reports it.
type Spend struct {
	USD    float64
	Millis int
}

// Spent is what the model cost a request, reported with the result: a
// decision is never cheaper than it says.
type Spent struct {
	USD    float64 `json:"usd"`
	Millis int     `json:"ms"`
	Cached bool    `json:"cached"`
	Note   string  `json:"note"`
}

// promptVersion changes whenever the question to the model does, so a cached
// decision is never read for a question it did not answer.
const promptVersion = "intake-light/v3"

// system is what the model is told once: what each intent and artifact means.
// Without it, a cheap model reads "crie um PRD" as a spec (bench B4).
const system = `Você decide pontos em aberto de um pedido de desenvolvimento, só entre as opções dadas.
intent: create = criar algo que não existe; change = alterar comportamento ou código existente; fix = corrigir um defeito; review = auditar ou revisar; document = documentar contrato; status = consultar estado; migrate = migrar o projeto; deploy = infraestrutura e publicação; explain = entender ou explicar algo do projeto, sem mudar nada.
artifact: prd = documento de produto; spec = especificação técnica (SDD); code = código de backend; ui = telas; infra = IaC e pipelines; docs = documentação de contrato; audit = auditoria; status = panorama; migration = migração da estrutura.
context: o contexto de negócio a que o pedido se refere, entre os listados; vazio se nenhum servir.
confident: false quando o pedido e as evidências não bastam para decidir — nunca chute.
reason: uma frase curta, de até 20 palavras.`

// decision is the model's answer.
type decision struct {
	Intent    string `json:"intent"`
	Artifact  string `json:"artifact"`
	Context   string `json:"context"`
	Confident bool   `json:"confident"`
	Reason    string `json:"reason"`
}

// openPoints are what the rules left open that the model can settle from the
// index: an intent no word named, or a context among several the search
// relates. A new context is not one of them — whether "sincronização de
// pedidos" is its own context is a product decision, the person's to make.
func openPoints(res *Result) (intent bool, contexts []string) {
	intent = res.intentGuessed
	if res.Context == nil {
		for _, c := range res.related {
			if len(contexts) == 3 {
				break
			}
			contexts = append(contexts, c.name)
		}
		if len(contexts) < 2 {
			contexts = nil
		}
	}
	return intent, contexts
}

// consult asks the model to settle the open points, and returns what it
// settled. A failed call, an answer outside the options or an unsure answer
// settles nothing: the rules' questions stand.
func consult(opts Options, res *Result) (settled, *Spent, bool) {
	openIntent, contexts := openPoints(res)
	if !openIntent && contexts == nil {
		return settled{}, nil, false
	}
	prompt := promptFor(res, contexts)
	schema := schemaFor(contexts)
	key := cacheKey(prompt, schema)

	answer, spent, err := cached(opts.CacheDir, key)
	if err != nil {
		var sp Spend
		answer, sp, err = opts.Model.Decide(context.Background(), system, prompt, schema)
		spent = &Spent{USD: sp.USD, Millis: sp.Millis}
		if err != nil {
			spent.Note = "modelo light consultado sem resposta válida (" + err.Error() + "): as regras decidem"
			return settled{}, spent, false
		}
		store(opts.CacheDir, key, answer)
	}
	var d decision
	if err := json.Unmarshal(answer, &d); err != nil {
		spent.Note = "modelo light respondeu fora do formato: as regras decidem"
		return settled{}, spent, false
	}
	cost := fmt.Sprintf("US$ %.4f, %.1f s", spent.USD, float64(spent.Millis)/1000)
	if spent.Cached {
		cost = "decisão em cache, sem custo"
	}
	if !d.Confident {
		spent.Note = "modelo light não teve base para decidir (" + cost + "): segue como pergunta"
		return settled{}, spent, false
	}
	s := settled{why: "modelo light: " + d.Reason}
	var took []string
	if openIntent && d.Intent == "explain" {
		// A question about the project ends in no artifact.
		s.intent = d.Intent
		took = append(took, "intenção explain")
	} else if openIntent && slices.Contains(role.Intents, d.Intent) && slices.Contains(role.Artifacts, d.Artifact) {
		s.intent, s.artifact = d.Intent, d.Artifact
		took = append(took, "intenção "+d.Intent+" · "+d.Artifact)
	}
	if contexts != nil && slices.Contains(contexts, d.Context) {
		s.context = d.Context
		took = append(took, "contexto "+d.Context)
	}
	if len(took) == 0 {
		spent.Note = "modelo light respondeu fora das opções (" + cost + "): as regras decidem"
		return settled{}, spent, false
	}
	spent.Note = "modelo light decidiu " + strings.Join(took, ", ") + " (" + cost + ")"
	return s, spent, true
}

// promptFor is the request, the options and the evidence — pointers only,
// the same the plan uses.
func promptFor(res *Result, contexts []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Pedido: %q\n", res.Request)
	if len(contexts) > 0 {
		b.WriteString("\nContextos possíveis:\n")
		for _, c := range contexts {
			b.WriteString("- " + c + "\n")
		}
	}
	if len(res.Evidence) > 0 {
		b.WriteString("\nEvidências no projeto:\n")
		for _, e := range res.Evidence {
			b.WriteString("- " + e.String() + "\n")
		}
	}
	return b.String()
}

// schemaFor closes every field to its options: the model can only pick.
func schemaFor(contexts []string) []byte {
	ctx := map[string]any{"type": "string", "enum": append(append([]string{}, contexts...), "")}
	s := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"intent":    map[string]any{"type": "string", "enum": role.Intents},
			"artifact":  map[string]any{"type": "string", "enum": role.Artifacts},
			"context":   ctx,
			"confident": map[string]any{"type": "boolean"},
			"reason":    map[string]any{"type": "string", "maxLength": 240},
		},
		"required": []string{"intent", "artifact", "context", "confident", "reason"},
	}
	b, _ := json.Marshal(s)
	return b
}

func cacheKey(prompt string, schema []byte) string {
	h := sha256.New()
	for _, part := range []string{promptVersion, system, prompt, string(schema)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// cached reads a decision taken before for the same question.
func cached(dir, key string) ([]byte, *Spent, error) {
	if dir == "" {
		return nil, nil, os.ErrNotExist
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return nil, nil, err
	}
	return b, &Spent{Cached: true}, nil
}

// store keeps a decision. Best effort: a cache that cannot be written costs
// the next run a call, never the answer.
func store(dir, key string, answer []byte) {
	if dir == "" {
		return
	}
	if os.MkdirAll(dir, 0o755) == nil {
		_ = os.WriteFile(filepath.Join(dir, key+".json"), answer, 0o644)
	}
}

// fromAnswers turns the person's answers into what the run takes as given.
func fromAnswers(answers map[string]string) settled {
	s := settled{why: "resposta da pessoa"}
	if v := strings.TrimSpace(answers["context"]); v != "" {
		if rest, ok := strings.CutPrefix(v, "novo"); ok {
			s.newContext = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rest), ":"))
			// A bare "novo" takes the name the intake drew from the subject.
			s.forceNew = s.newContext == ""
		} else {
			s.context = v
		}
	}
	s.change = strings.TrimSpace(answers["change"])
	return s
}

// settleByDefault ends the last round: every open question takes its default
// — the first option, the one the evidence ranks highest — and is written down
// as an assumption, so nothing is decided silently.
func settleByDefault(root, text string, opts Options, s settled, res *Result) (*Result, error) {
	var assumed []string
	for _, q := range res.Questions {
		switch {
		case q.ID == "context" && len(q.Options) > 0:
			choice := q.Options[0]
			if rest, ok := strings.CutPrefix(choice, "novo:"); ok {
				s.newContext = strings.TrimSpace(rest)
			} else {
				s.context = choice
			}
			assumed = append(assumed, fmt.Sprintf("sem resposta para %q: seguiu com %s", q.Text, choice))
		case q.ID == "change":
			s.change = "não descrita — o papel parte da spec e das evidências"
			assumed = append(assumed, "sem resposta para \"o que muda\": o papel parte da spec e das evidências")
		default:
			assumed = append(assumed, fmt.Sprintf("sem resposta para %q", q.Text))
		}
	}
	s.why = "padrão, sem resposta"
	again, err := run(root, text, opts, s)
	if err != nil {
		return nil, err
	}
	again.Assumptions = append(again.Assumptions, assumed...)
	again.Questions, again.Ask = []Question{}, false
	again.Spent = res.Spent
	again.Brief = brief(again)
	fillTurns(again)
	return again, nil
}

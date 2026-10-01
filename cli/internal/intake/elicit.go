package intake

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// ElicitSchema names the shape of an elicitation file.
const ElicitSchema = "gofi.elicit/v1"

// ElicitDir is where elicitations are written, relative to the project root:
// outside git, and outside the documents — an open question is not yet
// anything the project decided.
const ElicitDir = ".gofi/elicit"

// Elicitation is what a phase leaves for the person to answer: the open
// decisions, and what it found already answered.
type Elicitation struct {
	Schema    string        `json:"schema"`
	Questions []ElicitQuest `json:"questions"`
	// Derived is what the request, the documents or the project already
	// answer, with the source: the role reads it as settled.
	Derived []string `json:"derived"`
}

// ElicitQuest is one open decision.
type ElicitQuest struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Options     []string `json:"options"`
	Recommended string   `json:"recommended"`
	Why         string   `json:"why"`
}

// elicitFile is where phase i of a plan writes its elicitation.
func elicitFile(r *Result, role string) string {
	ctx := "pedido"
	if r.Context != nil {
		ctx = r.Context.Name
	}
	return ElicitDir + "/" + ctx + "-" + role + ".json"
}

// elicitTurn is what runs an elicitation phase. It is always the role's
// SKILL.md followed on the phase's own model: invoking the skill would run it
// on the role's tier, which is what the phase exists to spare.
func elicitTurn(r *Result, i int) string {
	ph := r.Plan[i]
	skill := layout.Skills().Path(ph.Role, "SKILL.md")
	checklist := layout.Skills().Path(ph.Role, ph.Checklist)
	var b strings.Builder
	fmt.Fprintf(&b, "Siga o papel descrito em %s em **modo elicitação**, no nível %s (%s).\n\n", skill, ph.Tier, ph.TierWhy)
	fmt.Fprintf(&b, "Não escreva o documento nem altere arquivo do projeto: só leia e grave `%s`.\n\n", ph.ElicitFile)
	b.WriteString("1. Leia o pedido montado abaixo, o que o contexto já tem (PRD, spec, memória, código pelos ponteiros) e o `.gofi.yaml`.\n")
	fmt.Fprintf(&b, "2. Percorra o checklist `%s`, item a item.\n", checklist)
	b.WriteString("3. O que o pedido, os documentos, o projeto, o SDK ou as regras já respondem vai em `derived`, com a fonte — não vira pergunta.\n")
	b.WriteString("4. Só o que nada responde vira pergunta: texto curto, de 2 a 4 opções quando couber, a recomendada e o porquê em uma frase.\n")
	fmt.Fprintf(&b, "5. Grave em `%s`, exatamente neste formato:\n\n", ph.ElicitFile)
	fmt.Fprintf(&b, "```json\n{\"schema\": %q, \"questions\": [{\"id\": \"q1\", \"text\": \"...\", \"options\": [\"...\"], \"recommended\": \"...\", \"why\": \"...\"}], \"derived\": [\"... (fonte)\"]}\n```\n\n", ElicitSchema)
	b.WriteString("Termine com uma linha: quantas perguntas ficaram.\n\n")
	b.WriteString(r.Brief)
	return b.String()
}

// afterElicit is what a role is told when an elicitation ran before it.
const afterElicit = "As decisões em aberto foram levantadas e respondidas — estão em **Decisões confirmadas**, no fim, com o que o projeto já responde e a fonte de cada item. A pré-execução foi feita pela elicitação: vale como lida. Não a refaça — não releia o que está derivado nem reconstrua o índice; leia só o que a escrita exige (o template do documento, as regras de escrita e as seções da fonte que vai transcrever). Escreva o documento de uma vez: sem entrevista e sem pedir confirmação; a revisão é a parada depois do documento, e é a pessoa quem aprova — o documento sai como rascunho (`status: proposto` na spec, `draft` no PRD). O que ainda faltar, não suponha: grave como pergunta em `%s` e encerre.\n\n"

// ReadElicitation reads what a phase left for the person. Nil, with no
// error, when it left nothing.
func ReadElicitation(root, rel string) (*Elicitation, error) {
	if rel == "" {
		return nil, nil
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var e Elicitation
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	if e.Schema != ElicitSchema {
		return nil, fmt.Errorf("%s: schema %q, expected %s", rel, e.Schema, ElicitSchema)
	}
	for i := range e.Questions {
		if e.Questions[i].ID == "" {
			e.Questions[i].ID = fmt.Sprintf("q%d", i+1)
		}
	}
	return &e, nil
}

// ClearElicitation removes what a previous run left, so a phase is judged by
// what it writes now.
func ClearElicitation(root, rel string) {
	if rel != "" {
		_ = os.Remove(filepath.Join(root, filepath.FromSlash(rel)))
	}
}

// Questions are the elicitation's decisions as the conductors ask them: the
// recommended option first, as the default.
func (e *Elicitation) AsQuestions() []Question {
	qs := make([]Question, 0, len(e.Questions))
	for _, q := range e.Questions {
		opts := []string{}
		if q.Recommended != "" {
			opts = append(opts, q.Recommended)
		}
		for _, o := range q.Options {
			if o != q.Recommended {
				opts = append(opts, o)
			}
		}
		text := q.Text
		if q.Why != "" && q.Recommended != "" {
			text += " (recomendado: " + q.Recommended + " — " + q.Why + ")"
		}
		qs = append(qs, Question{ID: q.ID, Text: text, Options: opts})
	}
	return qs
}

// Defaults answers every question with its recommendation — what the person
// accepts by going on without answering. Each is written down as assumed.
func (e *Elicitation) Defaults() map[string]string {
	a := map[string]string{}
	for _, q := range e.Questions {
		v := q.Recommended
		if v == "" && len(q.Options) > 0 {
			v = q.Options[0]
		}
		a[q.ID] = v
	}
	return a
}

// WithDecisions is the turn of the role an elicitation prepared: the turn as
// planned, and the decisions — what the project answered and what the person
// did — at its end. An answer missing from answers is the recommendation,
// marked as assumed.
func WithDecisions(turn string, e *Elicitation, answers map[string]string) string {
	if e == nil || (len(e.Questions) == 0 && len(e.Derived) == 0) {
		return turn
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(turn, "\n"))
	b.WriteString("\n\n**Decisões confirmadas** (não pergunte de novo):\n\n")
	defaults := e.Defaults()
	for _, q := range e.Questions {
		if v := strings.TrimSpace(answers[q.ID]); v != "" {
			fmt.Fprintf(&b, "- %s → %s\n", q.Text, v)
		} else {
			fmt.Fprintf(&b, "- %s → %s (assumido: a recomendação, sem resposta)\n", q.Text, defaults[q.ID])
		}
	}
	for _, d := range e.Derived {
		fmt.Fprintf(&b, "- %s\n", d)
	}
	return b.String()
}

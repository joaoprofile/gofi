// Package intake turns a request in free text into a plan: the context it is
// about, the phases that take it there, the tier of each, the expertise the
// roles need, what is missing, and the assembled request each phase receives
// (0001 D4, steps 1, 2, 4 and 5).
//
// Everything here is deterministic and spends no token: the lexicon, the
// index, the document graph and the roles' contracts decide. A step the rules
// cannot settle becomes a question with options taken from the evidence, never
// a guess. The cheap-model step that settles what the rules cannot comes later
// and plugs into Questions.
package intake

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/expertise"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/retrieval"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// Schema names the shape of Result. A consumer — the extension, the MCP tool,
// a script — checks it before reading the rest.
const Schema = "gofi.intake/v1"

// Options are what the caller decides instead of the request.
type Options struct {
	// Language is the backend language, which names the code graph.
	Language string
	// Tier, when set, runs every phase at it: a request that names its tier
	// wins over the router (0001 D8).
	Tier host.Tier
	// Model settles what the rules could not, when set (0001 D4, step 3).
	// Nil keeps the intake to its rules: every open point is a question.
	Model Model
	// CacheDir keeps the model's decisions, keyed by what it was asked, so the
	// same request over the same index is decided once. Empty: no cache.
	CacheDir string
	// Answers are the person's answers to the previous round's questions, by
	// question id. Answering makes this round the last (0001 D4: two rounds at
	// most).
	Answers map[string]string
	// Proceed goes on without answering: every open point takes its default
	// and is written down as an assumption.
	Proceed bool
	// Engines keeps the index loaded between requests; nil loads it each time.
	Engines *Engines
}

// settled is what was decided outside the rules — by the person or by the
// model — and the run must take as given.
type settled struct {
	intent, artifact string
	context          string // a known context, or "" for none
	newContext       string // a new context's name
	forceNew         bool   // a new context, named from the request's subject
	change           string // what changes, in the person's words
	why              string // who decided, for the signals
}

// Result is the intake's answer.
type Result struct {
	Schema  string `json:"schema"`
	Request string `json:"request"`
	// Direct is set when the request invoked a skill outright: it goes as it
	// is, with no plan (0001: "sempre dá para passar direto").
	Direct string `json:"direct,omitempty"`
	// Turn is what to send, for a request with no plan that is not sent as
	// typed: a question about the project, with the sections that answer it.
	Turn string `json:"turn,omitempty"`
	// Chat is set when the request is no task — no verb, no artifact, nothing
	// of the project in it: it goes as it is, with no plan and no model asked.
	Chat     bool     `json:"chat,omitempty"`
	Intent   string   `json:"intent"`
	Artifact string   `json:"artifact"`
	Context  *Context `json:"context,omitempty"`
	Plan     []Phase  `json:"plan"`
	// Answer is set for a status request the index answers with no role.
	Answer      string       `json:"answer,omitempty"`
	Packs       []PackChoice `json:"packs"`
	Questions   []Question   `json:"questions"`
	Ask         bool         `json:"ask"`
	Assumptions []string     `json:"assumptions"`
	Evidence    []Pointer    `json:"evidence"`
	// Signals say what was read from the request and why — nothing here is a
	// black box.
	Signals []string `json:"signals"`
	// Next is what to offer when the plan ends short of code.
	Next string `json:"next,omitempty"`
	// Brief is the assembled request for the first phase (0001 D6).
	Brief   string   `json:"brief"`
	Missing []string `json:"missing"`
	// Spent is what the model cost this request, when it was consulted.
	Spent *Spent `json:"spent,omitempty"`
	// Change is what changes, as the person answered it.
	Change string `json:"change,omitempty"`
	// VerdictFile is where the QA writes its verdict — the context's memory,
	// relative to the project root — when a phase has OnReject.
	VerdictFile string `json:"verdict_file,omitempty"`

	// intentGuessed is set when no word of the request named the intent.
	intentGuessed bool
	related       []candidate
	// specRole is the role that writes the spec, and specTier its contract's
	// tier: a blocked implementation's decisions are recorded there.
	specRole string
	specTier host.Tier
}

// Question is something the rules could not settle, with the options the
// evidence offers. The first option is the default.
type Question struct {
	ID      string   `json:"id"`
	Text    string   `json:"text"`
	Options []string `json:"options"`
}

// Pointer is a place to read, never the text itself.
type Pointer struct {
	Kind  string `json:"kind"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

func (p Pointer) String() string {
	if p.Kind == "symbol" {
		return fmt.Sprintf("%s %s:%d", p.Title, p.Path, p.Start)
	}
	if p.End > p.Start {
		return fmt.Sprintf("%s#%s L%d-%d", p.Path, p.Title, p.Start, p.End)
	}
	return fmt.Sprintf("%s#%s L%d", p.Path, p.Title, p.Start)
}

// projectAreas are where evidence comes from: what the project itself says
// and does. The expertise packs are chosen apart, by their contracts, and the
// SDK and skills are the roles' to read — evidence points at the project.
var projectAreas = []string{docs.AreaSpecs, docs.AreaPRD, docs.AreaMemory, docs.AreaInstitutional, docs.AreaKnowledge, retrieval.AreaCode}

// evidenceBudget caps what the evidence asks the agent to read, in tokens,
// estimated from the lines of each section (0001 D4, step 2).
const evidenceBudget = 3000

// tokensPerLine is the estimate of what one line of a document costs to read.
const tokensPerLine = 12

// Run reads a request and returns its plan. root is the project root.
//
// The rules decide first. What they leave open is settled by the model, when
// one is given and the point is one it can settle from the index; what is
// still open is a question. With answers — or told to proceed — the round is
// the last: whatever is still open takes its default and is written down.
func Run(root, text string, opts Options) (*Result, error) {
	s := fromAnswers(opts.Answers)
	final := len(opts.Answers) > 0 || opts.Proceed
	res, err := run(root, text, opts, s)
	if err != nil || res.Direct != "" || res.Chat {
		return res, err
	}
	if !final && opts.Model != nil {
		if decided, spent, ok := consult(opts, res); ok {
			again, err := run(root, text, opts, decided)
			if err != nil {
				return nil, err
			}
			again.Spent = spent
			again.Signals = append(again.Signals, spent.Note)
			res = again
		} else if spent != nil {
			res.Spent = spent
			res.Signals = append(res.Signals, spent.Note)
		}
	}
	if final && len(res.Questions) > 0 {
		return settleByDefault(root, text, opts, s, res)
	}
	return res, nil
}

// run is one pass of the rules, taking what was already settled as given.
func run(root, text string, opts Options, s settled) (*Result, error) {
	res := &Result{Schema: Schema, Request: text, Plan: []Phase{}, Packs: []PackChoice{},
		Questions: []Question{}, Assumptions: []string{}, Evidence: []Pointer{}, Signals: []string{}, Missing: []string{}}

	roles, problems := role.Load(layout.Skills().Abs(root))
	for _, p := range problems {
		res.Missing = append(res.Missing, "contract: "+p.Error())
	}
	if len(roles) == 0 {
		res.Missing = append(res.Missing, "no skill has a contract — run `gofi update skills`")
	}

	// 1. Reception.
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "/") {
		name, _, _ := strings.Cut(trimmed[1:], " ")
		res.Direct = name
		res.Signals = append(res.Signals, "/"+name+" invocado direto: passa como está")
		res.Brief = trimmed
		return res, nil
	}
	lex := baseLexicon()
	ws := words(normalize(text))
	skip := map[int]bool{}
	intent, hasIntent := find(lex.Intents, ws)
	weak := false
	if !hasIntent {
		intent, hasIntent = find(lex.WeakIntents, ws)
		weak = hasIntent
	}
	if s.intent != "" {
		intent, hasIntent, weak = match{ID: s.intent, Word: s.why, Pos: -1}, true, false
	}
	if hasIntent {
		res.Intent = intent.ID
		if weak {
			res.Signals = append(res.Signals, fmt.Sprintf("intenção %s ← \"%s\" (verbo genérico: nenhum outro diz o que fazer)", intent.ID, intent.Word))
		} else {
			res.Signals = append(res.Signals, fmt.Sprintf("intenção %s ← \"%s\"", intent.ID, intent.Word))
		}
		markAll(lex.Intents, ws, skip)
		markAll(lex.WeakIntents, ws, skip)
	}
	artifact, hasArtifact := find(lex.Artifacts, ws)
	// A role's verb names its artifact ("especifique" → spec), unless it is
	// a verb of every task ("implemente", "desenvolva") and the request names
	// the artifact outright: in "desenvolva a tela …" the screen decides.
	if roleArtifact, word, ok := contractArtifact(roles, ws); ok && (!hasArtifact || (word.Pos < artifact.Pos && !isIntentWord(lex, word.Word))) {
		artifact, hasArtifact = match{ID: roleArtifact, Word: word.Word, Pos: word.Pos}, true
	}
	if s.artifact != "" {
		artifact, hasArtifact = match{ID: s.artifact, Word: s.why, Pos: -1}, true
	}
	if hasArtifact {
		res.Artifact = artifact.ID
		res.Signals = append(res.Signals, fmt.Sprintf("artefato %s ← \"%s\"", artifact.ID, artifact.Word))
		markAll(lex.Artifacts, ws, skip)
		if artifact.Pos >= 0 {
			skip[artifact.Pos] = true
		}
	}
	if !hasIntent {
		res.intentGuessed = true
		res.Intent = intentFor(res.Artifact, text)
		res.Signals = append(res.Signals, "intenção "+res.Intent+" ← nenhum verbo do léxico; deduzida do artefato ou da forma do pedido")
	}
	if res.Artifact == "" && res.Intent != "explain" {
		res.Artifact = defaultArtifact[res.Intent]
		res.Signals = append(res.Signals, "artefato "+res.Artifact+" ← padrão de "+res.Intent)
	}

	// The index: search, and the graph of contexts.
	open := retrieval.Open
	if opts.Engines != nil {
		open = opts.Engines.Open
	}
	engine, err := open(root, opts.Language)
	if err != nil {
		return nil, err
	}
	res.Missing = append(res.Missing, engine.Missing...)
	var ex *docs.Explorer
	if _, g, _ := docs.Load(root); g != nil {
		ex = &docs.Explorer{Graph: g}
	}
	known := map[string]bool{}
	if ex != nil {
		for _, c := range ex.Contexts() {
			known[c] = true
		}
	}

	// The context and its state.
	related := relatedContexts(engine, text, known)
	res.related = related
	switch {
	case s.context != "" && known[s.context]:
		c := stateOf(ex, s.context)
		c.Why = s.why
		res.Context = &c
	case s.newContext != "":
		res.Context = &Context{Name: s.newContext, New: true, Why: s.why}
	case s.forceNew:
		if name := subject(ws, lex, skip); name != "" {
			res.Context = &Context{Name: name, New: true, Why: s.why + ": contexto novo, nome tirado do assunto"}
		}
	default:
		res.Context = resolveContext(res, engine, ws, skip, lex, known, contextKeywords(root, ex, known), related, ex)
	}
	res.Change = s.change

	// 2. Evidence, as pointers within the budget.
	res.Evidence = evidence(engine, text, res.Context)

	// Not a task: no verb (a generic one — "preciso", "quero" — does not
	// count), no artifact, and nothing of the project named or found — a
	// greeting, a word of thanks. It goes as written, at once:
	// planning it would put a made-up plan in front of it, and asking the light
	// tier costs seconds to learn it has no basis.
	chat := !hasIntent && !hasArtifact && res.Context == nil && len(related) == 0 && len(res.Evidence) == 0
	// A generic verb alone — "preciso de ajuda" — is conversation too, unless
	// the request names an artifact or a context that exists.
	if weak && !hasArtifact && (res.Context == nil || res.Context.New) {
		chat = true
	}
	if chat {
		res.Chat = true
		res.Signals = append(res.Signals, "sem tarefa: nenhum verbo, artefato ou assunto do projeto — segue como escrito, sem plano")
		res.Brief = trimmed
		return res, nil
	}

	// 4. Plan.
	if res.Intent == "explain" {
		// A question about the project changes nothing: no role runs. It goes
		// to the conversation as asked, with the sections that answer it, so
		// the model reads those instead of searching the tree.
		res.Turn = explainTurn(res)
		res.Signals = append(res.Signals, fmt.Sprintf("pergunta sobre o projeto: sem fase — vai à conversa com %d ponteiro(s) para o que responde", len(res.Evidence)))
	} else if res.Intent == "status" && res.Context != nil && !res.Context.New {
		res.Answer = "gofi show ctx:" + res.Context.Name
		res.Signals = append(res.Signals, "consulta de estado: o índice responde, sem papel")
	} else if len(roles) > 0 {
		p := newPlanner(roles)
		res.specRole = p.producers["spec"]
		res.specTier = p.roles[res.specRole].Tier
		phases, err := p.plan(res.Intent, res.Artifact, res.Context)
		if err != nil {
			res.Missing = append(res.Missing, err.Error())
		} else {
			route(phases, p.roles, res.Context, opts.Tier)
			res.Plan = phases
		}
		res.Next = nextAfter(res.Artifact, p)
	}

	// Expertise for the roles of the plan.
	if packs, _ := expertise.Load(root); len(packs) > 0 {
		res.Packs = choosePacks(packs, res.Plan, ws, res.Evidence)
	}

	// 3. Sufficiency, by rules: what the roles need and the request lacks.
	res.Questions, res.Assumptions = sufficiency(res, related, roles, ws, skip, s)
	// Lists are never null in the contract: a consumer iterates, it does not
	// check for nil.
	if res.Questions == nil {
		res.Questions = []Question{}
	}
	if res.Packs == nil {
		res.Packs = []PackChoice{}
	}
	if res.Assumptions == nil {
		res.Assumptions = []string{}
	}
	if res.Evidence == nil {
		res.Evidence = []Pointer{}
	}
	res.Ask = len(res.Questions) > 0

	// 5. The assembled request, and each phase's turn.
	res.Brief = brief(res)
	fillTurns(res)
	return res, nil
}

// fillTurns writes into each phase what a conductor sends to run it.
func fillTurns(r *Result) {
	for i := range r.Plan {
		ph := &r.Plan[i]
		if ph.Elicit {
			ph.ElicitFile = elicitFile(r, ph.Role)
			ph.Turn = elicitTurn(r, i)
			ph.RoleTurn = ph.Turn
			continue
		}
		ph.Turn = PhaseTurn(r, i, false)
		if Raised(r, i) {
			ph.RoleTurn = PhaseTurn(r, i, true)
		}
		ph.ReviewAfter = ApprovalAfter(r, i)
		if i > 0 && r.Plan[i-1].Elicit && r.Plan[i-1].Role == ph.Role {
			// The role an elicitation prepared writes at once; the conductor
			// adds the decisions at the end (WithDecisions).
			ph.Turn = prependBrief(ph.Turn, fmt.Sprintf(afterElicit, elicitFile(r, ph.Role)))
			if ph.RoleTurn != "" {
				ph.RoleTurn = prependBrief(ph.RoleTurn, fmt.Sprintf(afterElicit, elicitFile(r, ph.Role)))
			}
		}
	}
	fillBlocks(r)
	fillRetries(r)
}

// prependBrief puts note right before the assembled request in a turn.
func prependBrief(turn, note string) string {
	if i := strings.Index(turn, "# Pedido montado"); i >= 0 {
		return turn[:i] + note + turn[i:]
	}
	return note + turn
}

// markAll marks every word of the request a vocabulary covers, so a subject
// is named from what is left.
func markAll(vocab map[string][]string, ws []string, skip map[int]bool) {
	for _, entries := range vocab {
		for _, e := range entries {
			phrase := words(normalize(e))
			for i := 0; i+len(phrase) <= len(ws); i++ {
				if len(phrase) > 0 && equalWords(ws[i:i+len(phrase)], phrase) {
					for j := range phrase {
						skip[i+j] = true
					}
				}
			}
		}
	}
}

// contractArtifact finds a verb a role's contract declares, and the artifact
// that role produces.
func contractArtifact(roles []role.Role, ws []string) (string, match, bool) {
	vocab := map[string][]string{}
	for _, r := range roles {
		if r.Contract.Produces != "" && len(r.Contract.Intents) > 0 {
			vocab[r.Contract.Produces] = append(vocab[r.Contract.Produces], r.Contract.Intents...)
		}
	}
	m, ok := find(vocab, ws)
	return m.ID, m, ok
}

// intentFor deduces the intent when no verb says it: from the artifact the
// request names, or from a question's shape.
func intentFor(artifact, text string) string {
	switch artifact {
	case "docs":
		return "document"
	case "audit":
		return "review"
	case "status":
		return "status"
	case "migration":
		return "migrate"
	case "infra":
		return "deploy"
	case "prd", "spec", "ui", "code":
		return "create"
	}
	if strings.HasSuffix(strings.TrimSpace(text), "?") {
		return "status"
	}
	return "change"
}

// resolveContext decides the context a request is about.
func resolveContext(res *Result, engine *retrieval.Engine, ws []string, skip map[int]bool, lex Lexicon, known map[string]bool, keywords map[string][]string, related []candidate, ex *docs.Explorer) *Context {
	stop := map[string]bool{}
	for _, w := range lex.Stopwords {
		stop[normalize(w)] = true
	}
	// A document that defines a subject — a PRD, a spec — is about the head
	// of that subject; code or a screen being created lands in the context any
	// word names.
	headOnly := res.Intent == "create" && (res.Artifact == "prd" || res.Artifact == "spec")
	if name, pos := named(engine, ws, skip, stop, known, keywords, headOnly); name != "" {
		c := stateOf(ex, name)
		c.Why = fmt.Sprintf("o pedido cita o contexto %s (\"%s\")", name, ws[pos])
		skip[pos] = true
		if res.Intent == "create" && !(headOnly && res.Artifact == "prd") {
			// Creating inside a context that exists is a change to it.
			res.Signals = append(res.Signals, "criar dentro de "+name+", que existe: tratado como mudança")
			res.Intent = "change"
		}
		return &c
	}
	if res.Intent == "create" {
		name := subject(ws, lex, skip)
		if name == "" {
			return nil
		}
		return &Context{Name: name, New: true, Why: "pedido de criação sem contexto existente citado: contexto novo, nome tirado do assunto"}
	}
	if len(related) == 1 || (len(related) > 1 && related[0].score >= dominance*related[1].score) {
		c := stateOf(ex, related[0].name)
		c.Why = fmt.Sprintf("a busca aponta %s (%.1f) com folga sobre os demais", related[0].name, related[0].score)
		return &c
	}
	return nil
}

// evidence collects the pointers the request reaches, the target context's
// first, until the reading they ask for passes the budget.
func evidence(engine *retrieval.Engine, text string, ctx *Context) []Pointer {
	query := text
	if ctx != nil && !ctx.New {
		query += " " + ctx.Name
	}
	var out []Pointer
	spent := 0
	for _, h := range engine.Find(retrieval.Query{Text: query, Areas: projectAreas, Limit: 12}) {
		lines := max(h.End-h.Start+1, 1)
		cost := lines * tokensPerLine
		if h.Kind == retrieval.KindSymbol {
			cost = tokensPerLine
		}
		if spent+cost > evidenceBudget && len(out) > 0 {
			break
		}
		spent += cost
		kind := "section"
		if h.Kind == retrieval.KindSymbol {
			kind = "symbol"
		}
		out = append(out, Pointer{Kind: kind, Path: filepath.ToSlash(h.Path), Title: h.Title, Start: h.Start, End: h.End})
	}
	return out
}

// nextAfter is the role to offer when a plan ends at a document: a PRD leads
// to its spec, a spec to its implementation.
func nextAfter(artifact string, p planner) string {
	next := map[string]string{"prd": "spec", "spec": "code"}[artifact]
	return p.producers[next]
}

// sufficiency checks what the planned roles need against what the request
// and the index gave (0001 D4, step 3). What cannot be settled is a question
// with options from the evidence; what can be assumed is written down.
func sufficiency(res *Result, related []candidate, roles []role.Role, ws []string, skip map[int]bool, s settled) ([]Question, []string) {
	needs := map[string]bool{}
	contracts := map[string]role.Contract{}
	for _, r := range roles {
		contracts[r.Name] = r.Contract
	}
	for _, ph := range res.Plan {
		for _, n := range contracts[ph.Role].Needs {
			needs[n] = true
		}
	}
	var qs []Question
	var assume []string
	var options []string
	for _, c := range related {
		if len(options) == 3 {
			break
		}
		options = append(options, c.name)
	}
	ctx := res.Context
	switch {
	case !needs["context"]:
	case s.context != "" || s.newContext != "" || s.forceNew:
		// Settled by the person or the model: not asked again.
	case ctx == nil && len(options) > 0:
		qs = append(qs, Question{ID: "context", Text: "Em qual contexto?", Options: options})
	case ctx == nil:
		qs = append(qs, Question{ID: "context", Text: "Em qual contexto? Nenhum contexto do índice casa com o pedido."})
	case ctx.New && len(options) > 0:
		qs = append(qs, Question{ID: "context",
			Text:    fmt.Sprintf("Contexto novo `%s`, ou parte de um que já existe?", ctx.Name),
			Options: append([]string{"novo: " + ctx.Name}, options...)})
	case ctx.New:
		assume = append(assume, "contexto novo `"+ctx.Name+"`: nada no índice se relaciona ao pedido")
	}
	// "What changes?" is only a question when there is something to change —
	// the artifact exists — and the request says nothing about it: no word
	// but the context, the verb and words that point without naming ("aquilo
	// do login"). Code yet to be written is defined by its PRD and spec, not
	// by the request. Asked here, it costs nothing; left to the agent, it is
	// the expensive model exploring the project to guess.
	if needs["change"] && s.change == "" && (res.Intent == "change" || res.Intent == "fix") && ctx.has(res.Artifact) {
		stop := map[string]bool{}
		lex := baseLexicon()
		for _, w := range append(append([]string{}, lex.Stopwords...), lex.Vague...) {
			stop[normalize(w)] = true
		}
		content := 0
		for i := range ws {
			if !skip[i] && !stop[ws[i]] && len(ws[i]) > 3 {
				content++
			}
		}
		if content == 0 {
			qs = append(qs, Question{ID: "change", Text: "O que muda? O pedido não descreve a mudança."})
		}
	}
	return qs, assume
}

// isIntentWord reports whether a word is one of the lexicon's own verbs —
// a verb of any task, not a role's.
func isIntentWord(lex Lexicon, word string) bool {
	w := normalize(word)
	for _, entries := range lex.Intents {
		for _, e := range entries {
			if normalize(e) == w {
				return true
			}
		}
	}
	return false
}

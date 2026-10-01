package intake

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/host"
)

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copyTree copies a tree of the repository's ai/ into the project.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		write(t, dst, filepath.ToSlash(rel), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func spec(ctx, entities, keywords, body string) string {
	return "---\ntipo: spec\nformato: sdd\ncontexto: " + ctx + "\nversao: \"1.0\"\nstatus: aprovado\nentidades: [" + entities + "]\nkeywords: [" + keywords + "]\n---\n# SDD — " + ctx + "\n\n" + body
}

func memory(ctx string) string {
	return "---\nformato: memoria\ncontexto: " + ctx + "\nversao: \"1.0\"\nstatus: implementado\n---\n# " + ctx + "\n\n## Estado atual\n\nImplementado.\n"
}

// project is a gofi project with the shipped contracts and packs, and
// contexts in every state the planner tells apart.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	skills, _ := filepath.Glob("../../../ai/skills/*/contract.yaml")
	for _, s := range skills {
		b, _ := os.ReadFile(s)
		write(t, root, ".claude/skills/"+filepath.Base(filepath.Dir(s))+"/contract.yaml", string(b))
	}
	copyTree(t, "../../../ai/expertise", filepath.Join(root, ".claude/expertise"))

	write(t, root, "specs/pricing/sdd-pricing.md", spec("pricing", "pricing_rule", "preco, envio de precos",
		"## 1. Fluxo de envio de preços\n\nOs preços são enviados ao marketplace em lote.\n\n## 2. Regras\n\nRN-01 — preço mínimo.\n"))
	write(t, root, ".claude/memory/contexts/pricing.md", memory("pricing"))

	write(t, root, "prd/order/prd-order.md", "---\ntipo: prd\nformato: prd\ncontexto: order\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [pedido]\n---\n# PRD — order\n\n## Problema\n\nPedidos.\n")
	write(t, root, "specs/order/sdd-order.md", spec("order", "order_order, order_item", "pedido, pedidos",
		"## 1. Pedido\n\nO pedido nasce aberto e é faturado.\n\n## 2. Cancelamento\n\nPedido faturado não pode ser cancelado.\n"))
	write(t, root, ".claude/memory/contexts/order.md", memory("order"))

	// Contexts coupled to order by its tables.
	for _, c := range []string{"invoice", "shipping", "integration"} {
		write(t, root, "specs/"+c+"/sdd-"+c+".md", spec(c, "order_order", c, "## 1. Regras\n\nUsa o pedido.\n"))
	}
	// A context with a PRD only.
	write(t, root, "prd/catalog/prd-catalog.md", "---\ntipo: prd\nformato: prd\ncontexto: catalog\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [catalogo, produto]\n---\n# PRD — catalog\n\n## Problema\n\nO catálogo de produtos.\n")
	// The project's lexicon: its domain words to its context names.
	write(t, root, ".claude/lexicon/sinonimos.md", "| termo | sinônimos |\n|---|---|\n| order | pedido |\n| catalog | catálogo |\n| pricing | preço |\n")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	return root
}

// roles is the plan's roles, in order. An elicitation is a step of the role
// it prepares, not a role of its own: it is left out (see elicit_test.go).
func roles(r *Result) []string {
	var out []string
	for _, p := range r.Plan {
		if !p.Elicit {
			out = append(out, p.Role)
		}
	}
	return out
}

func runIntake(t *testing.T, root, text string, opts Options) *Result {
	t.Helper()
	r, err := Run(root, text, opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The first example of 0001: a PRD for something the index does not have.
func TestCreatePRDForSomethingNew(t *testing.T) {
	r := runIntake(t, project(t), "crie um PRD de sincronização de pedidos", Options{})
	if r.Intent != "create" || r.Artifact != "prd" {
		t.Fatalf("read as %s · %s", r.Intent, r.Artifact)
	}
	if !slices.Equal(roles(r), []string{"gofi-pd"}) {
		t.Errorf("plan = %v, want only gofi-pd", roles(r))
	}
	if r.Context == nil || !r.Context.New || !strings.Contains(r.Context.Name, "sincronizacao") {
		t.Errorf("context = %+v, want a new one named from the subject", r.Context)
	}
	if r.Next != "gofi-spec" {
		t.Errorf("next = %q, want the spec offered", r.Next)
	}
	// The one question left is where it belongs, with order as an option.
	if !r.Ask || len(r.Questions) != 1 || r.Questions[0].ID != "context" || !slices.Contains(r.Questions[0].Options, "order") {
		t.Errorf("questions = %+v", r.Questions)
	}
}

// The second example: a change of behaviour in a context with spec and code.
func TestChangeBehaviourOfAnImplementedContext(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit no intervalo dos envios de preços", Options{})
	if r.Intent != "change" || r.Artifact != "code" || r.Context == nil || r.Context.Name != "pricing" {
		t.Fatalf("read as %s · %s · %+v", r.Intent, r.Artifact, r.Context)
	}
	if want := []string{"gofi-spec", "gofi-eng", "gofi-qa"}; !slices.Equal(roles(r), want) {
		t.Errorf("plan = %v, want %v", roles(r), want)
	}
	if r.Ask {
		t.Errorf("nothing to ask, got %+v", r.Questions)
	}
	if !strings.Contains(r.Brief, "specs/pricing/sdd-pricing.md") || !strings.Contains(r.Brief, "/gofi-spec") {
		t.Errorf("brief lacks the spec or the plan:\n%s", r.Brief)
	}
	if len(r.Evidence) == 0 || !strings.HasPrefix(r.Evidence[0].Path, "specs/pricing") {
		t.Errorf("evidence should start in pricing: %+v", r.Evidence)
	}
}

// A fix keeps the spec: the behaviour it describes does not change.
func TestFixSkipsTheSpec(t *testing.T) {
	r := runIntake(t, project(t), "corrija o bug no cancelamento de pedidos", Options{})
	if want := []string{"gofi-eng", "gofi-qa"}; !slices.Equal(roles(r), want) {
		t.Errorf("plan = %v, want %v (context %+v)", roles(r), want, r.Context)
	}
}

// A context coupled to many others raises the roles that write into it.
func TestWideReachRaisesTheTier(t *testing.T) {
	r := runIntake(t, project(t), "corrija o bug no cancelamento de pedidos", Options{})
	for _, p := range r.Plan {
		if p.Role == "gofi-eng" && (p.Tier != host.Deep || !strings.Contains(p.TierWhy, "acoplado")) {
			t.Errorf("eng on order should be deep, got %s (%s)", p.Tier, p.TierWhy)
		}
		if p.Role == "gofi-qa" && p.Tier != host.Standard {
			t.Errorf("qa only reads, stays standard: %s", p.Tier)
		}
	}
	r = runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	for _, p := range r.Plan {
		if p.Role == "gofi-eng" && p.Tier != host.Standard {
			t.Errorf("pricing is not wide, eng stays standard: %s (%s)", p.Tier, p.TierWhy)
		}
	}
}

// A tier the caller names wins over the router.
func TestAskedTierWins(t *testing.T) {
	r := runIntake(t, project(t), "corrija o bug no cancelamento de pedidos", Options{Tier: host.Light})
	for _, p := range r.Plan {
		if p.Tier != host.Light || p.TierWhy != "pedido explícito" {
			t.Errorf("%s: %s (%s)", p.Role, p.Tier, p.TierWhy)
		}
	}
}

func TestDocumentAndReview(t *testing.T) {
	root := project(t)
	if r := runIntake(t, root, "documente a API de pedidos", Options{}); !slices.Equal(roles(r), []string{"gofi-doc"}) {
		t.Errorf("document: %v", roles(r))
	}
	if r := runIntake(t, root, "audite o contexto de pricing", Options{}); !slices.Equal(roles(r), []string{"gofi-qa"}) {
		t.Errorf("review: %v", roles(r))
	}
}

// The state of a known context is the index's to answer, with no role.
func TestStatusIsAnsweredByTheIndex(t *testing.T) {
	r := runIntake(t, project(t), "como está o contexto de pedidos?", Options{})
	if r.Answer != "gofi show ctx:order" || len(r.Plan) != 0 || r.Ask {
		t.Errorf("answer=%q plan=%v ask=%v", r.Answer, roles(r), r.Ask)
	}
}

// A skill invoked outright passes as it is.
func TestDirectInvocationPasses(t *testing.T) {
	r := runIntake(t, project(t), "/gofi-eng implemente o cancelamento", Options{})
	if r.Direct != "gofi-eng" || len(r.Plan) != 0 {
		t.Errorf("direct=%q plan=%v", r.Direct, roles(r))
	}
}

// A change with no context the rules can settle is a question, with the
// contexts the search relates as options.
func TestAmbiguousContextIsAsked(t *testing.T) {
	// Three contexts carry the same "Regras" section: none wins by a margin.
	r := runIntake(t, project(t), "altere as regras", Options{})
	if r.Context != nil {
		t.Fatalf("no context should be settled, got %+v", r.Context)
	}
	if !r.Ask || r.Questions[0].ID != "context" || len(r.Questions[0].Options) < 2 {
		t.Errorf("questions = %+v", r.Questions)
	}
}

// The expertise comes with a reason, and only for roles in the plan.
func TestPacksAreChosenWithAReason(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	got := map[string]string{}
	for _, p := range r.Packs {
		got[p.Pack] = p.Why
	}
	if got["harness-protocols"] == "" {
		t.Errorf("the protocols serve every role: %+v", r.Packs)
	}
	if _, ok := got["ui-design"]; ok {
		t.Errorf("no ui role in the plan, ui-design was chosen: %+v", r.Packs)
	}
	for p, why := range got {
		if why == "" {
			t.Errorf("%s chosen without a reason", p)
		}
	}
}

// Nothing is decided without saying why.
func TestEveryDecisionIsExplained(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	if len(r.Signals) < 2 || r.Context.Why == "" {
		t.Errorf("signals=%v context.why=%q", r.Signals, r.Context.Why)
	}
	for _, p := range r.Plan {
		if p.Why == "" || p.TierWhy == "" {
			t.Errorf("%s without a reason: %+v", p.Role, p)
		}
	}
}

// Evidence points at the project, never at gofi's own packs or skills, and
// the contract's lists are never null.
func TestEvidenceIsTheProjects(t *testing.T) {
	r := runIntake(t, project(t), "crie um PRD de sincronização de pedidos", Options{})
	for _, e := range r.Evidence {
		if strings.HasPrefix(e.Path, ".claude/expertise/") || strings.HasPrefix(e.Path, ".claude/skills/") || strings.HasPrefix(e.Path, ".claude/sdk/") {
			t.Errorf("evidence outside the project: %s", e.Path)
		}
	}
	r = runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	if r.Questions == nil || r.Assumptions == nil || r.Evidence == nil || r.Packs == nil {
		t.Errorf("a list is nil: %+v", r)
	}
	for _, p := range r.Packs {
		if p.Pack == "diagramming" {
			t.Errorf("a flow is not a diagram: %+v", p)
		}
	}
}

// Changing what exists without saying what is the one change the intake asks
// about.
func TestChangeWithoutWhatIsAsked(t *testing.T) {
	r := runIntake(t, project(t), "altere o pricing", Options{})
	if !r.Ask || r.Questions[0].ID != "change" {
		t.Errorf("questions = %+v", r.Questions)
	}
}

// Words that point without naming say nothing about the change: asked here,
// at no cost, instead of the agent exploring the project to guess.
func TestAVagueChangeIsAsked(t *testing.T) {
	root := project(t)
	for _, text := range []string{"ajuste aquilo do pricing", "corrija o problema do pricing", "altere isso no pricing"} {
		r := runIntake(t, root, text, Options{})
		if !r.Ask || len(r.Questions) == 0 || r.Questions[0].ID != "change" {
			t.Errorf("%q: questions = %+v", text, r.Questions)
		}
	}
	if r := runIntake(t, root, "ajuste a mensagem de erro do pricing", Options{}); r.Ask {
		t.Errorf("a named change was asked: %+v", r.Questions)
	}
}

// Each phase carries what a conductor sends to run it, and where it stops.
func TestPhasesCarryTheirTurns(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	if len(r.Plan) != 4 || !r.Plan[0].Elicit {
		t.Fatalf("plan = %v", roles(r))
	}
	spec, eng := r.Plan[1], r.Plan[2]
	if !strings.HasPrefix(spec.Turn, "/gofi-spec ") || !spec.ReviewAfter || eng.ReviewAfter {
		t.Errorf("spec turn=%q review=%v, eng review=%v", spec.Turn[:20], spec.ReviewAfter, eng.ReviewAfter)
	}
	raised := runIntake(t, project(t), "corrija o bug no cancelamento de pedidos", Options{})
	if eng := raised.Plan[0]; eng.RoleTurn == "" || !strings.HasPrefix(eng.RoleTurn, "Siga o papel") {
		t.Errorf("a raised phase carries no role turn: %+v", eng)
	}
}

// A context named in English is found from the person's language through the
// keywords of its own documents, with no lexicon: catalog's PRD lists
// catalogo.
func TestAContextIsNamedByItsDocumentsKeywords(t *testing.T) {
	root := project(t)
	if err := os.Remove(filepath.Join(root, ".claude/lexicon/sinonimos.md")); err != nil {
		t.Fatal(err)
	}
	r := runIntake(t, root, "especifique o catálogo de produtos", Options{})
	if r.Context == nil || r.Context.Name != "catalog" || r.Context.New || r.Ask {
		t.Errorf("context = %+v, questions = %+v", r.Context, r.Questions)
	}
}

// A greeting is no task: it goes as it is, with no plan, no question and no
// model asked — a request that names the project still gets its plan.
func TestAGreetingIsNotPlanned(t *testing.T) {
	root := project(t)
	model := &fakeModel{answer: decisionJSON("change", "code", "", false)}
	for _, text := range []string{"oi", "obrigado!", "bom dia, tudo bem?"} {
		r, err := Run(root, text, Options{Model: model})
		if err != nil {
			t.Fatal(err)
		}
		if !r.Chat || len(r.Plan) != 0 || r.Ask || len(r.Questions) != 0 {
			t.Errorf("%q: chat=%v plan=%v ask=%v", text, r.Chat, roles(r), r.Ask)
		}
	}
	if model.calls != 0 {
		t.Error("the light tier was asked about a greeting")
	}
	if r := runIntake(t, root, "corrija o bug no cancelamento de pedidos", Options{}); r.Chat || len(r.Plan) == 0 {
		t.Errorf("a task was taken for chat: %+v", r)
	}
}

// Every assembled request reads clean: no criterion carries a formatting
// error, and the lists of the contract are never null.
func TestTheBriefHasNoFormattingErrors(t *testing.T) {
	root := project(t)
	for _, text := range []string{"qual o estado do pricing?", "documente o pricing", "audite o contexto de pricing", "altere o fluxo de pricing, adicionando um rate limit"} {
		r := runIntake(t, root, text, Options{})
		if strings.Contains(r.Brief, "%!") {
			t.Errorf("%q: brief has a formatting error:\n%s", text, r.Brief)
		}
		if r.Packs == nil || r.Questions == nil || r.Evidence == nil {
			t.Errorf("%q: a list is nil", text)
		}
	}
}

// The verbs people use are read by the rules, with no model asked: a
// question about the project goes to the conversation with pointers; a
// generic verb decides only when nothing else does.
func TestTheLexiconReadsEverydayRequests(t *testing.T) {
	root := project(t)
	cases := []struct {
		text, intent string
	}{
		{"Analise este repositório e me diga o que ele faz?", "explain"},
		{"explique como funciona o pricing", "explain"},
		{"descreva o fluxo de envio de preços", "explain"},
		{"me dica o que faz o cancelamento de pedidos", "explain"},
		{"como funciona o cálculo do pricing?", "explain"},
		// Creating inside a context that exists is a change to it.
		{"implemente o catálogo", "change"},
		{"desenvolva a tela de cancelamento de pedidos", "change"},
		{"faça um PRD de sincronização de pedidos", "create"},
		{"faça a correção do bug no cancelamento de pedidos", "fix"},
		{"resolva o erro no envio de pricing", "fix"},
		{"melhore o desempenho do envio de pricing", "change"},
		{"verifique o contexto de pricing", "review"},
	}
	model := &fakeModel{answer: decisionJSON("change", "code", "", false)}
	for _, c := range cases {
		r, err := Run(root, c.text, Options{Model: model})
		if err != nil {
			t.Fatal(err)
		}
		if r.Intent != c.intent {
			t.Errorf("%q: intent %s, want %s (%v)", c.text, r.Intent, c.intent, r.Signals)
		}
	}
	if model.calls != 0 {
		t.Errorf("the light tier was asked %d time(s) about requests the rules read", model.calls)
	}
	r := runIntake(t, root, "explique como funciona o pricing", Options{})
	if len(r.Plan) != 0 || r.Answer != "" || r.Ask || !strings.HasPrefix(r.Turn, "explique como funciona o pricing") || !strings.Contains(r.Turn, "specs/pricing/sdd-pricing.md") {
		t.Errorf("explain: plan=%v answer=%q ask=%v turn=%q", roles(r), r.Answer, r.Ask, r.Turn)
	}
	if r := runIntake(t, root, "desenvolva a tela de cancelamento de pedidos", Options{}); r.Artifact != "ui" {
		t.Errorf("a screen named outright lost to the role's verb: %s", r.Artifact)
	}
	// A generic verb alone is still conversation.
	if r := runIntake(t, root, "preciso de ajuda", Options{}); !r.Chat {
		t.Errorf("a generic verb alone was planned: %v", r.Signals)
	}
}

// The phase after a review stop records the person's approval of the
// document they reviewed; no other phase does.
func TestThePhaseAfterAReviewRecordsTheApproval(t *testing.T) {
	r := runIntake(t, project(t), "altere o fluxo de pricing, adicionando um rate limit", Options{})
	var eng, qa Phase
	for _, ph := range r.Plan {
		switch ph.Role {
		case "gofi-eng":
			eng = ph
		case "gofi-qa":
			qa = ph
		}
	}
	if !strings.Contains(eng.Turn, "aprovou `specs/pricing/sdd-pricing.md`") || !strings.Contains(eng.Turn, "status: aprovado") {
		t.Errorf("eng is not told to record the approval: %q", eng.Turn[:200])
	}
	if strings.Contains(qa.Turn, "status: aprovado` no frontmatter") {
		t.Error("a phase after no review stop records an approval")
	}
}

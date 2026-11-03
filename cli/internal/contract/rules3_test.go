package contract

import (
	"strings"
	"testing"
)

const front3 = "---\ntipo: %s\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n"

func doc3(kind string) string { return strings.Replace(front3, "%s", kind, 1) }

// findingsOf checks every document under root and returns the messages about
// one path.
func findingsOf(t *testing.T, root, path string) string {
	t.Helper()
	all := corpus(t, root)
	var picked []Doc
	for _, d := range all {
		if d.Path == path {
			picked = append(picked, d)
		}
	}
	if len(picked) != 1 {
		t.Fatalf("%s not in the corpus", path)
	}
	return messages(Check(root, all, picked[0], mustPlaceholders(t, root)))
}

func mustHave(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in:\n%s", w, got)
		}
	}
}

const goodReq = `### Requisito: REQ-01 — Total positivo
O sistema DEVE recusar pedido com total zero.

#### Cenário: REQ-01.1 — Total zero
- DADO um carrinho com total zero
- QUANDO o cliente fecha o pedido
- ENTÃO recebe recusa
`

func TestLivingSpecRules(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", doc3("spec")+`# Spec

## Requisitos

`+goodReq+`
### Requisito: REQ-01 — De novo
O sistema DEVE algo.

### Requisito: REQ-02 — Sem cenário e sem obrigação
Algo acontece.

### Requisito: REQ-03 — Cenários errados
O sistema DEVE x.

#### Cenário: REQ-04.1 — Do vizinho
- DADO a
- QUANDO b
- ENTÃO c

#### Cenário: REQ-03.1 — Incompleto
- DADO a

[PRECISA ESCLARECER: limite]
`)
	got := findingsOf(t, root, "specs/pedidos/spec.md")
	mustHave(t, got,
		"error falta a seção ## Propósito",
		"error L21: REQ-01 repetido (já declarado na L13)",
		"warn L24: REQ-02 sem DEVE / NÃO DEVE",
		"error L24: REQ-02 sem cenário",
		"error L30: REQ-04.1 está sob REQ-03",
		"error L35: REQ-03.1 sem QUANDO, ENTÃO",
		"error L38: dúvida em aberto",
	)
	if strings.Contains(got, "falta a seção ## Requisitos") {
		t.Errorf("an existing section reported missing:\n%s", got)
	}
}

// A change touching two contexts, one of them new: the proposal, its deltas
// and its design are checked against each other and against the living specs.
func TestChangeRules(t *testing.T) {
	root := project(t)
	write(t, root, "specs/pedidos/spec.md", doc3("spec")+"# Spec\n\n## Propósito\n\nx\n\n## Requisitos\n\n"+goodReq)
	dir := "specs/changes/rate-limit/"
	write(t, root, dir+"proposal.md", strings.Replace(doc3("mudanca"), "contexto: pedidos", "contextos: [pedidos, estoque]\ndepende_de: [criar-clientes, sumida]", 1)+
		"# Mudança\n\n## Por quê\n\nx\n\n## O que muda\n\nx\n")
	write(t, root, dir+"deltas/pedidos.md", doc3("delta")+`# Delta

## ADICIONADO

### Requisito: REQ-01 — Já existe
O sistema DEVE x.

#### Cenário: REQ-01.9 — x
- DADO a
- QUANDO b
- ENTÃO c

### Requisito: REQ-07 — Limite por cliente
O sistema NÃO DEVE aceitar mais de 10 por minuto.

#### Cenário: REQ-07.1 — Décimo primeiro
- DADO dez pedidos
- QUANDO chega o décimo primeiro
- ENTÃO recusa

## MODIFICADO

### Requisito: REQ-09 — Não existe
O sistema DEVE y.

#### Cenário: REQ-09.1 — y
- DADO a
- QUANDO b
- ENTÃO c

## REMOVIDO

### Requisito: REQ-01 — Removido duas vezes
**Por quê:** x
`)
	write(t, root, dir+"deltas/frete.md", strings.Replace(doc3("delta"), "contexto: pedidos", "contexto: frete", 1)+"# Delta\n\n## MODIFICADO\n\n### Requisito: REQ-01 — x\nO sistema DEVE x.\n\n#### Cenário: REQ-01.1 — x\n- DADO a\n- QUANDO b\n- ENTÃO c\n")
	write(t, root, dir+"design.md", doc3("design")+"# Design\n\n## Abordagem\n\nUsa REQ-88.\n\n## Mapa de requisitos\n\n| REQ-01 | x |\n")
	// Another open change adding REQ-07 to pedidos: archiving both would collide.
	write(t, root, "specs/changes/outra/proposal.md", doc3("mudanca")+"# Outra\n\n## Por quê\n\n## O que muda\n")
	write(t, root, "specs/changes/outra/deltas/pedidos.md", doc3("delta")+"# Delta\n\n## ADICIONADO\n\n### Requisito: REQ-07 — Outro\nO sistema DEVE z.\n\n#### Cenário: REQ-07.1 — z\n- DADO a\n- QUANDO b\n- ENTÃO c\n")
	write(t, root, "specs/changes/archive/2026-10-01-criar-clientes/proposal.md", doc3("mudanca"))

	mustHave(t, findingsOf(t, root, dir+"proposal.md"),
		"error estoque está em contextos, mas a mudança não tem deltas/estoque.md",
		"error frete tem delta, mas não está em contextos",
		`error depende_de: a mudança "sumida" não existe`,
	)
	if got := findingsOf(t, root, dir+"proposal.md"); strings.Contains(got, "criar-clientes") {
		t.Errorf("an archived dependency was reported:\n%s", got)
	}
	delta := findingsOf(t, root, dir+"deltas/pedidos.md")
	mustHave(t, delta,
		"REQ-01 já existe na spec viva de pedidos: use MODIFICADO",
		"REQ-07 também é adicionado pela mudança outra",
		"REQ-09 em MODIFICADO não existe na spec viva de pedidos",
		"REQ-01 repetido",
	)
	mustHave(t, findingsOf(t, root, dir+"deltas/frete.md"),
		"REQ-01 em MODIFICADO, mas frete ainda não tem spec viva: só se ADICIONA")
	mustHave(t, findingsOf(t, root, dir+"design.md"),
		"error REQ-07 é adicionado ou modificado pela mudança e não está no ## Mapa de requisitos",
		"REQ-88 citado no design não existe",
	)
}

func TestAChangeThatTouchesCodeNeedsADesign(t *testing.T) {
	root := project(t)
	dir := "specs/changes/criar-pedidos/"
	write(t, root, dir+"proposal.md", doc3("mudanca")+"# M\n\n## Por quê\n\nx\n\n## O que muda\n\nx\n")
	write(t, root, dir+"deltas/pedidos.md", doc3("delta")+"# D\n\n## ADICIONADO\n\n"+goodReq)
	mustHave(t, findingsOf(t, root, dir+"proposal.md"), "não tem design.md")

	write(t, root, dir+"proposal.md", strings.Replace(doc3("mudanca"), "keywords:", "design: n/a\nkeywords:", 1)+"# M\n\n## Por quê\n\nx\n\n## O que muda\n\nx\n")
	// A requirement that did not exist is code that does not: n/a is for a
	// change that changes none.
	mustHave(t, findingsOf(t, root, dir+"proposal.md"), "a mudança adiciona requisitos e declara design: n/a")
}

func TestChangesThatDependOnEachOther(t *testing.T) {
	root := project(t)
	for _, c := range [][2]string{{"a", "b"}, {"b", "a"}} {
		write(t, root, "specs/changes/"+c[0]+"/proposal.md",
			strings.Replace(doc3("mudanca"), "keywords:", "depende_de: ["+c[1]+"]\ndesign: n/a\nkeywords:", 1)+"# M\n\n## Por quê\n\n## O que muda\n")
		write(t, root, "specs/changes/"+c[0]+"/deltas/pedidos.md", doc3("delta")+"# D\n\n## ADICIONADO\n\n"+strings.ReplaceAll(goodReq, "REQ-01", "REQ-0"+map[string]string{"a": "5", "b": "6"}[c[0]]))
	}
	mustHave(t, findingsOf(t, root, "specs/changes/a/proposal.md"), "ciclo de dependência: a → b → a")
}

// A living spec changes only through an archived change; a broken seal on
// one says that, not just that it is stale.
func TestALivingSpecEditedByHand(t *testing.T) {
	root := project(t)
	body := doc3("spec") + "# Spec\n\n## Propósito\n\nx\n\n## Requisitos\n\n" + goodReq
	write(t, root, "specs/pedidos/spec.md", body)
	if _, err := Approve(root, only(t, root)); err != nil {
		t.Fatal(err)
	}
	approved := read(t, root, "specs/pedidos/spec.md")
	write(t, root, "specs/pedidos/spec.md", strings.Replace(approved, "total zero.", "total negativo.", 1))
	mustHave(t, findingsOf(t, root, "specs/pedidos/spec.md"), "error spec viva editada fora de uma mudança")
}

// REQ-01 numbered in two contexts of one change is two requirements: a bare
// REQ-01 in the design is ambiguous, ctx/REQ-01 is not.
func TestARepeatedIDIsQualifiedInTheDesign(t *testing.T) {
	root := archiveFixture(t, false)
	deltaPedidos := read(t, root, "specs/changes/adicionar-rate-limit/deltas/pedidos.md")
	write(t, root, "specs/changes/adicionar-rate-limit/deltas/pedidos.md", strings.Replace(deltaPedidos, "## REMOVIDO", "## MODIFICADO\n\n### Requisito: REQ-01 — Total positivo\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO zero\n- QUANDO fecha\n- ENTÃO recusa\n\n## REMOVIDO", 1))
	design := "specs/changes/adicionar-rate-limit/design.md"
	write(t, root, design, strings.Replace(designRateLimit, "| estoque/REQ-01 | Estoque.Baixa |", "| REQ-01 | Os dois |", 1))
	mustHave(t, findingsOf(t, root, design),
		"estoque/REQ-01 não está no ## Mapa de requisitos — REQ-01 existe em mais de um contexto da mudança",
		"pedidos/REQ-01 não está no ## Mapa de requisitos")

	write(t, root, design, strings.Replace(designRateLimit, "| estoque/REQ-01 | Estoque.Baixa |", "| estoque/REQ-01 | Estoque |\n| pedidos/REQ-01 | Pedido |", 1))
	if got := findingsOf(t, root, design); strings.Contains(got, "Mapa de requisitos") {
		t.Errorf("qualified ids still reported:\n%s", got)
	}
}

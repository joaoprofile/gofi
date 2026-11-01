package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// findingsFor keeps the findings about one document.
func findingsFor(fs []Finding, path string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.Path == path {
			out = append(out, f)
		}
	}
	return out
}

// The QA writes ressalvas next to status; a memory carrying it is the format,
// not drift.
func TestValidateAcceptsTheAuditCaveats(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, ".claude/memory/contexts/churn.md", `---
formato: 2
contexto: churn
versao: "2.0"
status: aprovado
ressalvas: 0
spec: specs/churn/sdd-churn.md
---
# churn
`)
	if got := findingsFor(Validate(root), ".claude/memory/contexts/churn.md"); len(got) > 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestValidateRejectsAnUnknownField(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, ".claude/memory/contexts/churn.md", `---
formato: 2
contexto: churn
versao: "2.0"
status: aprovado
ressalva: 0
---
# churn
`)
	got := findingsFor(Validate(root), ".claude/memory/contexts/churn.md")
	if len(got) != 1 || !got[0].Error || !strings.Contains(got[0].Message, `"ressalva"`) {
		t.Errorf("findings = %+v, want one error naming the misspelt field", got)
	}
}

// A declared dependency is an edge to the context, when the context exists;
// one with no document is the spec check's to report, not a node to invent.
func TestGraphLinksADeclaredDependency(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, "specs/envio/sdd-envio.md", `---
tipo: spec
formato: 2
contexto: envio
versao: "1.0"
status: proposto
depende_de: [churn, estoque]
keywords: [envio]
---
# SDD — Envio
`)
	_, g := build(t, root)
	if !hasEdge(g, "specs/envio/sdd-envio.md", PrefixCtx+"churn", EdgeDepends) {
		t.Error("no depende edge to an existing context")
	}
	if hasEdge(g, "specs/envio/sdd-envio.md", PrefixCtx+"estoque", EdgeDepends) {
		t.Error("an edge to a context nothing belongs to")
	}
	if _, ok := g.Nodes[PrefixCtx+"estoque"]; ok {
		t.Error("a context node was invented for a dependency")
	}
	if got := findingsFor(Validate(root), "specs/envio/sdd-envio.md"); len(got) > 0 {
		t.Errorf("depende_de is not part of the format: %+v", got)
	}
}

func hasEdge(g *Graph, from, to, kind string) bool {
	for _, e := range g.Edges {
		if e == (Edge{from, to, kind}) {
			return true
		}
	}
	return false
}

// A change names the contexts it touches in contextos; that stands for the
// required contexto.
func TestAChangeNamesItsContextsInContextos(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, "specs/changes/rate-limit/proposal.md", "---\ntipo: mudanca\nformato: 3\ncontextos: [churn]\nversao: \"1.0\"\nstatus: proposto\n---\n# M\n")
	if got := findingsFor(Validate(root), "specs/changes/rate-limit/proposal.md"); len(got) > 0 {
		t.Errorf("findings = %+v", got)
	}
}

// A living spec specifies its requirements and their scenarios; a delta adds,
// modifies or removes them; a change belongs to every context it names.
func TestGraphHoldsRequirementsAndScenarios(t *testing.T) {
	root := corpusFixture(t)
	writeFixture(t, root, "specs/pedidos/spec.md", "---\ntipo: spec\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: aprovado\n---\n# Spec\n\n## Requisitos\n\n### Requisito: REQ-04 — Cancelamento\nO sistema DEVE x.\n\n#### Cenário: REQ-04.1 — x\n- DADO a\n- QUANDO b\n- ENTÃO c\n")
	writeFixture(t, root, "specs/changes/rate/proposal.md", "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos, churn]\nversao: \"1.0\"\nstatus: proposto\n---\n# M\n")
	writeFixture(t, root, "specs/changes/rate/deltas/pedidos.md", "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\n---\n# D\n\n## ADICIONADO\n\n### Requisito: REQ-07 — Limite\nO sistema DEVE y.\n\n## MODIFICADO\n\n### Requisito: REQ-04 — Cancelamento\nO sistema DEVE z.\n")
	_, g := build(t, root)

	spec, delta := "specs/pedidos/spec.md", "specs/changes/rate/deltas/pedidos.md"
	for _, e := range []Edge{
		{spec, PrefixReq + "pedidos/REQ-04", EdgeSpecifies},
		{PrefixReq + "pedidos/REQ-04", PrefixScenario + "pedidos/REQ-04.1", EdgeSpecifies},
		{delta, PrefixReq + "pedidos/REQ-07", EdgeAdds},
		{delta, PrefixReq + "pedidos/REQ-04", EdgeModifies},
		{"specs/changes/rate/proposal.md", PrefixCtx + "pedidos", EdgeBelongs},
		{"specs/changes/rate/proposal.md", PrefixCtx + "churn", EdgeBelongs},
	} {
		if !hasEdge(g, e[0], e[1], e[2]) {
			t.Errorf("missing edge %v", e)
		}
	}
	if n := g.Nodes[PrefixReq+"pedidos/REQ-04"]; n.Kind != NodeRequirement || n.Status != "vivo" {
		t.Errorf("REQ-04 = %+v", n)
	}
	if n := g.Nodes[PrefixReq+"pedidos/REQ-07"]; n.Status != "proposto" {
		t.Errorf("REQ-07, only in a delta = %+v", n)
	}
}

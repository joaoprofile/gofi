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

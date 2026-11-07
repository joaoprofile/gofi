package contract

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// changeTemplates writes the templates a change is created from, and a pack
// design template for go.
func changeTemplates(t *testing.T, root string) {
	t.Helper()
	write(t, root, layout.Templates().Path(ProposalTemplate), "---\ntipo: mudanca\nformato: 3\ncontextos: [{contextos}]\nstatus: proposto\natualizado: {YYYY-MM-DD}\n---\n# Mudança — {mudanca}\n")
	write(t, root, layout.Templates().Path(DeltaTemplate), "---\ntipo: delta\nformato: 3\ncontexto: {contexto}\n---\n# Delta — {contexto} — {mudanca}\n")
	write(t, root, layout.Templates().Path(DesignTemplate), "---\ntipo: design\nformato: 3\ncontexto: {contexto}\n---\n# Design neutro\n")
	write(t, root, layout.SDK().Path("go", "templates", DesignTemplate), "---\ntipo: design\nformato: 3\ncontexto: {contexto}\n---\n# Design Go\n")
}

func TestCreateWritesTheChangeFromTheTemplates(t *testing.T) {
	root := project(t)
	changeTemplates(t, root)
	written, err := Create(root, NewChange{ID: "adicionar-rate-limit", Contexts: []string{"pedidos", "clientes"}, Language: "go", Date: "2026-11-01"})
	if err != nil {
		t.Fatal(err)
	}
	want := "specs/changes/adicionar-rate-limit/deltas/clientes.md specs/changes/adicionar-rate-limit/deltas/pedidos.md specs/changes/adicionar-rate-limit/design.md specs/changes/adicionar-rate-limit/proposal.md"
	if got := strings.Join(written, " "); got != want {
		t.Errorf("written = %s", got)
	}
	proposal := read(t, root, "specs/changes/adicionar-rate-limit/proposal.md")
	for _, w := range []string{"contextos: [clientes, pedidos]", "atualizado: 2026-11-01", "# Mudança — adicionar-rate-limit"} {
		if !strings.Contains(proposal, w) {
			t.Errorf("proposal lacks %q:\n%s", w, proposal)
		}
	}
	if d := read(t, root, "specs/changes/adicionar-rate-limit/deltas/pedidos.md"); !strings.Contains(d, "contexto: pedidos") || !strings.Contains(d, "# Delta — pedidos — adicionar-rate-limit") {
		t.Errorf("delta:\n%s", d)
	}
	if d := read(t, root, "specs/changes/adicionar-rate-limit/design.md"); !strings.Contains(d, "# Design Go") {
		t.Errorf("the pack's design template was not used:\n%s", d)
	}

	// The same id again, open or archived, is refused; so is a bad id.
	if _, err := Create(root, NewChange{ID: "adicionar-rate-limit", Contexts: []string{"pedidos"}, NoGenesis: true}); err == nil {
		t.Error("an open change was created twice")
	}
	write(t, root, "specs/changes/archive/2026-10-01-criar-pedidos/proposal.md", "x")
	if _, err := Create(root, NewChange{ID: "criar-pedidos", Contexts: []string{"pedidos"}, NoGenesis: true}); err == nil || !strings.Contains(err.Error(), "arquivada") {
		t.Errorf("an archived id was reused: %v", err)
	}
	for _, bad := range []NewChange{{ID: "Rate_Limit", Contexts: []string{"pedidos"}}, {ID: "x"}, {ID: "x", Contexts: []string{"a", "a"}}} {
		if _, err := Create(root, bad); err == nil {
			t.Errorf("%+v was accepted", bad)
		}
	}
}

func TestCreateWithoutADesign(t *testing.T) {
	root := project(t)
	changeTemplates(t, root)
	written, err := Create(root, NewChange{ID: "remover-desconto", Contexts: []string{"pedidos"}, Language: "rust", NoDesign: true, NoGenesis: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 || !strings.Contains(read(t, root, "specs/pedidos/changes/remover-desconto/proposal.md"), "design: n/a") {
		t.Errorf("written = %v", written)
	}
	// A language with no pack template takes the neutral one.
	if _, err := Create(root, NewChange{ID: "criar-estoque", Contexts: []string{"estoque"}, Language: "rust", NoGenesis: true}); err != nil {
		t.Fatal(err)
	}
	if d := read(t, root, "specs/estoque/changes/criar-estoque/design.md"); !strings.Contains(d, "# Design neutro") {
		t.Errorf("design:\n%s", d)
	}
}

func TestCreateWithoutTemplatesSaysWhich(t *testing.T) {
	root := t.TempDir()
	_, err := Create(root, NewChange{ID: "criar-pedidos", Contexts: []string{"pedidos"}})
	if err == nil || !strings.Contains(err.Error(), "proposal-template.md ausente") {
		t.Errorf("err = %v", err)
	}
}

func TestChangesListsWhereEachStands(t *testing.T) {
	root := changeLayout(t)
	all := corpus(t, root)
	for _, d := range all {
		if d.Change == "adicionar-rate-limit" {
			if _, err := Approve(root, d); err != nil {
				t.Fatal(err)
			}
		}
	}
	changes := Changes(corpus(t, root))
	if len(changes) != 1 || !changes[0].Sealed || changes[0].Status != "aprovado" || strings.Join(changes[0].Contexts, ",") != "pedidos,envio" {
		t.Fatalf("changes = %+v", changes)
	}
	d := read(t, root, "specs/changes/adicionar-rate-limit/design.md")
	write(t, root, "specs/changes/adicionar-rate-limit/design.md", d+"\nmais uma linha\n")
	changes = Changes(corpus(t, root))
	if changes[0].Sealed || len(changes[0].Stale) != 1 || changes[0].Stale[0] != "specs/changes/adicionar-rate-limit/design.md" {
		t.Errorf("after an edit = %+v", changes[0])
	}
}

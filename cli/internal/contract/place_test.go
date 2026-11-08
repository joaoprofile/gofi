package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A change is born in its first context's folder, however many it touches;
// a folder in specs/changes/ is no change at all. It is read, found, archived
// beside where it lives, and an id is unique across the contexts.
func TestAChangeLivesWithTheContextItChanges(t *testing.T) {
	if got := NewChangeDir("x", []string{"pedidos"}); got != "specs/pedidos/changes/x" {
		t.Errorf("one context: %s", got)
	}
	if got := NewChangeDir("x", []string{"clientes", "pedidos"}); got != "specs/clientes/changes/x" {
		t.Errorf("several: %s", got)
	}
	for rel, want := range map[string]string{
		"specs/pedidos/changes/x/proposal.md":           "x",
		"specs/pedidos/changes/x/deltas/pedidos.md":     "x",
		"specs/changes/y/proposal.md":                   "",
		"specs/pedidos/spec.md":                         "",
		"specs/pedidos/changes/archive/d-x/proposal.md": "",
		"specs/changes/archive/d-y/proposal.md":         "",
	} {
		if got := changeOf(rel); got != want {
			t.Errorf("changeOf(%s) = %q, want %q", rel, got, want)
		}
	}

	root := project(t)
	changeTemplates(t, root)
	if _, err := Create(root, NewChange{ID: "adicionar-filtro", Contexts: []string{"pedidos"}, Language: "rust", NoDesign: true, NoGenesis: true}); err != nil {
		t.Fatal(err)
	}
	if at, ok := FindChange(root, "adicionar-filtro"); !ok || at != "specs/pedidos/changes/adicionar-filtro" {
		t.Errorf("found at %q", at)
	}
	all, err := Docs(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range all {
		if d.Change == "adicionar-filtro" && d.Kind == KindChange {
			found = true
		}
	}
	if !found {
		t.Error("the change in the context's folder is not read")
	}
	if _, err := Create(root, NewChange{ID: "adicionar-filtro", Contexts: []string{"clientes", "pedidos"}, NoGenesis: true}); err == nil || !strings.Contains(err.Error(), "specs/pedidos/changes/adicionar-filtro") {
		t.Errorf("an id taken in a context's folder: %v", err)
	}
	// Archived in the context's archive/, its id stays taken.
	_ = os.MkdirAll(filepath.Join(root, "specs", "pedidos", "changes", "archive", "2026-01-01-velha"), 0o755)
	if _, err := Create(root, NewChange{ID: "velha", Contexts: []string{"pedidos"}, NoGenesis: true}); err == nil {
		t.Error("an id archived in a context's folder was taken again")
	}
	if got := archiveOf("specs/pedidos/changes/x", "2026-11-04", "x"); got != "specs/pedidos/changes/archive/2026-11-04-x" {
		t.Errorf("archive %s", got)
	}
}

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
)

// An open genesis is where gofi puts it: change list says nothing of it —
// neither unseen nor misplaced —, and once archived and committed its
// proposal gone from the context's folder is no loss.
func TestAGenesisIsNoHazard(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs/pedidos/proposal.md"), proposalFor("pedidos"))
	gitIn(t, root, "init", "--quiet", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "genesis")
	if dir, ok := contract.FindChange(root, "pedidos"); !ok || dir != "specs/pedidos" {
		t.Fatalf("the genesis is at %q", dir)
	}
	out, err := execCmd(newChangeCmd, "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "gofi change migrate") || strings.Contains(out, "não lê") || strings.Contains(out, "does not read") {
		t.Errorf("a genesis was said out of place:\n%s", out)
	}

	writeFile(t, filepath.Join(root, "specs/pedidos/changes/archive/2026-11-07-pedidos/proposal.md"), strings.Replace(proposalFor("pedidos"), "proposto", "arquivado", 1))
	_ = os.Remove(filepath.Join(root, "specs/pedidos/proposal.md"))
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "archive")
	if out, _ := execCmd(newChangeCmd, "list"); strings.Contains(out, "git checkout") {
		t.Errorf("an archived genesis was said lost:\n%s", out)
	}
}

// The first change of a context made before the genesis, in
// specs/<ctx>/changes/<id>/, is left alone by gofi change migrate and moved
// into the context's folder by --genesis, sealed as it was and by git — its id
// the context's from then on. One another change depends on, and one of a
// context that has a spec, stay where they are.
func TestMigrateGenesisMovesAFirstChangeIntoItsContext(t *testing.T) {
	root := committedChangeProject(t, "true")
	if out, err := execCmd(newChangeCmd, "migrate"); err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if dir, _ := contract.FindChange(root, "criar-pedidos"); dir != "specs/pedidos/changes/criar-pedidos" {
		t.Fatalf("criar-pedidos is in %q", dir)
	}
	writeFile(t, filepath.Join(root, "specs/clientes/spec.md"), "---\ntipo: spec\nformato: 3\ncontexto: clientes\n---\n# Spec\n")
	writeFile(t, filepath.Join(root, "specs/clientes/changes/mudar-clientes/proposal.md"), proposalFor("clientes"))
	writeFile(t, filepath.Join(root, "specs/estoque/changes/criar-estoque/proposal.md"), proposalFor("estoque"))
	writeFile(t, filepath.Join(root, "specs/changes/depois/proposal.md"), strings.Replace(strings.Replace(proposalFor("estoque"), "[estoque]", "[estoque, clientes]", 1), "status: proposto", "status: proposto\ndepende_de: [criar-estoque]", 1))
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "more changes")

	if _, err := execCmd(newChangeCmd, "migrate", "--genesis", "--dry-run"); err != nil || !exists(filepath.Join(root, "specs/pedidos/changes/criar-pedidos")) {
		t.Fatalf("a dry run moved: %v", err)
	}
	out, err := execCmd(newChangeCmd, "migrate", "--genesis")
	if err != nil {
		t.Fatalf("migrate --genesis: %v\n%s", err, out)
	}
	if dir, ok := contract.FindChange(root, "pedidos"); !ok || dir != "specs/pedidos" {
		t.Fatalf("the genesis is at %q:\n%s", dir, out)
	}
	if _, ok := contract.FindChange(root, "criar-pedidos"); ok {
		t.Error("the old id is still a change")
	}
	if exists(filepath.Join(root, "specs/pedidos/changes")) {
		t.Error("the empty changes/ folder was left")
	}
	if !sealed(t, root, "pedidos") {
		t.Error("the move broke the seal")
	}
	if st := gitIn(t, root, "status", "--porcelain", "--", "specs"); !strings.Contains(st, "R ") {
		t.Errorf("the move must be git's, a rename:\n%s", st)
	}
	for id, dir := range map[string]string{"mudar-clientes": "specs/clientes/changes/mudar-clientes", "criar-estoque": "specs/estoque/changes/criar-estoque"} {
		if got, _ := contract.FindChange(root, id); got != dir {
			t.Errorf("%s moved to %q", id, got)
		}
	}
	if !strings.Contains(out, "criar-estoque") {
		t.Errorf("migrate must say why it left criar-estoque:\n%s", out)
	}
}

// A change cannot take a context's name: it would be read as that context's
// genesis.
func TestAChangeIsNotNamedAsAContext(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs/clientes/spec.md"), "---\ntipo: spec\nformato: 3\ncontexto: clientes\n---\n# Spec\n")
	if _, err := contract.Create(root, contract.NewChange{ID: "clientes", Contexts: []string{"clientes"}}); err == nil || !strings.Contains(err.Error(), "contexto") {
		t.Errorf("a change named as a context: %v", err)
	}
}

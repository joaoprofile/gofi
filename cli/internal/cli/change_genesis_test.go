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
	if strings.Contains(out, "não lê") || strings.Contains(out, "does not read") {
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

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

// Changes of one context created by an older gofi sit in specs/changes/,
// where the skills no longer send the agents. gofi change migrate moves each
// into its context's folder, sealed as it was, by git; a change of several
// contexts and one a run works on stay where they are.
func TestMigrateMovesAOneContextChangeIntoItsContext(t *testing.T) {
	root := committedChangeProject(t, "true")
	writeFile(t, filepath.Join(root, "specs/changes/varios/proposal.md"), strings.Replace(proposalFor("pedidos"), "[pedidos]", "[pedidos, clientes]", 1))
	writeFile(t, filepath.Join(root, "specs/changes/ocupada/proposal.md"), proposalFor("pedidos"))
	_ = os.MkdirAll(loop.Dir(root, "r-1"), 0o755)
	if err := loop.SaveState(root, &loop.State{Schema: loop.StateSchema, Run: "r-1", Change: "ocupada", Node: "implement", Taken: map[string]int{}}); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "older changes")

	if out, err := execCmd(newChangeCmd, "migrate", "--dry-run"); err != nil || !exists(filepath.Join(root, "specs/changes/criar-pedidos")) {
		t.Fatalf("a dry run moved: %v\n%s", err, out)
	}
	out, err := execCmd(newChangeCmd, "migrate")
	if err != nil {
		t.Fatalf("migrate: %v\n%s", err, out)
	}
	if dir, ok := contract.FindChange(root, "criar-pedidos"); !ok || dir != "specs/pedidos/changes/criar-pedidos" {
		t.Fatalf("criar-pedidos is in %q:\n%s", dir, out)
	}
	if !sealed(t, root, "criar-pedidos") {
		t.Error("the move broke the seal")
	}
	if st := gitIn(t, root, "status", "--porcelain", "--", "specs"); !strings.Contains(st, "R ") {
		t.Errorf("the move must be git's, a rename:\n%s", st)
	}
	for _, id := range []string{"varios", "ocupada"} {
		if dir, _ := contract.FindChange(root, id); dir != "specs/changes/"+id {
			t.Errorf("%s moved to %q", id, dir)
		}
	}
	if !strings.Contains(out, "ocupada") {
		t.Errorf("migrate must say why it left ocupada:\n%s", out)
	}
}

// gofi spec check warns about a change of one context in specs/changes/, and
// says what moves it; it does not block, so a project not migrated yet works.
func TestSpecCheckWarnsAboutAMisplacedChange(t *testing.T) {
	committedChangeProject(t, "true")
	out, err := execCmd(newSpecCmd, "check", "criar-pedidos")
	if err != nil {
		t.Fatalf("a misplaced change must warn, not fail: %v\n%s", err, out)
	}
	if !strings.Contains(out, "gofi change migrate") {
		t.Errorf("spec check did not warn:\n%s", out)
	}
}

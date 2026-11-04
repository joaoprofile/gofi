package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// A run cannot win by cheating (0002, D12): what it changed of its contract,
// a test it removed and a skip it added are refused; the open change it
// carries out, there uncommitted from the start, and the memory gofi writes
// are not.
func TestIntegrityRefusesWhatTheRunMayNotChange(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	write := func(rel, body string) { writeFile(t, filepath.Join(root, rel), body) }
	write("specs/pedidos/spec.md", "# Pedidos\n")
	write("backend/pedidos/total.go", "package pedidos\n")
	write("backend/pedidos/total_test.go", "package pedidos\n\nfunc TestA(t *testing.T) {}\n")
	write("backend/pedidos/old_test.go", "package pedidos\n")
	gitIn(t, root, "init", "--quiet", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "init")
	write("specs/changes/criar-pedidos/proposal.md", "# M\n")
	ctx := context.Background()
	if err := captureIntegrity(ctx, root, "r-1"); err != nil {
		t.Fatal(err)
	}
	lp, _ := pack.Load("go")
	if found, err := integrity(ctx, root, "r-1", lp); err != nil || len(found) != 0 {
		t.Fatalf("nothing changed yet: %+v, %v", found, err)
	}

	write("specs/pedidos/spec.md", "# Pedidos\n\nO total pode ser zero.\n")
	write(".claude/memory/contexts/pedidos.md", "status: aprovado\n")
	write("backend/pedidos/total.go", "package pedidos\n\nfunc Total() int { return 1 }\n")
	write("backend/pedidos/total_test.go", "package pedidos\n\nfunc TestA(t *testing.T) {\n\tt.Skip(\"depois\")\n}\n")
	_ = os.Remove(filepath.Join(root, "backend/pedidos/old_test.go"))
	found, err := integrity(ctx, root, "r-1", lp)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Rule+" "+f.URI)
	}
	want := "integrity: contrato specs/pedidos/spec.md|integrity: teste removido backend/pedidos/old_test.go|integrity: teste pulado backend/pedidos/total_test.go"
	if strings.Join(got, "|") != want {
		t.Errorf("found:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.ReplaceAll(want, "|", "\n  "))
	}
	for _, f := range found {
		if f.Rule == "integrity: teste pulado" && f.Line != 4 {
			t.Errorf("the skip is placed at line %d", f.Line)
		}
	}
}

// A test the run found is not left proving less: an assertion taken out of
// it, not put back elsewhere, is refused.
func TestIntegrityRefusesARemovedAssertion(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := t.TempDir()
	test := "backend/pedidos/total_test.go"
	writeFile(t, filepath.Join(root, test), "package pedidos\n\nfunc TestTotal(t *testing.T) {\n\tif Total() != 1 {\n\t\tt.Errorf(\"total\")\n\t}\n\tif Zero() != 0 {\n\t\tt.Errorf(\"zero\")\n\t}\n}\n")
	gitIn(t, root, "init", "--quiet", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "init")
	ctx := context.Background()
	if err := captureIntegrity(ctx, root, "r-1"); err != nil {
		t.Fatal(err)
	}
	lp, _ := pack.Load("go")
	// Rewording an assertion is no loss.
	writeFile(t, filepath.Join(root, test), "package pedidos\n\nfunc TestTotal(t *testing.T) {\n\tif Total() != 1 {\n\t\tt.Errorf(\"o total\")\n\t}\n\tif Zero() != 0 {\n\t\tt.Errorf(\"zero\")\n\t}\n}\n")
	if found, _ := integrity(ctx, root, "r-1", lp); len(found) != 0 {
		t.Fatalf("a reworded assertion: %+v", found)
	}
	writeFile(t, filepath.Join(root, test), "package pedidos\n\nfunc TestTotal(t *testing.T) {\n\tif Total() != 1 {\n\t\tt.Errorf(\"total\")\n\t}\n\t_ = Zero()\n}\n")
	found, err := integrity(ctx, root, "r-1", lp)
	if err != nil || len(found) != 1 || found[0].Rule != "integrity: asserção removida" || found[0].Line == 0 {
		t.Errorf("found %+v, %v", found, err)
	}
}

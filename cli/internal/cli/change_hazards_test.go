package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const proposalOf = "---\ntipo: mudanca\nformato: 3\ncontextos: [%s]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [x]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n"

func proposalFor(ctx string) string { return strings.Replace(proposalOf, "%s", ctx, 1) }

// The change restringir-interruptores-de-tipo vanished from disk, its
// deletion was committed with other files, and gofi change list said
// "nenhuma mudança aberta". Now it says the change was lost, by which commit,
// and how to restore it; a change archived the way gofi archives is no loss.
func TestChangeListSaysAChangeDeletedWithoutBeingArchived(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs/pricing/changes/restringir/proposal.md"), proposalFor("pricing"))
	writeFile(t, filepath.Join(root, "specs/pedidos/changes/aplicada/proposal.md"), proposalFor("pedidos"))
	writeFile(t, filepath.Join(root, "specs/pedidos/changes/sem-commit/proposal.md"), proposalFor("pedidos"))
	gitIn(t, root, "init", "--quiet", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "changes")

	_ = os.RemoveAll(filepath.Join(root, "specs/pricing/changes/restringir"))
	writeFile(t, filepath.Join(root, "specs/pedidos/changes/archive/2026-11-04-aplicada/proposal.md"), strings.Replace(proposalFor("pedidos"), "proposto", "arquivado", 1))
	_ = os.RemoveAll(filepath.Join(root, "specs/pedidos/changes/aplicada"))
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "chore: update gofi")
	deleter := gitIn(t, root, "log", "-1", "--format=%h")
	_ = os.RemoveAll(filepath.Join(root, "specs/pedidos/changes/sem-commit"))

	out, err := execCmd(newChangeCmd, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "restringir") || !strings.Contains(out, "git checkout "+deleter+"^ -- specs/pricing/changes/restringir") {
		t.Errorf("the change deleted by a commit was not said, with its restore:\n%s", out)
	}
	if !strings.Contains(out, "git checkout -- specs/pedidos/changes/sem-commit") {
		t.Errorf("the change deleted on disk was not said:\n%s", out)
	}
	if strings.Contains(out, "aplicada") {
		t.Errorf("an archived change was said lost:\n%s", out)
	}
}

// A change written by hand in a folder gofi does not read is said; one in its
// context's folder, of one context or several, is not.
func TestChangeListSaysAChangeGofiDoesNotRead(t *testing.T) {
	root := configuredProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs/pricing/alinhar-tela/proposal.md"), proposalFor("pricing"))
	writeFile(t, filepath.Join(root, "specs/pricing/changes/so-pricing/proposal.md"), proposalFor("pricing"))
	writeFile(t, filepath.Join(root, "specs/pricing/changes/varios/proposal.md"), strings.Replace(proposalFor("pricing"), "[pricing]", "[pricing, market]", 1))

	out, err := execCmd(newChangeCmd, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "specs/pricing/alinhar-tela") {
		t.Errorf("a change in a folder gofi does not read was not said:\n%s", out)
	}
	for _, id := range []string{"so-pricing", "varios"} {
		if strings.Contains(out, "! specs/pricing/changes/"+id) {
			t.Errorf("%s is where it belongs:\n%s", id, out)
		}
	}
}

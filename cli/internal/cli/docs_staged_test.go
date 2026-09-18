package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/docs"
)

// Regression: post-checkout and post-merge rewrote every tracked INDEX.md, and
// pre-commit rebuilt all of them — dirtying contexts nobody touched.
func TestGofiHookBodies_DocsPassIsScopedToTheCommit(t *testing.T) {
	bodies := gofiHookBodies()
	if !strings.Contains(bodies["pre-commit"], "gofi docs build --staged") {
		t.Errorf("pre-commit must index only what the commit stages:\n%s", bodies["pre-commit"])
	}
	for _, hook := range []string{"post-checkout", "post-merge"} {
		if strings.Contains(bodies[hook], "docs build") {
			t.Errorf("%s must not rewrite tracked indexes:\n%s", hook, bodies[hook])
		}
		if !strings.Contains(bodies[hook], "gofi graph build") {
			t.Errorf("%s still rebuilds the code graph:\n%s", hook, bodies[hook])
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func spec(t *testing.T, root, rel, ctx, keywords string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntipo: spec\nformato: 2\ncontexto: " + ctx + "\nsubmodulo: n/a\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [" + keywords + "]\n---\n# t\n\n## X\n\nt.\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStagedBuild_StagesOnlyTheIndexesOfTheCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	spec(t, root, "specs/billing/sdd-billing.md", "billing", "fatura")
	spec(t, root, "specs/order/sdd-order.md", "order", "venda")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-qm", "base")

	spec(t, root, "specs/billing/sdd-billing.md", "billing", "fatura, competencia") // someone else's drift, unstaged
	spec(t, root, "specs/order/sdd-order.md", "order", "venda, pedido")
	runGit(t, root, "add", "specs/order/sdd-order.md")

	paths, err := stagedPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	b := &docs.Builder{Root: root, Scope: docs.StagedScope(paths)}
	if _, _, _, err := b.Build(); err != nil {
		t.Fatal(err)
	}
	if err := stageFiles(root, b.Written); err != nil {
		t.Fatal(err)
	}

	staged := strings.Fields(runGit(t, root, "diff", "--cached", "--name-only"))
	for _, want := range []string{"specs/order/INDEX.md", "specs/order/sdd-order.md", "specs/INDEX.md"} {
		if !slices.Contains(staged, want) {
			t.Errorf("%s must be staged, got %v", want, staged)
		}
	}
	for _, f := range staged {
		if strings.Contains(f, "billing") {
			t.Errorf("billing was not in the commit and must not be staged: %v", staged)
		}
	}
	if strings.Contains(runGit(t, root, "status", "--short", "specs/billing/INDEX.md"), "INDEX.md") {
		t.Error("the billing index must not even be rewritten")
	}
}

// Regression: in a project whose .gitignore re-ignores .gofi/docs (negation
// written before `.gofi/*`), staging the JSON failed and aborted the build.
// The JSON is the hook's own `git add`; the scoped build stages only Markdown.
func TestStageFiles_LeavesJSONToTheHook(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("!.gofi/docs/\n.gofi/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(root, docs.OutDir, "index.json")
	mdPath := filepath.Join(root, "specs", "order", "INDEX.md")
	for _, p := range []string{jsonPath, mdPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("v1"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-f", "-A")
	runGit(t, root, "commit", "-qm", "base")
	for _, p := range []string{jsonPath, mdPath} {
		if err := os.WriteFile(p, []byte("v2"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := stageFiles(root, []string{jsonPath, mdPath}); err != nil {
		t.Fatalf("staging must not fail on an ignored JSON dir: %v", err)
	}
	staged := strings.Fields(runGit(t, root, "diff", "--cached", "--name-only"))
	if !slices.Contains(staged, "specs/order/INDEX.md") || slices.Contains(staged, filepath.ToSlash(filepath.Join(docs.OutDir, "index.json"))) {
		t.Errorf("only the Markdown index is staged here, got %v", staged)
	}
}

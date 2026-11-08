package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/worktree"
)

// writingSession is an agent that leaves a file in the tree it works on.
type writingSession struct{ fakeSession }

func (w writingSession) Send(ctx context.Context, t engine.Turn) (<-chan engine.Event, error) {
	_ = os.WriteFile(filepath.Join(w.root, "work.txt"), []byte("half done\n"), 0o644)
	return w.fakeSession.Send(ctx, t)
}

func execCmd(cmd func() *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	c := cmd()
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs(args)
	err := c.Execute()
	return out.String(), err
}

// committedChangeProject is a project in git with the change criar-pedidos
// approved and committed, its build sensor as given.
func committedChangeProject(t *testing.T, build string) string {
	t.Helper()
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := configuredProject(t)
	t.Chdir(root)
	runPorts.regenerate = func(context.Context, string) error { return nil }
	t.Cleanup(func() { runPorts.open, runPorts.regenerate = nil, nil })
	installRoles(t, root)
	quietPorts(t)
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  max_iterations: 1\n  sensors:\n    test: \"true\"\n    build: \""+build+"\"\n")
	dir := filepath.Join(root, "specs", "pedidos", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "design.md"), minimalDesign("pedidos"))
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")
	if _, err := execCmd(newChangeCmd, "approve", "criar-pedidos"); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "--quiet", "-b", "main")
	writeFile(t, filepath.Join(root, ".gitignore"), ".gofi/*\n")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "init")
	return root
}

// A run in a worktree that ends without being accepted gives its checkout
// back and keeps what the agents wrote on its branch: before, every failed
// run left a full checkout in the cache forever.
func TestARunInAWorktreeNotAcceptedGivesItsCheckoutBack(t *testing.T) {
	root := committedChangeProject(t, "false")
	var prompts []string
	runPorts.open = func(dir string) engine.Session {
		return writingSession{fakeSession{root: dir, prompts: &prompts}}
	}

	out, err := execCmd(newRunCmd, "criar-pedidos", "--worktree")
	if err == nil {
		t.Fatalf("a run whose build never passes was accepted:\n%s", out)
	}
	runs, _ := loop.List(root)
	if len(runs) != 1 || runs[0].State.Outcome == "accepted" {
		t.Fatalf("runs = %+v", runs)
	}
	s := runs[0].State
	if s.Worktree != "" || s.Kept != "gofi/criar-pedidos" {
		t.Errorf("state: worktree %q, kept %q", s.Worktree, s.Kept)
	}
	if trees, _ := worktree.Trees(context.Background(), root); len(trees) != 0 {
		t.Errorf("the checkout was left: %d worktrees", len(trees))
	}
	if got := gitIn(t, root, "show", "gofi/criar-pedidos:work.txt"); got != "half done" {
		t.Errorf("the branch must keep the agents' work, got %q", got)
	}
}

// gofi run --gc gives back a worktree whose run never got a state — the
// runs that died furnishing it — and keeps one that can still be resumed.
func TestGcGivesBackOnlyWorktreesNoRunNeeds(t *testing.T) {
	root := committedChangeProject(t, "true")
	ctx := context.Background()
	if _, err := worktree.Prepare(ctx, root, "r-dead", "criar-pedidos"); err != nil {
		t.Fatal(err)
	}
	paused, err := worktree.Prepare(ctx, root, "r-paused", "criar-pedidos")
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(loop.Dir(root, "r-paused"), 0o755)
	if err := loop.SaveState(root, &loop.State{Schema: loop.StateSchema, Run: "r-paused", Change: "criar-pedidos", Node: "implement", Worktree: paused.Dir, Branch: paused.Branch, Base: "main", Taken: map[string]int{}}); err != nil {
		t.Fatal(err)
	}

	out, err := execCmd(newRunCmd, "--gc")
	if err != nil {
		t.Fatalf("gc: %v\n%s", err, out)
	}
	trees, _ := worktree.Trees(ctx, root)
	if len(trees) != 1 || filepath.Base(trees[0].Dir) != "r-paused" {
		t.Fatalf("worktrees left = %+v\n%s", trees, out)
	}
	if b := gitIn(t, root, "branch", "--list", "gofi/criar-pedidos"); b != "" {
		t.Errorf("the dead run's empty branch was kept: %q", b)
	}
	if !strings.Contains(out, "r-paused") {
		t.Errorf("gc must say why it kept r-paused:\n%s", out)
	}
}

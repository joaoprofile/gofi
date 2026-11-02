package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/loop"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// A run in a worktree works on its own branch and, accepted, lands on the
// branch the project is on: the archive is on main, by fast-forward, and the
// worktree and its branch are gone (0002, D14).
func TestARunInAWorktreeLandsOnTheCurrentBranch(t *testing.T) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := configuredProject(t)
	t.Chdir(root)
	var prompts []string
	var sessionDirs []string
	runPorts.open = func(dir string) engine.Session {
		sessionDirs = append(sessionDirs, dir)
		return fakeSession{root: dir, prompts: &prompts}
	}
	regenerated := 0
	runPorts.regenerate = func(context.Context, string) error { regenerated++; return nil }
	t.Cleanup(func() { runPorts.open, runPorts.regenerate = nil, nil })
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  sensors:\n    test: \"true\"\n    build: \"true\"\n")
	dir := filepath.Join(root, "specs", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\ndesign: n/a\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")

	run := func(cmd func() *cobra.Command, args ...string) (string, error) {
		var out bytes.Buffer
		c := cmd()
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}
	if _, err := run(newChangeCmd, "approve", "criar-pedidos"); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "--quiet", "-b", "main")
	writeFile(t, filepath.Join(root, ".gitignore"), ".gofi/*\n")

	// Not committed, the change is not in a worktree: refused before any turn.
	if _, err := run(newRunCmd, "criar-pedidos", "--worktree"); err == nil || len(prompts) != 0 {
		t.Fatalf("an uncommitted change ran in a worktree: %v", err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "init")

	out, err := run(newRunCmd, "criar-pedidos", "--worktree")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	for _, d := range sessionDirs {
		if !strings.Contains(d, filepath.Join(".gofi", "worktrees")) {
			t.Errorf("an agent worked in %s, not in the run's worktree", d)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "pedidos", "spec.md")); err != nil {
		t.Errorf("the archive did not land on main:\n%s", out)
	}
	if n := gitIn(t, root, "rev-list", "--merges", "--count", "main"); n != "0" {
		t.Error("landing made a merge commit")
	}
	if log := gitIn(t, root, "log", "--format=%s", "-1"); log != "feat(pedidos): criar-pedidos" {
		t.Errorf("last commit on main: %q", log)
	}
	if b := gitIn(t, root, "branch", "--list", "gofi/*"); b != "" {
		t.Errorf("the landed branch was left: %s", b)
	}
	runs, _ := loop.List(root)
	if len(runs) != 1 || runs[0].State.Landing != "integrada" {
		t.Fatalf("runs = %+v", runs)
	}
	if regenerated == 0 {
		t.Error("the commit did not regenerate the worktree's derived files")
	}
	if _, err := os.Stat(runs[0].State.Worktree); err == nil {
		t.Error("the landed worktree was left")
	}
	if ps, _ := run(newPsCmd); !strings.Contains(ps, "accepted/integrada") {
		t.Errorf("ps:\n%s", ps)
	}
}

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/verdict"
)

var reCommitFile = regexp.MustCompile("Grave a mensagem do commit dela em `([^`]+)`")

// scriptedSession is a turn that does what a role would on disk: the
// implementation writes a file, the audit writes the verdict it is given.
type scriptedSession struct {
	root    string
	verdict string
}

func (s scriptedSession) Send(_ context.Context, t engine.Turn) (<-chan engine.Event, error) {
	switch {
	case isRole(t.Prompt, "gofi-eng"):
		_ = os.MkdirAll(filepath.Join(s.root, "backend", "pedidos"), 0o755)
		_ = os.WriteFile(filepath.Join(s.root, "backend", "pedidos", "total.go"), []byte("package pedidos\n"), 0o644)
	case isRole(t.Prompt, "gofi-qa"):
		m := reVerdictCmd.FindStringSubmatch(t.Prompt)
		iter, _ := strconv.Atoi(m[3])
		v, err := verdict.Parse([]byte(s.verdict), m[1])
		if err != nil {
			panic(err)
		}
		v.Run, v.Iter = m[2], iter
		_, _ = verdict.Write(s.root, v)
	case isRole(t.Prompt, "gofi-deliver"):
		m := reCommitFile.FindStringSubmatch(t.Prompt)
		_ = os.WriteFile(filepath.Join(s.root, m[1]), []byte("feat(pedidos): recusar o pedido de total zero\n\nUm pedido sem valor não fecha.\n"), 0o644)
	}
	ch := make(chan engine.Event, 1)
	ch <- engine.Done{CostUSD: 0.01, Duration: time.Second}
	close(ch)
	return ch, nil
}

func (scriptedSession) Cancel()      {}
func (scriptedSession) Close() error { return nil }

// checkpointProject is a project in git on main with everything committed
// but an approved change, as gofi change approve leaves it.
func checkpointProject(t *testing.T, verdictJSON string) (string, func(...string) (string, error)) {
	for k, v := range map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"} {
		t.Setenv(k, v)
	}
	root := configuredProject(t)
	t.Chdir(root)
	runPorts.open = func(dir string) engine.Session { return scriptedSession{root: dir, verdict: verdictJSON} }
	runPorts.regenerate = func(context.Context, string) error { return nil }
	t.Cleanup(func() { runPorts.open, runPorts.regenerate = nil, nil })
	installRoles(t, root)
	writeFile(t, layout.Roles().Abs(root, "gofi-triage", "contract.yaml"), readFile(t, filepath.Join(shippedSkills, "gofi-triage", "contract.yaml")))
	writeFile(t, layout.Roles().Abs(root, "gofi-triage", "SKILL.md"), "# /gofi-triage\n")
	quietPorts(t)
	writeFile(t, filepath.Join(root, ".gofi.yaml"), readFile(t, filepath.Join(root, ".gofi.yaml"))+"loop:\n  sensors:\n    test: \"true\"\n    build: \"true\"\n")
	writeFile(t, filepath.Join(root, ".gitignore"), ".gofi/*\n")
	gitIn(t, root, "init", "--quiet", "-b", "main")
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "--quiet", "-m", "init")

	dir := filepath.Join(root, "specs", "changes", "criar-pedidos")
	writeFile(t, filepath.Join(dir, "proposal.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [pedidos]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# M\n\n## Por quê\n\nx\n\n## O que muda\n\nREQ-01.\n")
	writeFile(t, filepath.Join(dir, "design.md"), minimalDesign("pedidos"))
	writeFile(t, filepath.Join(dir, "deltas", "pedidos.md"), "---\ntipo: delta\nformato: 3\ncontexto: pedidos\nversao: \"1.0\"\nstatus: proposto\nkeywords: [pedido]\n---\n# D\n\n## Propósito\n\nPedidos.\n\n## ADICIONADO\n\n### Requisito: REQ-01 — Total\nO sistema DEVE recusar total zero.\n\n#### Cenário: REQ-01.1 — Zero\n- DADO total zero\n- QUANDO fecha\n- ENTÃO recusa\n")
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		c := newRunCmd()
		if args[0] == "approve" {
			c = newChangeCmd()
		}
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}
	if out, err := run("approve", "criar-pedidos"); err != nil {
		t.Fatalf("approve: %v\n%s", err, out)
	}
	return root, run
}

// Accepted, the change is committed with the message the delivery wrote.
func TestTheCommitCarriesTheDeliverysMessage(t *testing.T) {
	root, run := checkpointProject(t, `{"result":"aprovado"}`)
	writeFile(t, layout.Roles().Abs(root, "gofi-deliver", "contract.yaml"), readFile(t, filepath.Join(shippedSkills, "gofi-deliver", "contract.yaml")))
	writeFile(t, layout.Roles().Abs(root, "gofi-deliver", "SKILL.md"), "# /gofi-deliver\n")
	gitIn(t, root, "add", "-A", ".claude")
	gitIn(t, root, "commit", "--quiet", "-m", "deliver")
	out, err := run("criar-pedidos")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if got := gitIn(t, root, "log", "-1", "--format=%B"); got != "feat(pedidos): recusar o pedido de total zero\n\nUm pedido sem valor não fecha." {
		t.Errorf("message = %q\n%s", got, out)
	}
}

// In the project's tree, an accepted change is one commit on the current
// branch — its code, its archive and the living spec (0002, D14).
func TestAnAcceptedRunInTheTreeIsOneCommit(t *testing.T) {
	root, run := checkpointProject(t, `{"result":"aprovado"}`)
	out, err := run("criar-pedidos")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if got := gitIn(t, root, "log", "-1", "--format=%s"); got != "feat(pedidos): criar-pedidos" {
		t.Errorf("head = %q\n%s", got, out)
	}
	files := gitIn(t, root, "show", "--name-only", "--format=", "HEAD")
	for _, want := range []string{"backend/pedidos/total.go", "specs/pedidos/spec.md", "specs/changes/archive/"} {
		if !strings.Contains(files, want) {
			t.Errorf("the commit lacks %s:\n%s", want, files)
		}
	}
	if s := gitIn(t, root, "status", "--porcelain"); s != "" {
		t.Errorf("the tree is not clean after the run:\n%s", s)
	}
}

// Not accepted, the run leaves no commit: its work is on gofi/<change>, the
// tree back where it started, the approved change still there.
func TestARunNotAcceptedInTheTreeKeepsItsWorkApart(t *testing.T) {
	root, run := checkpointProject(t, `{"result":"reprovado","findings":[{"severity":"major","owner":"spec","rule":"pedidos/REQ-01","message":"ambíguo"}]}`)
	head := gitIn(t, root, "rev-parse", "HEAD")
	out, err := run("criar-pedidos")
	if err == nil || !strings.Contains(out, "gofi/criar-pedidos") {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if got := gitIn(t, root, "rev-parse", "HEAD"); got != head {
		t.Error("a run not accepted committed on main")
	}
	if _, err := os.Stat(filepath.Join(root, "backend", "pedidos", "total.go")); err == nil {
		t.Error("the work not accepted is still in the tree")
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "changes", "criar-pedidos", "proposal.md")); err != nil {
		t.Error("the approved change left the tree")
	}
	if got := gitIn(t, root, "show", "--name-only", "--format=", "gofi/criar-pedidos"); !strings.Contains(got, "backend/pedidos/total.go") {
		t.Errorf("the branch holds %q", got)
	}
}

// A tree with changes of the person's is not where a run starts.
func TestARunDoesNotStartOnADirtyTree(t *testing.T) {
	root, run := checkpointProject(t, `{"result":"aprovado"}`)
	writeFile(t, filepath.Join(root, "README.md"), "mine\n")
	_, err := run("criar-pedidos")
	if err == nil || !strings.Contains(err.Error(), "README.md") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".gofi", "runs")); err == nil {
		entries, _ := os.ReadDir(filepath.Join(root, ".gofi", "runs"))
		if len(entries) > 0 {
			t.Error("a run started on a dirty tree")
		}
	}
}

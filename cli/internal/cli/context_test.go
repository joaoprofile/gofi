package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// adoptionProject is a gofi project whose code names two contexts.
func adoptionProject(t *testing.T) string {
	t.Helper()
	root := configuredProject(t)
	writeFile(t, filepath.Join(root, "src", "domain", "pedidos", "service.go"), "package pedidos\n")
	writeFile(t, filepath.Join(root, "src", "domain", "pedidos", "service_test.go"), "package pedidos\n")
	writeFile(t, filepath.Join(root, "src", "domain", "estoque", "repo.go"), "package estoque\n")
	writeFile(t, filepath.Join(root, "migrations", "001.sql"), "CREATE TABLE pedidos (id int);\n")
	writeFile(t, filepath.Join(root, "docs", "pedidos-regras.md"), "# Pedidos\n\n## Regras de negócio\nRN-01 x.\n\n## Requisitos\nx\n")
	return root
}

// gofi context adopt lists the candidates, writes the memory of the ones
// named — status adotado, its folder, its tables and documents, what is
// missing — and leaves a memory that exists as it was.
func TestContextAdoptWritesTheMemoryOfTheContextsChosen(t *testing.T) {
	root := adoptionProject(t)
	t.Chdir(root)
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := newContextCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"adopt"}, args...))
		err := cmd.Execute()
		return out.String(), err
	}
	if out, err := run(); err != nil || !strings.Contains(out, "pedidos") || !strings.Contains(out, "estoque") {
		t.Fatalf("candidates: %v\n%s", err, out)
	}
	memory := func(name string) string {
		b, _ := os.ReadFile(layout.Contexts().Abs(root, name+".md"))
		return string(b)
	}
	if memory("pedidos") != "" {
		t.Fatal("listing wrote a memory")
	}
	if _, err := run("pedidos"); err != nil {
		t.Fatal(err)
	}
	m := memory("pedidos")
	for _, want := range []string{"contexto: pedidos", "status: adotado", "diretorio: src/domain/pedidos/", "`pedidos`", "`docs/pedidos-regras.md` (spec)", "/gofi-spec"} {
		if !strings.Contains(m, want) {
			t.Errorf("memory lacks %q:\n%s", want, m)
		}
	}
	if memory("estoque") != "" {
		t.Error("a context not named was adopted")
	}
	if _, err := run("inexistente"); err == nil {
		t.Error("an unknown context was adopted")
	}
	writeFile(t, layout.Contexts().Abs(root, "pedidos.md"), "---\ncontexto: pedidos\nstatus: spec\nversao: \"1.0\"\nkeywords: [x]\n---\n# meu\n")
	out, err := run("--all")
	if err != nil || memory("estoque") == "" || !strings.Contains(memory("pedidos"), "# meu") || !strings.Contains(out, "pedidos") {
		t.Errorf("--all: %v\n%s", err, out)
	}
}

// gofi init asks which contexts to adopt: Enter all, numbers some, n none.
func TestInitAsksWhichContextsToAdopt(t *testing.T) {
	for answer, want := range map[string][]string{"\n": {"estoque", "pedidos"}, "2\n": {"pedidos"}, "n\n": nil} {
		root := adoptionProject(t)
		var out bytes.Buffer
		askAdoption(strings.NewReader(answer), &out, root)
		var got []string
		for _, n := range []string{"estoque", "pedidos"} {
			if _, err := os.Stat(layout.Contexts().Abs(root, n+".md")); err == nil {
				got = append(got, n)
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("answer %q adopted %v, want %v\n%s", answer, got, want, out.String())
		}
	}
}

// A context adopted with a living spec already points at it, and its status
// is the spec's: only what is missing is said to be.
func TestAnAdoptedContextKeepsTheContractItHas(t *testing.T) {
	root := adoptionProject(t)
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs", "pedidos", "spec.md"), "# Pedidos\n")
	cmd := newContextCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"adopt", "pedidos"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(layout.Contexts().Abs(root, "pedidos.md"))
	m := string(b)
	if !strings.Contains(m, "status: spec") || !strings.Contains(m, "spec: specs/pedidos/spec.md") || strings.Contains(m, "a spec viva (`/gofi-spec`)") || !strings.Contains(m, "o PRD") {
		t.Errorf("memory:\n%s", m)
	}
}

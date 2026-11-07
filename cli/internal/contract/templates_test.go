package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// repoAI is the framework's own ai/ tree: these tests read the templates a
// project really gets, so a template and the check cannot drift apart.
const repoAI = "../../../ai"

// realTemplates copies the corpus templates and the go pack's into a project.
func realTemplates(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for src, dst := range map[string]string{
		filepath.Join(repoAI, "templates"):              layout.Templates().Path(),
		filepath.Join(repoAI, "sdk", "go", "templates"): layout.SDK().Path("go", "templates"),
	} {
		entries, err := os.ReadDir(src)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(src, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			write(t, root, dst+"/"+e.Name(), string(b))
		}
	}
	return root
}

var (
	reAnyPlaceholder = regexp.MustCompile(`\{[^{}\n]*\}`)
	reSection2       = regexp.MustCompile(`(?m)^## `)
)

// fillIn does what a writer does to a template, mechanically: every
// placeholder gets a value, REQ-NN a number, open questions are answered and
// the sections that do not apply are taken out.
func fillIn(text string, drop ...string) string {
	text = strings.NewReplacer("{REQ-NN}", "REQ-01", "REQ-NN.1", "REQ-01.1", "REQ-NN", "REQ-01").Replace(text)
	text = reAnyPlaceholder.ReplaceAllString(text, "x")
	var out []string
	for _, ln := range strings.Split(text, "\n") {
		if !strings.Contains(ln, "[PRECISA ESCLARECER") {
			out = append(out, ln)
		}
	}
	text = strings.Join(out, "\n")
	for _, title := range drop {
		start := strings.Index(text, "## "+title+"\n")
		if start < 0 {
			continue
		}
		rest := text[start+3:]
		end := len(text)
		if loc := reSection2.FindStringIndex(rest); loc != nil {
			end = start + 3 + loc[0]
		}
		text = text[:start] + text[end:]
	}
	return text
}

// The skeleton gofi change new writes from the real templates is never
// approvable as it is — and once a writer fills it in, it is.
// Both ways a first change is born: the genesis, in the context's own
// folder, and one in changes/<id>/.
func TestTheRealTemplatesAndTheCheckAgree(t *testing.T) {
	for _, tc := range []struct {
		name      string
		noGenesis bool
		id, dir   string
	}{
		{"genesis", false, "pedidos", "specs/pedidos"},
		{"changes folder", true, "criar-pedidos", "specs/pedidos/changes/criar-pedidos"},
	} {
		t.Run(tc.name, func(t *testing.T) { realTemplatesAgree(t, tc.noGenesis, tc.id, tc.dir) })
	}
}

func realTemplatesAgree(t *testing.T, noGenesis bool, id, dir string) {
	root := realTemplates(t)
	written, err := Create(root, NewChange{ID: "criar-pedidos", Contexts: []string{"pedidos"}, Language: "go", Date: "2026-11-01", NoGenesis: noGenesis})
	if err != nil {
		t.Fatal(err)
	}
	placeholders := mustPlaceholders(t, root)
	all := corpus(t, root)
	kinds := map[string]string{}
	for _, d := range all {
		kinds[d.Path] = d.Kind
		if Errors(Check(root, all, d, placeholders)) == 0 {
			t.Errorf("%s from the template is approvable as it is", d.Path)
		}
	}
	for _, p := range written {
		if kinds[p] == "" {
			t.Errorf("%s is not read as a contract document", p)
		}
	}
	if d := read(t, root, dir+"/design.md"); !strings.Contains(d, "gofi-sdk-go") {
		t.Error("the change took the neutral design, not the go pack's")
	}

	// pedidos has no living spec yet: the delta only adds.
	for _, p := range written {
		drop := []string{}
		if strings.Contains(p, "/deltas/") {
			drop = []string{"MODIFICADO", "REMOVIDO"}
		}
		write(t, root, p, fillIn(read(t, root, p), drop...))
	}
	all = corpus(t, root)
	for _, d := range all {
		if got := Check(root, all, d, placeholders); Errors(got) > 0 {
			t.Errorf("%s filled in does not pass:\n%s", d.Path, messages(got))
		}
	}
	for _, f := range docs.Validate(root) {
		if f.Error {
			t.Errorf("corpus finding on a filled-in template: %s: %s", f.Path, f.Message)
		}
	}

	// Approved and archived, it leaves a living spec that passes.
	for _, d := range all {
		if _, err := Approve(root, d); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Archive(root, corpus(t, root), id, "2026-11-01"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	living, ok := Living(corpus(t, root), "pedidos")
	if !ok {
		t.Fatal("no living spec after archiving")
	}
	if got := Check(root, corpus(t, root), living, placeholders); Errors(got) > 0 {
		t.Errorf("living spec:\n%s", messages(got))
	}
	// The change went to the context's archive, the spec in its place.
	if living.Path != "specs/pedidos/spec.md" {
		t.Errorf("living spec at %s", living.Path)
	}
	for _, name := range []string{"proposal.md", "design.md", "deltas/pedidos.md"} {
		if _, err := os.Stat(filepath.Join(root, "specs/pedidos/changes/archive/2026-11-01-"+id, name)); err != nil {
			t.Errorf("archive lacks %s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(root, dir, name)); err == nil {
			t.Errorf("%s/%s was left behind", dir, name)
		}
	}
	if left := Changes(corpus(t, root)); len(left) != 0 {
		t.Errorf("open changes after the archive: %+v", left)
	}
}

// The living spec template, filled in, is a spec the check accepts — the
// baseline gofi-spec writes for a context that already exists.
func TestTheSpecTemplateFilledInPasses(t *testing.T) {
	root := realTemplates(t)
	tmpl := read(t, root, layout.Templates().Path("spec-template.md"))
	write(t, root, "specs/pedidos/spec.md", strings.Replace(fillIn(tmpl), "contexto: x", "contexto: pedidos", 1))
	all := corpus(t, root)
	if got := Check(root, all, only(t, root), mustPlaceholders(t, root)); Errors(got) > 0 {
		t.Errorf("findings:\n%s", messages(got))
	}
	write(t, root, "specs/pedidos/spec.md", tmpl)
	if Errors(Check(root, corpus(t, root), only(t, root), mustPlaceholders(t, root))) == 0 {
		t.Error("the spec template is approvable as it is")
	}
}

// The PRD template, filled in, is a PRD the corpus accepts.
func TestThePRDTemplateFilledInPasses(t *testing.T) {
	root := realTemplates(t)
	tmpl := read(t, root, layout.Templates().Path("prd-template.md"))
	write(t, root, "prd/pedidos/prd-pedidos.md", strings.Replace(fillIn(tmpl), "contexto: x", "contexto: pedidos", 1))
	for _, f := range docs.Validate(root) {
		if f.Error {
			t.Errorf("%s: %s", f.Path, f.Message)
		}
	}
}

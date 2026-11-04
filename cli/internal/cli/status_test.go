package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The next step of a context is the first thing it lacks, in the order it
// is done; a missing PRD is only a suggestion.
func TestTheNextStepIsWhatAContextLacksFirst(t *testing.T) {
	for want, c := range map[string]ContextStatus{
		"gofi context adopt p":                    {Name: "p", Sources: 3},
		"gofi spec migrate p":                     {Name: "p", Memory: "adotado", Legacy: true},
		"/gofi-spec p":                            {Name: "p", Memory: "adotado"},
		"gofi spec check p":                       {Name: "p", Memory: "spec", Spec: "s", SpecState: "com-erros"},
		"gofi spec approve p":                     {Name: "p", Memory: "spec", Spec: "s", SpecState: "vencida"},
		"gofi review c1 · gofi change approve c1": {Name: "p", Memory: "spec", Spec: "s", SpecState: "aprovada", Changes: []string{"c1"}},
		"gofi run c1":                             {Name: "p", Memory: "spec", Spec: "s", SpecState: "aprovada", Changes: []string{"c1"}, Sealed: []string{"c1"}},
		"gofi trace p":                            {Name: "p", Memory: "aprovado", Spec: "s", SpecState: "aprovada", Requirements: [2]int{2, 1}},
		"":                                        {Name: "p", Memory: "aprovado", Spec: "s", SpecState: "aprovada", Requirements: [2]int{2, 2}, Scenarios: [2]int{3, 3}},
	} {
		if got := nextStep(c); got != want {
			t.Errorf("%+v: next %q, want %q", c, got, want)
		}
	}
}

// gofi status reads every context the project has — by its memory, its
// specs, its changes or its code — and says where each one is.
func TestStatusSaysWhereEachContextIs(t *testing.T) {
	root := adoptionProject(t) // code: pedidos, estoque
	t.Chdir(root)
	writeFile(t, filepath.Join(root, "specs", "legado", "sdd-legado.md"), "---\nversao: \"1.0\"\n---\n# Legado\n")
	writeFile(t, layout.Contexts().Abs(root, "estoque.md"), "---\ncontexto: estoque\nstatus: adotado\nversao: \"1.0\"\nkeywords: [estoque]\n---\n# estoque\n")
	writeFile(t, filepath.Join(root, "prd", "estoque", "prd-estoque.md"), "# PRD\n")
	change(t, root, "repor-estoque", "")
	var out bytes.Buffer
	cmd := newStatusCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema   string          `json:"schema"`
		Contexts []ContextStatus `json:"contexts"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	got := map[string]ContextStatus{}
	for _, c := range doc.Contexts {
		got[c.Name] = c
	}
	if c := got["pedidos"]; c.Memory != "" || c.Sources != 1 || c.Next != "gofi context adopt pedidos" {
		t.Errorf("pedidos %+v", c)
	}
	if c := got["legado"]; !c.Legacy || c.Next != "gofi spec migrate legado" {
		t.Errorf("legado %+v", c)
	}
	if c := got["estoque"]; c.Memory != "adotado" || c.PRD == "" || c.Next != "/gofi-spec estoque" {
		t.Errorf("estoque %+v", c)
	}
	if c := got["repor-estoque"]; len(c.Changes) != 1 || !strings.HasPrefix(c.Next, "gofi review repor-estoque") {
		t.Errorf("the change's context %+v", c)
	}
	out.Reset()
	cmd = newStatusCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"pedidos"})
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), "→ gofi context adopt pedidos") || strings.Contains(out.String(), "estoque —") {
		t.Errorf("one context: %v\n%s", err, out.String())
	}
}

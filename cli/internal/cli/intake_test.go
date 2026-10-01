package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/intake"
)

// `gofi intake --json` answers in the versioned schema, from the contracts
// installed in the project.
func TestIntakeCommandJSON(t *testing.T) {
	restoreGlobals(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gofi.yaml"), "version: 2\nproject:\n  name: demo\n")
	skills, _ := filepath.Glob("../../../ai/skills/*/contract.yaml")
	for _, s := range skills {
		b, _ := os.ReadFile(s)
		writeFile(t, filepath.Join(root, ".claude/skills", filepath.Base(filepath.Dir(s)), "contract.yaml"), string(b))
	}
	writeFile(t, filepath.Join(root, "specs/pricing/sdd-pricing.md"),
		"---\ntipo: spec\nformato: sdd\ncontexto: pricing\nversao: \"1.0\"\nstatus: aprovado\nkeywords: [preco]\n---\n# SDD — pricing\n\n## 1. Envio de preços\n\nEm lote.\n")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"intake", "altere o envio de pricing, adicionando um rate limit", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var r intake.Result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out.String())
	}
	if r.Schema != intake.Schema || r.Context == nil || r.Context.Name != "pricing" || len(r.Plan) != 4 || !r.Plan[0].Elicit || r.Plan[1].Role != "gofi-spec" {
		t.Errorf("result = %+v missing=%v", r, r.Missing)
	}
	if err := NewRoot().ParseFlags([]string{"--tier", "cheap"}); err == nil {
		cmd := NewRoot()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"intake", "x", "--tier", "cheap"})
		if err := cmd.Execute(); err == nil {
			t.Error("an unknown tier must be refused")
		}
	}
}

// --serve answers one request per line, keeping the index loaded, and a bad
// line gets an error line instead of ending the process.
func TestIntakeServe(t *testing.T) {
	root := intakeProject(t)
	t.Chdir(root)
	in := strings.NewReader(`{"request":"altere o envio de pricing, adicionando um rate limit","no_model":true}
not json
{"request":"oi","no_model":true}
`)
	var out bytes.Buffer
	if err := serveIntake(root, in, &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d:\n%s", len(lines), out.String())
	}
	var first, third intake.Result
	_ = json.Unmarshal([]byte(lines[0]), &first)
	_ = json.Unmarshal([]byte(lines[2]), &third)
	if first.Schema != intake.Schema || first.Context == nil || first.Context.Name != "pricing" || len(first.Plan) == 0 {
		t.Errorf("first = %s", lines[0])
	}
	if !strings.Contains(lines[1], `"error"`) {
		t.Errorf("bad line = %s", lines[1])
	}
	if !third.Chat {
		t.Errorf("third = %s", lines[2])
	}
}

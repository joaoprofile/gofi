package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// gofi scan reads a repository that is no gofi project, writes nothing, and
// its first step is to adopt it; --json is the same report, with the steps.
func TestScanSaysWhatARepositoryHoldsAndWhatIsNext(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/loja\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "domain", "pedidos", "service.go"), "package pedidos\n")
	writeFile(t, filepath.Join(root, "domain", "pedidos", "service_test.go"), "package pedidos\n")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "# time\n")
	writeFile(t, filepath.Join(root, "docs", "requisitos.md"), "# Requisitos\n\n## Regras de negócio\nRN-01 x.\n\n## Critérios de aceite\nx\n")
	run := func(args ...string) string {
		var out bytes.Buffer
		cmd := newScanCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	var doc struct {
		Schema string   `json:"schema"`
		Next   []string `json:"next"`
		Gofi   struct {
			Config bool `json:"config"`
		} `json:"gofi"`
		Contexts []struct {
			Name string `json:"name"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal([]byte(run(root, "--json")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != "gofi-scan/v1" || doc.Gofi.Config || len(doc.Contexts) != 1 || doc.Contexts[0].Name != "pedidos" {
		t.Errorf("report %+v", doc)
	}
	if len(doc.Next) < 3 || !strings.HasPrefix(doc.Next[0], "gofi init") || !strings.Contains(strings.Join(doc.Next, "\n"), "CLAUDE.md") || !strings.Contains(strings.Join(doc.Next, "\n"), "/gofi-spec") {
		t.Errorf("next %v", doc.Next)
	}
	text := run(root)
	if !strings.Contains(text, "docs/requisitos.md") || !strings.Contains(text, "pedidos") {
		t.Errorf("report:\n%s", text)
	}
	if _, err := filepath.Glob(filepath.Join(root, ".gofi", "*")); err != nil {
		t.Fatal(err)
	}
	if m, _ := filepath.Glob(filepath.Join(root, ".gofi")); len(m) > 0 {
		t.Error("scan wrote .gofi/")
	}
}

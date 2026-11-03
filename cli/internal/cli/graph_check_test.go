package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/sensor"
)

// layered is a configured project with a context in three layers: the
// handler calls the service, the service the repository.
func layered(t *testing.T) string {
	t.Helper()
	root := configuredProject(t)
	write := func(rel, body string) { writeFile(t, filepath.Join(root, "src", rel), body) }
	write("domain/pedidos/repository/pedidos_repository.go", "package repository\n\n// Save keeps an order.\nfunc Save(id int) error { return nil }\n")
	write("domain/pedidos/service/pedidos_service.go", "package service\n\nimport \"example.com/app/domain/pedidos/repository\"\n\n// Create creates an order.\nfunc Create(id int) error { return repository.Save(id) }\n")
	write("domain/pedidos/handler/pedidos_handler.go", "package handler\n\nimport \"example.com/app/domain/pedidos/service\"\n\n// Post answers a request.\nfunc Post(id int) error { return service.Create(id) }\n")
	write("domain/faturas/service/faturas_service.go", "package service\n\nimport \"example.com/app/domain/pedidos/repository\"\n\n// Bill bills an order.\nfunc Bill(id int) error { return repository.Save(id) }\n")
	return root
}

// The graph stage holds the change to the graph the run started from: what
// it touched, by layer and context, and what depends on it; a layer it made
// reach another the pack forbids is refused, at its symbol.
func TestGraphCheckHoldsTheChangeToItsBaseline(t *testing.T) {
	root := layered(t)
	runGraphBuildIn(t, root)
	const run = "r-1"
	if err := captureBaseline(root, run); err != nil {
		t.Fatal(err)
	}
	// The change: the repository gains a check, the handler now reaches the
	// repository itself.
	writeFile(t, filepath.Join(root, "src/domain/pedidos/repository/pedidos_repository.go"),
		"package repository\n\n// Save keeps an order.\nfunc Save(id int) error {\n\tif id < 0 {\n\t\treturn nil\n\t}\n\treturn nil\n}\n")
	writeFile(t, filepath.Join(root, "src/domain/pedidos/handler/pedidos_handler.go"),
		"package handler\n\nimport (\n\t\"example.com/app/domain/pedidos/repository\"\n\t\"example.com/app/domain/pedidos/service\"\n)\n\n// Post answers a request.\nfunc Post(id int) error {\n\tif err := repository.Save(id); err != nil {\n\t\treturn err\n\t}\n\treturn service.Create(id)\n}\n")

	t.Chdir(root)
	var out bytes.Buffer
	cmd := newIndexCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"code", "--no-html", "--baseline", loop.BaselineDir(run) + "/gofi_graph.json", "--diff", loop.DiffFile(run), "--report", loop.GraphReport(run)})
	err := cmd.Execute()
	if err == nil || !strings.Contains(out.String(), "deny: handler -> repository") {
		t.Fatalf("a broken layer rule was not refused: %v\n%s", err, out.String())
	}

	d, err := loop.ReadDiff(root, run)
	if err != nil || d == nil {
		t.Fatalf("diff: %v, %v", d, err)
	}
	if strings.Join(d.Layers, ",") != "data,api" || strings.Join(d.Contexts, ",") != "pedidos" {
		t.Errorf("layers %v, contexts %v", d.Layers, d.Contexts)
	}
	// Whatever depends on the repository's Save is in reach — faturas too.
	if !strings.Contains(strings.Join(d.Reach, ","), "faturas") || d.Dependents == 0 {
		t.Errorf("reach %v, %d dependents", d.Reach, d.Dependents)
	}
	report, err := os.ReadFile(filepath.Join(root, loop.GraphReport(run)))
	if err != nil {
		t.Fatal(err)
	}
	got := sensor.Read("graph", pack.FormatSARIF, report, nil, "")
	if len(got) != 1 || got[0].Severity != "blocker" || got[0].Symbol != "domain/pedidos/handler.Post" ||
		got[0].File != "src/domain/pedidos/handler/pedidos_handler.go" {
		t.Errorf("findings %+v", got)
	}

	// The verdict's finding placed by line is keyed by its symbol.
	at := symbolAt(root)
	if s := at("src/domain/pedidos/handler/pedidos_handler.go", 11); s != "domain/pedidos/handler.Post" {
		t.Errorf("symbolAt = %q", s)
	}
	if s := at("README.md", 1); s != "" {
		t.Errorf("symbolAt(outside the tree) = %q", s)
	}
}

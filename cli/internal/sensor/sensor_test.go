package sensor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// module writes a Go module under root/backend with the given files.
func module(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/loja\n\ngo 1.26\n"
	for name, body := range files {
		p := filepath.Join(root, "backend", filepath.FromSlash(name))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func goRunner(t *testing.T, root string) Runner {
	t.Helper()
	p, err := pack.Load("go")
	if err != nil || p == nil {
		t.Fatalf("go pack: %v", err)
	}
	cmds := map[string]pack.Command{}
	for k, c := range p.Sensors {
		cmds[k] = c.Expand("backend")
	}
	return Runner{Root: root, Commands: cmds, Env: []string{"GOWORK=off", "GOFLAGS=-count=1"}}
}

var stages = []string{"build", "static", "test"}

// Code that does not build stops at the build, its errors placed by file
// and line from the project root.
func TestABuildErrorIsPlaced(t *testing.T) {
	root := module(t, map[string]string{"frete/frete.go": "package frete\n\nfunc Calcular() int { return \"x\" }\n"})
	res := goRunner(t, root).Run(context.Background(), stages)
	if len(res.Failed) != 1 || res.Failed[0] != "build" || len(res.Findings) == 0 {
		t.Fatalf("result %+v", res)
	}
	f := res.Findings[0]
	if f.File != "backend/frete/frete.go" || f.Line != 3 || f.Severity != "blocker" {
		t.Errorf("finding %+v", f)
	}
}

// A failing test is a finding with its name and what it printed; a skipped
// one fails the stage too, though go test exits clean.
func TestAFailedOrSkippedTestFailsTheStage(t *testing.T) {
	root := module(t, map[string]string{
		"frete/frete.go":      "package frete\n\nfunc Calcular() int { return 1 }\n",
		"frete/frete_test.go": "package frete\n\nimport \"testing\"\n\nfunc TestCalcular(t *testing.T) {\n\tif Calcular() != 2 {\n\t\tt.Errorf(\"Calcular = %d, want 2\", Calcular())\n\t}\n}\n",
	})
	res := goRunner(t, root).Run(context.Background(), stages)
	if len(res.Failed) != 1 || res.Failed[0] != "test" || len(res.Findings) != 1 || res.Findings[0].Rule != "TestCalcular" ||
		!strings.Contains(res.Findings[0].Message, "Calcular = 1, want 2") {
		t.Fatalf("failing test: %+v", res)
	}

	root = module(t, map[string]string{
		"frete/frete.go":      "package frete\n\nfunc Calcular() int { return 1 }\n",
		"frete/frete_test.go": "package frete\n\nimport \"testing\"\n\nfunc TestCalcular(t *testing.T) { t.Skip(\"depois\") }\n",
	})
	res = goRunner(t, root).Run(context.Background(), stages)
	if len(res.Failed) != 1 || res.Failed[0] != "test" || len(res.Findings) != 1 || !strings.Contains(res.Findings[0].Message, "foi pulado") {
		t.Fatalf("skipped test: %+v", res)
	}
}

// Clean code passes every stage, and a stage with no command is said to be
// unchecked, never passed.
func TestCleanCodePasses(t *testing.T) {
	root := module(t, map[string]string{
		"frete/frete.go":      "package frete\n\nfunc Calcular() int { return 1 }\n",
		"frete/frete_test.go": "package frete\n\nimport \"testing\"\n\nfunc TestCalcular(t *testing.T) {\n\tif Calcular() != 1 {\n\t\tt.Fail()\n\t}\n}\n",
	})
	res := goRunner(t, root).Run(context.Background(), append(stages, "integrity"))
	if len(res.Failed) != 0 || !strings.Contains(res.Output, "integrity: não verificado") {
		t.Fatalf("result %+v", res)
	}
}

// SARIF, which any linter writes, is read by rule, level and place.
func TestSARIFIsRead(t *testing.T) {
	doc := `{"runs":[{"results":[{"ruleId":"errcheck","level":"error","message":{"text":"erro ignorado"},
	"locations":[{"physicalLocation":{"artifactLocation":{"uri":"backend/frete/frete.go"},"region":{"startLine":7}}}]}]}]}`
	got := Read("static", pack.FormatSARIF, []byte(doc), nil, "")
	if len(got) != 1 || got[0].Rule != "errcheck" || got[0].Line != 7 || got[0].Severity != "blocker" {
		t.Errorf("findings %+v", got)
	}
}

// The graph stage places a broken rule at its symbol: SARIF's logical
// location.
func TestSARIFSymbolIsRead(t *testing.T) {
	doc := `{"runs":[{"results":[{"ruleId":"deny: handler -> repository","level":"error","message":{"text":"x"},
	"locations":[{"physicalLocation":{"artifactLocation":{"uri":"backend/domain/p/handler/h.go"},"region":{"startLine":9}},
	"logicalLocations":[{"fullyQualifiedName":"domain/p/handler.Create"}]}]}]}]}`
	got := Read("graph", pack.FormatSARIF, []byte(doc), nil, "")
	if len(got) != 1 || got[0].Symbol != "domain/p/handler.Create" || got[0].File != "backend/domain/p/handler/h.go" {
		t.Errorf("findings %+v", got)
	}
}

// JUnit XML, as jest-junit, pytest and surefire write it: a failed and a
// skipped case are blockers, a passing one is nothing; nested suites are
// read too.
func TestJUnitIsRead(t *testing.T) {
	doc := `<?xml version="1.0"?>
<testsuites><testsuite name="pontos" file="src/pontos/pontos.service.spec.ts">
  <testcase classname="PontosService" name="REQ-03.1 consulta válida"/>
  <testcase classname="PontosService" name="REQ-03.2 valor inválido"><failure message="expected 400">at x.spec.ts:20</failure></testcase>
  <testcase classname="PontosService" name="REQ-03.3"><skipped/></testcase>
</testsuite></testsuites>`
	got := Read("test", pack.FormatJUnit, []byte(doc), nil, "backend")
	if len(got) != 2 || got[0].File != "backend/src/pontos/pontos.service.spec.ts" || !strings.Contains(got[0].Message, "expected 400") ||
		!strings.Contains(got[1].Message, "pulado") {
		t.Errorf("findings %+v", got)
	}
	if got := Read("test", pack.FormatJUnit, []byte(`<testsuite><testcase name="ok"/></testsuite>`), nil, ""); len(got) != 0 {
		t.Errorf("a passing suite: %+v", got)
	}
}

// tsc and javac through Maven place an error in shapes of their own.
func TestTheCompilersShapesArePlaced(t *testing.T) {
	out := "src/pontos/pontos.controller.ts(12,5): error TS2322: Type 'string' is not assignable to type 'number'.\n" +
		"[ERROR] /app/src/main/java/loja/pontos/PontosResource.java:[30,17] cannot find symbol\n"
	got := Read("build", pack.FormatLines, []byte(out), nil, "backend")
	if len(got) != 2 || got[0].File != "backend/src/pontos/pontos.controller.ts" || got[0].Line != 12 ||
		got[1].File != "/app/src/main/java/loja/pontos/PontosResource.java" || got[1].Line != 30 {
		t.Errorf("findings %+v", got)
	}
}

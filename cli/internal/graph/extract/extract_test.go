package extract_test

import (
	"encoding/json"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/analyze"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

const fixture = "testdata/sample"

func opts() extract.Options { return extract.Options{Root: fixture, MaxFileKB: 2048} }

func findEdge(g *model.Graph, from, to string, rel model.Rel) *model.Edge {
	for _, e := range g.Edges {
		if e.From == from && e.To == to && e.Rel == rel {
			return e
		}
	}
	return nil
}

func TestFastDeclaresSymbols(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}
	analyze.Run(g)
	want := []string{
		model.PkgID("sample/api"),
		model.TypeID("sample/store", "Repo"),
		model.TypeID("sample/store", "Memory"),
		model.MethodID("sample/api", "Server", "Start"),
		model.FuncID("sample/api", "Bootstrap"),
	}
	for _, id := range want {
		if !g.Has(id) {
			t.Errorf("missing node: %s", id)
		}
	}
	if n := g.Get(model.MethodID("sample/api", "Server", "Start")); n == nil || n.File == "" || n.Line == 0 {
		t.Error("method without file/line origin")
	}
}

// Fast mode must not invent a target when two types have a method with the same
// name. We prefer counting it as ambiguous over producing a wrong edge.
func TestFastDoesNotGuessAmbiguousMethod(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}
	from := model.FuncID("sample/api", "Bootstrap")
	for _, target := range []string{
		model.MethodID("sample/api", "Server", "Start"),
		model.MethodID("sample/api", "Worker", "Start"),
	} {
		if e := findEdge(g, from, target, model.RelCalls); e != nil {
			t.Errorf("fast mode created ambiguous edge %s -> %s", from, target)
		}
	}
	if g.Stats.Ambiguous == 0 {
		t.Error("expected ambiguous call count greater than zero")
	}
}

func TestDeepResolvesCallsAndImplementations(t *testing.T) {
	o := opts()
	o.Deep = true
	g, err := extract.Deep(o)
	if err != nil {
		t.Fatal(err)
	}
	analyze.Run(g)

	from := model.FuncID("sample/api", "Bootstrap")
	for _, target := range []string{
		model.MethodID("sample/api", "Server", "Start"),
		model.MethodID("sample/api", "Worker", "Start"),
	} {
		e := findEdge(g, from, target, model.RelCalls)
		if e == nil {
			t.Fatalf("deep mode did not resolve the call %s -> %s", from, target)
		}
		if e.Conf < 1.0 {
			t.Errorf("type-resolved call should have confidence 1.0, got %.2f", e.Conf)
		}
		if e.File == "" || e.Line == 0 {
			t.Error("edge without origin in the code")
		}
	}
	repo := model.TypeID("sample/store", "Repo")
	for _, impl := range []string{"Memory", "Audit"} {
		if findEdge(g, model.TypeID("sample/store", impl), repo, model.RelImplements) == nil {
			t.Errorf("did not detect that %s implements Repo", impl)
		}
	}
	if g.Stats.Ambiguous != 0 {
		t.Errorf("deep mode should have no ambiguity, got %d", g.Stats.Ambiguous)
	}
}

func TestDetectsMutualRecursion(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}
	analyze.Run(g)
	if len(g.Cycles) == 0 {
		t.Fatal("expected to detect the even/odd cycle")
	}
	found := false
	for _, c := range g.Cycles {
		if len(c) == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("even/odd cycle not found, cycles: %v", g.Cycles)
	}
}

// The same codebase has to produce exactly the same file: without that the
// graph would generate diff noise on every commit.
func TestDeterministicOutput(t *testing.T) {
	render := func() string {
		g, err := extract.Fast(opts())
		if err != nil {
			t.Fatal(err)
		}
		analyze.Run(g)
		g.Sort()
		b, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if a, b := render(), render(); a != b {
		t.Error("two scans of the same codebase produced different graphs")
	}
}

func TestEdgesAlwaysHaveOriginAndConfidence(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges {
		if e.Conf <= 0 || e.Conf > 1 {
			t.Errorf("edge %s->%s with invalid confidence %.2f", e.From, e.To, e.Conf)
		}
		if e.Rel != model.RelImplements && e.File == "" {
			t.Errorf("edge %s->%s (%s) without origin file", e.From, e.To, e.Rel)
		}
	}
}

// A repository split into several modules and joined by a go.work is one unit of
// work, and the graph has to hold all of it. The gofi SDK is published that way:
// reading only its root module left 15 of its 176 files in the graph, so every
// call into base/errs or obs dead-ended.
func TestGoWorkModulesAreOneGraph(t *testing.T) {
	g, err := extract.Fast(extract.Options{Root: "testdata/multimod", MaxFileKB: 2048})
	if err != nil {
		t.Fatal(err)
	}
	if g.Stats.Files != 2 {
		t.Fatalf("files = %d, want 2 (the sibling module was skipped)", g.Stats.Files)
	}
	// The sibling's import path comes from its own go.mod, not from its folder:
	// deriving it from the root module would name it example.com/root/svc.
	if n := g.Get(model.PkgID("other.example.com/service")); n == nil || n.External {
		t.Fatalf("sibling module package missing or external: %+v", n)
	}
	if e := findEdge(g, model.FuncID("example.com/root", "Boot"),
		model.FuncID("other.example.com/service", "Name"), model.RelCalls); e == nil {
		t.Error("a call across the workspace modules did not resolve")
	}
}

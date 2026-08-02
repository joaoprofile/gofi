package report_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/analyze"
	"github.com/joaoprofile/gofi-cli/internal/graph/extract"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
	"github.com/joaoprofile/gofi-cli/internal/graph/report"
)

// The graph is committed to the repository and rebuilt on every commit, so any
// non-determinism turns into permanent diff noise. These goldens pin the whole
// pipeline — scan, analyze, render — not just the parts with obvious ordering.
//
// Regenerate with: go test ./internal/graph/report -update

var update = flag.Bool("update", false, "rewrite the golden files")

func build(t *testing.T) *model.Graph {
	t.Helper()
	g, err := extract.Fast(extract.Options{Root: "../extract/testdata/sample", MaxFileKB: 2048})
	if err != nil {
		t.Fatal(err)
	}
	analyze.Run(g)
	g.Sort()
	return g
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden ausente: %v (rode com -update)", err)
	}
	if string(got) == string(want) {
		return
	}
	// Drop the real output next to the test run so the diff can be inspected
	// without re-running with -update and losing the expected file.
	if dir := t.ArtifactDir(); dir != "" {
		_ = os.WriteFile(filepath.Join(dir, name), got, 0o644)
		t.Logf("saida atual salva em %s", filepath.Join(dir, name))
	}
	t.Errorf("%s divergiu do golden", name)
}

func TestGoldenGraphJSON(t *testing.T) {
	b, err := json.MarshalIndent(build(t), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "gofi_graph.json", append(b, '\n'))
}

func TestGoldenReportMarkdown(t *testing.T) {
	checkGolden(t, "gofi_graph_report.md", []byte(report.Markdown(build(t))))
}

// The graph is committed, so two developers scanning the same commit have to
// produce the same bytes — and they never share a checkout path. Anything
// absolute that reaches the file (the scan root, a wall clock, a duration)
// would turn every pull into a conflict.
func TestGraphCarriesNothingMachineSpecific(t *testing.T) {
	scan := func(dir string) string {
		t.Helper()
		if err := os.CopyFS(dir, os.DirFS("../extract/testdata/sample")); err != nil {
			t.Fatal(err)
		}
		g, err := extract.Fast(extract.Options{Root: dir, MaxFileKB: 2048})
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

	a, b := scan(filepath.Join(t.TempDir(), "alice")), scan(filepath.Join(t.TempDir(), "bob"))
	if a != b {
		t.Error("the same code scanned from two checkouts produced different graphs")
	}
	if strings.Contains(a, t.TempDir()) {
		t.Error("an absolute path from this machine reached the graph")
	}
}

// Two scans of the same tree must agree byte for byte. The goldens above would
// catch a drift across commits; this catches one that only shows up between two
// runs in the same process, which is what a map iteration order bug looks like.
func TestScanIsReproducible(t *testing.T) {
	render := func() string {
		b, err := json.Marshal(build(t))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if a, b := render(), render(); a != b {
		t.Error("duas varreduras da mesma base produziram grafos diferentes")
	}
}

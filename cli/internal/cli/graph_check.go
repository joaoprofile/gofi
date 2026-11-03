package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/graph"
	"github.com/joaoprofile/gofi/cli/internal/graph/diff"
	"github.com/joaoprofile/gofi/cli/internal/graph/model"
	"github.com/joaoprofile/gofi/cli/internal/graph/query"
	"github.com/joaoprofile/gofi/cli/internal/graph/rules"
	"github.com/joaoprofile/gofi/cli/internal/graph/workspace"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// projectGraph is the path of the project scope's graph, and the folder its
// paths are relative to; ok is false when the project has none.
func projectGraph(root string) (file, tree string, ok bool) {
	ix, err := workspace.LoadIndex(root)
	if err != nil {
		return "", "", false
	}
	info, ok := ix.Scope(workspace.ScopeProject)
	if !ok {
		return "", "", false
	}
	return filepath.Join(graph.Dir(root), filepath.FromSlash(info.Dir), graph.GraphFile), filepath.ToSlash(info.Root), true
}

// captureBaseline keeps the graph of the code a run starts from, for the
// graph stage to tell what the run did from what was there (0002, D11). A
// project with no graph yet has no baseline: the stage then checks nothing
// it cannot hold the run to.
func captureBaseline(work, run string) error {
	src, _, ok := projectGraph(work)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dst := filepath.Join(work, filepath.FromSlash(loop.BaselineDir(run)), graph.GraphFile)
	if _, err := os.Stat(dst); err == nil {
		return nil // a resumed run keeps the one it started from
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// graphCheck holds what the code became to the graph a change started from
// — the graph stage of a run, after the index is rebuilt: the change's
// diff, with what depends on it, to diffOut; the architecture rules it
// broke to report, as SARIF. A broken rule fails it.
func graphCheck(out io.Writer, cfg *config.GofiConfig, root, baseline, diffOut, report string) error {
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(root, filepath.FromSlash(p))
	}
	file, tree, ok := projectGraph(root)
	if !ok {
		return nil
	}
	now, err := model.Load(file)
	if err != nil {
		return err
	}
	old, err := model.Load(abs(baseline))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "  grafo: sem a baseline da execução — nada a comparar")
		return nil
	}
	if err != nil {
		return err
	}
	lp, err := pack.Load(backendLanguage(cfg))
	if err != nil {
		return err
	}
	var names []string
	if lp != nil {
		for _, l := range lp.Layers {
			names = append(names, l.Name)
		}
	}
	var lines []string
	if lp != nil {
		lines = append(lines, lp.Graph.Rules...)
	}
	lines = append(lines, cfg.Graph.ProjectRules()...)
	rs, err := rules.Parse(lines, names)
	if err != nil {
		return err
	}
	layerOf := func(f string) string { l, _ := lp.LayerOf(f); return l }
	d := diff.Of(old, now)
	violations := rules.Check(rs, d, now, layerOf)
	if diffOut != "" {
		if err := writeGraphDiff(abs(diffOut), d, old, now, lp, tree); err != nil {
			return err
		}
	}
	if report != "" {
		if err := writeGraphSARIF(abs(report), violations, now, tree); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "  grafo: %d arquivos mudaram, %d símbolos tocados, %d removidos, %d regras quebradas\n",
		len(d.Files), len(d.Touched()), len(d.Removed), len(violations))
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(out, "    %s:%d %s\n", path.Join(tree, v.File), v.Line, v.Message)
		}
		return fmt.Errorf("%d regras de arquitetura quebradas pela mudança", len(violations))
	}
	return nil
}

// writeGraphDiff writes the diff a run reads: what was touched, in which
// layer of the plan and context, and what depends on it.
func writeGraphDiff(file string, d *diff.Delta, old, now *model.Graph, lp *pack.Pack, tree string) error {
	gd := &loop.GraphDiff{Touched: []loop.Touch{}, Layers: []string{}, Contexts: []string{}, Reach: []string{}}
	layers, contexts := map[string]bool{}, map[string]bool{}
	contextOf := func(n *model.Node) string {
		if n.Context != "" {
			return n.Context
		}
		return lp.ContextOf(n.File)
	}
	for _, id := range d.Touched() {
		n := now.Get(id)
		_, plan := lp.LayerOf(n.File)
		t := loop.Touch{Symbol: model.ShortID(id, now.Module), File: path.Join(tree, n.File), Layer: plan, Context: contextOf(n)}
		gd.Touched = append(gd.Touched, t)
		layers[plan], contexts[t.Context] = true, true
	}
	for _, id := range d.Removed {
		n := old.Get(id)
		gd.Removed = append(gd.Removed, model.ShortID(id, old.Module))
		_, plan := lp.LayerOf(n.File)
		layers[plan], contexts[contextOf(n)] = true, true
	}
	for _, l := range role.Layers {
		if layers[l] {
			gd.Layers = append(gd.Layers, l)
		}
	}
	delete(contexts, "")
	gd.Contexts = slices.Sorted(maps.Keys(contexts))
	reach, tests := map[string]bool{}, map[string]bool{}
	deps := query.Impact(now, d.Touched(), 0, 0)
	gd.Dependents = len(deps)
	for _, dep := range deps {
		n := now.Get(dep.ID)
		if c := contextOf(n); c != "" {
			reach[c] = true
		}
		if lp.IsTest(n.File) {
			tests[path.Join(tree, n.File)] = true
		}
	}
	gd.Reach = append([]string{}, slices.Sorted(maps.Keys(reach))...)
	gd.Tests = slices.Sorted(maps.Keys(tests))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	gd.Schema = loop.DiffSchema
	data, err := json.MarshalIndent(gd, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// writeGraphSARIF writes the broken rules in SARIF, the format the sensor
// stages are read in, each placed at its symbol.
func writeGraphSARIF(file string, violations []rules.Violation, g *model.Graph, tree string) error {
	type logical struct {
		FullyQualifiedName string `json:"fullyQualifiedName"`
	}
	type location struct {
		Physical struct {
			Artifact struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
			Region struct {
				StartLine int `json:"startLine,omitempty"`
			} `json:"region"`
		} `json:"physicalLocation"`
		Logical []logical `json:"logicalLocations,omitempty"`
	}
	type result struct {
		RuleID  string `json:"ruleId"`
		Level   string `json:"level"`
		Message struct {
			Text string `json:"text"`
		} `json:"message"`
		Locations []location `json:"locations"`
	}
	results := []result{}
	for _, v := range violations {
		r := result{RuleID: v.Rule, Level: "error"}
		r.Message.Text = v.Message
		var loc location
		if v.File != "" {
			loc.Physical.Artifact.URI = path.Join(tree, v.File)
			loc.Physical.Region.StartLine = v.Line
		}
		loc.Logical = []logical{{FullyQualifiedName: model.ShortID(v.Symbol, g.Module)}}
		r.Locations = []location{loc}
		results = append(results, r)
	}
	doc := map[string]any{
		"version": "2.1.0",
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"runs":    []any{map[string]any{"tool": map[string]any{"driver": map[string]any{"name": "gofi-graph"}}, "results": results}},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, append(data, '\n'), 0o644)
}

// symbolAt names the symbol of the project's code graph at a line of a
// file, from the project root: what a finding placed by line is about.
// Empty when the project has no graph, or no symbol holds the line.
func symbolAt(root string) func(file string, line int) string {
	file, tree, ok := projectGraph(root)
	if !ok {
		return func(string, int) string { return "" }
	}
	g, err := model.Load(file)
	if err != nil {
		return func(string, int) string { return "" }
	}
	return func(f string, line int) string {
		rel := filepath.ToSlash(f)
		if tree != "" && tree != "." {
			r, err := filepath.Rel(tree, rel)
			if err != nil || r == ".." || len(r) > 2 && r[:3] == "../" {
				return ""
			}
			rel = filepath.ToSlash(r)
		}
		if id := query.At(g, rel, line); id != "" {
			return model.ShortID(id, g.Module)
		}
		return ""
	}
}

// planReach is the contexts that depend on the code some files — or
// folders, ending in / — hold, in the graph the run started from: what a
// plan's files reach before a line of it is written. nil when the run has
// no baseline.
func planReach(work, run, language string) func(files []string) []string {
	_, tree, ok := projectGraph(work)
	if !ok {
		return nil
	}
	g, err := model.Load(filepath.Join(work, filepath.FromSlash(loop.BaselineDir(run)), graph.GraphFile))
	if err != nil {
		return nil
	}
	lp, _ := pack.Load(language)
	return func(files []string) []string {
		var targets []string
		for _, f := range files {
			rel := strings.TrimPrefix(filepath.ToSlash(f), "./")
			if tree != "" && tree != "." {
				if !strings.HasPrefix(rel, tree+"/") {
					continue
				}
				rel = strings.TrimPrefix(rel, tree+"/")
			}
			for _, n := range g.Nodes {
				if n.External || n.Kind == model.KindPackage || n.File == "" {
					continue
				}
				if n.File == rel || strings.HasSuffix(rel, "/") && strings.HasPrefix(n.File, rel) {
					targets = append(targets, n.ID)
				}
			}
		}
		reach := map[string]bool{}
		for _, d := range query.Impact(g, targets, 0, 0) {
			n := g.Get(d.ID)
			c := n.Context
			if c == "" {
				c = lp.ContextOf(n.File)
			}
			if c != "" {
				reach[c] = true
			}
		}
		return slices.Sorted(maps.Keys(reach))
	}
}

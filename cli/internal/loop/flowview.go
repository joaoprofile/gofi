package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FlowView is the flow a run carries out, as whatever draws it reads it:
// written into the run's folder when it starts, so the picture is of the
// flow that ran — a project's own, or gofi's — not of one read later.
type FlowView struct {
	Schema string     `json:"schema"`
	Name   string     `json:"name"`
	Start  string     `json:"start"`
	Nodes  []ViewNode `json:"nodes"`
	Edges  []ViewEdge `json:"edges"`
}

// ViewNode is one node, with what it does.
type ViewNode struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Stage    string   `json:"stage,omitempty"`
	Stages   []string `json:"stages,omitempty"`
	Outcome  string   `json:"outcome,omitempty"`
	Optional bool     `json:"optional,omitempty"`
	Parallel bool     `json:"parallel,omitempty"`
}

// ViewEdge is one edge, its condition in the flow's own words.
type ViewEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	When string `json:"when,omitempty"`
	Max  int    `json:"max,omitempty"`
}

// FlowViewSchema names the format.
const FlowViewSchema = "gofi-flow-view/v1"

// View is the flow, drawable.
func (f *Flow) View() FlowView {
	v := FlowView{Schema: FlowViewSchema, Name: f.Name, Start: f.Start}
	names := make([]string, 0, len(f.Nodes))
	for n := range f.Nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		node := f.Nodes[n]
		v.Nodes = append(v.Nodes, ViewNode{Name: n, Kind: node.Kind, Stage: node.Stage, Stages: node.Stages, Outcome: node.Outcome, Optional: node.Optional, Parallel: node.Parallel})
	}
	for _, e := range f.Edges {
		ve := ViewEdge{From: e.From, To: e.To, Max: f.limit(e.From, e.To)}
		if !e.When.Empty() {
			ve.When = condition(e.When)
		}
		v.Edges = append(v.Edges, ve)
	}
	return v
}

// condition is a When in the flow's own words, on one line: the YAML it
// was written in, in flow style.
func condition(w When) string {
	var n yaml.Node
	if n.Encode(w) != nil {
		return ""
	}
	n.Style = yaml.FlowStyle
	b, err := yaml.Marshal(&n)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(string(b)), "{"), "}")
}

// FlowFile is where a run keeps the flow it carries out.
func FlowFile(root, run string) string { return filepath.Join(Dir(root, run), "flow.json") }

func (r *Runner) saveFlow(run string) {
	data, err := json.MarshalIndent(r.Flow.View(), "", "  ")
	if err == nil {
		_ = os.WriteFile(FlowFile(r.Root, run), append(data, '\n'), 0o644)
	}
}

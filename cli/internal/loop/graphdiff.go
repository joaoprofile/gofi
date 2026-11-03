package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	slash "path"
	"path/filepath"
	"slices"
)

// DiffSchema names the format of a run's graph diff.
const DiffSchema = "gofi-graph-diff/v1"

// GraphDiff is what the graph stage says the change did to the code, against
// the graph the run started from (0002, D11): the symbols it touched, by
// layer and context, and what depends on them. The run dispatches the audit
// by it, sizes the tier by it and gives the audit its scope; a model never
// says it.
type GraphDiff struct {
	Schema  string   `json:"schema"`
	Touched []Touch  `json:"touched"`
	Removed []string `json:"removed,omitempty"`
	// Layers are the plan's layers the touched symbols are in; Contexts
	// the contexts.
	Layers   []string `json:"layers"`
	Contexts []string `json:"contexts"`
	// Reach are the contexts of the symbols that depend on what was
	// touched, Dependents how many there are, Tests the test files among
	// them.
	Reach      []string `json:"reach"`
	Dependents int      `json:"dependents"`
	Tests      []string `json:"tests,omitempty"`
}

// Touch is one symbol the change added or changed.
type Touch struct {
	Symbol  string `json:"symbol"`
	File    string `json:"file,omitempty"`
	Layer   string `json:"layer,omitempty"`
	Context string `json:"context,omitempty"`
}

// DiffFile is where the graph stage of a run writes its diff, relative to
// the tree the run works in.
func DiffFile(run string) string { return slash.Join(RunsDir, run, "diff.json") }

// BaselineDir is where a run keeps the code graph it started from.
func BaselineDir(run string) string { return slash.Join(RunsDir, run, "baseline") }

// GraphReport is where the graph stage of a run writes what it refused.
func GraphReport(run string) string { return slash.Join(RunsDir, run, "graph.sarif") }

// ReadDiff reads a run's graph diff; nil, with no error, when the graph
// stage wrote none.
func ReadDiff(dir, run string) (*GraphDiff, error) {
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(DiffFile(run))))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var d GraphDiff
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", DiffFile(run), err)
	}
	if d.Schema != DiffSchema {
		return nil, fmt.Errorf("%s: schema %q, expected %s", DiffFile(run), d.Schema, DiffSchema)
	}
	return &d, nil
}

// WriteDiff writes a run's graph diff.
func WriteDiff(dir, run string, d *GraphDiff) error {
	d.Schema = DiffSchema
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(dir, filepath.FromSlash(DiffFile(run)))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

// outside are the contexts in reach that are not the change's own.
func outside(reach []string, own string) []string {
	var out []string
	for _, c := range reach {
		if c != own && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}

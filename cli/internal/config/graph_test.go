package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every project written before the graph existed has no `graph:` block, and
// those projects must still get a graph. Absence is the default, not "off".
func TestGraphDefaultsToOn(t *testing.T) {
	var g *Graph
	if !g.On() || !g.HooksOn() {
		t.Error("an absent graph block turned the graph off")
	}
	if g.UseDeep() || g.Excludes() != nil {
		t.Error("an absent graph block carried settings")
	}

	empty := &Graph{}
	if !empty.On() || !empty.HooksOn() {
		t.Error("an empty graph block turned the graph off")
	}
}

func TestGraphOptOut(t *testing.T) {
	no := false
	// Turning the graph off must take its hooks with it: a hook rebuilding
	// something the project does not want is worse than no hook.
	if off := (&Graph{Enabled: &no}); off.On() || off.HooksOn() {
		t.Error("enabled:false left the graph or its hooks on")
	}
	if g := (&Graph{Hooks: &no}); !g.On() || g.HooksOn() {
		t.Error("hooks:false should keep the graph and drop only the hooks")
	}
}

// The block is optional in both directions: a config that never mentions it
// must not grow an empty `graph:` key on the next save.
func TestGraphIsOmittedWhenUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Save(path, validConfig()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "graph:") {
		t.Errorf("an unset graph block was written:\n%s", b)
	}
}

func TestGraphRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	cfg := validConfig()
	no := false
	cfg.Graph = &Graph{Hooks: &no, Deep: true, Exclude: []string{"testdata"}}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Graph.HooksOn() || !got.Graph.UseDeep() {
		t.Errorf("graph block did not survive: %+v", got.Graph)
	}
	if len(got.Graph.Excludes()) != 1 || got.Graph.Excludes()[0] != "testdata" {
		t.Errorf("excludes = %v", got.Graph.Excludes())
	}
}

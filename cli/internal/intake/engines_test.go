package intake

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The index stays loaded between requests, and is loaded again when a
// document it indexes changes.
func TestEnginesKeepTheIndexUntilItChanges(t *testing.T) {
	root := project(t)
	c := &Engines{}
	a, err := c.Open(root, "go")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := c.Open(root, "go"); b != a {
		t.Error("the index was loaded again with nothing changed")
	}
	spec := filepath.Join(root, "specs", "pricing", "sdd-pricing.md")
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(spec, later, later); err != nil {
		t.Fatal(err)
	}
	if b, _ := c.Open(root, "go"); b == a {
		t.Error("an edited document kept the old index")
	}
	// The same plan either way.
	r1 := runIntake(t, root, "altere o fluxo de pricing, adicionando um rate limit", Options{})
	r2 := runIntake(t, root, "altere o fluxo de pricing, adicionando um rate limit", Options{Engines: c})
	if r1.Context.Name != r2.Context.Name || len(r1.Plan) != len(r2.Plan) || len(r1.Evidence) != len(r2.Evidence) {
		t.Errorf("a kept index planned differently: %v vs %v", roles(r1), roles(r2))
	}
}

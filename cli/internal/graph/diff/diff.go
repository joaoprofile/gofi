// Package diff compares the code graph a change started from with the graph
// of the code it left (0002, D11): which symbols it added, changed and
// removed, the edges it made, the cycles it closed. A rule is held to what
// a change did, not to what the code already was.
package diff

import (
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/graph/model"
)

// Delta is what changed between two graphs of the same tree.
type Delta struct {
	// Files are the files added, edited or removed.
	Files []string
	// Added and Changed are symbols of the new graph — Changed those whose
	// file was edited —, Removed of the old one.
	Added, Changed, Removed []string
	// Edges are the edges of the new graph the old one did not have.
	Edges []*model.Edge
	// Cycles are the call cycles of the new graph that were not in the old.
	Cycles [][]string
}

// Of compares old with new. A nil old is a tree with no graph yet: all of
// new is added.
func Of(old, new *model.Graph) *Delta {
	if old == nil {
		old = model.New(new.Module, "", new.Mode)
		old.Index()
	}
	d := &Delta{}
	files := map[string]bool{}
	for p, f := range new.Files {
		if o, ok := old.Files[p]; !ok || o.Hash != f.Hash {
			files[p] = true
		}
	}
	for p := range old.Files {
		if _, ok := new.Files[p]; !ok {
			files[p] = true
		}
	}
	d.Files = sorted(files)
	for _, n := range new.Nodes {
		if n.External || n.Kind == model.KindPackage {
			continue
		}
		switch o := old.Get(n.ID); {
		case o == nil:
			d.Added = append(d.Added, n.ID)
		case files[n.File] || o.Sig != n.Sig || o.Lines != n.Lines:
			d.Changed = append(d.Changed, n.ID)
		}
	}
	for _, n := range old.Nodes {
		if !n.External && n.Kind != model.KindPackage && new.Get(n.ID) == nil {
			d.Removed = append(d.Removed, n.ID)
		}
	}
	seen := map[string]bool{}
	for _, e := range old.Edges {
		seen[e.Key()] = true
	}
	for _, e := range new.Edges {
		if !seen[e.Key()] {
			d.Edges = append(d.Edges, e)
		}
	}
	was := map[string]bool{}
	for _, c := range old.Cycles {
		was[cycleKey(c)] = true
	}
	for _, c := range new.Cycles {
		if !was[cycleKey(c)] {
			d.Cycles = append(d.Cycles, c)
		}
	}
	slices.Sort(d.Added)
	slices.Sort(d.Changed)
	slices.Sort(d.Removed)
	return d
}

// Touched are the symbols of the new graph the change added or changed.
func (d *Delta) Touched() []string {
	out := append(append([]string{}, d.Added...), d.Changed...)
	slices.Sort(out)
	return out
}

// Empty reports whether nothing changed.
func (d *Delta) Empty() bool {
	return len(d.Files) == 0 && len(d.Added) == 0 && len(d.Changed) == 0 && len(d.Removed) == 0 && len(d.Edges) == 0
}

func cycleKey(c []string) string {
	s := append([]string{}, c...)
	slices.Sort(s)
	return strings.Join(s, "\x00")
}

func sorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

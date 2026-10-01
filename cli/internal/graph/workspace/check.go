package workspace

import (
	"os"
	"path/filepath"

	"github.com/gofi-labs/gofi/cli/internal/graph"
	"github.com/gofi-labs/gofi/cli/internal/graph/model"
)

// Freshness is how a scope on disk compares with its tree.
type Freshness string

const (
	Fresh   Freshness = "fresh"   // the graph matches the files it was built from
	Stale   Freshness = "stale"   // a file was added, removed or edited since
	Unknown Freshness = "unknown" // the extractor cannot tell without running
	Missing Freshness = "missing" // declared, but no graph was ever built
	Orphan  Freshness = "orphan"  // built, but its tree is gone
)

// ScopeState is one scope as a status check sees it.
type ScopeState struct {
	ScopeInfo
	State Freshness `json:"state"`
}

// Check compares every scope with its tree, without building anything. The
// scopes are the ones the index lists plus the ones the project declares and
// never built, so a surface added to .gofi.yaml shows up as missing rather
// than not at all. Scan options come from opt and must be the ones the build
// used, or every scope reads as stale.
func Check(opt Options) ([]ScopeState, error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return nil, err
	}
	ix, err := LoadIndex(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var out []ScopeState
	if ix != nil {
		for _, s := range ix.Scopes {
			out = append(out, ScopeState{ScopeInfo: s, State: freshness(root, s, opt)})
		}
	}
	for _, sc := range Scopes(opt) {
		if ix.hasScope(sc.Name) {
			continue
		}
		out = append(out, ScopeState{
			ScopeInfo: ScopeInfo{Name: sc.Name, Root: relSlash(root, sc.Root), Language: sc.Language, Framework: sc.Framework},
			State:     Missing,
		})
	}
	return out, nil
}

func freshness(root string, s ScopeInfo, opt Options) Freshness {
	g, err := model.Load(filepath.Join(graph.Dir(root), filepath.FromSlash(s.Dir), graph.GraphFile))
	if err != nil {
		return Missing
	}
	tree := filepath.Join(root, filepath.FromSlash(s.Root))
	if _, err := os.Stat(tree); err != nil {
		return Orphan
	}
	fresh, known := graph.Fresh(g, tree, graph.BuildOptions{
		ProjectRoot: root,
		Language:    s.Language,
		WithTests:   opt.WithTests,
		Exclude:     opt.Exclude,
		MaxFileKB:   opt.MaxFileKB,
	})
	switch {
	case !known:
		return Unknown
	case fresh:
		return Fresh
	}
	return Stale
}

func (ix *Index) hasScope(name string) bool {
	_, ok := ix.Scope(name)
	return ok
}

package workspace

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/joaoprofile/gofi/cli/internal/graph"
	"github.com/joaoprofile/gofi/cli/internal/graph/model"
)

// Workspace reads back the graphs a project has. Scopes are opened on demand:
// answering "where is this symbol declared" usually only needs the project
// graph, and the SDK graph is the larger of the two.
type Workspace struct {
	Index *Index

	root string
	lang string

	mu     sync.Mutex
	graphs map[string]*model.Graph
}

// Load opens the graphs of a project. A project built by a single-tree scan
// rather than by a workspace build has no index; it is read as the single scope
// it is, so a caller never has to know which command produced the graph.
func Load(projectRoot, language string) *Workspace {
	lang := (graph.BuildOptions{Language: language}).Lang()
	ix, err := LoadIndex(projectRoot)
	if err != nil {
		ix = &Index{
			Schema:   model.SchemaVersion,
			Tool:     model.Tool,
			Language: lang,
			Scopes:   []ScopeInfo{{Name: ScopeProject, Dir: ".", Language: lang}},
		}
	}
	return &Workspace{
		Index:  ix,
		root:   projectRoot,
		lang:   lang,
		graphs: map[string]*model.Graph{},
	}
}

// Graph returns the graph of one scope, reading it from disk the first time.
func (w *Workspace) Graph(scope string) (*model.Graph, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.graph(scope)
}

func (w *Workspace) graph(scope string) (*model.Graph, error) {
	if g, ok := w.graphs[scope]; ok {
		return g, nil
	}
	info, ok := w.Index.Scope(scope)
	if !ok {
		return nil, fmt.Errorf("escopo %q nao existe neste projeto", scope)
	}
	path := filepath.Join(graph.Dir(w.root), filepath.FromSlash(info.Dir), graph.GraphFile)
	g, err := model.Load(path)
	if err != nil {
		return nil, fmt.Errorf("escopo %s: %w", scope, err)
	}
	w.graphs[scope] = g
	return g, nil
}

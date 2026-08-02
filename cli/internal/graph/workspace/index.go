package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// IndexFile lists the scopes a project has. It is the entry point for a reader
// that does not yet know how many graphs there are or where they live.
const IndexFile = "gofi_graph_index.json"

// Index is the contents of IndexFile.
type Index struct {
	Schema   string      `json:"schema"`
	Tool     string      `json:"tool"`
	Language string      `json:"language"`
	Scopes   []ScopeInfo `json:"scopes"`
}

// ScopeInfo is one scope as recorded in the index.
type ScopeInfo struct {
	Name string `json:"name"`
	// Dir is relative to the directory holding the index, so the file survives
	// the project being moved or checked out somewhere else.
	Dir      string `json:"dir"`
	Root     string `json:"root"` // relative to the project root
	Language string `json:"language"`
	// Framework is what .gofi.yaml declared for this scope — react,
	// react-native. Empty for a scope that declared none.
	Framework string `json:"framework,omitempty"`
	Key       string `json:"key,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Nodes     int    `json:"nodes"`
	Edges     int    `json:"edges"`
	Files     int    `json:"files"`
}

// Scope returns the recorded entry for a scope name.
func (ix *Index) Scope(name string) (ScopeInfo, bool) {
	if ix == nil {
		return ScopeInfo{}, false
	}
	for _, s := range ix.Scopes {
		if s.Name == name {
			return s, true
		}
	}
	return ScopeInfo{}, false
}

func newIndex(opt Options, results []ScopeResult, prev *Index) *Index {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		root = opt.Root
	}
	lang := (graph.BuildOptions{Language: opt.Language}).Lang()
	base := graph.Dir(root, lang)

	ix := &Index{Schema: model.SchemaVersion, Tool: model.Tool, Language: lang}
	for _, r := range results {
		if r.Err != nil || r.Result == nil || r.Result.Graph == nil {
			continue
		}
		g := r.Result.Graph
		ix.Scopes = append(ix.Scopes, ScopeInfo{
			Name:      r.Scope.Name,
			Dir:       relSlash(base, r.Scope.Dir),
			Root:      relSlash(root, r.Scope.Root),
			Language:  r.Scope.Language,
			Framework: r.Scope.Framework,
			Key:       r.Scope.Key,
			Mode:      g.Mode,
			Nodes:     g.Stats.Nodes,
			Edges:     g.Stats.Edges,
			Files:     g.Stats.Files,
		})
	}

	// A scope can be absent from this run and still be readable: the graphs are
	// committed, the sources they were built from are not always. A fresh clone
	// has no .gofi/gofi-sdk-<lang>/ checkout, so the SDK scope never gets built,
	// but its graph came down with the repository and readers should still find
	// it. Dropping the entry would both hide it and rewrite a tracked file on
	// every clone.
	for _, s := range prev.scopesNotIn(ix.Scopes) {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(s.Dir), graph.GraphFile)); err != nil {
			continue
		}
		ix.Scopes = append(ix.Scopes, s)
	}
	return ix
}

// scopesNotIn returns the recorded scopes that have no counterpart in have.
func (ix *Index) scopesNotIn(have []ScopeInfo) []ScopeInfo {
	if ix == nil {
		return nil
	}
	var out []ScopeInfo
	for _, s := range ix.Scopes {
		if !slices.ContainsFunc(have, func(h ScopeInfo) bool { return h.Name == s.Name }) {
			out = append(out, s)
		}
	}
	return out
}

// relSlash expresses target relative to base in slash form, falling back to the
// absolute path when the two share no ancestor.
func relSlash(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

// IndexPath is where the index of a project lives.
func IndexPath(projectRoot, language string) string {
	return filepath.Join(graph.Dir(projectRoot, language), IndexFile)
}

// Save writes the index next to the graphs it describes.
func (ix *Index) Save(projectRoot, language string) error {
	path := IndexPath(projectRoot, language)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(ix, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// LoadIndex reads the index of a project. A missing index is not an error to
// the caller that only wants to know what was built last time.
func LoadIndex(projectRoot, language string) (*Index, error) {
	b, err := os.ReadFile(IndexPath(projectRoot, language))
	if err != nil {
		return nil, err
	}
	var ix Index
	if err := json.Unmarshal(b, &ix); err != nil {
		return nil, err
	}
	return &ix, nil
}

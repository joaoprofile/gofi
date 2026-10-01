package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofi-labs/gofi/cli/internal/docs"
	"github.com/gofi-labs/gofi/cli/internal/layout"
	"github.com/gofi-labs/gofi/cli/internal/retrieval"
)

// Engines keeps the search index loaded between requests, for a process that
// plans many of them — `gofi intake --serve`, which the editor keeps open.
// Loading it is most of what planning costs (measured: ~410 ms of ~400–800),
// and it is reloaded only when something it was read from changed: the index
// files, the documents they index, or the project's lexicon.
type Engines struct {
	mu    sync.Mutex
	cache map[string]cachedEngine
}

type cachedEngine struct {
	sig    string
	engine *retrieval.Engine
}

// Open returns the loaded index of a project, loading it again when its
// sources changed.
func (c *Engines) Open(root, language string) (*retrieval.Engine, error) {
	sig := signature(root)
	key := root + "\x00" + language
	c.mu.Lock()
	defer c.mu.Unlock()
	if got, ok := c.cache[key]; ok && got.sig == sig {
		return got.engine, nil
	}
	e, err := retrieval.Open(root, language)
	if err != nil {
		return nil, err
	}
	if c.cache == nil {
		c.cache = map[string]cachedEngine{}
	}
	c.cache[key] = cachedEngine{sig: sig, engine: e}
	return e, nil
}

// signature sums what the index is read from — each file's size and time of
// change — so any edit, rebuild or new synonym is a different signature.
func signature(root string) string {
	h := sha256.New()
	stamp := func(path string) {
		if st, err := os.Stat(path); err == nil {
			fmt.Fprintf(h, "%s %d %d\n", path, st.Size(), st.ModTime().UnixNano())
		}
	}
	walk := func(dir string) {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				stamp(path)
			}
			return nil
		})
	}
	walk(filepath.Join(root, ".gofi", "index"))
	walk(layout.Lexicon().Abs(root))
	if idx, _, _ := docs.Load(root); idx != nil {
		for _, d := range idx.Docs {
			stamp(filepath.Join(root, filepath.FromSlash(d.Path)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

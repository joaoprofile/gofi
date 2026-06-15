package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Cache locates and prepares the on-disk cache for fetched sources. From
// v2.4 onwards the cache is per-project (rooted at <projectRoot>/.gofi/cache/)
// instead of per-user. $GOFI_CACHE_DIR can be set to point every project at
// the same shared directory if the user prefers.
type Cache struct {
	Root string
}

// ProjectCache returns the cache rooted at <projectRoot>/.gofi/cache, unless
// $GOFI_CACHE_DIR overrides the location.
func ProjectCache(projectRoot string) (*Cache, error) {
	if root := os.Getenv("GOFI_CACHE_DIR"); root != "" {
		return &Cache{Root: root}, nil
	}
	if projectRoot == "" {
		return nil, errors.New("project root required (or set $GOFI_CACHE_DIR)")
	}
	return &Cache{Root: filepath.Join(projectRoot, ".gofi", "cache")}, nil
}

// SourcePath returns the directory that holds the extracted contents for the
// given (resolved) Ref. Layout: <Root>/sources/<host>/<owner>/<repo>/<sha>/
func (c *Cache) SourcePath(r Ref) string {
	return filepath.Join(c.Root, "sources", r.Host, r.Owner, r.Repo, r.Ref)
}

// EnsureRoot creates the cache root directory if it does not exist.
func (c *Cache) EnsureRoot() error {
	if c.Root == "" {
		return fmt.Errorf("cache root is empty")
	}
	return os.MkdirAll(c.Root, 0o755)
}

// HasSource reports whether the cache already contains the resolved Ref.
func (c *Cache) HasSource(r Ref) bool {
	info, err := os.Stat(c.SourcePath(r))
	return err == nil && info.IsDir()
}

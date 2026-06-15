package sources

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client fetches sources from GitHub. The zero value works against
// api.github.com without authentication; production use should set Token.
type Client struct {
	BaseURL   string       // default: https://api.github.com
	Token     string       // default: $GOFI_GITHUB_TOKEN, then $GITHUB_TOKEN
	HTTP      *http.Client // default: 30s timeout
	UserAgent string       // default: gofi-cli
	Cache     *Cache       // required
}

// NewClient constructs a Client with sensible defaults. Cache is required.
func NewClient(cache *Cache) (*Client, error) {
	if cache == nil {
		return nil, errors.New("cache is required")
	}
	token := os.Getenv("GOFI_GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Client{
		BaseURL:   "https://api.github.com",
		Token:     token,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: "gofi-cli",
		Cache:     cache,
	}, nil
}

// Resolve looks up the commit SHA for r.Ref and returns a new Ref with the SHA
// substituted in. Tags, branches and SHAs all work.
func (c *Client) Resolve(r Ref) (Ref, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.BaseURL, r.Owner, r.Repo, r.Ref)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Ref{}, err
	}
	c.applyHeaders(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Ref{}, fmt.Errorf("github resolve %s: %w", r, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return Ref{}, fmt.Errorf("github resolve %s: HTTP %d: %s", r, resp.StatusCode, string(body))
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Ref{}, fmt.Errorf("github resolve %s: decode: %w", r, err)
	}
	if payload.SHA == "" {
		return Ref{}, fmt.Errorf("github resolve %s: empty sha", r)
	}
	return r.WithRef(payload.SHA), nil
}

// FetchTarball ensures r (already resolved to a SHA) is available in the
// cache, downloading and extracting the GitHub tarball if necessary. Returns
// the absolute path of the extracted directory.
func (c *Client) FetchTarball(resolved Ref) (string, error) {
	dest := c.Cache.SourcePath(resolved)
	if c.Cache.HasSource(resolved) {
		return dest, nil
	}
	if err := c.Cache.EnsureRoot(); err != nil {
		return "", err
	}
	if err := c.FetchTarballTo(resolved, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// FetchTarballTo downloads and extracts the GitHub tarball for resolved into
// dest unconditionally. Any pre-existing content at dest is removed first so
// callers can use this to refresh a flat checkout (e.g. .gofi/gofi-sdk-<lang>/)
// without dealing with leftover files from the previous SHA. Extraction goes
// through a sibling .tmp dir and renames into place for atomicity.
func (c *Client) FetchTarballTo(resolved Ref, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	url := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", c.BaseURL, resolved.Owner, resolved.Repo, resolved.Ref)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	c.applyHeaders(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("github tarball %s: %w", resolved, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("github tarball %s: HTTP %d: %s", resolved, resp.StatusCode, string(body))
	}

	tmpDir := dest + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	if err := extractTarGz(resp.Body, tmpDir, true); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("extract tarball: %w", err)
	}
	_ = os.RemoveAll(dest)
	if err := os.Rename(tmpDir, dest); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("install %s: %w", dest, err)
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.UserAgent)
}

// extractTarGz reads a gzip+tar stream and writes regular files and
// directories under dest. When stripFirst is true, the first path component of
// every entry is dropped (GitHub tarballs always start with a single top-level
// directory like "owner-repo-sha7/").
func extractTarGz(src io.Reader, dest string, stripFirst bool) error {
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := h.Name
		if stripFirst {
			parts := strings.SplitN(filepath.ToSlash(name), "/", 2)
			if len(parts) < 2 {
				continue // top-level only — the wrapping dir
			}
			name = parts[1]
		}
		if name == "" {
			continue
		}
		// guard against zip-slip
		target := filepath.Join(dest, name)
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("tar entry escapes destination: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeXGlobalHeader:
			// skip — not needed for source distribution
		}
	}
}

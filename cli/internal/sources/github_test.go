package sources

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGitHub spins up an httptest.Server that mimics the two GitHub endpoints
// the Client uses: /repos/.../commits/<ref> and /repos/.../tarball/<sha>.
func fakeGitHub(t *testing.T, sha string, files map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/commits/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"sha":"` + sha + `"}`))
		case strings.Contains(r.URL.Path, "/tarball/"):
			tarball := buildTarGz(t, "owner-repo-"+sha[:7], files)
			w.Header().Set("Content-Type", "application/x-gzip")
			_, _ = w.Write(tarball)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func buildTarGz(t *testing.T, topDir string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	// top-level directory entry
	if err := tw.WriteHeader(&tar.Header{Name: topDir + "/", Typeflag: tar.TypeDir, Mode: 0755}); err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		full := topDir + "/" + path
		if err := tw.WriteHeader(&tar.Header{
			Name:     full,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestClient_Resolve(t *testing.T) {
	srv := fakeGitHub(t, "1234567890abcdef1234567890abcdef12345678", nil)
	defer srv.Close()

	c, err := NewClient(&Cache{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	r, err := Parse("github.com/owner/repo@v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := c.Resolve(r)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Ref != "1234567890abcdef1234567890abcdef12345678" {
		t.Errorf("unexpected sha: %s", resolved.Ref)
	}
}

func TestClient_FetchTarball(t *testing.T) {
	files := map[string]string{
		".claude/CLAUDE.md":               "# CLAUDE\n",
		".claude/commands/gofi-spec.md":   "skill content\n",
		".claude/knowledge/shared/glo.md": "glossary\n",
	}
	sha := "1234567890abcdef1234567890abcdef12345678"
	srv := fakeGitHub(t, sha, files)
	defer srv.Close()

	c, err := NewClient(&Cache{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	r, err := Parse("github.com/owner/repo@" + sha)
	if err != nil {
		t.Fatal(err)
	}
	dest, err := c.FetchTarball(r)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// files must exist with no top-level dir
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(dest, p))
		if err != nil {
			t.Errorf("missing %s: %v", p, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: got %q want %q", p, got, want)
		}
	}

	// second call hits cache (no extra HTTP)
	dest2, err := c.FetchTarball(r)
	if err != nil {
		t.Fatalf("fetch (cache): %v", err)
	}
	if dest2 != dest {
		t.Errorf("cache miss on second fetch")
	}
}

func TestClient_Resolve_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := NewClient(&Cache{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL
	r, _ := Parse("github.com/owner/repo@nonexistent")
	if _, err := c.Resolve(r); err == nil {
		t.Fatal("expected error on 404")
	}
}

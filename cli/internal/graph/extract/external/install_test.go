package external

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func source(t *testing.T, body string) (path, sum string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "build-output")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256([]byte(body))
	return path, hex.EncodeToString(h[:])
}

func TestInstallFromFile(t *testing.T) {
	root := t.TempDir()
	from, sum := source(t, "#!/bin/sh\nexit 0\n")

	got, err := Install(t.Context(), InstallOptions{ProjectRoot: root, Language: "Java", From: from, SHA256: sum})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	want := filepath.Join(root, ExtractorsDir, BinaryName("java"))
	if got.Path != want {
		t.Errorf("path = %q, want %q", got.Path, want)
	}
	if got.SHA256 != sum {
		t.Errorf("sha256 = %q, want %q", got.SHA256, sum)
	}
	// The point of installing is that Find can then locate it.
	t.Setenv("PATH", t.TempDir())
	spec, err := Find(root, "java")
	if err != nil || spec.Path != want {
		t.Errorf("Find after install: %+v, %v", spec, err)
	}
}

func TestInstallOverwrites(t *testing.T) {
	root := t.TempDir()
	first, _ := source(t, "v1")
	if _, err := Install(t.Context(), InstallOptions{ProjectRoot: root, Language: "rust", From: first}); err != nil {
		t.Fatal(err)
	}
	second, sum := source(t, "v2")
	got, err := Install(t.Context(), InstallOptions{ProjectRoot: root, Language: "rust", From: second})
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if got.SHA256 != sum {
		t.Errorf("the upgrade did not replace the old binary")
	}
}

func TestInstallFromURL(t *testing.T) {
	body := "downloaded extractor"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	h := sha256.Sum256([]byte(body))
	root := t.TempDir()
	got, err := Install(t.Context(), InstallOptions{
		ProjectRoot: root, Language: "rust", From: srv.URL,
		SHA256: hex.EncodeToString(h[:]), Client: srv.Client(),
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	b, err := os.ReadFile(got.Path)
	if err != nil || string(b) != body {
		t.Errorf("content = %q (%v)", b, err)
	}
}

// A failed install must leave nothing behind: a partial file would be executed
// on the next build as if it were a working extractor.
func TestInstallLeavesNothingOnFailure(t *testing.T) {
	root := t.TempDir()
	from, _ := source(t, "tampered")

	_, err := Install(t.Context(), InstallOptions{
		ProjectRoot: root, Language: "java", From: from,
		SHA256: strings.Repeat("ab", 32),
	})
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("err = %v, want a digest mismatch", err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, ExtractorsDir))
	if len(entries) != 0 {
		t.Errorf("left %d files behind", len(entries))
	}
}

func TestInstallRejects(t *testing.T) {
	from, _ := source(t, "x")
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		opt  InstallOptions
		want string
	}{
		{"no language", InstallOptions{From: from}, "linguagem"},
		// A language name is used to build a path, so it must not be able to
		// steer the write out of the extractors directory.
		{"traversal", InstallOptions{Language: "../../etc/cron.d/x", From: from}, "linguagem invalida"},
		{"separator", InstallOptions{Language: "ja/va", From: from}, "linguagem invalida"},
		{"no source", InstallOptions{Language: "java"}, "--from"},
		{"plain http", InstallOptions{Language: "java", From: "http://example.com/x"}, "https"},
		{"missing file", InstallOptions{Language: "java", From: filepath.Join(t.TempDir(), "nope")}, "no such file"},
		{"empty source", InstallOptions{Language: "java", From: empty}, "vazia"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opt.ProjectRoot = t.TempDir()
			_, err := Install(t.Context(), tt.opt)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestInstallReportsHTTPStatus(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Install(t.Context(), InstallOptions{
		ProjectRoot: t.TempDir(), Language: "java", From: srv.URL, Client: srv.Client(),
	})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want the status to be reported", err)
	}
}

func TestUninstall(t *testing.T) {
	root := t.TempDir()
	from, _ := source(t, "x")
	got, err := Install(t.Context(), InstallOptions{ProjectRoot: root, Language: "java", From: from})
	if err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(root, "java"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(got.Path); !os.IsNotExist(err) {
		t.Errorf("extractor still there: %v", err)
	}
	if err := Uninstall(root, "java"); err == nil || !strings.Contains(err.Error(), "nenhum extractor") {
		t.Errorf("second Uninstall: err = %v", err)
	}
}

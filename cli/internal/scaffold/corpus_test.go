package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedCorpusIndex(t *testing.T) {
	root := t.TempDir()

	for _, corpus := range []string{"specs", "prd"} {
		if err := SeedCorpusIndex(root, corpus); err != nil {
			t.Fatalf("SeedCorpusIndex(%s): %v", corpus, err)
		}
		got, err := os.ReadFile(filepath.Join(root, corpus, "INDEX.md"))
		if err != nil {
			t.Fatalf("read %s/INDEX.md: %v", corpus, err)
		}
		body := string(got)
		if !strings.Contains(body, "manifesto de retrieval (RAG)") {
			t.Errorf("%s INDEX missing RAG header:\n%s", corpus, body)
		}
		if !strings.Contains(body, "gen-index.sh "+corpus) {
			t.Errorf("%s INDEX missing regen command", corpus)
		}
	}
}

func TestSeedCorpusIndexPreservesExisting(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(root, "specs", "INDEX.md")
	if err := os.WriteFile(real, []byte("real index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SeedCorpusIndex(root, "specs"); err != nil {
		t.Fatalf("SeedCorpusIndex: %v", err)
	}
	got, _ := os.ReadFile(real)
	if string(got) != "real index" {
		t.Errorf("seed overwrote a populated index: %q", got)
	}
}

func TestSeedCorpusIndexRejectsUnknown(t *testing.T) {
	if err := SeedCorpusIndex(t.TempDir(), "notes"); err == nil {
		t.Error("expected error for unknown corpus")
	}
}

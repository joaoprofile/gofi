package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestInstallInstitutionalMirrorFullReplace(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, ".claude", "institutional", "winnerbox")

	// Pre-existing local content that must NOT survive a full-replace mirror.
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(dest, "stale-local.md")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Multi-product repo: this product's folder lives under winnerbox/.
	repo := fstest.MapFS{
		"winnerbox/INDEX.md":  {Data: []byte("# INDEX winnerbox")},
		"winnerbox/domain.md": {Data: []byte("# domain")},
		"otherprod/INDEX.md":  {Data: []byte("# other")},
	}

	created, err := InstallInstitutionalMirror(repo, "winnerbox", root, "winnerbox", TemplateData{ProjectName: "winnerbox"})
	if err != nil {
		t.Fatalf("mirror: %v", err)
	}
	if len(created) != 2 {
		t.Errorf("expected 2 files copied, got %d", len(created))
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale local file survived the full-replace mirror")
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "INDEX.md")); string(b) != "# INDEX winnerbox" {
		t.Errorf("INDEX.md not mirrored: %q", b)
	}
	// The other product's folder must not leak in.
	if _, err := os.Stat(filepath.Join(dest, "otherprod")); !os.IsNotExist(err) {
		t.Error("otherprod leaked into the mirror")
	}
}

func TestInstallInstitutionalMirrorMissingSubdir(t *testing.T) {
	repo := fstest.MapFS{"otherprod/INDEX.md": {Data: []byte("x")}}
	_, err := InstallInstitutionalMirror(repo, "winnerbox", t.TempDir(), "winnerbox", TemplateData{})
	if !errors.Is(err, ErrNoInstitutionalSubdir) {
		t.Errorf("expected ErrNoInstitutionalSubdir, got %v", err)
	}
}

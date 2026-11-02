package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

// What a generator rewrites after an install is recorded as installed, so the
// next update does not take it for the team's edit.
func TestAGeneratedFileIsNotTakenForAnEdit(t *testing.T) {
	root := t.TempDir()
	rel := ".claude/sdk/go/api/examples.md"
	p := filepath.Join(root, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	// The install pass records the shipped file…
	if err := (Manifest{rel: hashBytes([]byte("shipped"))}).merge(root); err != nil {
		t.Fatal(err)
	}
	// …and the generator replaces it right after.
	_ = os.WriteFile(p, []byte("generated"), 0o644)
	pr := &preserver{base: LoadManifest(root)}
	if !pr.keeps(rel, []byte("generated")) {
		t.Fatal("precondition: the stale record reads as an edit")
	}
	if err := RecordGenerated(root, ".claude/sdk/go/api"); err != nil {
		t.Fatal(err)
	}
	pr = &preserver{base: LoadManifest(root)}
	if pr.keeps(rel, []byte("generated")) {
		t.Error("the generated file is still taken for an edit")
	}
}

package layout

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func present(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// Outside Go the old layout kept the project graph one level down, under the
// language's name, and that is the directory that becomes the code index.
func TestMigrateTakesTheProjectLanguageDir(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".gofi/graph/java/gofi_graph_index.json")
	write(t, root, ".gofi/graph/java/gofi_graph.json")

	m, err := Migrate(root, "java")
	if err != nil {
		t.Fatal(err)
	}
	if !present(root, CodeDir+"/gofi_graph.json") || present(root, ".gofi/graph") {
		t.Fatalf("java graph not moved: %+v", m)
	}
}

func TestMigrateNeverOverwrites(t *testing.T) {
	root := t.TempDir()
	write(t, root, CodeDir+"/gofi_graph.json")
	write(t, root, ".gofi/graph/gofi_graph.json")
	write(t, root, ".gofi/graph/only-old.json")

	if _, err := Migrate(root, "go"); err != nil {
		t.Fatal(err)
	}
	if present(root, CodeDir+"/only-old.json") || present(root, ".gofi/graph") {
		t.Error("the old copy was merged into the current index instead of dropped")
	}

	again, err := Migrate(root, "go")
	if err != nil || !again.Empty() {
		t.Errorf("second run changed %+v (%v)", again, err)
	}
}

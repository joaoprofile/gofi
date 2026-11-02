package loop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

func writeFlow(t *testing.T, root, name, body string) {
	t.Helper()
	p := layout.Flows().Abs(root, name+".yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

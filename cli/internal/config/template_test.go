package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templatePath is the schema reference document. Nothing renders it — it exists
// so a reader can see a whole .gofi.yaml with its comments — which is exactly
// why it drifts silently. These tests are the only thing that notices.
const templatePath = "../../templates/gofi.yaml.tmpl"

func readTemplate(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(templatePath))
	if err != nil {
		t.Fatalf("read the schema reference: %v", err)
	}
	return string(b)
}

// A model missing from the reference reads as a model gofi does not offer, and
// the reference is where someone looks before editing ai.models by hand.
func TestTemplateListsEveryModel(t *testing.T) {
	tmpl := readTemplate(t)
	for _, id := range AllModels() {
		if !strings.Contains(tmpl, id) {
			t.Errorf("%s is offered by Models() but absent from %s", id, templatePath)
		}
	}
}

// The reference names each block; one added to the schema without a line here
// leaves the document describing a config that no longer exists.
func TestTemplateNamesEveryInitBlock(t *testing.T) {
	tmpl := readTemplate(t)
	for _, key := range initBlocks {
		if !strings.Contains(tmpl, "\n"+key+":") {
			t.Errorf("block %q is written by init but absent from %s", key, templatePath)
		}
	}
}

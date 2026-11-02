package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
)

// A change's surfaces are what its proposal declares; declaring none, the
// project's backend. In a project with more than one surface, saying nothing
// is warned about.
func TestAChangeTouchesTheSurfacesItDeclares(t *testing.T) {
	root := t.TempDir()
	proposal := func(id, extra string) {
		writeFile(t, filepath.Join(root, "specs", "changes", id, "proposal.md"),
			"---\ntipo: mudanca\nformato: 3\ncontextos: [loja]\nversao: \"1.0\"\nstatus: proposto\n"+extra+"keywords: [x]\n---\n# M\n")
	}
	proposal("telas", "superficies: [web]\n")
	proposal("calado", "")
	cfg := &config.GofiConfig{Backend: &config.Backend{Language: "go", Path: "backend"},
		Frontend: &config.UISurface{Framework: "react", Path: "web"}}

	facts, err := changeFacts(cfg, root, "telas")
	if err != nil || !facts["change.surface(web)"] || facts["change.surface(backend)"] {
		t.Errorf("declared: %v, %v", facts, err)
	}
	if facts, _ := changeFacts(cfg, root, "calado"); !facts["change.surface(backend)"] || len(facts) != 1 {
		t.Errorf("undeclared: %v", facts)
	}
	all, _ := contract.Docs(root)
	warned := undeclaredSurfaces(cfg, all)
	if len(warned) != 1 || !strings.Contains(warned[0].Path, "calado") || warned[0].Error {
		t.Errorf("warnings = %+v", warned)
	}
	cfg.Frontend = nil
	if got := undeclaredSurfaces(cfg, all); len(got) != 0 {
		t.Errorf("a one-surface project was warned: %+v", got)
	}
}

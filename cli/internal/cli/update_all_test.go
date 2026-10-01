package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/layout"
)

// projectZoneFixture fills every area the project owns — and the project
// block of AGENTS.md — so a write there cannot go unnoticed.
func projectZoneFixture(t *testing.T, root string) {
	t.Helper()
	for rel, body := range map[string]string{
		".claude/knowledge/shared/nossa-regra.md": "# Nossa regra\n",
		".claude/knowledge/eng/retry.md":          "# Retry\n",
		".claude/memory/contexts/pedido.md":       "---\nversao: \"1.0\"\n---\n# Pedido\n",
		".claude/institutional/demo/INDEX.md":     "# Institucional\n",
		".claude/lexicon/sinonimos.md":            "pedido = order\n",
		"specs/pedido/sdd-pedido.md":              "---\ntipo: spec\n---\n# SDD\n",
		"prd/pedido/prd-pedido.md":                "---\ntipo: prd\n---\n# PRD\n",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	agents := filepath.Join(root, "AGENTS.md")
	b, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, agents, string(b)+"\n<!-- gofi:project:begin -->\n## Projeto\n\nregra do time\n<!-- gofi:project:end -->\n")
}

// zoneSnapshot hashes every file of the project zone, and the project block
// of AGENTS.md.
func zoneSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, dir := range []string{
		layout.Knowledge().Dir, layout.Memory().Dir, layout.Institutional().Dir,
		layout.Lexicon().Dir, layout.Specs().Dir, layout.PRD().Dir,
	} {
		_ = filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			b, _ := os.ReadFile(p)
			sum := sha256.Sum256(b)
			rel, _ := filepath.Rel(root, p)
			out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
			return nil
		})
	}
	for _, f := range []string{".gofi.yaml"} {
		b, _ := os.ReadFile(filepath.Join(root, f))
		sum := sha256.Sum256(b)
		out[f] = hex.EncodeToString(sum[:])
	}
	b, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	s := string(b)
	if i := strings.Index(s, "<!-- gofi:project:begin -->"); i >= 0 {
		out["AGENTS.md#project"] = s[i:]
	}
	return out
}

// bumpUpstream changes every gofi-zone tree upstream, so each target has
// something to write.
func bumpUpstream(t *testing.T) {
	t.Helper()
	repo := os.Getenv("GOFI_AGENTS_LOCAL_DIR")
	for rel, body := range map[string]string{
		"ai/skills/gofi-pd/SKILL.md":              "# /gofi-pd — v2",
		"ai/AGENTS.md":                            "# AGENTS — v2",
		"ai/expertise/diagramming/PACK.md":        "---\npack: diagramming\n---\n# v2\n",
		"ai/expertise/diagramming/conventions.md": "# Conventions v2\n",
		"ai/templates/sdd-template.md":            "# SDD — v2",
		"ai/sdk/go/knowledge/error-handling.md":   "error handling v2",
	} {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(rel)), body)
	}
}

func sameZone(t *testing.T, before, after map[string]string, what string) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s changed %s in the project zone", what, k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s created %s in the project zone", what, k)
		}
	}
}

// The contract of the two zones (0002, D1): no update writes where the
// project owns — not one target, not all of them, not under --force.
func TestNoUpdateWritesTheProjectZone(t *testing.T) {
	for _, run := range []struct {
		name string
		fn   func() error
	}{
		{"update skills", func() error { return runSkillsUpdate(true, false) }},
		{"update agents", func() error { return runAgentsUpdate(true, false) }},
		{"update expertise", func() error { return runExpertiseUpdate(true, false) }},
		{"update templates", func() error { return runTemplatesUpdate(true, false) }},
		{"update sdk", func() error { return runSDKUpdate(true, false) }},
		{"update", func() error { return runUpdateAll(true, false) }},
		{"update --force", func() error { return runUpdateAll(true, true) }},
	} {
		t.Run(run.name, func(t *testing.T) {
			root := setupProject(t)
			projectZoneFixture(t, root)
			bumpUpstream(t)
			before := zoneSnapshot(t, root)
			if err := run.fn(); err != nil {
				t.Fatalf("%s: %v", run.name, err)
			}
			sameZone(t, before, zoneSnapshot(t, root), run.name)
		})
	}
}

// Without a target, every gofi-zone target lands in one run.
func TestUpdateAllRefreshesTheGofiZone(t *testing.T) {
	root := setupProject(t)
	bumpUpstream(t)
	if err := runUpdateAll(true, false); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		".claude/skills/gofi-pd/SKILL.md":              "v2",
		"AGENTS.md":                                    "v2",
		".claude/expertise/diagramming/conventions.md": "v2",
		".claude/templates/sdd-template.md":            "v2",
		".claude/sdk/go/knowledge/error-handling.md":   "v2",
	} {
		if got := readFile(t, filepath.Join(root, filepath.FromSlash(rel))); !strings.Contains(got, want) {
			t.Errorf("%s not updated: %q", rel, got)
		}
	}
}

// A template the team edited is kept, and the plan says where the change
// belongs instead.
func TestUpdateTemplatesKeepsAnEditedTemplate(t *testing.T) {
	root := setupProject(t)
	bumpUpstream(t)
	tmpl := filepath.Join(root, ".claude/templates/sdd-template.md")
	writeFile(t, tmpl, "# SDD — nosso")
	if err := runTemplatesUpdate(true, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, tmpl); got != "# SDD — nosso" {
		t.Errorf("an edited template was overwritten: %q", got)
	}
	if err := runTemplatesUpdate(true, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, tmpl); !strings.Contains(got, "v2") {
		t.Errorf("--force should put upstream back: %q", got)
	}
}

// The KEEPS line of a gofi-zone target says where the team's change belongs.
func TestKeepsLineNamesKnowledge(t *testing.T) {
	s := updateScope{Keeps: []string{"x"}, Hint: keepsHint}
	s.write("a/", "1 changed")
	if out := s.String(); !strings.Contains(out, "knowledge/") || !strings.Contains(out, "overrides:") {
		t.Errorf("KEEPS lacks the hint:\n%s", out)
	}
}

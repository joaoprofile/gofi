package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// libraryFixture adds the reference libraries to the corpus fixture, as
// gofi init lays them out: plain markdown, mostly without frontmatter.
func libraryFixture(t *testing.T) string {
	t.Helper()
	root := corpusFixture(t)
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude/knowledge/shared/clean-code.md", "# Clean Code\n\n## Funções pequenas\n\nTexto.\n\n## Comentários explicam o porquê\n\nTexto.\n")
	write(".claude/knowledge/INDEX.md", "# Índice do conhecimento\n\n> Derivado — regenere com `gofi index docs`.\n")
	write(".claude/sdk/go/knowledge/structure.md", "# Estrutura Go\n\n## Pastas por camada\n\nTexto.\n")
	write(".claude/sdk/web/knowledge/structure.md", "# Estrutura Web\n\n## Pastas por feature\n\nTexto.\n")
	write(".claude/sdk/go/sdk-docs/sqln.md", "---\nkeywords: [paginacao, cursor]\n---\n# sqln\n\n## Paginação\n\nTexto.\n")
	write(".claude/institutional/acme/INDEX.md", "# Institucional — manifesto\n\n## Quando carregar cada chunk\n\nTexto.\n")
	write(".claude/institutional/acme/glossary.md", "# Glossário\n\n## Termos do domínio\n\nTexto.\n")
	write(".claude/memory/project.md", "# Projeto\n\n## Serviços\n\nTexto.\n")
	write(".claude/skills/gofi-eng/SKILL.md", "---\nname: gofi-eng\n---\n# gofi-eng\n\n## Workflow\n")
	write(".claude/skills/gofi-eng/references/layers.md", "# Camadas\n\n## Ordem de criação das camadas\n\nTexto.\n")
	// a spec that cites knowledge by path, and one name shared by two languages
	write("specs/account/notes.md", "---\ntipo: spec\ncontexto: account\n---\n# Notas\n\nSegue .claude/knowledge/shared/clean-code.md e .claude/sdk/web/knowledge/structure.md.\n")
	return root
}

func indexed(idx *Index) map[string]Doc {
	out := map[string]Doc{}
	for _, d := range idx.Docs {
		out[d.Path] = d
	}
	return out
}

func TestLibrariesAreIndexedWithoutFrontmatter(t *testing.T) {
	idx, _ := build(t, libraryFixture(t))
	got := indexed(idx)
	for _, want := range []string{
		".claude/knowledge/shared/clean-code.md",
		".claude/sdk/go/knowledge/structure.md",
		".claude/sdk/go/sdk-docs/sqln.md",
		".claude/institutional/acme/INDEX.md",
		".claude/institutional/acme/glossary.md",
		".claude/memory/project.md",
		".claude/skills/gofi-eng/references/layers.md",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("%s should be indexed", want)
		}
	}
	for _, skip := range []string{".claude/knowledge/INDEX.md", ".claude/skills/gofi-eng/SKILL.md"} {
		if _, ok := got[skip]; ok {
			t.Errorf("%s should not be indexed", skip)
		}
	}
	if d := got[".claude/knowledge/shared/clean-code.md"]; d.Title != "Clean Code" || len(d.Sections) != 2 || d.Kind != AreaKnowledge {
		t.Errorf("clean-code indexed as %+v", d)
	}
	// context memory is reached by both its corpus and the memory library
	n := 0
	for _, d := range idx.Docs {
		if d.Path == ".claude/memory/contexts/churn.md" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("context memory indexed %d times, want once", n)
	}
}

func TestCitedLibraryPathsBecomeEdges(t *testing.T) {
	_, g := build(t, libraryFixture(t))
	has := func(from, to string) bool {
		for _, e := range g.Edges {
			if e[0] == from && e[1] == to && e[2] == EdgeCites {
				return true
			}
		}
		return false
	}
	if !has("specs/account/notes.md", ".claude/knowledge/shared/clean-code.md") {
		t.Error("a knowledge path cited in a spec should be an edge")
	}
	// structure.md exists for go and web: the cited one is web, not whichever
	// stem was registered first
	if !has("specs/account/notes.md", ".claude/sdk/web/knowledge/structure.md") || has("specs/account/notes.md", ".claude/sdk/go/knowledge/structure.md") {
		t.Error("a cited path must resolve to that exact file")
	}
}

func TestOrphansLeaveLibrariesOut(t *testing.T) {
	_, g := build(t, libraryFixture(t))
	e := &Explorer{Graph: g}
	for _, o := range mustOrphans(e, false) {
		if isLibrary(o) {
			t.Errorf("reference material is looked up, not cited: %s is not an orphan", o)
		}
	}
}

// A section range names lines of the file, as an editor or Read(offset) counts
// them — frontmatter included. The range is the whole point of the index: an
// agent sent to the wrong lines reads the wrong rule.
func TestSectionRangesPointAtTheFileLines(t *testing.T) {
	root := libraryFixture(t)
	idx, _ := build(t, root)
	checked := 0
	for _, d := range idx.Docs {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d.Path)))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(raw), "\n")
		for _, s := range d.Sections {
			if s.Start < 1 || s.End > len(lines) || s.End < s.Start {
				t.Errorf("%s §%s: range L%d-%d outside the file (%d lines)", d.Path, s.Heading, s.Start, s.End, len(lines))
				continue
			}
			head := lines[s.Start-1]
			if !strings.HasPrefix(head, "#") || !strings.Contains(head, s.Heading) {
				t.Errorf("%s: L%d is %q, want the heading %q", d.Path, s.Start, head, s.Heading)
			}
			for _, l := range lines[s.Start:s.End] {
				if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "### ") {
					t.Errorf("%s §%s: range L%d-%d runs into the next heading %q", d.Path, s.Heading, s.Start, s.End, l)
					break
				}
			}
			checked++
		}
	}
	if checked < 10 {
		t.Fatalf("only %d sections checked", checked)
	}
}

// Every section gofi ships in a pack is found by the search, and no PACK.md:
// the contract is the router's. A pack folder that collides with a skipped
// name would otherwise vanish from the index without a word.
func TestShippedPacksAreIndexed(t *testing.T) {
	root := corpusFixture(t)
	src := "../../../ai/expertise"
	var sections []string
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dst := filepath.Join(root, ".claude", "expertise", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if d.Name() != "PACK.md" {
			sections = append(sections, ".claude/expertise/"+filepath.ToSlash(rel))
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil || len(sections) == 0 {
		t.Fatalf("copy packs: %v (%d sections)", err, len(sections))
	}
	idx, _ := build(t, root)
	got := indexed(idx)
	for _, s := range sections {
		if _, ok := got[s]; !ok {
			t.Errorf("%s is not indexed", s)
		}
	}
	for p := range got {
		if strings.HasSuffix(p, "/PACK.md") {
			t.Errorf("%s should not be indexed", p)
		}
	}
}

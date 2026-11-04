package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func memoryWith(versions int, state string) string {
	var b strings.Builder
	b.WriteString("---\ncontexto: pricing\nversao: \"1.0\"\nstatus: spec\nkeywords: [preco]\n---\n# pricing\n\n## Estado atual\n\n" + state + "\n\n## Histórico de versões\n\n| Versão | Data | Mudança |\n|--------|------|---------|\n")
	for v := versions; v >= 1; v-- {
		fmt.Fprintf(&b, "| v1.%d | 2026-10-%02d | mudança %d |\n", v, v%28+1, v)
	}
	return b.String()
}

// The pricing memory reached 86 KB, its changelog never moved out, and every
// role of every turn read it whole. The head keeps the ten newest versions;
// the older ones go on top of the context's history.md, newest first.
func TestAMemoryKeepsItsTenNewestVersionsInTheHead(t *testing.T) {
	root := configuredProject(t)
	if _, err := writeMemory(root, "pricing", []byte(memoryWith(15, "O estado."))); err != nil {
		t.Fatal(err)
	}
	head := readFile(t, filepath.Join(root, ".claude/memory/contexts/pricing.md"))
	if !strings.Contains(head, "| v1.15 |") || !strings.Contains(head, "| v1.6 |") || strings.Contains(head, "| v1.5 |") {
		t.Errorf("the head must keep v1.15..v1.6:\n%s", head)
	}
	if !strings.Contains(head, "Versões anteriores: `.claude/memory/contexts/pricing/history.md`.") {
		t.Errorf("the head must point at the history:\n%s", head)
	}
	hist := readFile(t, filepath.Join(root, ".claude/memory/contexts/pricing/history.md"))
	if !strings.Contains(hist, "| v1.5 |") || !strings.Contains(hist, "| v1.1 |") || strings.Index(hist, "| v1.5 |") > strings.Index(hist, "| v1.1 |") {
		t.Errorf("the history must hold v1.5..v1.1, newest first:\n%s", hist)
	}

	// The next write overflows two more: they are newer than what the
	// history holds, so they go on top of it; the pointer is not repeated.
	if _, err := writeMemory(root, "pricing", []byte(strings.Replace(memoryWith(17, "O estado."), "| v1.6 |", "| v1.6 |", 1))); err != nil {
		t.Fatal(err)
	}
	hist = readFile(t, filepath.Join(root, ".claude/memory/contexts/pricing/history.md"))
	if strings.Index(hist, "| v1.7 |") < 0 || strings.Index(hist, "| v1.7 |") > strings.Index(hist, "| v1.5 |") {
		t.Errorf("newer overflow must go on top:\n%s", hist)
	}
	head = readFile(t, filepath.Join(root, ".claude/memory/contexts/pricing.md"))
	if strings.Count(head, "Versões anteriores:") != 1 {
		t.Errorf("the pointer must appear once:\n%s", head)
	}
}

func TestAMemoryWithFewVersionsIsWrittenAsItIs(t *testing.T) {
	root := configuredProject(t)
	doc := memoryWith(3, "O estado.")
	if _, err := writeMemory(root, "pricing", []byte(doc)); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(root, ".claude/memory/contexts/pricing.md")); got != doc {
		t.Errorf("a short memory changed:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/memory/contexts/pricing/history.md")); !os.IsNotExist(err) {
		t.Error("a short memory wrote a history")
	}
}

func TestAStateTooBigIsSaid(t *testing.T) {
	if memoryNote(memoryWith(1, strings.Repeat("x", stateLimit+1))) == "" {
		t.Error("a state past the limit was not said")
	}
	if memoryNote(memoryWith(1, "curto")) != "" {
		t.Error("a short state was said")
	}
}

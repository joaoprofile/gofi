package loop

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxChangeText is how much of the change a brief carries inline: its
// proposal, deltas and design are what every turn works from, and a turn that
// reads them in the brief spends no calls finding them.
const maxChangeText = 40000

// changeText is the change a turn works on, its documents inline in the
// order they are read — proposal, deltas, design, tasks — without their
// frontmatter. Past maxChangeText the design and the tasks are named, not
// carried: the requirements are what must fit.
func changeText(work, dir string) string {
	if dir == "" {
		return ""
	}
	abs := filepath.Join(work, filepath.FromSlash(dir))
	type doc struct{ rel, body string }
	read := func(rel string) (doc, bool) {
		b, err := os.ReadFile(filepath.Join(abs, filepath.FromSlash(rel)))
		if err != nil {
			return doc{}, false
		}
		return doc{rel, strings.TrimSpace(withoutFrontmatter(string(b)))}, true
	}
	var must, may []doc
	if d, ok := read("proposal.md"); ok {
		must = append(must, d)
	}
	deltas, _ := filepath.Glob(filepath.Join(abs, "deltas", "*.md"))
	sort.Strings(deltas)
	for _, p := range deltas {
		if filepath.Base(p) == "INDEX.md" {
			continue
		}
		if d, ok := read("deltas/" + filepath.Base(p)); ok {
			must = append(must, d)
		}
	}
	for _, rel := range []string{"design.md", "tasks.md"} {
		if d, ok := read(rel); ok {
			may = append(may, d)
		}
	}
	if len(must) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "**A mudança, na íntegra** — é o contrato deste turno; não precisa abrir estes arquivos de `%s/`:\n\n", dir)
	size := 0
	write := func(d doc) {
		fmt.Fprintf(&b, "<documento caminho=\"%s/%s\">\n%s\n</documento>\n\n", dir, d.rel, d.body)
		size += len(d.body)
	}
	for _, d := range must {
		write(d)
	}
	var left []string
	for _, d := range may {
		if size+len(d.body) > maxChangeText {
			left = append(left, dir+"/"+d.rel)
			continue
		}
		write(d)
	}
	if len(left) > 0 {
		fmt.Fprintf(&b, "Grande demais para vir aqui: %s — leia a §seção que a sua tarefa pede (`gofi show <arquivo>` lista as seções).\n\n", strings.Join(left, ", "))
	}
	return b.String()
}

// withoutFrontmatter is a document's body.
func withoutFrontmatter(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	if end := strings.Index(s[4:], "\n---"); end >= 0 {
		rest := s[4+end+4:]
		return strings.TrimPrefix(rest, "\n")
	}
	return s
}

// loopReading is what a turn of the loop reads besides the change: it
// replaces the reading list of the role's skill, written for a person's
// session, where nothing comes with the request.
const loopReading = "**O que ler neste turno:** a mudança está acima, inteira. Além dela, só o que a sua tarefa pede: a seção `## Estado atual` da memória do contexto (não o arquivo todo), o código que você vai tocar e os módulos do `.claude/knowledge/INDEX.md` que tratam dele. Não releia o `AGENTS.md`, o índice do conhecimento inteiro nem os protocolos — as leis que valem aqui já estão no seu prompt de sistema.\n\n"

// withChange puts the change after the brief's first paragraph — where it
// is the same words for every role of the run, the prefix their turns share.
func withChange(brief, work string, s *State) string {
	text := changeText(work, s.ChangeDir)
	if text == "" {
		return brief
	}
	head, rest, ok := strings.Cut(brief, "\n\n")
	if !ok {
		return brief + "\n\n" + text + loopReading
	}
	return head + "\n\n" + text + loopReading + rest
}

package loop

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

// maxChangeText is how much of the change a brief carries inline: its
// proposal, deltas and design are what every turn works from, and a turn that
// reads them in the brief spends no calls finding them.
const maxChangeText = 40000

// leanStages are the stages that do not work from the requirements: the
// delivery writes the commit and the route docs from the diff, the learner
// its lesson from the run's findings. They get the proposal and the names of
// the rest — the whole change, up to 40KB, was written to a cache no other
// turn of theirs reads.
var leanStages = []string{"deliver", "learn"}

// changeText is the change a turn works on, its documents inline in the
// order they are read — proposal, deltas, design, tasks — without their
// frontmatter. Past maxChangeText the design and the tasks are named, not
// carried: the requirements are what must fit. lean carries the proposal
// alone and names the rest.
func changeText(work, dir string, lean bool) string {
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
	var named []string
	if lean {
		for _, d := range append(must[1:], may...) {
			named = append(named, dir+"/"+d.rel)
		}
		if must[0].rel != "proposal.md" {
			named = append([]string{dir + "/" + must[0].rel}, named...)
			must = nil
		} else {
			must = must[:1]
		}
		may = nil
	}
	if len(must) == 0 {
		return fmt.Sprintf("**A mudança** está em `%s/`: %s — leia a §seção que a sua tarefa pede (`gofi show <arquivo>` lista as seções).\n\n", dir, strings.Join(named, ", "))
	}
	var b strings.Builder
	if lean {
		fmt.Fprintf(&b, "**A proposta da mudança** — não precisa abrir este arquivo de `%s/`:\n\n", dir)
	} else {
		fmt.Fprintf(&b, "**A mudança, na íntegra** — é o contrato deste turno; não precisa abrir estes arquivos de `%s/`:\n\n", dir)
	}
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
	if len(named) > 0 {
		fmt.Fprintf(&b, "O resto da mudança, se a sua tarefa pedir: %s — leia a §seção que ela pede (`gofi show <arquivo>` lista as seções).\n\n", strings.Join(named, ", "))
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

// loopReadingLean is loopReading for a turn that gets the proposal alone.
const loopReadingLean = "**O que ler neste turno:** só o que a sua tarefa pede — o diff, o código que ela cita, a §seção da mudança que faltar. Não releia o `AGENTS.md`, o índice do conhecimento inteiro nem os protocolos — as leis que valem aqui já estão no seu prompt de sistema.\n\n"

// withChange puts the change after the brief's first paragraph — where it
// is the same words for every role of the run, the prefix their turns share
// — and the role's skill after the change, the same words on every turn of
// that role. What changes from turn to turn comes after both.
func withChange(brief, work string, s *State, rl role.Role) string {
	lean := slices.Contains(leanStages, rl.Contract.Stage)
	text := changeText(work, s.ChangeDir, lean)
	switch {
	case text == "":
	case lean:
		text += loopReadingLean
	default:
		text += loopReading
	}
	text += roleText(work, rl)
	if text == "" {
		return brief
	}
	head, rest, ok := strings.Cut(brief, "\n\n")
	if !ok {
		return brief + "\n\n" + text
	}
	return head + "\n\n" + text + rest
}

// roleText is an internal role's skill, inline. Named by its path, the
// first call of every turn was the Read that loads it — a fresh session
// each turn — and the guard refused a whole-file Read that early. A skill
// that is not installed is named, as before.
func roleText(work string, rl role.Role) string {
	if !rl.Contract.Internal() {
		return ""
	}
	rel := layout.RoleSkill(rl.Name, true)
	b, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Sprintf("Siga o papel descrito em %s.\n\n", rel)
	}
	return fmt.Sprintf("**O seu papel, na íntegra** — não precisa abrir `%s`:\n\n<documento caminho=\"%s\">\n%s\n</documento>\n\n",
		rel, rel, strings.TrimSpace(withoutFrontmatter(string(b))))
}

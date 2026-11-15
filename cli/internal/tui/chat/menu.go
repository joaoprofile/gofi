package chat

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/joaoprofile/gofi/cli/internal/engine"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// command is one thing a "/" can run: the chat's own, a gofi command that
// reads the project without a model, a skill, or one of the engine's.
type command struct {
	name string // without the slash
	hint string // what follows the name, when it takes something
	desc string
	kind int
}

// The kinds, in the order the menu lists them.
const (
	kindChat   = iota // answered here: /help, /clear, /model…
	kindGofi          // a gofi command run on the project, no model called
	kindSkill         // a skill of the project
	kindEngine        // the engine's own commands and the person's skills
)

// chatCommands are the chat's own, in the order /help lists them.
func chatCommands() []command {
	c := func(name, hint, key string) command {
		return command{name: name, hint: hint, desc: i18n.T(key), kind: kindChat}
	}
	return []command{
		c("help", "", "chat.help.help"),
		c("clear", "", "chat.help.clear"),
		c("model", "[n|nome]", "chat.help.model"),
		c("resume", "[n]", "chat.help.resume"),
		c("cost", "", "chat.help.cost"),
		c("brief", "", "chat.help.brief"),
		c("send", "<texto>", "chat.help.send"),
		c("exit", "", "chat.help.exit"),
	}
}

// gofiCommands read the project where it is — the runs on disk, the index —
// and call no model: what they answer costs no token.
func gofiCommands() []command {
	c := func(name, hint, key string) command {
		return command{name: name, hint: hint, desc: i18n.T(key), kind: kindGofi}
	}
	return []command{
		c("watch", "", "chat.help.watch"),
		c("runs", "", "chat.help.runs"),
		c("find", "<texto>", "chat.help.find"),
		c("show", "<símbolo>", "chat.help.show"),
		c("impact", "<símbolo>", "chat.help.impact"),
		c("path", "<de> <para>", "chat.help.path"),
	}
}

// gofiArgs is the gofi command line a "/" of kindGofi stands for, and
// whether the text is one. /watch and /runs are the chat's to draw.
func gofiArgs(text string) ([]string, bool) {
	name, rest, _ := strings.Cut(strings.TrimPrefix(text, "/"), " ")
	switch name {
	case "find", "show", "impact", "path":
		return append([]string{name}, strings.Fields(rest)...), true
	}
	return nil, false
}

// hidden are commands of the engine that mean nothing here: they drive its
// own terminal, or a session this is not.
var hidden = map[string]bool{
	"color": true, "focus": true, "heapdump": true, "workflow-launch-exec": true,
	"design-consent": true, "design-revoke": true, "extra-usage": true, "agents": true,
}

// commands is everything a "/" offers, the chat's own first. A name the chat
// answers itself hides the engine's command of the same name.
func (m model) commands() []command {
	out := append(chatCommands(), gofiCommands()...)
	seen := map[string]bool{"quit": true}
	for _, c := range out {
		seen[c.name] = true
	}
	add := func(c command) {
		if c.name == "" || seen[c.name] || hidden[c.name] || strings.HasPrefix(c.name, "_") {
			return
		}
		seen[c.name] = true
		out = append(out, c)
	}
	project := map[string]string{}
	for _, s := range m.onDisk {
		project[s.name] = s.desc
	}
	// What the engine said it runs, each with what it does.
	var rest []command
	for _, c := range m.described {
		k := command{name: c.Name, hint: c.Hint, desc: firstSentence(c.Description), kind: kindEngine}
		if _, ok := project[c.Name]; ok {
			k.kind = kindSkill
		}
		rest = append(rest, k)
	}
	// Until it says, the skills on disk and the names it gave with a turn.
	for _, s := range m.onDisk {
		rest = append(rest, command{name: s.name, desc: s.desc, kind: kindSkill})
	}
	for _, name := range m.skills {
		kind := kindEngine
		if _, ok := project[name]; ok {
			kind = kindSkill
		}
		rest = append(rest, command{name: name, kind: kind})
	}
	sort.SliceStable(rest, func(i, j int) bool {
		if rest[i].kind != rest[j].kind {
			return rest[i].kind < rest[j].kind
		}
		return rest[i].name < rest[j].name
	})
	for _, c := range rest {
		add(c)
	}
	return out
}

// skill is a skill the project installed, read from disk.
type skill struct{ name, desc string }

// readProjectSkills lists .claude/skills with what each one says it does,
// before the engine is up to say so.
func readProjectSkills(root string) []skill {
	dir := layout.Skills().Abs(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, skill{name: e.Name(), desc: skillDescription(filepath.Join(dir, e.Name(), "SKILL.md"))})
	}
	return out
}

// skillDescription is the description in a SKILL.md's front matter.
func skillDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return ""
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			return firstSentence(strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
	return ""
}

// firstSentence keeps a description to what fits a line of a menu.
func firstSentence(s string) string {
	s = strings.Join(strings.Fields(strings.SplitN(s, "\n", 2)[0]), " ")
	if i := strings.Index(s, ". "); i > 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(s, ".")
}

// menu is the list under the prompt while a /command or an @path is being
// typed: what matches, and the one enter or tab takes.
type menu struct {
	items []menuItem
	sel   int
	// from is where the word being completed starts in the prompt.
	from int
	path bool // an @path rather than a /command
}

type menuItem struct {
	value string // what replaces the word
	hint  string
	desc  string
	kind  int
}

// menuRows is how many items the menu shows at once.
const menuRows = 8

// openMenu builds the menu for the word under the cursor, or none: a
// /command is the first word of the prompt, an @path any word of it.
func (m model) openMenu() *menu {
	before := m.input.Before()
	start := strings.LastIndexAny(before, " \t\n") + 1
	word := before[start:]
	from := len([]rune(before[:start]))
	switch {
	case strings.HasPrefix(word, "/") && start == 0:
		items := matchCommands(m.commands(), strings.ToLower(word[1:]))
		if len(items) == 0 {
			return nil
		}
		return &menu{items: items, from: from}
	case strings.HasPrefix(word, "@"):
		var items []menuItem
		for _, p := range pathCandidates(m.opts.Root, word[1:]) {
			items = append(items, menuItem{value: p})
		}
		if len(items) == 0 {
			return nil
		}
		return &menu{items: items, from: from, path: true}
	}
	return nil
}

// matchCommands keeps the commands a typed word could mean: the ones that
// start with it first, then the ones that hold it.
func matchCommands(all []command, word string) []menuItem {
	var starts, holds []menuItem
	for _, c := range all {
		it := menuItem{value: "/" + c.name, hint: c.hint, desc: c.desc, kind: c.kind}
		switch name := strings.ToLower(c.name); {
		case strings.HasPrefix(name, word):
			starts = append(starts, it)
		case strings.Contains(name, word):
			holds = append(holds, it)
		}
	}
	return append(starts, holds...)
}

func (u *menu) move(d int) {
	n := len(u.items)
	u.sel = ((u.sel+d)%n + n) % n
}

// view draws the menu: the items around the one chosen, each with what it
// does, and how many there are.
func (u *menu) view(k look, width int) []string {
	first := 0
	if len(u.items) > menuRows {
		first = min(max(u.sel-menuRows/2, 0), len(u.items)-menuRows)
	}
	last := min(first+menuRows, len(u.items))
	// A hint says what follows the name in a few words; a long one is the
	// engine explaining itself, and gives way to the description.
	label := func(it menuItem) string {
		if it.hint == "" {
			return it.value
		}
		return it.value + " " + ansi.Truncate(it.hint, 18, "…")
	}
	nameW := 0
	for _, it := range u.items[first:last] {
		nameW = max(nameW, ansi.StringWidth(label(it)))
	}
	nameW = min(nameW, max(width*2/5, 12))
	var out []string
	for i := first; i < last; i++ {
		it := u.items[i]
		name := label(it)
		name = ansi.Truncate(name, nameW, "…")
		pad := strings.Repeat(" ", max(nameW-ansi.StringWidth(name), 0)+2)
		desc := ansi.Truncate(it.desc, max(width-nameW-6, 0), "…")
		line := "  " + k.text.Render(name) + pad + k.dim.Render(desc)
		if i == u.sel {
			line = k.accent.Render("❯ "+name) + pad + k.text.Render(desc)
		}
		out = append(out, ansi.Truncate(line, max(width-1, 10), "…"))
	}
	foot := i18n.T("chat.menu.keys")
	if len(u.items) > menuRows {
		foot += "  " + i18n.T("chat.menu.count", u.sel+1, len(u.items))
	}
	return append(out, k.dim.Render(ansi.Truncate("  "+foot, max(width-1, 10), "…")))
}

// describe keeps what the engine said it runs.
func (m *model) describe(meta engine.Meta) {
	if len(meta.Commands) > 0 {
		m.described = meta.Commands
	}
	if len(meta.SlashCommands) > 0 {
		m.skills = meta.SlashCommands
	}
}

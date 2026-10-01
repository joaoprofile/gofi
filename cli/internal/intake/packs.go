package intake

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/expertise"
)

// PackChoice is an expertise pack the request needs, and the signal that
// selected it.
type PackChoice struct {
	Pack string `json:"pack"`
	Dir  string `json:"dir"`
	Why  string `json:"why"`
}

// choosePacks selects the packs that serve a role of the plan and whose
// contract matches the request (0002 D5). A pack's imports are not checked:
// the index does not record what a file imports, and a signal that cannot be
// verified must not select anything.
func choosePacks(packs []expertise.Pack, phases []Phase, ws []string, evidence []Pointer) []PackChoice {
	served := map[string]bool{}
	for _, ph := range phases {
		served[strings.TrimPrefix(ph.Role, "gofi-")] = true
	}
	var out []PackChoice
	for _, p := range packs {
		if !slices.ContainsFunc(p.Serves, func(r string) bool { return served[r] }) {
			continue
		}
		if why := packSignal(p, ws, evidence); why != "" {
			out = append(out, PackChoice{Pack: p.Name, Dir: p.Dir, Why: why})
		}
	}
	return out
}

// packSignal is the first signal of the pack's contract the request carries,
// described, or "" when none does.
func packSignal(p expertise.Pack, ws []string, evidence []Pointer) string {
	w := p.When
	if w.Always {
		return "vale para toda tarefa"
	}
	for _, in := range w.Intents {
		if word, ok := mentions(ws, in); ok {
			return "intenção \"" + word + "\" no pedido"
		}
	}
	for _, en := range w.Entities {
		if word, ok := mentions(ws, en); ok {
			return "entidade \"" + word + "\" no pedido"
		}
	}
	for _, e := range evidence {
		for _, g := range w.Paths {
			if globMatch(g, e.Path) {
				return "evidência em " + e.Path + " casa com " + g
			}
		}
		if e.Kind == "symbol" {
			for _, g := range w.Symbols {
				if ok, _ := path.Match(g, e.Title); ok {
					return "símbolo " + e.Title + " casa com " + g
				}
			}
		}
	}
	return ""
}

// mentions reports whether the request carries a signal word or phrase. A
// word matches a longer form of itself ("integrar" in "integração") by a
// shared stem of at least five letters.
func mentions(ws []string, signal string) (string, bool) {
	sig := words(normalize(signal))
	if len(sig) == 0 {
		return "", false
	}
	if len(sig) > 1 {
		for i := 0; i+len(sig) <= len(ws); i++ {
			if equalWords(ws[i:i+len(sig)], sig) {
				return signal, true
			}
		}
		return "", false
	}
	s := sig[0]
	stem := s
	if len(stem) > 5 {
		stem = stem[:len(stem)-2]
	}
	for _, w := range ws {
		if w == s || (len(stem) >= 5 && strings.HasPrefix(w, stem)) {
			return w, true
		}
	}
	return "", false
}

// globMatch matches a slash path against a pattern where ** crosses folders.
func globMatch(pattern, p string) bool {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			b.WriteString(".*")
			i++
			if i+1 < len(pattern) && pattern[i+1] == '/' {
				i++
			}
		case c == '*':
			b.WriteString("[^/]*")
		case c == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	return err == nil && re.MatchString(p)
}

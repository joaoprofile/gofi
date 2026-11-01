package contract

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// TemplateFile is the spec template the placeholders are read from.
const TemplateFile = "sdd-template.md"

// requiredSections are the sections the loop works from: the overview says
// what the context is, the operations what to implement, the rules what to
// test and audit against. A spec without one of them has nothing to anchor
// that work to. Matched by number, so a reworded title still counts.
var requiredSections = []struct{ num, name string }{
	{"1", "Visão Geral"},
	{"4", "Operações"},
	{"5", "Regras de Negócio"},
}

var (
	reSection = regexp.MustCompile(`^##\s+(\d+(?:\.\d+)?)[.\s]`)
	reRule    = regexp.MustCompile(`^###\s+(RN-\d+)\b`)
	reFence   = regexp.MustCompile("^\\s*(```|~~~)")
	// A template placeholder is a braced phrase that starts in upper case:
	// {Contexto}, {Nome da Regra}. Lower-case braces are left alone, because
	// a route parameter ({id}) is written exactly like one.
	rePlaceholder = regexp.MustCompile(`\{[A-ZÀ-Ý][^{}\n]*\}`)
	reOpenMarker  = regexp.MustCompile(`\b(TODO|TBD|FIXME)\b|\?\?\?`)
)

// Check reports what stands between one spec and the loop: what keeps it from
// being approved, and an approval that no longer holds. Errors block; warnings
// are worth reading and block nothing.
func Check(root string, s Spec, placeholders map[string]bool) []docs.Finding {
	return append(content(s, placeholders), sealFindings(s)...)
}

// content checks what the spec says: its sections, rules and leftovers.
func content(s Spec, placeholders map[string]bool) []docs.Finding {
	var out []docs.Finding
	add := func(line int, isErr bool, format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		if line > 0 {
			msg = fmt.Sprintf("L%d: %s", line, msg)
		}
		out = append(out, docs.Finding{Path: s.Path, Message: msg, Error: isErr})
	}

	if s.Context == "" {
		add(0, true, "sem contexto no frontmatter")
	}

	sections := map[string]bool{}
	rules := map[string]int{}
	var ruleOrder []string
	inRules, fenced := false, false
	var rule string
	ruleText := map[string]bool{}
	for i, ln := range s.Body {
		line := s.Offset + i
		// Placeholders first, on every line: a heading or a code block can
		// carry one as well as prose.
		for _, p := range rePlaceholder.FindAllString(ln, -1) {
			if placeholders[p] {
				add(line, true, "placeholder do template %s não preenchido", p)
			}
		}
		if reFence.MatchString(ln) {
			fenced = !fenced
			continue
		}
		if !fenced {
			if m := reSection.FindStringSubmatch(ln); m != nil {
				sections[m[1]] = true
				inRules, rule = m[1] == "5", ""
				continue
			}
			if inRules {
				if m := reRule.FindStringSubmatch(ln); m != nil {
					rule = m[1]
					if first, dup := rules[rule]; dup {
						add(line, true, "%s repetida (já declarada na L%d)", rule, first)
					} else {
						rules[rule] = line
						ruleOrder = append(ruleOrder, rule)
					}
					continue
				}
				if strings.HasPrefix(ln, "### ") {
					rule = ""
				} else if rule != "" && strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(ln), ">")) != "" {
					ruleText[rule] = true
				}
			}
			if m := reOpenMarker.FindString(ln); m != "" {
				add(line, true, "marcador em aberto %q: decida antes de aprovar", m)
			}
		}
	}

	for _, sec := range requiredSections {
		if !sections[sec.num] {
			add(0, true, "falta a seção %s (%s)", sec.num, sec.name)
		}
	}
	if sections["5"] && len(rules) == 0 {
		add(0, false, "§5 sem nenhuma regra RN-xx: o loop não terá o que rastrear até o teste")
	}
	for _, r := range ruleOrder {
		if !ruleText[r] {
			add(rules[r], true, "%s sem descrição", r)
		}
	}
	return out
}

// Placeholders reads the template's own placeholders, so the check knows a
// leftover from a phrase that merely sits in braces. Without the template
// there is nothing to compare against, and ok is false.
func Placeholders(root string) (map[string]bool, bool) {
	b, err := os.ReadFile(layout.Templates().Abs(root, TemplateFile))
	if err != nil {
		return nil, false
	}
	set := map[string]bool{}
	for _, p := range rePlaceholder.FindAllString(string(b), -1) {
		set[p] = true
	}
	return set, true
}

// CheckAll checks the specs given and adds the corpus findings — format,
// unknown fields, lexicon — that `gofi index check` reports for the same
// files, so one command says everything that stands between a spec and its
// approval.
func CheckAll(root string, specs []Spec) []docs.Finding {
	placeholders, ok := Placeholders(root)
	var out []docs.Finding
	if !ok {
		out = append(out, docs.Finding{Path: layout.Templates().Path(TemplateFile),
			Message: "template ausente: placeholders não verificados — rode `gofi update templates`"})
	}
	mine := map[string]bool{}
	for _, s := range specs {
		mine[s.Path] = true
		out = append(out, Check(root, s, placeholders)...)
	}
	for _, f := range docs.Validate(root) {
		if mine[f.Path] {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Errors counts the findings that block an approval.
func Errors(fs []docs.Finding) int {
	n := 0
	for _, f := range fs {
		if f.Error {
			n++
		}
	}
	return n
}

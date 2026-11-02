package contract

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The rules of format 3 (0003, S7). Sections are matched by title, not by
// number: the contract between roles is the requirement id now.

var (
	reClarify  = regexp.MustCompile(`\[(PRECISA ESCLARECER|NEEDS CLARIFICATION)\b`)
	reMust     = regexp.MustCompile(`\bDEVE\b`) // NÃO DEVE carries it too
	reReqCited = regexp.MustCompile(`\b(?:[a-z0-9][a-z0-9_-]*/)?REQ-\d{2,}\b`)
)

// rules3 checks a format-3 document by its kind.
func (f *finder) rules3(all []Doc, root string) {
	f.clarifications()
	switch f.doc.Kind {
	case KindSpec:
		f.livingSpec()
	case KindChange:
		f.proposal(all, root)
	case KindDelta:
		f.delta(all)
	case KindDesign:
		f.design(all)
	}
}

// clarifications reports every open [PRECISA ESCLARECER: …] outside code: a
// document is approvable only once nothing in it is waiting for a decision.
func (f *finder) clarifications() {
	fenced := false
	for i, ln := range f.doc.Body {
		if docs.IsFence(ln) {
			fenced = !fenced
			continue
		}
		if !fenced && reClarify.MatchString(ln) {
			f.add(f.doc.Offset+i, true, "dúvida em aberto: decida e tire o marcador antes de aprovar")
		}
	}
}

func (f *finder) livingSpec() {
	f.requireSections("Propósito", "Requisitos")
	f.requirements(Requirements(f.doc), map[string]int{}, true)
	fenced := false
	for i, ln := range f.doc.Body {
		if docs.IsFence(ln) {
			if !fenced {
				f.add(f.doc.Offset+i, false, "bloco de código na spec: o como vai para o design da mudança")
			}
			fenced = !fenced
		}
	}
}

// requirements checks the form of requirement blocks: a well-formed id,
// unique across seen (shared by the sections of one delta), a DEVE, and at
// least one scenario with DADO, QUANDO and ENTÃO under the right parent.
// needScenarios is false for a removal, which carries a reason instead.
func (f *finder) requirements(reqs []Requirement, seen map[string]int, needScenarios bool) {
	for _, r := range reqs {
		if r.ID == "" {
			f.add(r.Line, true, "requisito sem ID: escreva `### Requisito: REQ-NN — título` (%q)", r.Title)
			continue
		}
		if first, dup := seen[r.ID]; dup {
			f.add(r.Line, true, "%s repetido (já declarado na L%d)", r.ID, first)
			continue
		}
		seen[r.ID] = r.Line
		if !needScenarios {
			continue
		}
		if !reMust.MatchString(strings.Join(r.Text, " ")) {
			f.add(r.Line, false, "%s sem DEVE / NÃO DEVE: um requisito diz o que o sistema deve fazer", r.ID)
		}
		if len(r.Scenarios) == 0 {
			f.add(r.Line, true, "%s sem cenário: escreva ao menos um `#### Cenário: %s.1 — título`", r.ID, r.ID)
		}
		for _, s := range r.Scenarios {
			f.scenario(r, s, seen)
		}
	}
}

func (f *finder) scenario(r Requirement, s Scenario, seen map[string]int) {
	if s.ID == "" {
		f.add(s.Line, true, "cenário sem ID: escreva `#### Cenário: %s.N — título` (%q)", r.ID, s.Title)
		return
	}
	if p := Parent(s.ID); p != r.ID {
		f.add(s.Line, true, "%s está sob %s: o cenário pertence ao requisito do seu número", s.ID, r.ID)
	}
	if first, dup := seen[s.ID]; dup {
		f.add(s.Line, true, "%s repetido (já declarado na L%d)", s.ID, first)
		return
	}
	seen[s.ID] = s.Line
	has := map[string]bool{}
	for _, st := range s.Steps {
		has[st.Keyword] = true
	}
	var missing []string
	for _, k := range []string{"DADO", "QUANDO", "ENTAO"} {
		if !has[k] {
			missing = append(missing, strings.Replace(k, "ENTAO", "ENTÃO", 1))
		}
	}
	if len(missing) > 0 {
		f.add(s.Line, true, "%s sem %s: um cenário é `- DADO …` · `- QUANDO …` · `- ENTÃO …`", s.ID, strings.Join(missing, ", "))
	}
}

// requireSections reports the ## sections missing, matched by title.
func (f *finder) requireSections(titles ...string) {
	have := map[string]bool{}
	fenced := false
	for _, ln := range f.doc.Body {
		if docs.IsFence(ln) {
			fenced = !fenced
			continue
		}
		if !fenced && reHeading2.MatchString(ln) {
			have[fold(strings.TrimSpace(strings.TrimPrefix(ln, "##")))] = true
		}
	}
	for _, t := range titles {
		if !have[fold(t)] {
			f.add(0, true, "falta a seção ## %s", t)
		}
	}
}

// changeDocs are the documents of one change, by kind.
type changeDocs struct {
	proposal *Doc
	deltas   []Doc
	design   *Doc
}

func docsOf(all []Doc, change string) changeDocs {
	var c changeDocs
	for i := range all {
		d := all[i]
		if d.Change != change || change == "" {
			continue
		}
		switch d.Kind {
		case KindChange:
			c.proposal = &all[i]
		case KindDelta:
			c.deltas = append(c.deltas, d)
		case KindDesign:
			c.design = &all[i]
		}
	}
	return c
}

func (f *finder) proposal(all []Doc, root string) {
	f.requireSections("Por quê", "O que muda")
	c := docsOf(all, f.doc.Change)
	if f.doc.Change == "" {
		f.add(0, true, "proposta fora de uma pasta de mudança: o lugar dela é specs/%s/<id>/", ChangesDir)
	}
	named := map[string]bool{}
	for _, ctx := range f.doc.Contexts {
		named[ctx] = true
		found := false
		for _, d := range c.deltas {
			found = found || d.Context == ctx
		}
		if !found {
			f.add(0, true, "%s está em contextos, mas a mudança não tem deltas/%s.md", ctx, ctx)
		}
	}
	for _, d := range c.deltas {
		if !named[d.Context] {
			f.add(0, true, "%s tem delta, mas não está em contextos", d.Context)
		}
	}
	touchesCode := false
	for _, d := range c.deltas {
		for _, s := range DeltaSections(d) {
			touchesCode = touchesCode || s.Op != OpRemoved && len(s.Requirements) > 0
		}
	}
	// Frontmatter.Get reads n/a as empty, so the declaration is read raw.
	noDesign := strings.Trim(strings.TrimSpace(f.doc.Front["design"]), `"'`) == "n/a"
	if touchesCode && c.design == nil && !noDesign {
		f.add(0, true, "a mudança adiciona ou modifica requisitos e não tem design.md (ou declare design: n/a)")
	}
	for _, s := range f.doc.Front.List(FieldSurfaces) {
		if !slices.Contains(Surfaces, s) {
			f.add(0, true, "%s: %q não é uma superfície (%s)", FieldSurfaces, s, strings.Join(Surfaces, ", "))
		}
	}
	for _, dep := range f.doc.Front.List(FieldDepends) {
		if dep == f.doc.Change {
			f.add(0, true, "%s: a mudança depende de si mesma", FieldDepends)
		} else if docsOf(all, dep).proposal == nil && !archived(root, dep) {
			f.add(0, true, "%s: a mudança %q não existe, aberta nem arquivada", FieldDepends, dep)
		}
	}
	if cycle := changeCycle(all); cycle != nil && contains(cycle, f.doc.Change) {
		f.add(0, true, "%s — nenhuma ordem aplica essas mudanças", (&CycleError{Path: cycle}).Error())
	}
}

// archived reports whether a change was applied: archive/<data>-<id>/.
func archived(root, change string) bool {
	m, _ := filepath.Glob(layout.Specs().Abs(root, ChangesDir, "archive", "*-"+change))
	for _, p := range m {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

// changeCycle finds a cycle among the open changes' depende_de.
func changeCycle(all []Doc) []string {
	deps := map[string][]string{}
	for _, d := range all {
		if d.Kind == KindChange && d.Change != "" {
			deps[d.Change] = d.Front.List(FieldDepends)
		}
	}
	return findCycle(deps)
}

func (f *finder) delta(all []Doc) {
	sections := DeltaSections(f.doc)
	if len(sections) == 0 {
		f.add(0, true, "delta sem ## ADICIONADO, ## MODIFICADO nem ## REMOVIDO")
		return
	}
	if f.doc.Change == "" {
		f.add(0, true, "delta fora de uma pasta de mudança: o lugar dele é specs/%s/<id>/deltas/", ChangesDir)
	}
	existing := map[string]bool{}
	living, hasLiving := Living(all, f.doc.Context)
	if !hasLiving && strings.TrimSpace(strings.Join(sectionBody(f.doc, "Propósito"), "")) == "" {
		f.add(0, true, "%s ainda não tem spec viva: o delta traz o ## Propósito que ela vai ter", f.doc.Context)
	}
	if hasLiving {
		for _, r := range Requirements(living) {
			existing[r.ID] = true
		}
	}
	addedElsewhere := map[string]string{}
	for _, d := range all {
		if d.Kind != KindDelta || d.Change == f.doc.Change || d.Context != f.doc.Context {
			continue
		}
		for _, s := range DeltaSections(d) {
			if s.Op == OpAdded {
				for _, r := range s.Requirements {
					addedElsewhere[r.ID] = d.Change
				}
			}
		}
	}

	seen := map[string]int{}
	for _, s := range sections {
		f.requirements(s.Requirements, seen, s.Op != OpRemoved)
		for _, r := range s.Requirements {
			if r.ID == "" {
				continue
			}
			switch {
			case s.Op == OpAdded && existing[r.ID]:
				f.add(r.Line, true, "%s já existe na spec viva de %s: use MODIFICADO, ou outro número", r.ID, f.doc.Context)
			case s.Op == OpAdded && addedElsewhere[r.ID] != "":
				f.add(r.Line, true, "%s também é adicionado pela mudança %s: o arquivamento colidiria", r.ID, addedElsewhere[r.ID])
			case s.Op != OpAdded && !hasLiving:
				f.add(r.Line, true, "%s em %s, mas %s ainda não tem spec viva: só se ADICIONA", r.ID, s.Op, f.doc.Context)
			case s.Op != OpAdded && !existing[r.ID]:
				f.add(r.Line, true, "%s em %s não existe na spec viva de %s", r.ID, s.Op, f.doc.Context)
			}
		}
	}
}

func (f *finder) design(all []Doc) {
	f.requireSections("Abordagem", "Mapa de requisitos")
	c := docsOf(all, f.doc.Change)
	// Ids are numbered per context: in a change that touches several, REQ-01
	// may be two requirements. The design cites ctx/REQ-NN where an id
	// repeats, and may cite it bare where it does not.
	touched := map[string]bool{} // ctx/REQ-NN
	owners := map[string]int{}   // REQ-NN -> how many touched contexts have it
	known := map[string]bool{}
	for _, d := range c.deltas {
		if living, ok := Living(all, d.Context); ok {
			for _, r := range Requirements(living) {
				known[r.ID], known[d.Context+"/"+r.ID] = true, true
			}
		}
		seen := map[string]bool{}
		for _, s := range DeltaSections(d) {
			for _, r := range s.Requirements {
				if r.ID == "" {
					continue
				}
				known[r.ID], known[d.Context+"/"+r.ID] = true, true
				if s.Op != OpRemoved && !seen[r.ID] {
					seen[r.ID] = true
					touched[d.Context+"/"+r.ID] = true
					owners[r.ID]++
				}
			}
		}
	}
	mapText := strings.Join(sectionBody(f.doc, "Mapa de requisitos"), "\n")
	cites := func(ref string) bool {
		return regexp.MustCompile(`\b` + regexp.QuoteMeta(ref) + `\b`).MatchString(mapText)
	}
	for _, ref := range sortedKeys(touched) {
		_, id, _ := strings.Cut(ref, "/")
		if cites(ref) || owners[id] == 1 && cites(id) {
			continue
		}
		if owners[id] > 1 {
			f.add(0, true, "%s não está no ## Mapa de requisitos — %s existe em mais de um contexto da mudança: cite %s", ref, id, ref)
		} else {
			f.add(0, true, "%s é adicionado ou modificado pela mudança e não está no ## Mapa de requisitos", id)
		}
	}
	cited := map[string]int{}
	for i, ln := range f.doc.Body {
		// A quote is guidance — the template's own, or a writer's note — not
		// a requirement the design answers to.
		if strings.HasPrefix(strings.TrimSpace(ln), ">") {
			continue
		}
		for _, id := range reReqCited.FindAllString(ln, -1) {
			if _, ok := cited[id]; !ok {
				cited[id] = f.doc.Offset + i
			}
		}
	}
	ids := make([]string, 0, len(cited))
	for id := range cited {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !known[id] {
			f.add(cited[id], true, "%s citado no design não existe na spec viva nem nos deltas da mudança", id)
		}
	}
}

// sectionBody is the lines under a ## section, matched by title, up to the
// next ## heading.
func sectionBody(d Doc, title string) []string {
	var out []string
	in := false
	for _, ln := range d.Body {
		if reHeading2.MatchString(ln) {
			in = fold(strings.TrimSpace(strings.TrimPrefix(ln, "##"))) == fold(title)
			continue
		}
		if in {
			out = append(out, ln)
		}
	}
	return out
}

// fold lowercases and strips the accents Portuguese titles carry, so a
// heading matches whether or not it was typed with them.
func fold(s string) string {
	r := strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e",
		"í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return r.Replace(strings.ToLower(s))
}

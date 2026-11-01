package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// FieldDepends lists, in a spec's frontmatter, the contexts whose code this
// one consumes. It is declared, never inferred: the order the loop builds the
// project in is a decision, and a table two contexts share does not say which
// of them comes first (0002, D7).
const FieldDepends = "depende_de"

// Deps maps each context that has a spec to the contexts its specs declare it
// depends on, sorted and without repeats. A context depending on itself is
// left out here and reported by the check.
func Deps(specs []Doc) map[string][]string {
	set := map[string]map[string]bool{}
	for _, s := range specs {
		if !isLiving(s) || s.Context == "" {
			continue
		}
		if set[s.Context] == nil {
			set[s.Context] = map[string]bool{}
		}
		for _, d := range s.Front.List(FieldDepends) {
			if d != s.Context {
				set[s.Context][d] = true
			}
		}
	}
	out := make(map[string][]string, len(set))
	for c, ds := range set {
		out[c] = sortedKeys(ds)
	}
	return out
}

// CycleError is a dependency cycle: no order can build its contexts.
type CycleError struct{ Path []string }

func (e *CycleError) Error() string {
	return "ciclo de dependência: " + strings.Join(e.Path, " → ")
}

// Order puts the contexts in levels: each depends only on contexts of earlier
// levels, so the contexts of one level can be built side by side. Within a
// level the order is by name, so the same specs always give the same order. A
// dependency on a context with no spec is ignored here — the check reports it.
func Order(specs []Doc) ([][]string, error) {
	return levels(Deps(specs))
}

// ChangeOrder puts the open changes in levels by the depende_de of their
// proposals, as Order does the contexts. A dependency on a change that is not
// open — archived, so already applied — is met and does not move the order.
func ChangeOrder(all []Doc) ([][]string, error) {
	deps := map[string][]string{}
	for _, d := range all {
		if d.Kind == KindChange && d.Change != "" {
			deps[d.Change] = d.Front.List(FieldDepends)
		}
	}
	return levels(deps)
}

// levels groups the keys of deps so each depends only on earlier groups, by
// name within one; a dependency outside deps is ignored, and a cycle is an
// error.
func levels(deps map[string][]string) ([][]string, error) {
	if cycle := findCycle(deps); cycle != nil {
		return nil, &CycleError{Path: cycle}
	}
	level := map[string]int{}
	var depth func(c string) int
	depth = func(c string) int {
		if l, ok := level[c]; ok {
			return l
		}
		l := 0
		for _, d := range deps[c] {
			if _, known := deps[d]; known {
				l = max(l, depth(d)+1)
			}
		}
		level[c] = l
		return l
	}
	var out [][]string
	for _, c := range sortedKeys(deps) {
		l := depth(c)
		for len(out) <= l {
			out = append(out, nil)
		}
		out[l] = append(out[l], c)
	}
	return out, nil
}

// findCycle returns the first cycle found walking the contexts by name, closed
// on the context it starts from, or nil.
func findCycle(deps map[string][]string) []string {
	const (
		white = iota
		grey
		black
	)
	color := map[string]int{}
	var stack []string
	var visit func(c string) []string
	visit = func(c string) []string {
		color[c] = grey
		stack = append(stack, c)
		for _, d := range deps[c] {
			if _, known := deps[d]; !known {
				continue
			}
			switch color[d] {
			case grey:
				for i, s := range stack {
					if s == d {
						return append(append([]string{}, stack[i:]...), d)
					}
				}
			case white:
				if cyc := visit(d); cyc != nil {
					return cyc
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[c] = black
		return nil
	}
	for _, c := range sortedKeys(deps) {
		if color[c] == white {
			if cyc := visit(c); cyc != nil {
				return cyc
			}
		}
	}
	return nil
}

// depFindings checks the dependencies the specs declare, against the whole
// corpus, and reports on the specs in mine: a dependency on a context with no
// spec, on itself, a cycle — errors, since no order could build them — and two
// contexts sharing a table with no dependency declared either way, a warning.
func depFindings(all []Doc, mine map[string]bool) []docs.Finding {
	deps := Deps(all)
	var out []docs.Finding
	for _, s := range all {
		if !mine[s.Path] || !isLiving(s) {
			continue
		}
		for _, d := range s.Front.List(FieldDepends) {
			switch _, known := deps[d]; {
			case d == s.Context:
				out = append(out, docs.Finding{Path: s.Path, Error: true,
					Message: fmt.Sprintf("%s: %s depende de si mesmo", FieldDepends, d)})
			case !known:
				out = append(out, docs.Finding{Path: s.Path, Error: true,
					Message: fmt.Sprintf("%s: %q não tem spec — crie a spec ou tire a dependência", FieldDepends, d)})
			}
		}
	}
	if cycle := findCycle(deps); cycle != nil {
		in := map[string]bool{}
		for _, c := range cycle {
			in[c] = true
		}
		for _, s := range all {
			if mine[s.Path] && isLiving(s) && in[s.Context] && len(s.Front.List(FieldDepends)) > 0 {
				out = append(out, docs.Finding{Path: s.Path, Error: true,
					Message: (&CycleError{Path: cycle}).Error() + " — nenhuma ordem constrói esses contextos"})
			}
		}
	}
	out = append(out, sharedTables(all, deps, mine)...)
	return out
}

// sharedTables warns once per pair of contexts that describe the same table
// with no dependency between them: one of them likely consumes the other.
func sharedTables(all []Doc, deps map[string][]string, mine map[string]bool) []docs.Finding {
	byTable := map[string]map[string]bool{}
	first := map[string]string{} // context -> its first spec, where the warning goes
	for _, s := range all {
		if !isLiving(s) || s.Context == "" {
			continue
		}
		if _, ok := first[s.Context]; !ok {
			first[s.Context] = s.Path
		}
		for _, t := range s.Front.List("entidades") {
			if byTable[t] == nil {
				byTable[t] = map[string]bool{}
			}
			byTable[t][s.Context] = true
		}
	}
	depends := func(a, b string) bool {
		for _, d := range deps[a] {
			if d == b {
				return true
			}
		}
		return false
	}
	pairs := map[[2]string][]string{}
	for _, t := range sortedKeys(byTable) {
		ctxs := sortedKeys(byTable[t])
		for i := 0; i < len(ctxs); i++ {
			for j := i + 1; j < len(ctxs); j++ {
				a, b := ctxs[i], ctxs[j]
				if !depends(a, b) && !depends(b, a) {
					pairs[[2]string{a, b}] = append(pairs[[2]string{a, b}], t)
				}
			}
		}
	}
	keys := make([][2]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i][0] < keys[j][0] || keys[i][0] == keys[j][0] && keys[i][1] < keys[j][1]
	})
	var out []docs.Finding
	for _, k := range keys {
		path := first[k[0]]
		if !mine[path] {
			if path = first[k[1]]; !mine[path] {
				continue
			}
		}
		out = append(out, docs.Finding{Path: path, Message: fmt.Sprintf(
			"%s e %s descrevem %s sem dependência declarada — se um consome o outro, declare %s",
			k[0], k[1], strings.Join(pairs[k], ", "), FieldDepends)})
	}
	return out
}

// isLiving reports whether a document is a spec that orders the project: a
// living spec, or a format-2 one. A delta's depende_de reaches the living
// spec when the change is archived, not before.
func isLiving(d Doc) bool { return d.Kind == KindSpec && d.Change == "" }

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

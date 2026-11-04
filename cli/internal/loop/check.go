package loop

import (
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/joaoprofile/gofi/cli/internal/role"
)

var reName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Problem is one thing wrong with a flow. Errors keep it from running;
// warnings are worth reading.
type Problem struct {
	Message string
	Error   bool
}

// Check validates a flow before anything runs on it: the shape of every node
// and condition, that each node is reachable and reaches an end, that the
// edges out of a node can be decided — and that every cycle is bounded, so no
// run can loop forever. roles are the roles installed, to warn about an agent
// node no role answers; nil skips that check.
func Check(f *Flow, roles []role.Role) []Problem {
	var out []Problem
	errf := func(format string, a ...any) {
		out = append(out, Problem{Message: fmt.Sprintf(format, a...), Error: true})
	}
	if f.Schema != Schema {
		errf("schema %q: this gofi reads %s", f.Schema, Schema)
	}
	if !reName.MatchString(f.Name) {
		errf("name %q: lowercase, digits and -", f.Name)
	}
	if _, ok := f.Nodes[f.Start]; !ok {
		errf("start %q is not a node", f.Start)
	}
	installed, staged := map[string]bool{}, map[string]bool{}
	for _, r := range roles {
		installed[r.Name] = true
		staged[r.Contract.Stage] = true
	}
	for _, name := range sortedNodes(f) {
		n := f.Nodes[name]
		switch n.Kind {
		case KindAgent:
			switch {
			case (n.Role == "") == (n.Stage == ""):
				errf("%s: an agent node names its role or its stage — one of them", name)
			case n.Stage != "" && !slices.Contains(role.Stages, n.Stage):
				errf("%s: stage %q is not a stage of the loop", name, n.Stage)
			case n.Stage != "" && roles != nil && !staged[n.Stage] && !n.Optional:
				out = append(out, Problem{Message: fmt.Sprintf("%s: no installed role works in stage %s", name, n.Stage)})
			case n.Role != "" && roles != nil && !installed[n.Role]:
				out = append(out, Problem{Message: fmt.Sprintf("%s: role %s is not installed", name, n.Role)})
			}
		case KindSensor:
			if len(n.Stages) == 0 {
				errf("%s: a sensor node lists its stages", name)
			}
			for _, s := range n.Stages {
				if !stages[s] {
					errf("%s: stage %q — build, test, static, graph, trace, integrity, security or coverage", name, s)
				}
			}
		case KindTerminal:
			if !outcomes[n.Outcome] {
				errf("%s: outcome %q — accepted, impasse, exhausted or stopped", name, n.Outcome)
			}
		case KindGate, KindPolicy:
		default:
			errf("%s: kind %q — agent, sensor, gate, policy or terminal", name, n.Kind)
		}
	}

	for i, e := range f.Edges {
		from, okFrom := f.Nodes[e.From]
		if !okFrom {
			errf("edge %d: from %q is not a node", i+1, e.From)
		}
		if _, ok := f.Nodes[e.To]; !ok {
			errf("edge %d: to %q is not a node", i+1, e.To)
		}
		if okFrom && from.Kind == KindTerminal {
			errf("edge %d: %s is a terminal — a run ends there", i+1, e.From)
		}
		for _, p := range checkWhen(e.When) {
			errf("edge %d (%s → %s): %s", i+1, e.From, e.To, p)
		}
	}
	for _, name := range sortedNodes(f) {
		if f.Nodes[name].Kind == KindTerminal {
			continue
		}
		edges := f.outgoing(name)
		if len(edges) == 0 {
			errf("%s: no edge out of it, and it is not a terminal", name)
			continue
		}
		for i, e := range edges {
			if e.When.Empty() && i != len(edges)-1 {
				errf("%s: the edge with no condition (to %s) must come last — the ones after it are never tried", name, e.To)
			}
		}
		if !edges[len(edges)-1].When.Empty() {
			errf("%s: no default edge — when no condition holds, the run would have nowhere to go", name)
		}
	}
	if len(f.Limits) > 0 {
		if n, ok := f.Nodes[f.OnLimit]; !ok || n.Kind != KindTerminal {
			errf("on_limit %q: a flow with limits names the terminal a spent limit ends the run in", f.OnLimit)
		}
	}
	for _, l := range f.Limits {
		if l.Max <= 0 {
			errf("limit %s → %s: max must be positive", l.From, l.To)
		}
		found := false
		for _, e := range f.Edges {
			found = found || e.From == l.From && e.To == l.To
		}
		if !found {
			errf("limit %s → %s: there is no such edge", l.From, l.To)
		}
	}

	if _, ok := f.Nodes[f.Start]; ok {
		reach := reachable(f, f.Start, false)
		for _, name := range sortedNodes(f) {
			if !reach[name] {
				errf("%s: unreachable from %s", name, f.Start)
			}
		}
		ends := map[string]bool{}
		for name, n := range f.Nodes {
			if n.Kind == KindTerminal {
				ends[name] = true
			}
		}
		for _, name := range sortedNodes(f) {
			if !reach[name] || ends[name] {
				continue
			}
			found := false
			for to := range reachable(f, name, false) {
				found = found || ends[to]
			}
			if !found {
				errf("%s: no terminal can be reached from it", name)
			}
		}
	}
	for _, cycle := range unboundedCycles(f) {
		errf("cycle %v has no limit on any of its edges — a run could loop forever", cycle)
	}
	return out
}

func checkWhen(w When) []string {
	var out []string
	if w.SensorFailed != nil {
		for _, s := range *w.SensorFailed {
			if !stages[s] {
				out = append(out, fmt.Sprintf("sensor_failed: unknown stage %q", s))
			}
		}
	}
	if w.Verdict != "" && !verdicts[w.Verdict] {
		out = append(out, fmt.Sprintf("verdict %q — clean, caveats or rejected", w.Verdict))
	}
	for _, o := range w.Owner {
		if !owners[o] {
			out = append(out, fmt.Sprintf("owner %q — eng, spec or pd", o))
		}
	}
	return out
}

// reachable is every node reachable from a node, itself included. A node with
// a limited edge reaches on_limit too: that is where a spent limit goes.
func reachable(f *Flow, from string, _ bool) map[string]bool {
	seen := map[string]bool{from: true}
	stack := []string{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		next := []string{}
		for _, e := range f.outgoing(n) {
			next = append(next, e.To)
			if f.limit(e.From, e.To) > 0 && f.OnLimit != "" {
				next = append(next, f.OnLimit)
			}
		}
		for _, to := range next {
			if _, ok := f.Nodes[to]; ok && !seen[to] {
				seen[to] = true
				stack = append(stack, to)
			}
		}
	}
	return seen
}

// unboundedCycles finds every strongly connected component that is a cycle —
// more than one node, or a node with an edge to itself — with no limited edge
// inside it (Tarjan).
func unboundedCycles(f *Flow) [][]string {
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	next := 0
	var connect func(v string)
	connect = func(v string) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, e := range f.outgoing(v) {
			if _, ok := f.Nodes[e.To]; !ok {
				continue
			}
			if _, seen := index[e.To]; !seen {
				connect(e.To)
				low[v] = min(low[v], low[e.To])
			} else if onStack[e.To] {
				low[v] = min(low[v], index[e.To])
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []string
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		in := map[string]bool{}
		for _, c := range comp {
			in[c] = true
		}
		cyclic, bounded := len(comp) > 1, false
		for _, e := range f.Edges {
			if in[e.From] && in[e.To] {
				cyclic = cyclic || e.From == e.To
				bounded = bounded || f.limit(e.From, e.To) > 0
			}
		}
		if cyclic && !bounded {
			sort.Strings(comp)
			out = append(out, comp)
		}
	}
	for _, n := range sortedNodes(f) {
		if _, seen := index[n]; !seen {
			connect(n)
		}
	}
	return out
}

func sortedNodes(f *Flow) []string {
	out := make([]string, 0, len(f.Nodes))
	for n := range f.Nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

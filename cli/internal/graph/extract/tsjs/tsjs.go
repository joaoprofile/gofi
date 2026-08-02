package tsjs

import (
	"bytes"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// scanner assembles the graph from the parsed files.
type scanner struct {
	opt    Options
	module string
	g      *model.Graph
	// parses keeps discovery order, which is the lexical walk order, so the
	// graph is built the same way on every machine.
	parses []*fileParse
	byRel  map[string]*fileParse
	res    *resolver

	// templates are the markup files found beside the modules, in walk order.
	templates []*srcFile
	// selectors maps an element name onto the component that answers to it. A
	// selector two components claim maps to the empty string: which of them a
	// tag meant is not knowable from the tag alone, and the graph would rather
	// miss the edge than invent it.
	selectors map[string]string
	// views maps a template file onto the component that renders it, taken from
	// the templateUrl the component itself declares.
	views map[string]string

	unresolved int
}

// Fast runs the syntactic scan and returns the graph.
func Fast(opt Options) (*model.Graph, error) {
	files, templates, err := discover(opt)
	if err != nil {
		return nil, err
	}
	module := ModuleName(opt.Root)
	for _, f := range slices.Concat(files, templates) {
		// A template shares the unit of the module beside it, which is what it
		// belongs to: the two are one component split across two files.
		f.Unit = unitPath(module, f.Rel)
	}

	g := model.New(module, opt.Root, "fast")
	// The extractor names the language it read, the same way an external one
	// declares it in its header. Which of the two it is cannot be told from the
	// graph otherwise: the scanner reads both with the same code.
	g.Language = opt.Language
	if g.Language == "" {
		g.Language = "typescript"
	}

	s := &scanner{
		opt:    opt,
		module: module,
		g:      g,
		parses: make([]*fileParse, len(files)),
		byRel:  make(map[string]*fileParse, len(files)),
		// The resolver is given the sources alone: no module specifier ever
		// names a template, and letting one resolve to markup would turn a
		// missing file into a phantom import edge.
		res:       newResolver(opt.Root, files),
		templates: templates,
		selectors: map[string]string{},
		views:     map[string]string{},
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i, sf := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			s.parses[i] = parseFile(sf)
		})
	}
	wg.Wait()

	for _, f := range s.parses {
		s.byRel[f.src.Rel] = f
		g.Files[f.src.Rel] = model.FileInfo{
			Path: f.src.Rel, Hash: f.src.Hash, Unit: f.src.Unit, Lines: f.src.Lines,
		}
		g.Stats.Files++
		g.Stats.LOC += f.src.Lines
	}
	for _, t := range templates {
		g.Files[t.Rel] = model.FileInfo{Path: t.Rel, Hash: t.Hash, Unit: t.Unit, Lines: t.Lines}
		g.Stats.Files++
		g.Stats.LOC += t.Lines
	}

	s.declare()
	s.link()
	s.resolveCalls()
	// The selector table has to be complete before any markup is read: a
	// template renders components no file in its own unit imports.
	s.renderTrees()
	s.finish()
	return g, nil
}

// ---------- identity ----------

// declID is the node ID of a declaration. A class-shaped declaration is a type
// and a function-shaped one is a callable, whatever the classification decided
// to call it — otherwise a class component and a function component of the same
// name would land on the same node.
func declID(unit string, d *decl) string {
	switch {
	case d.Kind == model.KindMethod:
		return model.MethodID(unit, d.Owner, d.Name)
	case d.Class, d.Kind == model.KindInterface, d.Kind == model.KindEnum, d.Kind == model.KindType:
		return model.TypeID(unit, d.Name)
	default:
		return model.FuncID(unit, d.Name)
	}
}

// isTypeKind reports whether a node is something referenced rather than called.
func isTypeKind(d *decl) bool {
	return d.Class || d.Kind == model.KindInterface || d.Kind == model.KindEnum || d.Kind == model.KindType
}

// declared reports whether a declaration earns a node. A plain top-level
// constant is left out, exactly as the Go scanner leaves out `var` and `const`:
// the graph answers architectural questions, and a table of every literal in
// the tree would only bury them.
func declared(d *decl) bool { return d.Kind != model.KindVar }

// ---------- declarations ----------

func (s *scanner) declare() {
	for _, f := range s.parses {
		unit := f.src.Unit
		unitNode := s.g.AddNode(&model.Node{
			ID: model.PkgID(unit), Kind: model.KindPackage,
			// The label is the path without its extension, not the file name: half
			// a front end is called index, and the folder is what tells them apart.
			Name: strings.TrimSuffix(f.src.Rel, path.Ext(f.src.Rel)),
			Unit: unit, File: f.src.Rel,
			Lines: f.src.Lines, Vis: model.VisPublic,
		})
		s.g.Stats.Packages++

		for _, imp := range f.imports {
			to, ok := s.res.resolve(f.src.Rel, imp.Spec)
			conf := 0.95
			target := ""
			if ok {
				if tf, known := s.byRel[to]; known {
					target = model.PkgID(tf.src.Unit)
				}
			}
			if target == "" {
				// Anything that does not land on a scanned file is a dependency
				// outside the tree. It gets a node so the graph can show what the
				// module leans on, but nothing inside it is ever described.
				target = model.PkgID(imp.Spec)
				s.g.AddNode(&model.Node{
					ID: target, Kind: model.KindPackage, Name: imp.Spec,
					Unit: imp.Spec, External: true, Vis: model.VisPublic,
				})
				conf = 1.0
			}
			s.g.AddEdge(&model.Edge{
				From: unitNode.ID, To: target, Rel: model.RelImports,
				File: f.src.Rel, Line: imp.Line, Conf: conf,
			})
		}

		for _, d := range f.decls {
			if !declared(d) {
				continue
			}
			id := declID(unit, d)
			s.g.AddNode(&model.Node{
				ID: id, Kind: d.Kind, Name: d.Name, Unit: unit, Owner: d.Owner,
				File: f.src.Rel, Line: d.Line, Lines: d.Lines, Vis: d.Vis,
			})
			switch {
			case d.Kind == model.KindMethod:
				s.g.Stats.Methods++
			case isTypeKind(d):
				s.g.Stats.Types++
			default:
				s.g.Stats.Funcs++
			}
			s.g.AddEdge(&model.Edge{
				From: model.PkgID(unit), To: id, Rel: model.RelContains,
				File: f.src.Rel, Line: d.Line, Conf: 1.0,
			})
			s.claim(f, d, id)
			// A method also belongs to the class that declares it.
			if d.Owner != "" {
				if owner, ok := f.byName[d.Owner]; ok {
					s.g.AddEdge(&model.Edge{
						From: declID(unit, owner), To: id, Rel: model.RelContains,
						File: f.src.Rel, Line: d.Line, Conf: 1.0,
					})
				}
			}
		}
	}
}

// claim records the element names a declaration answers to and the template it
// renders. Both are global: a component is reachable from any template in the
// tree, not only from the files that import it, which is exactly why the
// component tree of such a framework cannot be read from imports.
func (s *scanner) claim(f *fileParse, d *decl, id string) {
	for _, sel := range d.Selectors {
		if prev, taken := s.selectors[sel]; taken {
			if prev != id {
				s.selectors[sel] = ""
			}
			continue
		}
		s.selectors[sel] = id
	}
	if d.Template == "" {
		return
	}
	rel := path.Join(path.Dir(f.src.Rel), d.Template)
	if _, taken := s.views[rel]; taken {
		s.views[rel] = ""
		return
	}
	s.views[rel] = id
}

// ---------- structural edges ----------

// link turns the heritage and dependency names collected per declaration into
// edges, now that every file's declarations are known.
func (s *scanner) link() {
	for _, f := range s.parses {
		unit := f.src.Unit
		for _, d := range f.decls {
			if !declared(d) {
				continue
			}
			from := declID(unit, d)
			for _, spec := range []struct {
				names []string
				rel   model.Rel
			}{
				{d.Extends, model.RelExtends},
				{d.Implements, model.RelImplements},
				{d.Uses, model.RelUses},
			} {
				for _, name := range spec.names {
					to, conf, ok := s.symbol(f, name)
					if !ok {
						continue
					}
					s.g.AddEdge(&model.Edge{
						From: from, To: to, Rel: spec.rel,
						File: f.src.Rel, Line: d.Line, Conf: conf,
					})
				}
			}
		}
	}
}

// symbol resolves a name used in a file to the node it names: first among the
// file's own declarations, then through what it imported. A name that leads
// outside the tree resolves to nothing — the graph does not hold react's
// symbols, only the fact that react was imported.
func (s *scanner) symbol(f *fileParse, name string) (id string, conf float64, ok bool) {
	if d, hit := f.byName[name]; hit && declared(d) {
		return declID(f.src.Unit, d), 0.90, true
	}
	b, hit := f.bindings[name]
	if !hit {
		return "", 0, false
	}
	rel, inside := s.res.resolve(f.src.Rel, b.Spec)
	if !inside {
		return "", 0, false
	}
	tf, known := s.byRel[rel]
	if !known {
		return "", 0, false
	}
	// The exported name is what the target file declares; the local name is only
	// how this file spells it. A default import knows neither, so it is left to
	// the module edge alone.
	target := b.Name
	if target == "*" || target == "default" {
		target = name
	}
	d, hit := tf.byName[target]
	if !hit {
		// The import landed on a file inside the tree that does not declare the
		// name. Normally a barrel re-exporting someone else's symbol, which this
		// scanner does not follow — a real hole in the graph, and the only thing
		// worth counting as unresolved.
		s.unresolved++
		return "", 0, false
	}
	if !declared(d) {
		return "", 0, false
	}
	return declID(tf.src.Unit, d), 0.85, true
}

// ---------- calls ----------

// reJSXTag matches the opening of a rendered component. A lowercase tag is an
// HTML element and never a node in this graph.
var reJSXTag = regexp.MustCompile(`<([A-Z][\w$]*)`)

// callables are the declarations whose body is worth walking. A class is not
// one: what it does happens in its methods, and attributing their calls to the
// class would erase the distinction.
func callable(d *decl) bool {
	switch d.Kind {
	case model.KindFunc, model.KindMethod, model.KindHook, model.KindComponent:
		return !d.Class
	}
	return false
}

// owned reports whether an offset falls inside a declaration that will be
// walked in its own right. A single-file component's body is the whole script,
// so without this every call written inside a function declared there would be
// recorded twice — once against the function, once against the component.
func (f *fileParse) owned(off int, self *decl) bool {
	for _, d := range f.decls {
		if d != self && callable(d) && d.Start <= off && off < d.Stop {
			return true
		}
	}
	return false
}

func (s *scanner) resolveCalls() {
	for _, f := range s.parses {
		unit := f.src.Unit
		for _, d := range f.decls {
			if !callable(d) || d.Body < 0 || d.Body >= d.Stop {
				continue
			}
			from := declID(unit, d)
			body := f.code[d.Body:min(d.Stop, len(f.code))]

			for _, m := range reCall.FindAllSubmatchIndex(body, -1) {
				name := string(body[m[2]:m[3]])
				if reservedNames[name] {
					continue
				}
				// A qualified call names a member of a value, and without types
				// there is no honest way to say which one.
				if prevSignificant(body, m[2]) == '.' {
					continue
				}
				if d.SFC && f.owned(d.Body+m[2], d) {
					continue
				}
				s.ref(f, from, name, model.RelCalls, f.lineAt(d.Body+m[2]))
			}

			// A rendered child is a real dependency between components, and the
			// only place the component tree is written down. It is a use and not a
			// call: nothing in the parent invokes the child, the framework does.
			if f.src.JSX() {
				for _, m := range reJSXTag.FindAllSubmatchIndex(body, -1) {
					s.ref(f, from, string(body[m[2]:m[3]]), model.RelUses, f.lineAt(d.Body+m[2]))
				}
			}
		}
		if f.sfc != nil {
			s.markupRefs(f, declID(unit, f.sfc))
		}
	}
}

// reMarkupTag matches the opening of an element in a single-file component's
// markup, including the hyphenated spelling a component may be written with.
var reMarkupTag = regexp.MustCompile(`<([A-Za-z][\w$]*(?:-[\w$]+)*)`)

// markupRefs records the components a single-file component renders. The markup
// is the only place a Vue, Svelte or Astro component tree is written down, and
// it sits outside every declaration, so it is read separately from the script.
func (s *scanner) markupRefs(f *fileParse, from string) {
	for _, sp := range f.markup {
		region := f.src.Src[sp.start:sp.end]
		for _, m := range reMarkupTag.FindAllSubmatchIndex(region, -1) {
			name := string(region[m[2]:m[3]])
			switch {
			case strings.ContainsAny(name, "-_"):
				name = pascal(name)
			case !isPascal(name):
				continue // a bare lowercase tag is an HTML element
			}
			if name == "" {
				continue
			}
			s.ref(f, from, name, model.RelUses, f.lineAt(sp.start+m[2]))
		}
	}
}

// ---------- component trees written in markup ----------

// reElement matches the opening of an element in a template, hyphenated names
// included: a selector is normally written that way.
var reElement = regexp.MustCompile(`<([a-zA-Z][\w-]*)`)

// renderTrees records the components each component renders, for the frameworks
// that keep a component's markup out of the module that declares it. Neither
// the import list nor the class body says which children a component has —
// Angular's are declared in a module elsewhere and named only by selector — so
// without reading the markup the component tree is simply absent.
//
// Two places hold that markup: the template file the component points at, and
// the decorator itself when the template is written inline.
func (s *scanner) renderTrees() {
	for _, f := range s.parses {
		unit := f.src.Unit
		for _, d := range f.decls {
			if d.Meta.end <= d.Meta.start || !declared(d) {
				continue
			}
			// Reading the whole decorator block is safe: nothing else a
			// component declares in it — a selector, a style, a change
			// detection strategy — contains a `<`.
			s.selectorRefs(declID(unit, d), f.src.Rel,
				f.src.Src[d.Meta.start:d.Meta.end], f.lineAt(d.Meta.start))
		}
	}
	for _, t := range s.templates {
		if from := s.views[t.Rel]; from != "" {
			s.selectorRefs(from, t.Rel, t.Src, 1)
		}
	}
}

// selectorRefs turns the tags in a block of markup into edges to the components
// that answer to them. A tag matching no selector is a plain HTML element and
// is dropped without being counted, the same rule ref follows.
func (s *scanner) selectorRefs(from, file string, markup []byte, baseLine int) {
	for _, m := range reElement.FindAllSubmatchIndex(markup, -1) {
		to := s.selectors[string(markup[m[2]:m[3]])]
		if to == "" {
			continue
		}
		s.g.AddEdge(&model.Edge{
			From: from, To: to, Rel: model.RelUses, File: file,
			Line: baseLine + bytes.Count(markup[:m[2]], []byte{'\n'}), Conf: 0.85,
		})
	}
}

// ref records one resolved reference. A name in front of parentheses is only a
// call when it landed on something callable: `new Store()` names a class, and
// the graph says so rather than claiming the class was invoked.
//
// A name that resolves to nothing is dropped without being counted: most of them
// are locals, parameters and browser globals, which were never going to be nodes.
// Counting those would drown the one number that means something — see symbol.
func (s *scanner) ref(f *fileParse, from, name string, rel model.Rel, line int) {
	to, conf, ok := s.symbol(f, name)
	if !ok {
		return
	}
	if n := s.g.Get(to); n != nil && isNodeType(n.Kind) {
		rel = model.RelUses
	}
	s.g.AddEdge(&model.Edge{From: from, To: to, Rel: rel, File: f.src.Rel, Line: line, Conf: conf})
}

func isNodeType(k model.Kind) bool {
	switch k {
	case model.KindClass, model.KindInterface, model.KindEnum, model.KindType, model.KindService:
		return true
	}
	return false
}

// prevSignificant is the last non-space byte before an offset, or 0 at the
// start of the region.
func prevSignificant(b []byte, i int) byte {
	for i--; i >= 0; i-- {
		switch b[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return b[i]
		}
	}
	return 0
}

func (s *scanner) finish() {
	s.g.Stats.Nodes = len(s.g.Nodes)
	s.g.Stats.Edges = len(s.g.Edges)
	s.g.Stats.Unresolved = s.unresolved
	for _, e := range s.g.Edges {
		if e.Rel == model.RelCalls {
			s.g.Stats.CallEdges++
		}
	}
}

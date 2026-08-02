package tsjs

import (
	"bytes"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// decl is one declaration found in a file.
type decl struct {
	Name  string
	Kind  model.Kind
	Vis   model.Vis
	Owner string // the class a method belongs to
	Line  int
	Lines int
	Start int // byte offset of the declaration
	Body  int // byte offset of the opening brace, -1 when there is no body
	Stop  int // byte offset just past its body

	// Class records that the declaration was written as a class, which decides
	// which ID it gets: a component may be either a class or a function, and the
	// two must not collide in the graph.
	Class bool

	// SFC records that the declaration stands for a whole single-file component,
	// so its body is a script holding other declarations rather than one block
	// of its own.
	SFC bool

	// Meta is the decorator block written above the declaration, empty when it
	// has none. It is a span rather than the parsed contents because it is read
	// off the raw source: the literals inside it hold the selector, the template
	// URL and sometimes the template itself, and blanking has erased all three
	// from the code the rest of the scanner works on.
	Meta span
	// Selectors are the element names a decorated component answers to, and
	// Template the markup file it renders. Both come out of Meta.
	Selectors []string
	Template  string

	Extends    []string
	Implements []string
	Uses       []string // types named in field annotations and constructor parameters
}

// importRef is one module specifier a file mentions.
type importRef struct {
	Spec string
	Line int
}

// binding is a name a file imported, and where it came from.
type binding struct {
	Spec string
	// Name is the exported name behind the local one. A namespace import binds
	// "*", a default import binds "default".
	Name string
}

// fileParse is everything the scanner read out of one file.
type fileParse struct {
	src      *srcFile
	code     []byte
	lineOff  []int
	lineDep  []int
	imports  []importRef
	bindings map[string]binding
	decls    []*decl
	byName   map[string]*decl
	hasJSX   bool

	// markup and sfc are set only for a single-file component: the spans holding
	// the tree it renders, and the declaration standing for the file itself.
	markup []span
	sfc    *decl
}

var (
	reImportFrom = regexp.MustCompile("(?s)\\bimport\\s+(?:type\\s+)?([^\"'`;]*?)\\bfrom\\s*([\"'`])")
	reImportBare = regexp.MustCompile("\\bimport\\s+([\"'`])")
	reImportCall = regexp.MustCompile("\\bimport\\s*\\(\\s*([\"'`])")
	reRequire    = regexp.MustCompile("\\brequire\\s*\\(\\s*([\"'`])")
	reExportFrom = regexp.MustCompile("(?s)\\bexport\\s+(?:\\*(?:\\s+as\\s+[\\w$]+)?|\\{[^}]*\\})\\s*\\bfrom\\s*([\"'`])")

	reClass = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:default[\t ]+)?(?:declare[\t ]+)?(?:abstract[\t ]+)?class[\t ]+([A-Za-z_$][\w$]*)`)
	reFunc  = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:default[\t ]+)?(?:declare[\t ]+)?(?:async[\t ]+)?function[\t ]*\*?[\t ]*([A-Za-z_$][\w$]*)`)
	reIface = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:declare[\t ]+)?interface[\t ]+([A-Za-z_$][\w$]*)`)
	reType  = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:declare[\t ]+)?type[\t ]+([A-Za-z_$][\w$]*)[\t ]*[=<]`)
	reEnum  = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:declare[\t ]+)?(?:const[\t ]+)?enum[\t ]+([A-Za-z_$][\w$]*)`)
	reVar   = regexp.MustCompile(`(?m)^[\t ]*(export[\t ]+)?(?:declare[\t ]+)?(?:const|let|var)[\t ]+([A-Za-z_$][\w$]*)`)

	reDecorator = regexp.MustCompile(`^[\t ]*@([A-Za-z_$][\w$]*)`)
	// Both are read off the raw source, where the literal still has its
	// contents; in the blanked code they are two empty quotes.
	reSelector    = regexp.MustCompile("(?m)^[\\t ]*selector[\\t ]*:[\\t ]*['\"`]([^'\"`]*)['\"`]")
	reTemplateURL = regexp.MustCompile("(?m)^[\\t ]*templateUrl[\\t ]*:[\\t ]*['\"`]([^'\"`]*)['\"`]")

	reMethod    = regexp.MustCompile(`(?m)^[\t ]*(?:(public|private|protected)[\t ]+)?(?:static[\t ]+)?(?:abstract[\t ]+)?(?:async[\t ]+)?(?:\*[\t ]*)?(?:(?:get|set)[\t ]+)?([A-Za-z_$][\w$]*)[\t ]*(?:<[^>{;()]*>)?[\t ]*\(`)
	reFieldType = regexp.MustCompile(`(?m)^[\t ]*(?:(?:public|private|protected|readonly|static|declare)[\t ]+)*[A-Za-z_$][\w$]*[\t ]*[?!]?[\t ]*:[\t ]*([A-Za-z_$][\w$]*)`)
	reParamType = regexp.MustCompile(`:[\t ]*([A-Za-z_$][\w$]*)`)
	// An explicit type argument sits between the name and the parenthesis, and
	// leaving it out would lose every `useState<Foo>()` in a typed codebase.
	reCall = regexp.MustCompile(`([A-Za-z_$][\w$]*)[\t ]*(?:<[^<>()]*>)?[\t ]*\(`)
)

// reservedNames never become a node or a call edge, however much they look like
// one. `if (x)` is not a call to something named if.
var reservedNames = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true,
	"return": true, "typeof": true, "instanceof": true, "function": true,
	"await": true, "yield": true, "new": true, "delete": true, "void": true,
	"in": true, "of": true, "do": true, "else": true, "case": true, "with": true,
	"import": true, "export": true, "class": true, "const": true, "let": true,
	"var": true, "enum": true, "interface": true, "type": true, "super": true,
	"this": true, "throw": true, "try": true, "finally": true,
}

// primitives are annotations that name no declaration anywhere, so treating one
// as an unresolved reference would only inflate the count.
var primitives = map[string]bool{
	"string": true, "number": true, "boolean": true, "bigint": true,
	"symbol": true, "object": true, "any": true, "unknown": true,
	"never": true, "void": true, "null": true, "undefined": true,
	"Array": true, "Promise": true, "Record": true, "Map": true, "Set": true,
	"Date": true, "RegExp": true, "Function": true, "Partial": true, "Readonly": true,
}

// usable reports whether a name read off an annotation is worth resolving.
func usable(name string) bool { return isIdent(name) && !reservedNames[name] && !primitives[name] }

// decoratorKinds maps a decorator to what the class it decorates really is.
// Nothing here is a branch on which framework the project declared: a decorated
// class states outright what it is, and the frameworks that use decorators —
// Angular, Stencil, Lit — agree on the names that matter. Supporting one more
// is a few more entries in this table.
var decoratorKinds = map[string]model.Kind{
	"Component":     model.KindComponent,
	"Directive":     model.KindComponent,
	"customElement": model.KindComponent,
	"Injectable":    model.KindService,
	"NgModule":      model.KindClass,
	"Pipe":          model.KindClass,
}

// parseFile reads one file into declarations, imports and bindings.
func parseFile(sf *srcFile) *fileParse {
	src := sf.Src
	var script, markup []span
	if sf.SFC() {
		src, script, markup = splitSFC(sf.Ext, src)
	}
	code, lits := blank(src)
	f := &fileParse{
		src:      sf,
		code:     code,
		markup:   markup,
		bindings: map[string]binding{},
		byName:   map[string]*decl{},
	}
	f.indexLines()
	f.hasJSX = bytes.Contains(code, []byte("/>")) || bytes.Contains(code, []byte("</"))
	f.readImports(lits)
	f.readDecls()
	if sf.SFC() {
		f.declareSFC(script)
	}
	return f
}

// declareSFC gives a single-file component the declaration its own file never
// writes down. The file is the component: that is what `import Card from
// './Card.vue'` binds to, and with nothing to bind to every use of it would
// dead-end.
//
// Its body is the script, because in these formats the top level of the script
// is the component's own code — a Vue setup block or a Svelte instance script
// runs on every render, and what it calls is what the component depends on.
func (f *fileParse) declareSFC(script []span) {
	name := pascal(strings.TrimSuffix(path.Base(f.src.Rel), path.Ext(f.src.Rel)))
	if name == "" {
		return
	}
	if _, taken := f.byName[name]; taken {
		return
	}
	d := &decl{
		Name: name, Kind: model.KindComponent, Vis: model.VisPublic, SFC: true,
		Line: 1, Lines: f.src.Lines, Start: 0, Body: -1, Stop: len(f.code),
	}
	if len(script) > 0 {
		d.Body, d.Stop = script[0].start, script[len(script)-1].end
	}
	f.decls = append(f.decls, d)
	f.byName[name] = d
	f.sfc = d
}

// indexLines records where each line starts and how deep in braces it begins.
// Depth is what separates a top-level declaration from a function nested inside
// another one, which no line-anchored pattern could tell apart on its own.
func (f *fileParse) indexLines() {
	f.lineOff = []int{0}
	f.lineDep = []int{0}
	depth := 0
	for i, c := range f.code {
		switch c {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '\n':
			f.lineOff = append(f.lineOff, i+1)
			f.lineDep = append(f.lineDep, depth)
		}
	}
}

// lineAt is the 1-based line holding a byte offset.
func (f *fileParse) lineAt(off int) int {
	i := sort.SearchInts(f.lineOff, off+1) - 1
	if i < 0 {
		return 1
	}
	return i + 1
}

// depthAt is the brace depth at the start of the line holding an offset.
func (f *fileParse) depthAt(off int) int { return f.lineDep[f.lineAt(off)-1] }

// ---------- imports ----------

func (f *fileParse) readImports(lits map[int]string) {
	add := func(quoteOff int) {
		spec, ok := lits[quoteOff]
		if !ok || spec == "" {
			return
		}
		f.imports = append(f.imports, importRef{Spec: spec, Line: f.lineAt(quoteOff)})
	}

	for _, m := range reImportFrom.FindAllSubmatchIndex(f.code, -1) {
		quote := m[4]
		spec, ok := lits[quote]
		if !ok || spec == "" {
			continue
		}
		f.imports = append(f.imports, importRef{Spec: spec, Line: f.lineAt(quote)})
		f.bind(string(f.code[m[2]:m[3]]), spec)
	}
	for _, re := range []*regexp.Regexp{reImportBare, reImportCall, reRequire, reExportFrom} {
		for _, m := range re.FindAllSubmatchIndex(f.code, -1) {
			add(m[2])
		}
	}
}

// bind records what an import clause brought into scope, so that a call to one
// of those names can be traced back to the module that exports it.
func (f *fileParse) bind(clause, spec string) {
	for _, part := range splitClause(clause) {
		switch {
		case part == "":
		case strings.HasPrefix(part, "{"):
			inner := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			for _, item := range strings.Split(inner, ",") {
				name, local := importedName(item)
				if local != "" {
					f.bindings[local] = binding{Spec: spec, Name: name}
				}
			}
		case strings.HasPrefix(part, "*"):
			if _, local, ok := strings.Cut(part, " as "); ok {
				if local = strings.TrimSpace(local); local != "" {
					f.bindings[local] = binding{Spec: spec, Name: "*"}
				}
			}
		default:
			if isIdent(part) {
				f.bindings[part] = binding{Spec: spec, Name: "default"}
			}
		}
	}
}

// splitClause breaks an import clause on the commas that separate its parts,
// leaving the commas inside a named group alone.
func splitClause(clause string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range clause {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(clause[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(clause[start:]))
}

// importedName splits `A as B` into the exported name and the local one.
func importedName(item string) (name, local string) {
	item = strings.TrimSpace(item)
	item = strings.TrimPrefix(item, "type ")
	item = strings.TrimSpace(item)
	if item == "" {
		return "", ""
	}
	if n, l, ok := strings.Cut(item, " as "); ok {
		n, l = strings.TrimSpace(n), strings.TrimSpace(l)
		if isIdent(n) && isIdent(l) {
			return n, l
		}
		return "", ""
	}
	if !isIdent(item) {
		return "", ""
	}
	return item, item
}

// ---------- declarations ----------

type match struct {
	off      int
	nameOff  int
	nameEnd  int
	exported bool
	kind     model.Kind
}

func (f *fileParse) readDecls() {
	var ms []match
	for _, spec := range []struct {
		re   *regexp.Regexp
		kind model.Kind
	}{
		{reClass, model.KindClass},
		{reIface, model.KindInterface},
		{reEnum, model.KindEnum},
		{reType, model.KindType},
		{reFunc, model.KindFunc},
		{reVar, model.KindVar},
	} {
		for _, m := range spec.re.FindAllSubmatchIndex(f.code, -1) {
			if f.depthAt(m[0]) != 0 {
				continue // nested in another declaration, not part of the module's surface
			}
			name := string(f.code[m[4]:m[5]])
			if reservedNames[name] {
				continue
			}
			ms = append(ms, match{
				off: m[0], nameOff: m[4], nameEnd: m[5],
				exported: m[2] >= 0, kind: spec.kind,
			})
		}
	}
	slices.SortFunc(ms, func(a, b match) int { return a.off - b.off })

	// Two patterns can fire on one line — `export const enum X` is both an enum
	// and a const. The earlier, more specific one wins.
	starts := make([]int, 0, len(ms))
	for _, m := range ms {
		if len(starts) > 0 && f.lineAt(starts[len(starts)-1]) == f.lineAt(m.off) {
			continue
		}
		starts = append(starts, m.off)
		f.declare(m, nextStart(ms, m.off))
	}
}

func nextStart(ms []match, after int) int {
	for _, m := range ms {
		if m.off > after {
			return m.off
		}
	}
	return -1
}

func (f *fileParse) declare(m match, next int) {
	name := string(f.code[m.nameOff:m.nameEnd])
	body, stop := f.bodyOf(m.nameEnd, next)
	d := &decl{
		Name:  name,
		Kind:  m.kind,
		Class: m.kind == model.KindClass,
		Vis:   model.VisOf(m.exported),
		Line:  f.lineAt(m.off),
		Start: m.off,
		Body:  body,
		Stop:  stop,
	}
	d.Lines = f.lineAt(stop) - d.Line + 1

	header := strings.TrimSpace(string(f.code[m.nameEnd:min(stop, m.nameEnd+400)]))
	switch m.kind {
	case model.KindClass:
		d.Extends, d.Implements = parseHeritage(header)
		decorators, at := f.decoratorsAbove(m.off)
		d.Kind = classKindOf(decorators, name, f.hasJSX)
		if at >= 0 && at < m.off {
			d.Meta = span{at, m.off}
			meta := f.src.Src[at:m.off]
			d.Selectors = elementSelectors(meta)
			if u := reTemplateURL.FindSubmatch(meta); u != nil {
				d.Template = string(u[1])
			}
		}
	case model.KindInterface:
		ext, _ := parseHeritage(header)
		d.Extends = ext
	case model.KindFunc, model.KindVar:
		d.Kind = f.callableKind(name, m.kind, header)
		// The parameter annotations are the declaration's contract, and in a
		// React tree that is the props type — the single most useful edge a
		// component has.
		if body > m.nameEnd {
			d.Uses = paramTypes(string(f.code[m.nameEnd:body]))
		}
	}

	f.decls = append(f.decls, d)
	if _, dup := f.byName[name]; !dup {
		f.byName[name] = d
	}
	// Only a declaration actually written as a class has members. A component may
	// be either, and reading a function component's body for members turns every
	// `useEffect(() => {` into a method of it.
	if body >= 0 && d.Class {
		f.readMembers(d, body, stop)
	}
}

// bodyOf finds where a declaration's body opens and where it ends. A statement
// that closes with a semicolon before any brace has no body at all, which is
// what keeps `const a = 1` from claiming the block of whatever follows it.
func (f *fileParse) bodyOf(from, next int) (body, stop int) {
	limit := len(f.code)
	if next > 0 && next < limit {
		limit = next
	}
	paren := 0
	for i := from; i < limit; i++ {
		switch f.code[i] {
		case '(', '[':
			paren++
		case ')', ']':
			paren--
		case ';':
			if paren <= 0 {
				return -1, i
			}
		case '{':
			if paren <= 0 {
				return i, matchBrace(f.code, i)
			}
		}
	}
	return -1, limit - 1
}

// matchBrace returns the offset just past the brace closing the one at open.
func matchBrace(code []byte, open int) int {
	depth := 0
	for i := open; i < len(code); i++ {
		switch code[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(code)
}

// parseHeritage reads the extends and implements clauses off a class or
// interface header. Generic arguments are dropped: `extends Base<Props>` is a
// relation to Base.
func parseHeritage(header string) (ext, impl []string) {
	if i := strings.IndexByte(header, '{'); i >= 0 {
		header = header[:i]
	}
	extPart, implPart := header, ""
	if i := strings.Index(header, "implements"); i >= 0 {
		extPart, implPart = header[:i], header[i+len("implements"):]
	}
	if i := strings.Index(extPart, "extends"); i >= 0 {
		ext = typeNames(extPart[i+len("extends"):])
	}
	return ext, typeNames(implPart)
}

// typeNames pulls the base identifiers out of a heritage clause.
func typeNames(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if i := strings.IndexAny(part, "<("); i >= 0 {
			part = part[:i]
		}
		// A qualified name keeps only its last segment: the graph holds Store,
		// not ns.Store.
		if i := strings.LastIndexByte(part, '.'); i >= 0 {
			part = part[i+1:]
		}
		if part = strings.TrimSpace(part); usable(part) {
			out = append(out, part)
		}
	}
	return out
}

// ---------- classification ----------

// classKindOf decides what a class really is from the decorators above it. A
// decorated class is never guessed at: the decorator states outright whether it
// is a component, a service or a module.
func classKindOf(decorators []string, name string, jsx bool) model.Kind {
	for _, dec := range decorators {
		if k, ok := decoratorKinds[dec]; ok {
			return k
		}
	}
	// An undecorated class in a file that renders markup is a class component.
	if jsx && isPascal(name) {
		return model.KindComponent
	}
	return model.KindClass
}

// decoratorsAbove collects the decorators written on the lines just before a
// declaration, stopping at the first line that is neither a decorator nor blank,
// and returns where the topmost one starts — -1 when there is none. The offset
// is what lets the metadata be read back out of the raw source.
//
// A decorator's argument spans as many lines as it likes, and those lines may
// say anything at all: an inline template is a block of markup sitting between
// the @ and the class. They are skipped by brace depth rather than by what they
// look like, which is the only rule that holds for arbitrary content.
func (f *fileParse) decoratorsAbove(off int) (names []string, at int) {
	at = -1
	base := f.depthAt(off)
	for line := f.lineAt(off) - 1; line >= 1; line-- {
		text := f.lineText(line)
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || f.lineDep[line-1] > base {
			continue
		}
		m := reDecorator.FindStringSubmatch(text)
		if m == nil {
			// An argument list closing on its own line is back at the
			// declaration's depth; the @ that opened it is further up.
			if strings.HasPrefix(trimmed, ")") || strings.HasPrefix(trimmed, "]") {
				continue
			}
			break
		}
		names = append(names, m[1])
		at = f.lineOff[line-1]
	}
	return names, at
}

// elementSelectors reads the element names a decorated component answers to.
// Where a framework matches components by selector, the markup names them that
// way and never by their class, so this is the only bridge between a tag in a
// template and a node in the graph.
//
// Only the element form is kept. An attribute selector `[appHighlight]` and a
// class selector `.btn` are matched against something other than a tag name,
// and finding their uses would take a scan of every attribute in the tree.
func elementSelectors(meta []byte) []string {
	m := reSelector.FindSubmatch(meta)
	if m == nil {
		return nil
	}
	var out []string
	for _, part := range strings.Split(string(m[1]), ",") {
		if part = strings.TrimSpace(part); isElementName(part) {
			out = append(out, part)
		}
	}
	return out
}

// isElementName reports whether a selector is a plain tag name.
func isElementName(s string) bool {
	if s == "" || !(s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c == '-' || c == '_' ||
			c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !ok {
			return false
		}
	}
	return true
}

func (f *fileParse) lineText(line int) string {
	if line < 1 || line > len(f.lineOff) {
		return ""
	}
	start := f.lineOff[line-1]
	end := len(f.code)
	if line < len(f.lineOff) {
		end = f.lineOff[line] - 1
	}
	if end < start {
		return ""
	}
	return string(f.code[start:end])
}

// componentFactories wrap a component and give one back, so what they return is
// still a component however it is named. They are matched by name because that
// is how they are written; the project's declared framework is never consulted.
var componentFactories = []string{
	"memo(", "forwardRef(", "observer(", "defineComponent(",
	"React.FC", "React.FunctionComponent",
}

// callableKind separates the three things a top-level function can be in a
// front end: a hook, a component, or a plain function.
//
// The component rule is deliberately narrow. A PascalCase name alone proves
// nothing — `export const AppContext = createContext()` is not a component — so
// the declaration must also be a function, and the file must actually contain
// JSX. Everything that fails those tests stays a plain function, which is the
// honest answer.
func (f *fileParse) callableKind(name string, kind model.Kind, header string) model.Kind {
	callable := kind == model.KindFunc || isFunctionInitializer(header)
	if !callable {
		return model.KindVar
	}
	if isHookName(name) {
		return model.KindHook
	}
	if !isPascal(name) {
		return model.KindFunc
	}
	for _, factory := range componentFactories {
		if strings.Contains(header, factory) {
			return model.KindComponent
		}
	}
	if f.hasJSX {
		return model.KindComponent
	}
	return model.KindFunc
}

// isFunctionInitializer reports whether what follows `=` is a function rather
// than a value.
func isFunctionInitializer(header string) bool {
	_, rhs, ok := strings.Cut(header, "=")
	if !ok {
		return false
	}
	rhs = strings.TrimSpace(rhs)
	rhs = strings.TrimPrefix(rhs, "async ")
	rhs = strings.TrimSpace(rhs)
	switch {
	case strings.HasPrefix(rhs, "function"):
		return true
	case strings.HasPrefix(rhs, "("), strings.HasPrefix(rhs, "<"):
		return strings.Contains(header, "=>")
	}
	for _, factory := range componentFactories {
		if strings.HasPrefix(rhs, factory) {
			return true
		}
	}
	// A single-parameter arrow needs no parentheses: `const f = x => x + 1`.
	ident, rest, ok := strings.Cut(rhs, "=>")
	return ok && isIdent(strings.TrimSpace(ident)) && rest != ""
}

// isHookName applies the use-prefix convention, which React enforces with a
// lint rule and Vue follows for composables. It is a naming rule rather than a
// framework's API, so the graph agrees with the tooling the team already runs
// whichever of the two it is.
func isHookName(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "use") && name[3] >= 'A' && name[3] <= 'Z'
}

func isPascal(name string) bool { return name != "" && name[0] >= 'A' && name[0] <= 'Z' }

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := c == '_' || c == '$' ||
			c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			i > 0 && c >= '0' && c <= '9'
		if !ok {
			return false
		}
	}
	return true
}

// ---------- class members ----------

// readMembers records a class's methods and the types it depends on. In a
// framework built on dependency injection the constructor's parameter list is
// the dependency list, which makes it the most load-bearing line in the file.
func (f *fileParse) readMembers(owner *decl, body, stop int) {
	inner := f.code[body:stop]
	base := f.depthAt(body) + 1

	for _, m := range reMethod.FindAllSubmatchIndex(inner, -1) {
		off := body + m[0]
		if f.depthAt(off) != base {
			continue
		}
		name := string(inner[m[4]:m[5]])
		if reservedNames[name] {
			continue
		}
		mBody, mStop := f.bodyOf(body+m[5], -1)
		d := &decl{
			Name:  name,
			Kind:  model.KindMethod,
			Owner: owner.Name,
			Vis:   memberVis(inner, m),
			Line:  f.lineAt(off),
			Start: off,
			Body:  mBody,
			Stop:  mStop,
		}
		d.Lines = f.lineAt(mStop) - d.Line + 1
		f.decls = append(f.decls, d)

		if name == "constructor" {
			owner.Uses = append(owner.Uses, paramTypes(string(inner[m[5]:min(len(inner), m[5]+1200)]))...)
		}
	}

	for _, m := range reFieldType.FindAllSubmatchIndex(inner, -1) {
		if f.depthAt(body+m[0]) != base {
			continue
		}
		if t := string(inner[m[2]:m[3]]); usable(t) {
			owner.Uses = append(owner.Uses, t)
		}
	}
	slices.Sort(owner.Uses)
	owner.Uses = slices.Compact(owner.Uses)
}

func memberVis(inner []byte, m []int) model.Vis {
	if m[2] < 0 {
		return model.VisPublic // no modifier means public in both languages
	}
	switch string(inner[m[2]:m[3]]) {
	case "private":
		return model.VisPrivate
	case "protected":
		return model.VisProtected
	}
	return model.VisPublic
}

// paramTypes reads the annotated types out of a parameter list.
func paramTypes(text string) []string {
	if i := strings.IndexByte(text, ')'); i >= 0 {
		text = text[:i]
	}
	var out []string
	for _, m := range reParamType.FindAllStringSubmatch(text, -1) {
		if t := m[1]; usable(t) {
			out = append(out, t)
		}
	}
	return out
}

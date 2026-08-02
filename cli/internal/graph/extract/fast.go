package extract

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// parsedFile is a file already turned into an AST, with its import map ready.
type parsedFile struct {
	src     *srcFile
	file    *ast.File
	imports map[string]string // local alias -> import path
}

// pkgUnit groups all files belonging to the same package.
type pkgUnit struct {
	Path string
	Name string
	Dir  string
	// Context comes from a //gofi:context on the package clause and is the
	// default for every symbol declared here.
	Context string
	Files   []*parsedFile
	LOC     int
}

// scanner holds the state shared between fast mode and deep mode.
type scanner struct {
	opt      Options
	root     string
	fset     *token.FileSet
	pkgs     map[string]*pkgUnit
	pkgOrder []string
	g        *model.Graph

	funcsByPkg    map[string]map[string]string // package -> name -> node ID
	typesByPkg    map[string]map[string]string
	methodsByName map[string][]string

	unresolved int
	ambiguous  int
	extCalls   int
}

// builtins are language calls that do not become edges.
var builtins = map[string]bool{
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true,
	"bool": true, "byte": true, "rune": true, "string": true, "error": true,
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "float32": true, "float64": true, "any": true,
}

// newScanner discovers and parses every file of the module.
func newScanner(opt Options) (*scanner, error) {
	mods, err := loadModules(opt.Root)
	if err != nil {
		return nil, err
	}
	files, err := discover(opt, mods)
	if err != nil {
		return nil, err
	}
	mode := "fast"
	if opt.Deep {
		mode = "deep"
	}
	// The root module names the graph even when a go.work brought siblings in:
	// it is the one the scan was pointed at.
	g := model.New(mods[0].Path, opt.Root, mode)
	// This package only ever reads Go. Saying so in the graph is what lets a
	// reader tell one scope's graph from another's without knowing where it came
	// from — the same thing an external extractor declares in its header.
	g.Language = "go"
	s := &scanner{
		opt:           opt,
		root:          opt.Root,
		fset:          token.NewFileSet(),
		pkgs:          map[string]*pkgUnit{},
		g:             g,
		funcsByPkg:    map[string]map[string]string{},
		typesByPkg:    map[string]map[string]string{},
		methodsByName: map[string][]string{},
	}

	// Parsing is parallel: token.FileSet is safe for concurrent use.
	type result struct {
		sf  *srcFile
		af  *ast.File
		err error
	}
	results := make([]result, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for i, sf := range files {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			af, err := parser.ParseFile(s.fset, sf.Path, sf.Src, parser.ParseComments|parser.SkipObjectResolution)
			results[i] = result{sf: sf, af: af, err: err}
		})
	}
	wg.Wait()

	for _, r := range results {
		if r.af == nil {
			continue // a file with a syntax error is skipped, it does not break the scan
		}
		u, ok := s.pkgs[r.sf.PkgPath]
		if !ok {
			u = &pkgUnit{Path: r.sf.PkgPath, Name: r.af.Name.Name, Dir: r.sf.Dir}
			s.pkgs[r.sf.PkgPath] = u
			s.pkgOrder = append(s.pkgOrder, r.sf.PkgPath)
		}
		if u.Context == "" {
			u.Context = contextOf(r.af.Doc)
		}
		u.Files = append(u.Files, &parsedFile{src: r.sf, file: r.af})
		u.LOC += r.sf.Lines
		r.sf.Src = nil // the raw contents are already an AST; releasing halves memory use
		s.g.Files[r.sf.Rel] = model.FileInfo{
			Path: r.sf.Rel, Hash: r.sf.Hash, Unit: r.sf.PkgPath, Lines: r.sf.Lines,
		}
		s.g.Stats.Files++
		s.g.Stats.LOC += r.sf.Lines
	}
	s.resolveImports()
	return s, nil
}

// resolveImports builds, for each file, the alias -> import path map. When the
// imported package belongs to the module itself we use the real name declared
// in it, instead of guessing from the folder name.
func (s *scanner) resolveImports() {
	for _, u := range s.pkgs {
		for _, pf := range u.Files {
			pf.imports = map[string]string{}
			for _, imp := range pf.file.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				alias := ""
				switch {
				case imp.Name != nil:
					alias = imp.Name.Name
				default:
					if iu, ok := s.pkgs[p]; ok {
						alias = iu.Name
					} else {
						alias = path.Base(p)
					}
				}
				if alias == "_" || alias == "." {
					continue
				}
				pf.imports[alias] = p
			}
		}
	}
}

// isInternal reports whether the package belongs to the analyzed module.
func (s *scanner) isInternal(pkgPath string) bool {
	_, ok := s.pkgs[pkgPath]
	return ok
}

// Fast runs the purely syntactic scan.
func Fast(opt Options) (*model.Graph, error) {
	s, err := newScanner(opt)
	if err != nil {
		return nil, err
	}
	s.declare()
	s.resolveCalls()
	s.finish()
	return s.g, nil
}

// declare creates the package, type, func and method nodes, plus the
// structural edges (contains, imports, embeds, uses).
func (s *scanner) declare() {
	for _, pkgPath := range s.pkgOrder {
		u := s.pkgs[pkgPath]
		pkgNode := s.g.AddNode(&model.Node{
			ID: model.PkgID(pkgPath), Kind: model.KindPackage,
			Name: u.Name, Unit: pkgPath, Lines: u.LOC, Vis: model.VisPublic,
			Context: u.Context,
		})
		s.g.Stats.Packages++
		s.funcsByPkg[pkgPath] = map[string]string{}
		s.typesByPkg[pkgPath] = map[string]string{}

		for _, pf := range u.Files {
			// Import edges: always exact.
			for _, imp := range pf.file.Imports {
				p, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					continue
				}
				if !s.isInternal(p) {
					s.g.AddNode(&model.Node{
						ID: model.PkgID(p), Kind: model.KindPackage,
						Name: path.Base(p), Unit: p, External: true, Vis: model.VisPublic,
					})
				}
				s.g.AddEdge(&model.Edge{
					From: pkgNode.ID, To: model.PkgID(p), Rel: model.RelImports,
					File: pf.src.Rel, Line: s.line(imp.Pos()), Conf: 1.0,
				})
			}

			for _, decl := range pf.file.Decls {
				switch d := decl.(type) {
				case *ast.GenDecl:
					if d.Tok == token.TYPE {
						s.declareTypes(u, pf, d)
					}
				case *ast.FuncDecl:
					s.declareFunc(u, pf, d)
				}
			}
		}
	}
}

func (s *scanner) declareTypes(u *pkgUnit, pf *parsedFile, d *ast.GenDecl) {
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		kind := model.KindType
		sig := ""
		switch t := ts.Type.(type) {
		case *ast.StructType:
			kind = model.KindStruct
			sig = "struct com " + strconv.Itoa(len(t.Fields.List)) + " campo(s)"
		case *ast.InterfaceType:
			kind = model.KindInterface
			var ms []string
			for _, m := range t.Methods.List {
				for _, n := range m.Names {
					ms = append(ms, n.Name)
				}
			}
			sig = "interface{" + strings.Join(ms, ", ") + "}"
		default:
			sig = render(s.fset, ts.Type)
		}
		doc := firstDocLine(ts.Doc)
		if doc == "" {
			doc = firstDocLine(d.Doc)
		}
		id := model.TypeID(u.Path, ts.Name.Name)
		s.g.AddNode(&model.Node{
			ID: id, Kind: kind, Name: ts.Name.Name, Unit: u.Path,
			File: pf.src.Rel, Line: s.line(ts.Pos()), Lines: s.span(ts.Pos(), ts.End()),
			Vis: model.VisOf(ts.Name.IsExported()), Doc: doc, Sig: sig,
			Context: cmp.Or(contextOf(ts.Doc), contextOf(d.Doc), u.Context),
		})
		s.g.Stats.Types++
		s.typesByPkg[u.Path][ts.Name.Name] = id
		s.g.AddEdge(&model.Edge{
			From: model.PkgID(u.Path), To: id, Rel: model.RelContains,
			File: pf.src.Rel, Line: s.line(ts.Pos()), Conf: 1.0,
		})

		// Embedded fields and regular fields become separate edges: embeds
		// carries behavior inheritance, uses carries only a type dependency.
		switch t := ts.Type.(type) {
		case *ast.StructType:
			for _, f := range t.Fields.List {
				refs := s.typeRefs(pf, u.Path, f.Type)
				rel := model.RelUses
				if len(f.Names) == 0 {
					rel = model.RelEmbeds
				}
				for _, ref := range refs {
					s.g.AddEdge(&model.Edge{
						From: id, To: ref, Rel: rel,
						File: pf.src.Rel, Line: s.line(f.Pos()), Conf: 1.0,
					})
				}
			}
		case *ast.InterfaceType:
			for _, m := range t.Methods.List {
				if len(m.Names) == 0 {
					for _, ref := range s.typeRefs(pf, u.Path, m.Type) {
						s.g.AddEdge(&model.Edge{
							From: id, To: ref, Rel: model.RelEmbeds,
							File: pf.src.Rel, Line: s.line(m.Pos()), Conf: 1.0,
						})
					}
				}
			}
		}
	}
}

func (s *scanner) declareFunc(u *pkgUnit, pf *parsedFile, d *ast.FuncDecl) {
	name := d.Name.Name
	var id, recv string
	kind := model.KindFunc
	if d.Recv != nil && len(d.Recv.List) > 0 {
		recv = recvTypeName(d.Recv.List[0].Type)
		if recv == "" {
			return
		}
		kind = model.KindMethod
		id = model.MethodID(u.Path, recv, name)
		s.methodsByName[name] = append(s.methodsByName[name], id)
		s.g.Stats.Methods++
	} else {
		id = model.FuncID(u.Path, name)
		s.funcsByPkg[u.Path][name] = id
		s.g.Stats.Funcs++
	}
	s.g.AddNode(&model.Node{
		ID: id, Kind: kind, Name: name, Unit: u.Path, Owner: recv,
		File: pf.src.Rel, Line: s.line(d.Pos()), Lines: s.span(d.Pos(), d.End()),
		Vis: model.VisOf(d.Name.IsExported()), Doc: firstDocLine(d.Doc),
		Sig:     render(s.fset, d.Type),
		Context: cmp.Or(contextOf(d.Doc), u.Context),
	})
	s.g.AddEdge(&model.Edge{
		From: model.PkgID(u.Path), To: id, Rel: model.RelContains,
		File: pf.src.Rel, Line: s.line(d.Pos()), Conf: 1.0,
	})
	// A method also belongs to its receiver type.
	if recv != "" {
		if tid, ok := s.typesByPkg[u.Path][recv]; ok {
			s.g.AddEdge(&model.Edge{
				From: tid, To: id, Rel: model.RelContains,
				File: pf.src.Rel, Line: s.line(d.Pos()), Conf: 1.0,
			})
		}
	}
	// Signature: parameters and results are real type dependencies.
	for _, fl := range []*ast.FieldList{d.Type.Params, d.Type.Results} {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			for _, ref := range s.typeRefs(pf, u.Path, f.Type) {
				s.g.AddEdge(&model.Edge{
					From: id, To: ref, Rel: model.RelUses,
					File: pf.src.Rel, Line: s.line(f.Pos()), Conf: 1.0,
				})
			}
		}
	}
}

// resolveCalls walks function bodies and links caller to callee. In fast mode
// the resolution is heuristic and the confidence reflects that.
func (s *scanner) resolveCalls() {
	for _, pkgPath := range s.pkgOrder {
		u := s.pkgs[pkgPath]
		for _, pf := range u.Files {
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				from := s.declID(u.Path, fd)
				if from == "" {
					continue
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					s.resolveCall(u, pf, from, call)
					return true
				})
			}
		}
	}
}

func (s *scanner) resolveCall(u *pkgUnit, pf *parsedFile, from string, call *ast.CallExpr) {
	line := s.line(call.Pos())
	switch fun := unwrap(call.Fun).(type) {

	case *ast.Ident:
		if builtins[fun.Name] {
			return
		}
		if id, ok := s.funcsByPkg[u.Path][fun.Name]; ok {
			s.g.AddEdge(&model.Edge{From: from, To: id, Rel: model.RelCalls,
				File: pf.src.Rel, Line: line, Conf: 0.90})
			return
		}
		// A type name in front of parentheses is a conversion, not a call.
		if id, ok := s.typesByPkg[u.Path][fun.Name]; ok {
			s.g.AddEdge(&model.Edge{From: from, To: id, Rel: model.RelUses,
				File: pf.src.Rel, Line: line, Conf: 0.90})
			return
		}
		s.unresolved++

	case *ast.SelectorExpr:
		if x, ok := unwrap(fun.X).(*ast.Ident); ok {
			if impPath, ok := pf.imports[x.Name]; ok {
				// Package-qualified call: the target is certain.
				if id, ok := s.funcsByPkg[impPath][fun.Sel.Name]; ok {
					s.g.AddEdge(&model.Edge{From: from, To: id, Rel: model.RelCalls,
						File: pf.src.Rel, Line: line, Conf: 0.95})
					return
				}
				if s.isInternal(impPath) {
					if id, ok := s.typesByPkg[impPath][fun.Sel.Name]; ok {
						s.g.AddEdge(&model.Edge{From: from, To: id, Rel: model.RelUses,
							File: pf.src.Rel, Line: line, Conf: 0.90})
						return
					}
					s.unresolved++
					return
				}
				s.extCalls++
				return
			}
		}
		// Method call on a value: without types, we can only match by name.
		cands := s.methodsByName[fun.Sel.Name]
		switch {
		case len(cands) == 1:
			s.g.AddEdge(&model.Edge{From: from, To: cands[0], Rel: model.RelCalls,
				File: pf.src.Rel, Line: line, Conf: 0.55})
		case len(cands) > 1:
			s.ambiguous++
		default:
			s.extCalls++
		}
	}
}

// declID returns the node ID corresponding to a function declaration.
func (s *scanner) declID(pkgPath string, fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		recv := recvTypeName(fd.Recv.List[0].Type)
		if recv == "" {
			return ""
		}
		return model.MethodID(pkgPath, recv, fd.Name.Name)
	}
	return model.FuncID(pkgPath, fd.Name.Name)
}

// typeRefs extracts the internal types referenced by a type expression.
func (s *scanner) typeRefs(pf *parsedFile, pkgPath string, expr ast.Expr) []string {
	var out []string
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch t := e.(type) {
		case *ast.Ident:
			if id, ok := s.typesByPkg[pkgPath][t.Name]; ok {
				out = append(out, id)
			}
		case *ast.SelectorExpr:
			if x, ok := t.X.(*ast.Ident); ok {
				if impPath, ok := pf.imports[x.Name]; ok && s.isInternal(impPath) {
					if id, ok := s.typesByPkg[impPath][t.Sel.Name]; ok {
						out = append(out, id)
					}
				}
			}
		case *ast.StarExpr:
			walk(t.X)
		case *ast.ArrayType:
			walk(t.Elt)
		case *ast.Ellipsis:
			walk(t.Elt)
		case *ast.MapType:
			walk(t.Key)
			walk(t.Value)
		case *ast.ChanType:
			walk(t.Value)
		case *ast.IndexExpr:
			walk(t.X)
			walk(t.Index)
		case *ast.IndexListExpr:
			walk(t.X)
			for _, i := range t.Indices {
				walk(i)
			}
		case *ast.FuncType:
			for _, fl := range []*ast.FieldList{t.Params, t.Results} {
				if fl == nil {
					continue
				}
				for _, f := range fl.List {
					walk(f.Type)
				}
			}
		}
	}
	walk(expr)
	return out
}

// finish closes out the scan statistics.
func (s *scanner) finish() {
	s.g.Stats.Nodes = len(s.g.Nodes)
	s.g.Stats.Edges = len(s.g.Edges)
	s.g.Stats.Unresolved = s.unresolved
	s.g.Stats.Ambiguous = s.ambiguous
	for _, e := range s.g.Edges {
		if e.Rel == model.RelCalls {
			s.g.Stats.CallEdges++
		}
	}
}

// ---------- utilities ----------

func (s *scanner) line(p token.Pos) int { return s.fset.Position(p).Line }

func (s *scanner) span(from, to token.Pos) int {
	a, b := s.fset.Position(from).Line, s.fset.Position(to).Line
	if b < a {
		return 0
	}
	return b - a + 1
}

// unwrap strips generic instantiations to reach the base identifier.
func unwrap(e ast.Expr) ast.Expr {
	for {
		switch t := e.(type) {
		case *ast.ParenExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		default:
			return e
		}
	}
}

// recvTypeName extracts the receiver type name of a method.
func recvTypeName(e ast.Expr) string {
	switch t := unwrap(e).(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	}
	return ""
}

func render(fset *token.FileSet, x ast.Node) string {
	var b strings.Builder
	if err := printer.Fprint(&b, fset, x); err != nil {
		return ""
	}
	s := strings.Join(strings.Fields(b.String()), " ")
	if len(s) > 180 {
		s = s[:180] + "..."
	}
	return s
}

func firstDocLine(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	t := strings.TrimSpace(cg.Text())
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[:i]
	}
	if len(t) > 160 {
		t = t[:160] + "..."
	}
	return t
}

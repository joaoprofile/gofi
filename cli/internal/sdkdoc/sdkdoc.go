// Package sdkdoc writes the API reference of a Go SDK for the agents, from the
// SDK's own source.
//
// The reference used to be written by hand, in another repository, with no tie
// to the version of the code it described — and it drifted: functions renamed,
// packages added, a whole service orchestrator the agents never heard of. A
// reference generated from the checkout the project pinned cannot drift: it
// is the code's own doc comments and signatures, at that version.
//
// One file per package, one section per exported symbol — the granularity the
// document index ranks, so a search lands on criteria.From, not on the page
// that mentions it somewhere.
package sdkdoc

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// IndexFile lists the packages of the reference, one line each.
const IndexFile = "INDEX.md"

// skipDirs are never documented: runnable examples are installed as examples,
// internal packages are not the SDK's API, test data is not code.
var skipDirs = map[string]bool{"examples": true, "internal": true, "testdata": true, "vendor": true, ".git": true}

// Package is one documented package.
type Package struct {
	ImportPath string
	Name       string
	Synopsis   string
	File       string // the reference file, relative to the output directory
}

// Generate writes the reference of every package under srcDir into outDir,
// replacing what an earlier run wrote there. version names the SDK version
// the reference describes; it heads every file.
func Generate(srcDir, outDir, version string) ([]Package, error) {
	var pkgs []Package
	err := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if p != srcDir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		pkg, body, ok, err := document(srcDir, p, version)
		if err != nil || !ok {
			return err
		}
		pkgs = append(pkgs, pkg)
		return writeFile(filepath.Join(outDir, pkg.File), body)
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
	if err := writeFile(filepath.Join(outDir, IndexFile), index(pkgs, version)); err != nil {
		return nil, err
	}
	return pkgs, prune(outDir, pkgs)
}

// document renders one directory's package, if it has one worth documenting.
func document(srcDir, dir, version string) (Package, []byte, bool, error) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Package{}, nil, false, err
	}
	var files []*ast.File
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			return Package{}, nil, false, fmt.Errorf("parse %s: %w", filepath.Join(dir, n), err)
		}
		if f.Name.Name == "main" {
			return Package{}, nil, false, nil
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return Package{}, nil, false, nil
	}
	importPath, err := importPathOf(srcDir, dir)
	if err != nil {
		return Package{}, nil, false, err
	}
	d, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil {
		return Package{}, nil, false, err
	}
	if !exported(d) {
		return Package{}, nil, false, nil
	}
	rel, _ := filepath.Rel(srcDir, dir)
	rel = filepath.ToSlash(rel)
	pkg := Package{
		ImportPath: importPath,
		Name:       d.Name,
		Synopsis:   d.Synopsis(d.Doc),
		File:       fileName(rel),
	}
	return pkg, render(fset, d, rel, version), true, nil
}

func exported(d *doc.Package) bool {
	return len(d.Consts)+len(d.Vars)+len(d.Funcs)+len(d.Types) > 0
}

// fileName names a package's reference after its directory: sqln/criteria is
// sqln-criteria.md; the repository root package is the SDK's name.
func fileName(rel string) string {
	if rel == "." || rel == "" {
		return "root.md"
	}
	return strings.ReplaceAll(rel, "/", "-") + ".md"
}

// importPathOf is the module path of the nearest go.mod above dir, joined with
// the rest of the way down.
func importPathOf(srcDir, dir string) (string, error) {
	for cur := dir; ; cur = filepath.Dir(cur) {
		if mod, ok := modulePath(filepath.Join(cur, "go.mod")); ok {
			rel, _ := filepath.Rel(cur, dir)
			if rel == "." {
				return mod, nil
			}
			return mod + "/" + filepath.ToSlash(rel), nil
		}
		if cur == srcDir || cur == filepath.Dir(cur) {
			return "", fmt.Errorf("%s: no go.mod above it", dir)
		}
	}
}

func modulePath(gomod string) (string, bool) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); strings.HasPrefix(line, "module ") {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "module ")), `"`), true
		}
	}
	return "", false
}

func render(fset *token.FileSet, d *doc.Package, rel, version string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimPrefix(rel, "./"))
	fmt.Fprintf(&b, "`import \"%s\"` · %s\n\n", d.ImportPath, versionNote(version))
	b.WriteString("> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.\n\n")
	if text := strings.TrimSpace(d.Doc); text != "" {
		b.WriteString(text + "\n\n")
	}
	values := func(title string, vs []*doc.Value) {
		if len(vs) == 0 {
			return
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for _, v := range vs {
			code(&b, fset, v.Decl)
			comment(&b, v.Doc)
		}
	}
	values("Constantes", d.Consts)
	values("Variáveis", d.Vars)
	if len(d.Funcs) > 0 {
		b.WriteString("## Funções\n\n")
		for _, f := range d.Funcs {
			function(&b, fset, d.Name+"."+f.Name, f)
		}
	}
	for _, t := range d.Types {
		fmt.Fprintf(&b, "## %s.%s\n\n", d.Name, t.Name)
		code(&b, fset, t.Decl)
		comment(&b, t.Doc)
		for _, v := range t.Consts {
			code(&b, fset, v.Decl)
			comment(&b, v.Doc)
		}
		for _, v := range t.Vars {
			code(&b, fset, v.Decl)
			comment(&b, v.Doc)
		}
		for _, f := range t.Funcs {
			function(&b, fset, d.Name+"."+f.Name, f)
		}
		for _, m := range t.Methods {
			function(&b, fset, t.Name+"."+m.Name, m)
		}
	}
	return b.Bytes()
}

func function(b *bytes.Buffer, fset *token.FileSet, title string, f *doc.Func) {
	fmt.Fprintf(b, "### %s\n\n", title)
	decl := *f.Decl
	decl.Body = nil
	code(b, fset, &decl)
	comment(b, f.Doc)
}

func code(b *bytes.Buffer, fset *token.FileSet, node any) {
	var src bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&src, fset, node); err != nil {
		return
	}
	fmt.Fprintf(b, "```go\n%s\n```\n\n", strings.TrimSpace(src.String()))
}

func comment(b *bytes.Buffer, text string) {
	if text = strings.TrimSpace(text); text != "" {
		b.WriteString(text + "\n\n")
	}
}

func versionNote(version string) string {
	if version == "" {
		return "versão do checkout"
	}
	return "SDK " + version
}

func index(pkgs []Package, version string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Referência de API do SDK — %s\n\n", versionNote(version))
	b.WriteString("> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.\n")
	b.WriteString("> Procure um símbolo com `gofi find --in sdk \"<nome>\"` ou abra o pacote com `gofi show`.\n\n")
	b.WriteString("| Pacote | Arquivo | Resumo |\n|---|---|---|\n")
	for _, p := range pkgs {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", p.ImportPath, p.File, strings.ReplaceAll(p.Synopsis, "|", "\\|"))
	}
	return b.Bytes()
}

func writeFile(path string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// prune removes reference files for packages the SDK no longer has, so a
// package deleted upstream does not live on in the reference.
func prune(outDir string, pkgs []Package) error {
	keep := map[string]bool{IndexFile: true}
	for _, p := range pkgs {
		keep[p.File] = true
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !keep[e.Name()] {
			if err := os.Remove(filepath.Join(outDir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

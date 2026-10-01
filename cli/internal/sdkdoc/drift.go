package sdkdoc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Exports is every exported identifier of an SDK, by package name. Methods
// count under their package, which is how prose cites them (sqln.List).
type Exports map[string]map[string]bool

// ReadExports collects the exported identifiers under srcDir, examples and
// internal packages left out, as the reference does.
func ReadExports(srcDir string) (Exports, error) {
	out := Exports{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(srcDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != srcDir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil || f.Name.Name == "main" {
			return nil
		}
		set := out[f.Name.Name]
		if set == nil {
			set = map[string]bool{}
			out[f.Name.Name] = set
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					set[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							set[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								set[n.Name] = true
							}
						}
					}
				}
			}
		}
		return nil
	})
	return out, err
}

var reQualified = regexp.MustCompile(`\b([a-z][a-z0-9]*)\.([A-Z][A-Za-z0-9_]*)`)

// distinctive are the SDK package names nothing else is called. A generic
// name — driver, config, cache, kafka, worker, jwt — is also the standard
// library's, another library's or the project's own package, and a guard that
// fails CI cannot guess which one a sentence meant: a false alarm teaches a
// team to switch the guard off.
var distinctive = map[string]bool{
	"sqln": true, "netx": true, "msq": true, "obs": true, "iam": true, "gofi": true,
	"criteria": true, "msqtest": true, "buckettest": true, "awssign": true,
	"rdsauth": true, "awssm": true, "ocivault": true,
}

// Stale lists the SDK identifiers a text cites that the SDK does not have —
// pkg.Name where pkg is one of the SDK's distinctive package names and Name is
// not among its exports. Other names (http.Request, driver.Valuer) are not the
// SDK's to judge and are left alone.
func (e Exports) Stale(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range reQualified.FindAllStringSubmatch(text, -1) {
		pkg, name := m[1], m[2]
		set, ok := e[pkg]
		if !ok || !distinctive[pkg] || set[name] {
			continue
		}
		ref := pkg + "." + name
		if !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

// Drift is one document citing identifiers the SDK does not have.
type Drift struct {
	Path  string
	Stale []string
}

// CheckDocs reads every markdown file under dirs and reports those citing
// identifiers the SDK does not export. Paths are relative to root.
func (e Exports) CheckDocs(root string, dirs ...string) []Drift {
	var out []Drift
	for _, dir := range dirs {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if stale := e.Stale(string(b)); len(stale) > 0 {
				rel, _ := filepath.Rel(root, p)
				out = append(out, Drift{filepath.ToSlash(rel), stale})
			}
			return nil
		})
	}
	return out
}

// Package tsjs extracts a code graph from a TypeScript or JavaScript tree.
//
// It is a syntactic scanner, not a compiler: comments and string literals are
// blanked out, and declarations and module specifiers are read off what is left.
// That is enough to answer the architectural questions — which modules exist,
// what each one declares, who imports and calls whom — and it costs no
// toolchain. A front-end tree often has to be scanned on a machine with no node
// installed, so depending on the TypeScript compiler was never an option.
//
// Nothing here branches on the framework the project declared. A component is
// recognised by how it is written — a decorator, a use-prefixed name, a file
// that renders markup — so React, Angular, Vue, Svelte and Astro are read by
// the same code, and a stack none of them covers still gets its modules,
// imports and calls.
//
// What cannot be resolved is left out instead of guessed, the same rule the Go
// scanner follows in fast mode: an incomplete graph is useful, a wrong one is
// worse than none.
package tsjs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Options controls a scan.
type Options struct {
	Root string // directory to scan
	// Language is the name stamped on the graph. The scanner reads TypeScript
	// and JavaScript with the same code — they share a syntax for everything
	// that reaches the graph — but a reader still needs to know which of the two
	// the tree is written in.
	Language  string
	WithTests bool
	Exclude   []string // glob patterns of directories to ignore
	MaxFileKB int      // skip huge generated files (0 = no limit)
}

// sourceExts are the extensions this scanner reads. The single-file component
// formats are in here because they are TypeScript or JavaScript with markup
// wrapped around them: leaving them out would hand a Vue or a Svelte project a
// graph holding its configuration files and nothing else.
var sourceExts = map[string]bool{
	".ts": true, ".tsx": true, ".mts": true, ".cts": true,
	".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".vue": true, ".svelte": true, ".astro": true,
}

// templateExts are the markup files a component may keep its tree in when the
// framework puts it beside the module rather than inside it. A file with one of
// these extensions is only read when a source file of the same stem sits next
// to it — `user-list.component.html` beside `user-list.component.ts`. That rule
// costs no parsing, which is what lets Fingerprint apply it too, and it leaves
// out the page shells and coverage reports an .html sweep would otherwise drag
// in.
var templateExts = map[string]bool{".html": true}

// defaultIgnoredDirs are folders that hold dependencies or build output, never
// architecture. Every dot-directory is skipped too, which covers .next,
// .angular, .nuxt, .turbo and .expo without naming them one by one.
var defaultIgnoredDirs = map[string]bool{
	"node_modules":     true,
	"dist":             true,
	"build":            true,
	"out":              true,
	"coverage":         true,
	"vendor":           true,
	"storybook-static": true,
	"testdata":         true,
	"__snapshots__":    true,
}

// srcFile is one module found during the scan.
type srcFile struct {
	Rel   string // slash-separated path relative to the root
	Unit  string // module unit path recorded in the graph
	Ext   string
	Hash  string
	Lines int
	Src   []byte
}

// JSX reports whether the file may hold JSX at all. It gates the rule that
// turns a PascalCase function into a component, so that a plain factory in a
// .ts file is never mistaken for one.
func (f *srcFile) JSX() bool { return f.Ext == ".tsx" || f.Ext == ".jsx" }

// SFC reports whether the file is a single-file component, whose script has to
// be separated from its markup before anything else can read it.
func (f *srcFile) SFC() bool { return sfcExts[f.Ext] }

// ModuleName is the name the graph prefixes unit paths with. It comes from
// package.json, and falls back to the folder name — unlike a Go module path
// there is no file that must be there, and a tree without one is still worth
// scanning. Both sources are part of the checkout, so the result is the same on
// every machine.
func ModuleName(root string) string {
	if b, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil {
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(stripJSONComments(b), &pkg) == nil && pkg.Name != "" {
			return pkg.Name
		}
	}
	return filepath.Base(root)
}

// unitPath is the ID a module gets in the graph: the package name followed by
// the file's path without its extension.
//
// The unit is the file, not the folder that holds it. That is what an import
// specifier resolves to, and unlike a Go package a folder here has no shared
// namespace: two files beside each other may both declare `Button` without
// conflict, and folding them into one unit would merge two different symbols.
func unitPath(module, rel string) string {
	return module + "/" + strings.TrimSuffix(rel, path.Ext(rel))
}

// Fingerprint hashes every source file without parsing any of them, so that
// --update can decide in milliseconds whether a rebuild is worth it.
func Fingerprint(opt Options) (map[string]model.FileInfo, error) {
	sources, templates, err := discover(opt)
	if err != nil {
		return nil, err
	}
	module := ModuleName(opt.Root)
	out := make(map[string]model.FileInfo, len(sources)+len(templates))
	// A template is hashed like any other input. The component tree is read out
	// of it, so a graph that ignored its hash would go on describing a screen
	// whose markup had since been rewritten.
	for _, f := range slices.Concat(sources, templates) {
		out[f.Rel] = model.FileInfo{Path: f.Rel, Hash: f.Hash, Unit: unitPath(module, f.Rel), Lines: f.Lines}
	}
	return out, nil
}

// discover walks the tree and returns the source files and the component
// templates beside them, already hashed and with their contents in memory.
//
// The walk runs inside an os.Root opened on the scan root, so a symlink planted
// in the tree cannot make the scanner read outside it. Package managers that
// link workspaces around make that a real possibility here, not a theoretical
// one.
func discover(opt Options) (sources, templates []*srcFile, err error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return nil, nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	rfs := r.FS()

	var out, markup []*srcFile
	err = fs.WalkDir(rfs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable directory must not bring down the whole scan
		}
		name := d.Name()
		if d.IsDir() {
			if p == "." {
				return nil
			}
			if defaultIgnoredDirs[name] || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if !opt.WithTests && (name == "__tests__" || name == "__mocks__") {
				return fs.SkipDir
			}
			for _, pat := range opt.Exclude {
				if ok, _ := filepath.Match(pat, name); ok {
					return fs.SkipDir
				}
				if ok, _ := filepath.Match(pat, filepath.FromSlash(p)); ok {
					return fs.SkipDir
				}
			}
			return nil
		}
		ext := path.Ext(name)
		template := templateExts[ext]
		if !sourceExts[ext] && !template {
			return nil
		}
		// A declaration file describes an implementation that lives elsewhere.
		// Reading it would declare every symbol twice.
		if isDeclaration(name) {
			return nil
		}
		if !opt.WithTests && isTestFile(name) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || !info.Mode().IsRegular() {
			return nil
		}
		if opt.MaxFileKB > 0 && info.Size() > int64(opt.MaxFileKB)*1024 {
			return nil
		}
		src, rerr := fs.ReadFile(rfs, p)
		if rerr != nil {
			return nil
		}
		sum := sha256.Sum256(src)
		f := &srcFile{
			Rel:   p,
			Ext:   ext,
			Hash:  hex.EncodeToString(sum[:16]),
			Lines: bytes.Count(src, []byte{'\n'}) + 1,
			Src:   src,
		}
		if template {
			markup = append(markup, f)
		} else {
			out = append(out, f)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return out, pairedTemplates(out, markup), nil
}

// pairedTemplates keeps the markup files that belong to a module: the ones
// whose stem is also the stem of a source file in the same directory. Anything
// else — a page shell, a mail body, a fixture — is markup the graph has no
// declaration to attach to, and inventing one would describe a module that does
// not exist.
func pairedTemplates(sources, markup []*srcFile) []*srcFile {
	stems := make(map[string]bool, len(sources))
	for _, f := range sources {
		stems[strings.TrimSuffix(f.Rel, f.Ext)] = true
	}
	return slices.DeleteFunc(markup, func(f *srcFile) bool {
		return !stems[strings.TrimSuffix(f.Rel, f.Ext)]
	})
}

func isDeclaration(name string) bool {
	return strings.HasSuffix(name, ".d.ts") ||
		strings.HasSuffix(name, ".d.mts") ||
		strings.HasSuffix(name, ".d.cts")
}

func isTestFile(name string) bool {
	base := strings.TrimSuffix(name, path.Ext(name))
	return strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".spec")
}

package extract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// Fingerprint returns the hash of every Go file without parsing anything. This
// is what lets --update decide in milliseconds whether a rebuild is worth it.
func Fingerprint(opt Options) (map[string]model.FileInfo, error) {
	mods, err := loadModules(opt.Root)
	if err != nil {
		return nil, err
	}
	files, err := discover(opt, mods)
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.FileInfo, len(files))
	for _, f := range files {
		out[f.Rel] = model.FileInfo{Path: f.Rel, Hash: f.Hash, Unit: f.PkgPath, Lines: f.Lines}
	}
	return out, nil
}

// Options controls what goes into the scan.
type Options struct {
	Root       string   // repository root
	Deep       bool     // use go/types instead of heuristics
	WithTests  bool     // include _test.go files
	Exclude    []string // glob patterns of directories to ignore
	MaxFileKB  int      // skip huge generated files (0 = no limit)
	ShowErrors bool
}

// srcFile is a Go file found during the scan.
type srcFile struct {
	Path    string // absolute path
	Rel     string // path relative to the root (used in reports)
	Dir     string // absolute directory
	PkgPath string // import path derived from go.mod
	Hash    string
	Lines   int
	Src     []byte
}

// defaultIgnoredDirs are folders that are never part of the architecture.
var defaultIgnoredDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"dist":         true,
	"bin":          true,
}

// ModulePath reads the module path straight from go.mod, without depending on
// external libraries.
func ModulePath(root string) (string, error) {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("go.mod nao encontrado em %s: %w", root, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("diretiva module ausente em %s/go.mod", root)
}

// packagePath builds the import path of a directory. In almost every module it
// is the module path plus the subdirectory; the standard library is the
// exception, because packages of the "std" module are imported with no prefix
// at all ("bytes", not "std/bytes").
func packagePath(module, relDir string) string {
	relDir = filepath.ToSlash(relDir)
	if module == "std" {
		if relDir == "." {
			return "std"
		}
		return relDir
	}
	if relDir == "." {
		return module
	}
	return module + "/" + relDir
}

// modules are the Go modules a scan covers, each with the directory it roots,
// relative to the scan root and slash-separated ("." for the root module).
//
// Normally there is exactly one. A repository split into several modules and
// joined by a go.work is the exception that matters: the gofi SDK is published
// that way, and reading only its root module would leave the graph holding a
// handful of files while every package the project actually calls — base/errs,
// obs, sqln — sits in a sibling module and every call into it dead-ends.
type modules []moduleRef

type moduleRef struct {
	Dir  string // relative to the scan root, slash-separated; "." for the root
	Path string // module path from that directory's go.mod
}

// pkgPath resolves a directory to an import path through the innermost module
// that contains it.
func (m modules) pkgPath(relDir string) string {
	relDir = filepath.ToSlash(relDir)
	best := moduleRef{Dir: ".", Path: ""}
	for _, mod := range m {
		if mod.Dir != "." && relDir != mod.Dir && !strings.HasPrefix(relDir, mod.Dir+"/") {
			continue
		}
		if len(mod.Dir) >= len(best.Dir) || best.Path == "" {
			best = mod
		}
	}
	if best.Path == "" {
		return relDir
	}
	sub := strings.TrimPrefix(strings.TrimPrefix(relDir, best.Dir), "/")
	if best.Dir == "." {
		sub = relDir
	}
	if sub == "" {
		sub = "." // the module's own root directory
	}
	return packagePath(best.Path, sub)
}

// has reports whether a directory roots one of the scanned modules.
func (m modules) has(relDir string) bool {
	for _, mod := range m {
		if mod.Dir == relDir {
			return true
		}
	}
	return false
}

// loadModules reads the root module and, when the root carries a go.work, every
// module that file lists. go.work is the developer's own statement that these
// modules are one unit of work, which is exactly the question the graph is
// asking — so it is honoured rather than guessed at by walking for go.mod files.
func loadModules(root string) (modules, error) {
	path, err := ModulePath(root)
	if err != nil {
		return nil, err
	}
	out := modules{{Dir: ".", Path: path}}
	for _, dir := range workUses(root) {
		if dir == "." {
			continue
		}
		p, err := ModulePath(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil {
			continue
		}
		out = append(out, moduleRef{Dir: dir, Path: p})
	}
	return out, nil
}

// workUses returns the directories a go.work uses, relative to root. Entries
// pointing outside the scanned tree are dropped: the graph only holds files it
// actually read.
func workUses(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "go.work"))
	if err != nil {
		return nil
	}
	var out []string
	block := false
	add := func(s string) {
		s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
		if s == "" || strings.HasPrefix(s, "//") {
			return
		}
		s = path.Clean(filepath.ToSlash(s))
		if s == ".." || strings.HasPrefix(s, "../") || filepath.IsAbs(s) {
			return
		}
		out = append(out, s)
	}
	for line := range strings.Lines(string(b)) {
		s := strings.TrimSpace(line)
		switch {
		case block:
			if s == ")" {
				block = false
				continue
			}
			add(s)
		case s == "use (":
			block = true
		case strings.HasPrefix(s, "use "):
			add(strings.TrimPrefix(s, "use "))
		}
	}
	return out
}

// discover walks the tree and returns the relevant Go files, already with their
// hash and contents in memory.
//
// The walk runs inside an os.Root opened on the repository root, so a symlink
// planted in the tree cannot make the scanner read outside it. That matters
// because gofi points this at vendored SDK checkouts it did not author.
func discover(opt Options, mods modules) ([]*srcFile, error) {
	root, err := filepath.Abs(opt.Root)
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	rfs := r.FS()

	var out []*srcFile
	// Paths handed to the callback are slash-separated and relative to root.
	err = fs.WalkDir(rfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable directory must not bring down the whole scan
		}
		name := d.Name()
		if d.IsDir() {
			if path == "." {
				return nil
			}
			if defaultIgnoredDirs[name] || strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			// A subdirectory with its own go.mod is another module, and normally
			// another graph. The exception is a module this scan already covers,
			// because the root's go.work named it: then it is part of the same
			// unit of work and only its import paths differ.
			if _, err := fs.Stat(rfs, path+"/go.mod"); err == nil && !mods.has(path) {
				return fs.SkipDir
			}
			for _, pat := range opt.Exclude {
				if ok, _ := filepath.Match(pat, name); ok {
					return fs.SkipDir
				}
				if ok, _ := filepath.Match(pat, filepath.FromSlash(path)); ok {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if !opt.WithTests && strings.HasSuffix(name, "_test.go") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil || !info.Mode().IsRegular() {
			return nil
		}
		if opt.MaxFileKB > 0 && info.Size() > int64(opt.MaxFileKB)*1024 {
			return nil
		}
		src, rerr := fs.ReadFile(rfs, path)
		if rerr != nil {
			return nil
		}
		// Files marked as ignore never compile together with the package.
		if buildIgnored(src) {
			return nil
		}
		relDir := filepath.FromSlash(pathDir(path))
		sum := sha256.Sum256(src)
		out = append(out, &srcFile{
			Path:    filepath.Join(root, filepath.FromSlash(path)),
			Rel:     path,
			Dir:     filepath.Join(root, relDir),
			PkgPath: mods.pkgPath(relDir),
			Hash:    hex.EncodeToString(sum[:16]),
			Lines:   bytes.Count(src, []byte{'\n'}) + 1,
			Src:     src,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// pathDir is filepath.Dir for the slash-separated paths fs.WalkDir yields.
func pathDir(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return "."
}

// buildIgnored reports whether a build constraint excludes the file from every
// build. Only the header — the comments before the first line of code — can
// carry constraints, so unlike a substring search this is not fooled by
// "//go:build ignore" appearing inside a string literal or a doc comment.
func buildIgnored(src []byte) bool {
	for line := range bytes.Lines(src) {
		s := strings.TrimSpace(string(line))
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "//") {
			return false // the header ended; no constraint found
		}
		d, ok := ast.ParseDirective(0, s)
		if !ok || d.Tool != "go" || d.Name != "build" {
			continue
		}
		expr, err := constraint.Parse(s)
		if err != nil {
			return false
		}
		// Every tag is satisfied except "ignore", so a file gated on GOOS or a
		// custom tag still enters the graph — only a true opt-out drops it.
		return !expr.Eval(func(tag string) bool { return tag != "ignore" })
	}
	return false
}

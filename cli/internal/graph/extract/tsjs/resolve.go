package tsjs

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// resolver turns a module specifier into the file it names.
//
// Only the part of module resolution a code graph depends on is implemented:
// relative paths, directory entry points, and the path aliases declared in
// tsconfig. Anything else — conditional exports, package.json entry points,
// node_modules — leads outside the scanned tree, where the graph does not go.
type resolver struct {
	files   map[string]bool     // every discovered file, by slash path
	baseURL string              // relative to the scan root
	paths   map[string][]string // alias pattern -> targets, relative to baseURL
}

// resolutionExts are tried, in order, when a specifier carries no extension.
// TypeScript wins over JavaScript because a tree holding both is a TypeScript
// tree mid-migration, and the .ts file is the one with the source of truth. The
// single-file component formats come last: they are normally imported with the
// extension spelled out, and only a bundler configured to add it leaves it off.
var resolutionExts = []string{
	".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs",
	".vue", ".svelte", ".astro",
}

func newResolver(root string, files []*srcFile) *resolver {
	r := &resolver{files: make(map[string]bool, len(files))}
	for _, f := range files {
		r.files[f.Rel] = true
	}
	r.baseURL, r.paths = readTSConfig(root)
	return r
}

// resolve returns the file a specifier names, relative to the scan root, and
// whether it landed inside the tree. A specifier that resolves nowhere is an
// external dependency — react, @angular/core — not an error.
func (r *resolver) resolve(fromRel, spec string) (string, bool) {
	if spec == "" {
		return "", false
	}
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == ".." {
		return r.tryFile(path.Join(path.Dir(fromRel), spec))
	}
	// An alias is checked before baseURL: tsconfig gives it precedence, and the
	// two often point at the same tree through different names.
	if target, ok := r.matchAlias(spec); ok {
		return target, true
	}
	if r.baseURL != "" {
		return r.tryFile(path.Join(r.baseURL, spec))
	}
	return "", false
}

// matchAlias applies the tsconfig paths table. The most specific pattern wins,
// which is what the compiler does and what makes `@app/core/*` beat `@app/*`.
func (r *resolver) matchAlias(spec string) (string, bool) {
	best, bestLen := "", -1
	var bestTargets []string
	for pattern, targets := range r.paths {
		prefix, suffix, star := strings.Cut(pattern, "*")
		if !star {
			if spec != pattern {
				continue
			}
			if len(pattern) > bestLen {
				best, bestLen, bestTargets = "", len(pattern), targets
			}
			continue
		}
		if !strings.HasPrefix(spec, prefix) || !strings.HasSuffix(spec, suffix) ||
			len(spec) < len(prefix)+len(suffix) {
			continue
		}
		if len(prefix) > bestLen {
			best = spec[len(prefix) : len(spec)-len(suffix)]
			bestLen = len(prefix)
			bestTargets = targets
		}
	}
	if bestLen < 0 {
		return "", false
	}
	for _, t := range bestTargets {
		candidate := strings.ReplaceAll(t, "*", best)
		if rel, ok := r.tryFile(path.Join(r.baseURL, candidate)); ok {
			return rel, true
		}
	}
	return "", false
}

// tryFile applies the extension and directory-index rules to a resolved path.
func (r *resolver) tryFile(base string) (string, bool) {
	base = path.Clean(base)
	if base == "." || strings.HasPrefix(base, "..") {
		return "", false // outside the scanned tree
	}
	if r.files[base] {
		return base, true
	}
	// An ES module import spells the compiled name: './x.js' is written in the
	// source that ./x.ts produces. Resolving it back is the only way an
	// ESM-strict TypeScript project gets any import edges at all.
	if ext := path.Ext(base); ext == ".js" || ext == ".mjs" || ext == ".cjs" {
		stem := strings.TrimSuffix(base, ext)
		for _, e := range resolutionExts {
			if r.files[stem+e] {
				return stem + e, true
			}
		}
	}
	for _, e := range resolutionExts {
		if r.files[base+e] {
			return base + e, true
		}
	}
	for _, e := range resolutionExts {
		if r.files[base+"/index"+e] {
			return base + "/index" + e, true
		}
	}
	return "", false
}

// tsConfig is the sliver of tsconfig.json that matters to resolution.
type tsConfig struct {
	Extends         string `json:"extends"`
	CompilerOptions struct {
		BaseURL string              `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// configNames are read in order; the first that exists wins. jsconfig.json is
// what a JavaScript project uses to say the same things.
var configNames = []string{"tsconfig.json", "jsconfig.json"}

// readTSConfig returns the baseUrl and paths in force at the scan root, both
// relative to it.
//
// `extends` is followed because Angular and Nx put the alias table in a base
// config and leave the app config nearly empty — reading only the app config
// would find no aliases at all and silently drop every `@app/...` import edge.
func readTSConfig(root string) (string, map[string][]string) {
	for _, name := range configNames {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		base, paths := loadTSConfig(root, p, 0)
		if paths != nil || base != "" {
			return base, paths
		}
		return "", nil
	}
	return "", nil
}

// maxConfigExtends bounds the `extends` chain, so a config that points at
// itself cannot hang the scan.
const maxConfigExtends = 8

func loadTSConfig(root, file string, depth int) (string, map[string][]string) {
	if depth > maxConfigExtends {
		return "", nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", nil
	}
	var cfg tsConfig
	if err := json.Unmarshal(stripJSONComments(b), &cfg); err != nil {
		return "", nil
	}

	base, paths := "", map[string][]string{}
	if cfg.Extends != "" {
		parent := cfg.Extends
		if !strings.HasPrefix(parent, ".") {
			// A config extended by package name lives in node_modules, which the
			// scan does not read.
			parent = ""
		}
		if parent != "" {
			p := filepath.Join(filepath.Dir(file), filepath.FromSlash(parent))
			if filepath.Ext(p) == "" {
				p += ".json"
			}
			base, paths = loadTSConfig(root, p, depth+1)
			if paths == nil {
				paths = map[string][]string{}
			}
		}
	}

	if cfg.CompilerOptions.BaseURL != "" {
		if rel, ok := relativeToRoot(root, filepath.Dir(file), cfg.CompilerOptions.BaseURL); ok {
			base = rel
		}
	}
	for k, v := range cfg.CompilerOptions.Paths {
		paths[k] = v
	}
	if len(paths) == 0 {
		paths = nil
	}
	return base, paths
}

// relativeToRoot expresses a config-relative directory as a slash path under
// the scan root. A baseUrl pointing outside the scanned tree is dropped: the
// graph only holds files it actually read.
func relativeToRoot(root, dir, value string) (string, bool) {
	abs := filepath.Join(dir, filepath.FromSlash(value))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "", true
	}
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}

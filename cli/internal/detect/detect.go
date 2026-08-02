// Package detect recognises an existing codebase from its marker files, so
// `gofi init` can adopt a repository instead of interrogating the user about a
// layout that is already on disk. Detection is advisory: it pre-fills the
// wizard, and everything it reports can be overridden there.
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// maxDepth bounds how far below the root a surface is looked for. Two levels
// cover the layouts in the wild — backend/, frontend/, and the apps/web,
// packages/api shape of a workspace monorepo — without turning `gofi init`
// into a full tree walk.
const maxDepth = 2

// Surface is one detected part of the project.
//
// Path is relative to the scanned root and uses forward slashes; it is "." when
// the code sits at the root itself. Marker names the file that proved it, which
// the wizard shows so the user can tell whether the guess is worth accepting.
// Language is set for the backend, Framework for the web and mobile surfaces.
type Surface struct {
	Path      string
	Marker    string
	Language  string
	Framework string
}

// Found reports whether the surface was detected at all.
func (s Surface) Found() bool { return s.Path != "" }

// Result is what a scan of the workspace turned up. Any of the three may be
// absent — a repository with only a backend is as ordinary as a monorepo.
type Result struct {
	Backend Surface
	Web     Surface
	Mobile  Surface
}

// Any reports whether the scan recognised anything at all. A false means the
// directory is empty or unfamiliar, and `gofi init` should scaffold as usual.
func (r Result) Any() bool {
	return r.Backend.Found() || r.Web.Found() || r.Mobile.Found()
}

// Scan looks for backend, web and mobile surfaces under root, breadth-first so
// that the shallowest match wins: a go.mod at the root describes the project
// better than one in a nested example/ module.
func Scan(root string) Result {
	type node struct {
		abs, rel string
		depth    int
	}

	var res Result
	queue := []node{{abs: root, rel: ".", depth: 0}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(n.abs)
		if err != nil {
			continue
		}
		var files, subdirs []string
		for _, e := range entries {
			if e.IsDir() {
				if !ignoredDir(e.Name()) {
					subdirs = append(subdirs, e.Name())
				}
				continue
			}
			files = append(files, e.Name())
		}

		classify(n.abs, n.rel, files, &res)
		if res.Backend.Found() && res.Web.Found() && res.Mobile.Found() {
			return res
		}
		if n.depth == maxDepth {
			continue
		}

		sort.Strings(subdirs)
		for _, name := range subdirs {
			rel := name
			if n.rel != "." {
				rel = n.rel + "/" + name
			}
			queue = append(queue, node{abs: filepath.Join(n.abs, name), rel: rel, depth: n.depth + 1})
		}
	}
	return res
}

// classify assigns dir to whichever surfaces it proves and that are still
// unclaimed. A directory can prove more than one — a Go service that ships its
// own admin UI would, if both markers sat side by side.
func classify(abs, rel string, files []string, res *Result) {
	if !res.Backend.Found() {
		if lang, marker := backendLanguage(files); lang != "" {
			res.Backend = Surface{Path: rel, Marker: marker, Language: lang}
		}
	}
	if !slices.Contains(files, packageJSON) {
		return
	}
	deps := readDeps(abs)
	switch {
	case hasAny(deps, mobilePackages):
		if !res.Mobile.Found() {
			res.Mobile = Surface{Path: rel, Marker: packageJSON, Framework: config.FrameworkReactNative}
		}
	case hasAny(deps, webPackages):
		if !res.Web.Found() {
			res.Web = Surface{Path: rel, Marker: packageJSON, Framework: frameworkFromDeps(deps)}
		}
	case hasAny(deps, nodeBackendPackages):
		if !res.Backend.Found() {
			res.Backend = Surface{Path: rel, Marker: packageJSON, Language: config.LanguageNodeJS}
		}
	}
	// A package.json matching none of the three is left unclassified on
	// purpose: the root of a workspace monorepo carries one holding nothing but
	// tooling, and claiming it would steal the surface from the real apps/web.
}

const packageJSON = "package.json"

// backendMarkers are the files that identify a backend language, in lookup
// order. One directory rarely carries two, and when it does the first wins.
var backendMarkers = []struct{ file, language string }{
	{"go.mod", config.LanguageGo},
	{"Cargo.toml", config.LanguageRust},
	{"pom.xml", config.LanguageJava},
	{"build.gradle", config.LanguageJava},
	{"build.gradle.kts", config.LanguageJava},
	{"pyproject.toml", config.LanguagePython},
	{"requirements.txt", config.LanguagePython},
	{"setup.py", config.LanguagePython},
}

// backendExtensions cover the languages whose marker file is named after the
// project rather than the toolchain.
var backendExtensions = []struct{ ext, language string }{
	{".csproj", config.LanguageCSharp},
	{".sln", config.LanguageCSharp},
	{".fsproj", config.LanguageCSharp},
}

// backendLanguage returns the language proved by the given file names and the
// marker that proved it, or "" when none of them does.
func backendLanguage(files []string) (language, marker string) {
	for _, m := range backendMarkers {
		if slices.Contains(files, m.file) {
			return m.language, m.file
		}
	}
	for _, e := range backendExtensions {
		for _, f := range files {
			if strings.HasSuffix(f, e.ext) {
				return e.language, f
			}
		}
	}
	return "", ""
}

// mobilePackages / webPackages / nodeBackendPackages sort a package.json into a
// surface. Mobile is tested first: an Expo app depends on React as well.
var (
	mobilePackages      = []string{"expo", "react-native"}
	webPackages         = []string{"astro", "@angular/core", "vue", "svelte", "next", "react", "vite"}
	nodeBackendPackages = []string{"express", "fastify", "@nestjs/core", "koa", "@hapi/hapi"}
)

// frameworkPackages are the dependencies that identify a web framework, in the
// order they are looked for. Astro comes first because an Astro site renders
// components written for the others and so depends on them as well.
var frameworkPackages = []struct{ dep, framework string }{
	{"astro", config.FrameworkAstro},
	{"@angular/core", config.FrameworkAngular},
	{"vue", config.FrameworkVue},
	{"svelte", config.FrameworkSvelte},
}

// WebFramework reads the framework off the surface's own package.json. The
// wizard never asks, and the framework is not a cosmetic preset: it is what the
// graph is stamped with, so recording a Vue repository as React would describe
// it wrongly to every agent that reads it.
//
// React is the default: it is the stack the gofi presets target, and a tree
// declaring none of the others is React-shaped or close enough.
func WebFramework(dir string) string {
	return frameworkFromDeps(readDeps(dir))
}

func frameworkFromDeps(deps map[string]string) string {
	for _, f := range frameworkPackages {
		if _, ok := deps[f.dep]; ok {
			return f.framework
		}
	}
	return config.FrameworkReact
}

// readDeps returns dependencies and devDependencies of dir's package.json
// merged into one map, empty when the file is missing or unreadable.
func readDeps(dir string) map[string]string {
	body, err := os.ReadFile(filepath.Join(dir, packageJSON))
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(body, &pkg) != nil {
		return nil
	}
	merged := make(map[string]string, len(pkg.Dependencies)+len(pkg.DevDependencies))
	for _, m := range []map[string]string{pkg.DevDependencies, pkg.Dependencies} {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

func hasAny(deps map[string]string, names []string) bool {
	for _, n := range names {
		if _, ok := deps[n]; ok {
			return true
		}
	}
	return false
}

// ignoredDirs are never descended into: they hold dependencies, build output or
// tooling state, and any marker file inside them describes something other than
// the project's own layout.
var ignoredDirs = []string{
	".git", ".gofi", ".claude", ".idea", ".vscode",
	"node_modules", "vendor", "target", "dist", "build", "out",
	"bin", "obj", ".next", ".nuxt", ".venv", "venv", "__pycache__",
}

func ignoredDir(name string) bool {
	return slices.Contains(ignoredDirs, name)
}

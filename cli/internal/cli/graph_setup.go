package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/githooks"
	"github.com/joaoprofile/gofi-cli/internal/graph"
	"github.com/joaoprofile/gofi-cli/internal/graph/workspace"
	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

// graphHookBodies is what the managed git hooks run. --update turns each of
// them into a hash comparison when nothing changed, and every failure is
// swallowed on purpose: a commit, a checkout or a merge must not break because
// a derived file could not be rebuilt.
//
// --fast is what keeps this affordable. A hook runs on every commit, checkout
// and merge, and the type-checker is the expensive half of the scan, so the
// hooks stay syntactic even in a project that declares `graph: deep: true`:
// exactness is worth waiting for, but not on every commit. Deep is then the
// deliberate build — `gofi update`, or `gofi graph build --deep` run by whoever
// is about to conclude something the fast graph cannot support.
//
// pre-commit stages what it rebuilt, so the graph travels in the same commit as
// the code. The other two only rebuild: whatever they produce is a repair, and
// staging it behind the developer's back during a merge would be worse than
// leaving it for them to commit.
func graphHookBodies() map[string]string {
	const rebuild = `command -v gofi >/dev/null 2>&1 && gofi graph build --update --fast >/dev/null 2>&1 || true`
	return map[string]string{
		"pre-commit":    rebuild + "\ngit add -- " + graph.OutDir + " >/dev/null 2>&1 || true",
		"post-checkout": rebuild,
		"post-merge":    rebuild,
	}
}

// graphOptions turns the project's configuration into a workspace build.
func graphOptions(cfg *config.GofiConfig, root string) workspace.Options {
	opt := workspace.Options{
		Root:     root,
		Language: backendLang(cfg),
		Deep:     cfg.Graph.UseDeep(),
		Exclude:  cfg.Graph.Excludes(),
		Update:   true,
		SDKKey:   readInstalledSDKSha(root, backendLang(cfg)),
		// A project that declares no backend has no tree in the project's own
		// language to scan; its graph is the front end.
		SkipProject: backendLang(cfg) == "",
	}
	if cfg.Backend != nil {
		opt.SrcDir = cfg.Backend.Path
	}
	opt.Surfaces = graphSurfaces(cfg)
	return opt
}

// graphSurfaces turns every declared UI surface into a scope. The folders come
// from the configuration because that is where `gofi init` recorded them; no
// layout is assumed.
func graphSurfaces(cfg *config.GofiConfig) []workspace.Surface {
	var out []workspace.Surface
	for _, s := range cfg.UISurfaces() {
		if s.Surface.Path == "" {
			continue
		}
		out = append(out, workspace.Surface{
			Name: s.Name, Dir: s.Surface.Path,
			Language:  surfaceLang,
			Framework: s.Surface.Framework,
		})
	}
	return out
}

// surfaceLang is the language a UI surface is read with. Every framework a
// surface may declare is TypeScript or JavaScript underneath, and the scanner
// reads both with the same code, so the declared framework never decides which
// extractor runs — it is recorded in the graph and nothing more.
const surfaceLang = graph.LangTypeScript

// graphEnabled reports whether this project keeps a graph. A project with no
// backend still gets one when it declares a UI surface: an Angular repository
// is a code base like any other.
func graphEnabled(cfg *config.GofiConfig) bool {
	if cfg == nil || !cfg.Graph.On() {
		return false
	}
	return backendLang(cfg) != "" || len(graphSurfaces(cfg)) > 0
}

// buildGraphQuietly is the `gofi init` and `gofi update` path: best effort, one
// line of output, never fatal. A project scaffold must not fail because a
// derived file could not be produced — `gofi graph build` says why later.
func buildGraphQuietly(ctx context.Context, cfg *config.GofiConfig, root string) string {
	if !graphEnabled(cfg) {
		return ""
	}
	res, err := workspace.Build(ctx, graphOptions(cfg, root))
	if err != nil {
		return i18n.T("graph.setup.failed", err)
	}
	var nodes, edges int
	names := make([]string, 0, len(res.Built()))
	for _, s := range res.Built() {
		names = append(names, s.Scope.Name)
		nodes += s.Result.Graph.Stats.Nodes
		edges += s.Result.Graph.Stats.Edges
	}
	if len(names) == 0 {
		return i18n.T("graph.setup.empty")
	}
	// The mode is the one thing about a rebuilt graph nobody can see afterwards,
	// and it decides what the agents may conclude from it: in fast an absent edge
	// is not proof of an absent call. This path never forces deep — it rebuilds
	// in whatever `graph: deep:` says — so leaving it unsaid let a project read
	// syntactic guesses as certainty.
	return i18n.T("graph.setup.done", nodes, edges, strings.Join(names, ", "),
		(graph.BuildOptions{Deep: cfg.Graph.UseDeep()}).Mode(),
		relativeTo(root, graph.Dir(root, backendLang(cfg))))
}

// installGraphHooksQuietly keeps the graph in step with the code without the
// developer having to remember. Best effort for the same reason as the build:
// a project outside git, or one whose hooks directory is not writable, is still
// a working project.
func installGraphHooksQuietly(cfg *config.GofiConfig, root string) string {
	if !graphEnabled(cfg) || !cfg.Graph.HooksOn() {
		return ""
	}
	results, err := githooks.Install(root, graphHookBodies())
	if err != nil {
		return i18n.T("graph.hooks.failed", err)
	}
	changed := make([]string, 0, len(results))
	for _, r := range results {
		if r.Action != githooks.Unchanged {
			changed = append(changed, r.Hook)
		}
	}
	if len(changed) == 0 {
		return ""
	}
	return i18n.T("graph.hooks.done", strings.Join(changed, ", "))
}

// graphIsStale reports whether the graph on disk is older than the newest
// source file. It is what `gofi doctor` uses to tell a developer their agents
// are reading a map of code that has since moved.
//
// Every scanned folder counts, not just the backend: a front end edited all
// week would otherwise report a graph that is current while every component in
// it has moved.
func graphIsStale(cfg *config.GofiConfig, root string) (bool, error) {
	fi, err := os.Stat(filepath.Join(graph.Dir(root, backendLang(cfg)), graph.GraphFile))
	if err != nil {
		return false, err
	}
	stale := false
	var errs []error
	for _, src := range scannedDirs(cfg, root) {
		if stale {
			break
		}
		err := filepath.WalkDir(src, func(_ string, d os.DirEntry, err error) error {
			if err != nil || stale {
				return nil
			}
			if d.IsDir() {
				if name := d.Name(); name == ".git" || name == ".gofi" || name == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			info, err := d.Info()
			if err == nil && info.ModTime().After(fi.ModTime()) {
				stale = true
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return stale, errors.Join(errs...)
}

// scannedDirs are the folders the graph is built from, as absolute paths.
func scannedDirs(cfg *config.GofiConfig, root string) []string {
	var out []string
	if backendLang(cfg) != "" {
		src := root
		if cfg.Backend != nil && cfg.Backend.Path != "" {
			src = filepath.Join(root, cfg.Backend.Path)
		}
		out = append(out, src)
	}
	for _, s := range graphSurfaces(cfg) {
		dir := filepath.Join(root, s.Dir)
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			out = append(out, dir)
		}
	}
	if len(out) == 0 {
		out = append(out, root)
	}
	return out
}

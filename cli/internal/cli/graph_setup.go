package cli

import (
	"context"
	"strings"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/graph"
	"github.com/gofi-labs/gofi/cli/internal/graph/workspace"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
	"github.com/gofi-labs/gofi/cli/internal/layout"
)

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
// derived file could not be produced — `gofi index code` says why later.
func buildGraphQuietly(ctx context.Context, cfg *config.GofiConfig, root string) string {
	if !graphEnabled(cfg) {
		return ""
	}
	// An update run on a project from an older release must not rebuild beside
	// the old layout: the extractors it needs are still there.
	if m, err := layout.Migrate(root, backendLang(cfg)); err == nil && !m.Empty() {
		_ = ensureGofiIgnored(root)
	}
	opt := graphOptions(cfg, root)
	res, err := workspace.Build(ctx, opt)
	if err != nil {
		return i18n.T("graph.setup.failed", err)
	}
	// The manifest is a record, not the build: failing to write it leaves a
	// status check less informed, never a graph missing.
	_ = recordCodeBuild(root, opt, res.Index)
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
		relativeTo(root, graph.Dir(root)))
}

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/githooks"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// The hooks keep the versioned code graph in step with the code. None of them
// ever stops git: a hook that fails a commit because a derived file could not
// be built gets deleted, and then nothing keeps the graph current. When gofi
// cannot run they say so on one line — the old hooks failed in silence, and
// the graph drifted without anyone knowing.
var hookBodies = map[string]string{
	// The graph goes into the same commit as the code it describes. Nothing to
	// do when the commit only touches .gofi/; the build is incremental, so an
	// unchanged tree costs a hash of its files.
	"pre-commit": `# Keeps the versioned code graph in the commit. Never blocks it.
if command -v gofi >/dev/null 2>&1; then
  if git diff --cached --name-only | grep -qv '^\.gofi/'; then
    if gofi index code >/dev/null; then
      git add ` + layout.CodeDir + `
    else
      echo "gofi: the code graph was not updated — run 'gofi index code'" >&2
    fi
  fi
else
  echo "gofi: not on PATH — the code graph was not updated" >&2
fi`,
	// A branch switch ($3 = 1) or a merge brings code the local graph has not
	// seen. Nothing is staged: this is the working copy catching up.
	"post-checkout": `# Brings the local code graph up to date after a branch switch.
if [ "$3" = "1" ] && command -v gofi >/dev/null 2>&1; then
  gofi index code >/dev/null || echo "gofi: the code graph was not updated — run 'gofi index code'" >&2
fi`,
	"post-merge": `# Brings the local code graph up to date after a merge.
if command -v gofi >/dev/null 2>&1; then
  gofi index code >/dev/null || echo "gofi: the code graph was not updated — run 'gofi index code'" >&2
fi`,
}

func newIndexHooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks [remove]",
		Short: i18n.T("cmd.index.hooks.short"),
		Long: `Git hooks that keep the versioned code graph in step with the code.

  pre-commit     rebuilds the graph (incremental) and adds it to the commit
  post-checkout  brings the local graph up to date after a branch switch
  post-merge     the same after a merge

They never block git: when gofi cannot run they print one line and let the
commit through. gofi owns a marked block inside each hook, never the file —
husky, lefthook and hand-written hooks around it are kept, and core.hooksPath
is honoured. graph.hooks in .gofi.yaml says whether a project wants them;
gofi init installs them when it does, and 'gofi install hooks' puts them back.
Without an argument, lists what is installed.`,
		Example: `gofi index hooks
gofi install hooks
gofi index hooks remove`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"remove"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := graphProjectRoot()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			action := ""
			if len(args) == 1 {
				action = args[0]
			}
			switch action {
			case "":
				if installed := githooks.Installed(root); len(installed) > 0 {
					fmt.Fprintln(out, i18n.T("index.hooks.installed", strings.Join(installed, ", ")))
				} else {
					fmt.Fprintln(out, i18n.T("index.hooks.none"))
				}
				if legacy := githooks.Legacy(root); len(legacy) > 0 {
					fmt.Fprintln(out, i18n.T("index.hooks.legacy", strings.Join(legacy, ", ")))
				}
				return nil
			case "remove":
				res, err := githooks.Uninstall(root)
				for _, r := range res {
					fmt.Fprintf(out, "  %-14s %s\n", r.Hook, r.Action)
				}
				if err == nil && len(res) == 0 {
					fmt.Fprintln(out, i18n.T("index.hooks.none"))
				}
				return err
			}
			return fmt.Errorf("%q: use remove — to install, gofi install hooks", action)
		},
	}
	return cmd
}

// installIndexHooks writes the hooks, replacing any block an older release
// left in the same files.
func installIndexHooks(root string) ([]githooks.Result, error) {
	return githooks.Install(root, hookBodies)
}

// wantsIndexHooks reports whether a project asks for the hooks: a graph to
// keep, and graph.hooks not turned off.
func wantsIndexHooks(cfg *config.GofiConfig) bool {
	return graphEnabled(cfg) && cfg.Graph.HooksOn()
}

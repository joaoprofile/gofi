package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gofi-labs/gofi/cli/internal/config"
	"github.com/gofi-labs/gofi/cli/internal/extensions"
	"github.com/gofi-labs/gofi/cli/internal/githooks"
	"github.com/gofi-labs/gofi/cli/internal/host"
	"github.com/gofi-labs/gofi/cli/internal/i18n"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: i18n.T("cmd.install.short"),
		Long: `Install, or repair, what wires gofi into the editor and the coding agent:

  extensions  the GOFI AI panel, in every VSCode-family editor on PATH
  mcp         gofi's MCP server, in the file the project's host reads
  guard       the hook that keeps the agent asking the index first
  hooks       the git hooks that keep the code graph in the commit

gofi init installs all of it. Without a subcommand, this installs everything
the project asks for, as init would: the host decides what applies (the panel
drives Claude Code; the guard needs a host with hooks) and .gofi.yaml decides
the rest (ai.guard, graph.hooks). Each item reports on its own line, and one
that fails does not stop the others. Run again, it changes nothing that is
already in place.

Outside a gofi project only the extension can be installed.`,
		Example: `gofi install
gofi install mcp
gofi install guard
gofi install hooks
gofi install extensions --list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstallAll(cmd.Context(), cmd.OutOrStdout())
		},
	}
	cmd.AddCommand(newInstallExtensionsCmd(), newInstallMCPCmd(), newInstallGuardCmd(), newInstallHooksCmd())
	return cmd
}

// installOutcome is what installing one item came to.
type installOutcome struct {
	status installStatus
	detail string
}

type installStatus int

const (
	installDone installStatus = iota
	installSame
	installSkipped
	installFailed
)

func (s installStatus) mark() string {
	return [...]string{"✓", "=", "–", "✗"}[s]
}

// installTarget is the project the items are installed into.
type installTarget struct {
	root string
	cfg  *config.GofiConfig
	host host.Host
}

func loadInstallTarget() (installTarget, error) {
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return installTarget{}, err
	}
	h, ok := host.Get(cfg.AI.Host)
	if !ok {
		h = host.ClaudeCode
	}
	return installTarget{root: root, cfg: cfg, host: h}, nil
}

// runInstallAll installs every item the project asks for, in the order init
// does, and fails only after trying them all.
func runInstallAll(ctx context.Context, out io.Writer) error {
	t, err := loadInstallTarget()
	inProject := err == nil
	items := []struct {
		name string
		run  func() installOutcome
	}{
		{i18n.T("install.item.extension"), func() installOutcome {
			if inProject && !host.IsClaude(t.host.ID) {
				return installOutcome{installSkipped, i18n.T("install.skip.extension_host", t.host.Label)}
			}
			return installExtensionsNow(ctx)
		}},
		{i18n.T("install.item.mcp"), func() installOutcome { return installMCPOutcome(t) }},
		{i18n.T("install.item.guard"), func() installOutcome { return installGuardOutcome(t) }},
		{i18n.T("install.item.hooks"), func() installOutcome {
			if !wantsIndexHooks(t.cfg) {
				return installOutcome{installSkipped, i18n.T("install.skip.hooks_off")}
			}
			return installHooksOutcome(t)
		}},
	}
	failed := 0
	for i, it := range items {
		var o installOutcome
		if i > 0 && !inProject {
			o = installOutcome{installSkipped, i18n.T("install.skip.no_project")}
		} else {
			o = it.run()
		}
		if o.status == installFailed {
			failed++
		}
		fmt.Fprintf(out, "  %s %-12s %s\n", o.status.mark(), it.name, o.detail)
	}
	if failed > 0 {
		return fmt.Errorf("%s", i18n.T("install.failed", failed))
	}
	return nil
}

// --- extensions -------------------------------------------------------------

func newInstallExtensionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extensions",
		Aliases: []string{"extension"},
		Short:   i18n.T("cmd.install.extensions.short"),
		Long: `Install the GOFI AI extension into every VSCode-family editor found on PATH.

The extension is packaged inside this binary, so the install needs no network
and no Node toolchain, and the version you get always matches the CLI. Running
it again upgrades in place.

Without --editor, every editor found on PATH is targeted (VS Code, Cursor,
VS Code Insiders, VSCodium, Windsurf). The panel drives Claude Code: in a
project on another host it installs all the same, and says so.`,
		Example: `gofi install extensions
gofi install extensions --list
gofi install extensions --editor code`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, _ := cmd.Flags().GetBool("list")
			editorFlag, _ := cmd.Flags().GetString("editor")

			editors, err := targetEditors(editorFlag)
			if err != nil {
				return err
			}

			if list {
				return listExtensions(cmd, editors)
			}
			if t, err := loadInstallTarget(); err == nil && !host.IsClaude(t.host.ID) {
				fmt.Fprintln(cmd.OutOrStdout(), i18n.T("install.extension.other_host", t.host.Label))
			}
			return installExtensions(cmd, editors)
		},
	}
	cmd.Flags().Bool("list", false, "report what is installed instead of installing")
	cmd.Flags().String("editor", "", "target a single editor command (code, cursor, code-insiders, codium, windsurf)")
	return cmd
}

// targetEditors resolves the editors to act on, or explains why there are none.
func targetEditors(editorFlag string) ([]extensions.Editor, error) {
	if editorFlag != "" {
		editor, err := extensions.LookupEditor(editorFlag)
		if err != nil {
			return nil, err
		}
		return []extensions.Editor{editor}, nil
	}
	editors := extensions.DetectEditors()
	if len(editors) == 0 {
		return nil, errors.New(i18n.T("install.extension.no_editor"))
	}
	return editors, nil
}

func installExtensions(cmd *cobra.Command, editors []extensions.Editor) error {
	results, manifest, err := extensions.InstallEmbedded(cmd.Context(), editors)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "GOFI AI %s (%s)\n", manifest.Version, manifest.ID())

	var failures int
	for _, r := range results {
		if r.Err != nil {
			failures++
			fmt.Fprintf(out, "  ✗ %s — %v\n", r.Editor.Label, r.Err)
			continue
		}
		fmt.Fprintf(out, "  ✓ %s\n", r.Editor.Label)
	}

	if failures == len(results) {
		return errors.New(i18n.T("install.extension.all_failed"))
	}
	if failures > 0 {
		fmt.Fprintln(out, "\n"+i18n.T("install.extension.some_failed", failures, len(results)))
	}
	fmt.Fprintln(out, "\n"+i18n.T("install.extension.reopen"))
	return nil
}

func listExtensions(cmd *cobra.Command, editors []extensions.Editor) error {
	_, manifest, err := extensions.Embedded()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, i18n.T("install.extension.embedded", manifest.Version, manifest.ID()))

	for _, e := range editors {
		version, err := extensions.Installed(cmd.Context(), e, manifest.ID())
		switch {
		case err != nil:
			fmt.Fprintf(out, "  ? %s — %v\n", e.Label, err)
		case version == "":
			fmt.Fprintf(out, "  – %s — %s\n", e.Label, i18n.T("install.extension.absent"))
		case version == manifest.Version:
			fmt.Fprintf(out, "  ✓ %s — %s\n", e.Label, version)
		default:
			fmt.Fprintf(out, "  ↑ %s — %s\n", e.Label, i18n.T("install.extension.behind", version, manifest.Version))
		}
	}
	return nil
}

// installExtensionsOnInit is the `gofi init` hook, indirected so tests can
// scaffold a project without reaching out to the developer's real editors.
// Same pattern as detectToolchain.
var installExtensionsOnInit = installExtensionsQuietly

// installExtensionsQuietly is the `gofi init` path: best effort, one line of
// output, never fatal. A project scaffold must not fail because the machine
// has no editor on PATH.
func installExtensionsQuietly(ctx context.Context) string {
	return installExtensionsOutcome(ctx).detail
}

// installExtensionsNow is what `gofi install` runs for the extension,
// indirected so tests never reach the developer's real editors.
var installExtensionsNow = installExtensionsOutcome

// installExtensionsOutcome installs the extension in every editor on PATH and
// sums it up in one line.
func installExtensionsOutcome(ctx context.Context) installOutcome {
	editors := extensions.DetectEditors()
	if len(editors) == 0 {
		return installOutcome{installSkipped, i18n.T("install.extension.none_on_path")}
	}
	results, manifest, err := extensions.InstallEmbedded(ctx, editors)
	if err != nil {
		return installOutcome{installFailed, "GOFI AI — " + err.Error()}
	}

	var ok, failed []string
	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, r.Editor.Label)
			continue
		}
		ok = append(ok, r.Editor.Label)
	}
	switch {
	case len(ok) == 0:
		return installOutcome{installFailed, i18n.T("install.extension.failed_in", strings.Join(failed, ", "))}
	case len(failed) > 0:
		return installOutcome{installDone, i18n.T("install.extension.partly", manifest.Version, strings.Join(ok, ", "), strings.Join(failed, ", "))}
	}
	return installOutcome{installDone, i18n.T("install.extension.done", manifest.Version, strings.Join(ok, ", "))}
}

// --- mcp --------------------------------------------------------------------

func newInstallMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: i18n.T("cmd.install.mcp.short"),
		Long: `Register 'gofi mcp' where the project's host looks for MCP servers:
.mcp.json for Claude Code, .codex/config.toml for Codex, .vscode/mcp.json for
Copilot. Other servers in the file are kept as they are, and the file belongs
in git: whoever opens the project gets the same tools.`,
		Example: `gofi install mcp`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstallOne(cmd.OutOrStdout(), i18n.T("install.item.mcp"), installMCPOutcome)
		},
	}
}

func installMCPOutcome(t installTarget) installOutcome {
	changed, err := t.host.RegisterMCP(t.root)
	switch {
	case err != nil:
		return installOutcome{installFailed, err.Error()}
	case changed:
		return installOutcome{installDone, i18n.T("mcp.install.done", t.host.MCP.File)}
	}
	return installOutcome{installSame, i18n.T("mcp.install.unchanged", t.host.MCP.File)}
}

// --- guard ------------------------------------------------------------------

func newInstallGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard",
		Short: i18n.T("cmd.install.guard.short"),
		Long: `Write the guard hook, and the read-only gofi queries the agent may run
without asking, into the host's settings — in the mode ai.guard names (warn
when unset). To change the mode, use 'gofi guard <mode>'.`,
		Example: `gofi install guard`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstallOne(cmd.OutOrStdout(), i18n.T("install.item.guard"), installGuardOutcome)
		},
	}
}

func installGuardOutcome(t installTarget) installOutcome {
	if !t.host.Guard {
		return installOutcome{installSkipped, i18n.T("install.skip.guard_host", t.host.Label)}
	}
	mode := t.cfg.AI.GuardMode()
	changed, err := installAgentSettings(t.root, t.cfg.AI)
	switch {
	case err != nil:
		return installOutcome{installFailed, err.Error()}
	case changed:
		return installOutcome{installDone, i18n.T("install.guard.done", mode, claudeSettingsFile)}
	}
	return installOutcome{installSame, i18n.T("install.guard.same", mode, claudeSettingsFile)}
}

// --- hooks ------------------------------------------------------------------

func newInstallHooksCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "hooks",
		Aliases: []string{"hook"},
		Short:   i18n.T("cmd.install.hooks.short"),
		Long: `Install the git hooks that keep the versioned code graph in step with the
code: pre-commit, post-checkout and post-merge. gofi owns a marked block in
each, never the file. To remove them, 'gofi index hooks remove'.`,
		Example: `gofi install hooks`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInstallOne(cmd.OutOrStdout(), i18n.T("install.item.hooks"), installHooksOutcome)
		},
	}
}

func installHooksOutcome(t installTarget) installOutcome {
	res, err := installIndexHooks(t.root)
	if err != nil {
		return installOutcome{installFailed, err.Error()}
	}
	var touched []string
	for _, r := range res {
		if r.Action != githooks.Unchanged {
			touched = append(touched, r.Hook)
		}
	}
	if len(touched) == 0 {
		return installOutcome{installSame, i18n.T("install.hooks.same")}
	}
	return installOutcome{installDone, strings.Join(touched, ", ")}
}

// runInstallOne installs a single item into the current project.
func runInstallOne(out io.Writer, name string, run func(installTarget) installOutcome) error {
	t, err := loadInstallTarget()
	if err != nil {
		return err
	}
	o := run(t)
	fmt.Fprintf(out, "  %s %-12s %s\n", o.status.mark(), name, o.detail)
	if o.status == installFailed {
		return fmt.Errorf("%s", i18n.T("install.failed", 1))
	}
	return nil
}

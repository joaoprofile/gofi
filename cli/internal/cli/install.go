package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/extensions"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the editor tooling that ships with gofi",
		Long: `Install the editor-side tooling bundled with this gofi binary.

Today that is one extension: GOFI AI, the chat panel that talks to the
project's agents from inside the editor.`,
		Example: `gofi install extensions
gofi install extensions --list
gofi install extensions --editor cursor`,
	}
	cmd.AddCommand(newInstallExtensionsCmd())
	return cmd
}

func newInstallExtensionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extensions",
		Short: "Install (or update) the GOFI AI extension",
		Long: `Install the GOFI AI extension into every VSCode-family editor found on PATH.

The extension is packaged inside this binary, so the install needs no network
and no Node toolchain, and the version you get always matches the CLI. Running
it again upgrades in place.

Without --editor, every editor found on PATH is targeted (VS Code, Cursor,
VS Code Insiders, VSCodium, Windsurf).`,
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
		return nil, fmt.Errorf(
			"no VSCode-family editor found on PATH — install VS Code and enable its shell command " +
				"(Command Palette → \"Shell Command: Install 'code' command in PATH\")")
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
		return fmt.Errorf("the extension could not be installed in any editor")
	}
	if failures > 0 {
		fmt.Fprintf(out, "\n%d de %d editores falharam.\n", failures, len(results))
	}
	fmt.Fprintln(out, "\nReabra o editor e procure o ícone GOFI AI na barra lateral.")
	return nil
}

func listExtensions(cmd *cobra.Command, editors []extensions.Editor) error {
	_, manifest, err := extensions.Embedded()
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "GOFI AI %s (%s) — empacotada nesta CLI\n", manifest.Version, manifest.ID())

	for _, e := range editors {
		version, err := extensions.Installed(cmd.Context(), e, manifest.ID())
		switch {
		case err != nil:
			fmt.Fprintf(out, "  ? %s — %v\n", e.Label, err)
		case version == "":
			fmt.Fprintf(out, "  – %s — não instalada\n", e.Label)
		case version == manifest.Version:
			fmt.Fprintf(out, "  ✓ %s — %s\n", e.Label, version)
		default:
			fmt.Fprintf(out, "  ↑ %s — %s (instale para ir a %s)\n", e.Label, version, manifest.Version)
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
	editors := extensions.DetectEditors()
	if len(editors) == 0 {
		return "extensão GOFI AI — nenhum editor no PATH; rode `gofi install extensions` depois"
	}
	results, manifest, err := extensions.InstallEmbedded(ctx, editors)
	if err != nil {
		return fmt.Sprintf("extensão GOFI AI — %v", err)
	}

	var ok []string
	var failed []string
	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, r.Editor.Label)
			continue
		}
		ok = append(ok, r.Editor.Label)
	}
	if len(ok) == 0 {
		return fmt.Sprintf("extensão GOFI AI — falhou em %s; rode `gofi install extensions`", strings.Join(failed, ", "))
	}
	if len(failed) > 0 {
		return fmt.Sprintf("GOFI AI %s instalada em %s (falhou em %s)", manifest.Version, strings.Join(ok, ", "), strings.Join(failed, ", "))
	}
	return fmt.Sprintf("GOFI AI %s instalada em %s", manifest.Version, strings.Join(ok, ", "))
}

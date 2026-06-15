package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/tui/editor"
	"github.com/joaoprofile/gofi-cli/internal/tui/wizard"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Edit the project's .gofi.yaml",
		Long: `Open the current project's .gofi.yaml for editing.

Without flags: opens the file in $EDITOR (fast for power users); the file is
re-validated on save and rolled back if invalid.

With --wizard: reopens the interactive setup wizard pre-populated with the
current values, then writes the result back to .gofi.yaml.

The file is the source of truth for this project (sources URL, active agents,
test tasks, training entries, hsec block). Every project carries its own
configuration inside the repository — there is no longer a "global" config.`,
		Example: `gofi config
gofi config --wizard
gofi config --editor "code --wait"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			useWizard, _ := cmd.Flags().GetBool("wizard")
			editorOverride, _ := cmd.Flags().GetString("editor")
			if useWizard {
				return runConfigWizard()
			}
			return runConfigEditor(editorOverride)
		},
	}
	cmd.Flags().Bool("wizard", false, "edit via the interactive wizard instead of $EDITOR")
	cmd.Flags().String("editor", "", "editor command override (default: $VISUAL → $EDITOR → vi/notepad)")
	return cmd
}

func runConfigEditor(editorOverride string) error {
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("%w — run `gofi init` to create one", err)
	}
	path := filepath.Join(root, config.FileName)

	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	out, err := editor.Open(string(body), editorOverride, ".yaml")
	if err != nil {
		return err
	}
	if out == string(body) {
		fmt.Println("no changes; .gofi.yaml left as-is.")
		return nil
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	if _, err := config.Load(tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("the new .gofi.yaml is invalid: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	fmt.Printf("Saved %s.\n", path)
	return nil
}

func runConfigWizard() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("`gofi config --wizard` requires an interactive terminal")
	}
	cfg, root, err := loadProjectConfig()
	if err != nil {
		return fmt.Errorf("%w — run `gofi init` to create one", err)
	}

	res, err := wizard.Run(cfg)
	if err != nil {
		if errors.Is(err, wizard.ErrCancelled) {
			fmt.Println("config cancelled.")
			return nil
		}
		return err
	}

	updated := mergeWizardIntoConfig(cfg, res)
	path := filepath.Join(root, config.FileName)
	if err := config.Save(path, updated); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}
	fmt.Printf("\nSaved %s.\n", path)
	return nil
}

// mergeWizardIntoConfig copies user-editable fields from the wizard Result
// into the existing config, preserving training entries, test tasks and
// other state the wizard does not touch.
func mergeWizardIntoConfig(cfg *config.GofiConfig, r *wizard.Result) *config.GofiConfig {
	cfg.Project.Name = r.Name
	if r.Root != "" {
		cfg.Project.Root = r.Root
	}
	if r.Language != "" {
		path := r.SourcePath
		if path == "" {
			path = config.DefaultBackendPath
		}
		cfg.Backend = &config.Backend{Language: r.Language, Path: path}
	} else {
		cfg.Backend = nil
	}
	cfg.AI.Host = r.AIHost
	cfg.AI.Model = r.AIModel
	cfg.Agents = append([]string(nil), r.Agents...)
	cfg.Sources.Agents = r.AgentsRef
	if len(r.SDKURLs) > 0 {
		cfg.Sources.SDK = map[string]string{}
		for k, v := range r.SDKURLs {
			cfg.Sources.SDK[k] = v
		}
	} else {
		cfg.Sources.SDK = nil
	}
	cfg.Git.Remote = r.GitRemote
	return cfg
}

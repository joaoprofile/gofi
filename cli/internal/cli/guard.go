package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
)

func newGuardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guard [warn|enforce|off]",
		Short: i18n.T("cmd.guard.short"),
		Long: `Keep the agents asking the index before they search the tree.

A Claude Code hook watches each question. A raw search — Grep, Glob, reading a
whole file of more than 120 lines, grep or rg in the shell — made before any
gofi query (find, show or path) is:

  warn     let through, with a reminder the agent reads (once per question)
  enforce  refused, with the reason and the query to make instead
  off      left alone

A read with a line range, a small file and a path outside the project always
pass. The mode is ai.guard in .gofi.yaml; this command changes it and keeps the
hook in .claude/settings.json in step. Without an argument it shows the mode.`,
		Example: `gofi guard
gofi guard enforce
gofi guard off`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: config.GuardModes,
		RunE:      runGuard,
	}
}

func runGuard(cmd *cobra.Command, args []string) error {
	found, err := findProjectRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(found, config.FileName)
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(args) == 0 {
		s, err := loadClaudeSettings(found)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, i18n.T("guard.mode", cfg.AI.GuardMode()))
		if cfg.AI.GuardMode() != config.GuardOff && !s.hasGuard() {
			fmt.Fprintln(out, i18n.T("guard.not_installed", claudeSettingsFile))
		}
		return nil
	}
	mode := args[0]
	if !slices.Contains(config.GuardModes, mode) {
		return fmt.Errorf("mode %q: use %s", mode, strings.Join(config.GuardModes, ", "))
	}
	if cfg.AI.Guard != mode {
		cfg.AI.Guard = mode
		if err := config.Save(path, cfg); err != nil {
			return err
		}
	}
	if _, err := installAgentSettings(found, cfg.AI); err != nil {
		return err
	}
	fmt.Fprintln(out, i18n.T("guard.set", mode, config.FileName, claudeSettingsFile))
	return nil
}

// runHookGuard is `gofi hook guard`, run by Claude Code on every prompt and
// before the tools the guard watches. It always exits 0 and prints nothing it
// has no opinion on: the guard advises, and a failure of its own must never
// stop an agent.
func runHookGuard(stdin io.Reader, stdout io.Writer) {
	var in guard.Input
	if b, err := io.ReadAll(stdin); err != nil || json.Unmarshal(b, &in) != nil {
		return
	}
	root := projectRootFrom(in.Cwd)
	if root == "" {
		return
	}
	cfg, err := config.Load(filepath.Join(root, config.FileName))
	if err != nil {
		return
	}
	if out := guard.Decide(in, root, cfg.AI.GuardMode(), guard.DefaultStore(), guardMessage); out != nil {
		_, _ = stdout.Write(append(out, '\n'))
	}
}

func guardMessage(search string, deny bool) string {
	if deny {
		return i18n.T("guard.deny", search)
	}
	return i18n.T("guard.warn", search)
}

// projectRootFrom is findProjectRoot starting at dir instead of the process's
// working directory: a hook is told where the session is.
func projectRootFrom(dir string) string {
	if dir == "" {
		var err error
		if dir, err = os.Getwd(); err != nil {
			return ""
		}
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, config.FileName)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

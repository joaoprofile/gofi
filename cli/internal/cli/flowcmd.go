package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

func newFlowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flow",
		Short: i18n.T("cmd.flow.short"),
		Long: `The flows the loop runs: declared graphs of agents, sensors, gates, the policy
and the terminals, with conditions on facts and a limit on every cycle
(decision 0002, D21). gofi carries its own; a project overrides one by name in
.claude/flows/<name>.yaml.`,
		Example: `gofi flow check
gofi flow check delivery
gofi flow check .claude/flows/delivery.yaml`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check [<name>|<file>...]",
		Short: i18n.T("cmd.flow.check.short"),
		Long: `Check flows before anything runs on them. With no argument, every flow gofi
carries and every one the project declares.

Errors: an unknown node kind or field, an agent with no role, a sensor stage or
a terminal outcome that does not exist, a condition outside the vocabulary, an
edge to a node that is not there or out of a terminal, a node whose edges
cannot be decided (no default edge, or one that is not last), a node that is
unreachable or reaches no terminal, a limit on no edge or with no on_limit
terminal — and a cycle with no limit on any of its edges, which a run could
loop in forever. An agent whose role is not installed is a warning.`,
		Example: `gofi flow check
gofi flow check delivery`,
		RunE: runFlowCheck,
	})
	return cmd
}

func runFlowCheck(cmd *cobra.Command, args []string) error {
	root, err := findProjectRoot()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if len(args) == 0 {
		args = loop.Builtin()
		own, _ := filepath.Glob(layout.Flows().Abs(root, "*.yaml"))
		for _, p := range own {
			name := strings.TrimSuffix(filepath.Base(p), ".yaml")
			if !contains(args, name) {
				args = append(args, name)
			}
		}
	}
	var roles []string
	if rs, _ := role.Load(layout.RoleDirs(root)...); rs != nil {
		for _, r := range rs {
			roles = append(roles, r.Name)
		}
	}
	errs := 0
	for _, a := range args {
		var f *loop.Flow
		if strings.HasSuffix(a, ".yaml") {
			data, rerr := os.ReadFile(a)
			if rerr != nil {
				return rerr
			}
			f, err = loop.Parse(data, a)
		} else {
			f, err = loop.Load(root, a)
		}
		if err != nil {
			fmt.Fprintf(out, "  %s %s\n", i18n.T("index.check.error"), err)
			errs++
			continue
		}
		problems := loop.Check(f, roles)
		for _, p := range problems {
			kind := i18n.T("index.check.warn")
			if p.Error {
				kind = i18n.T("index.check.error")
				errs++
			}
			fmt.Fprintf(out, "  %s %s: %s\n", kind, f.Source, p.Message)
		}
		if len(problems) == 0 {
			fmt.Fprintln(out, "  "+i18n.T("flow.check.ok", f.Source, len(f.Nodes), len(f.Edges)))
		}
	}
	if errs > 0 {
		return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("flow.check.failed", errs))}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/guard"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

func newRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: i18n.T("cmd.role.short"),
		Long: `The roles installed in this project: the entry commands a person invokes and
the internal roles the loop dispatches by stage and trigger (decision 0002,
D18–D19). gofi role run runs one internal role's turn outside the flow — to
debug a role, never as the way to deliver.`,
		Example: `gofi role list
gofi role run gofi-qa "audite o contexto pedidos"`,
	}
	cmd.AddCommand(newRoleListCmd(), newRoleRunCmd())
	return cmd
}

func newRoleListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: i18n.T("cmd.role.list.short"),
		Long: `Every role installed — in .claude/skills/ and .claude/gofi/roles/ — with its
visibility (user: a slash command; internal: run by the loop), the stage it
works in, its tier and the triggers that dispatch it there. A contract that
does not parse is reported, and the command fails.`,
		Example: `gofi role list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := findProjectRoot()
			if err != nil {
				return err
			}
			roles, problems := role.Load(layout.RoleDirs(root)...)
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "  %-14s %-9s %-10s %-9s %s\n", "PAPEL", "VISÍVEL", "ESTÁGIO", "NÍVEL", "GATILHOS")
			for _, r := range roles {
				c := r.Contract
				visibility := role.VisibilityUser
				if c.Internal() {
					visibility = role.VisibilityInternal
				}
				fmt.Fprintf(out, "  %-14s %-9s %-10s %-9s %s\n", r.Name, visibility, dash(c.Stage), c.Tier, dash(triggers(c.Triggers)))
			}
			if len(problems) > 0 {
				return &ExitCodeError{Code: 1, Err: errors.Join(problems...)}
			}
			return nil
		},
	}
}

func newRoleRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <role> <request>",
		Short: i18n.T("cmd.role.run.short"),
		Long: `Run one turn of a role in a fresh session, on its tier's model, following its
SKILL.md with the request given. For debugging a role: the delivery of a
change is gofi run's, which dispatches the roles itself.`,
		Example: `gofi role run gofi-qa "audite o contexto pedidos"`,
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, root, err := loadProjectConfig()
			if err != nil {
				return err
			}
			roles, _ := role.Load(layout.RoleDirs(root)...)
			var found *role.Role
			for i := range roles {
				if roles[i].Name == args[0] {
					found = &roles[i]
				}
			}
			if found == nil {
				return &ExitCodeError{Code: 1, Err: errors.New(i18n.T("role.not_found", args[0]))}
			}
			agentsCallThisGofi()
			session, err := agentEngine(cfg, root, "acceptEdits", append(append([]string{}, guard.ReadOnlyRules...), guard.WriteRules...))
			if err != nil {
				return err
			}
			model := ""
			if h, ok := host.Get(cfg.AI.Host); ok && h.SkillModel {
				model = h.Model(found.Contract.Tier, cfg.AI.Tiers)
			}
			s := session(model)
			defer s.Close()
			prompt := fmt.Sprintf("Siga o papel descrito em %s.\n\n%s", layout.RoleSkill(found.Name, found.Contract.Internal()), strings.Join(args[1:], " "))
			if _, err := runTurn(cmd.OutOrStdout(), s, prompt); err != nil {
				return &ExitCodeError{Code: 1, Err: err}
			}
			return nil
		},
	}
}

// triggers reads a role's triggers back as they are written.
func triggers(ts []role.Trigger) string {
	var out []string
	for _, t := range ts {
		var parts []string
		if len(t.All) > 0 {
			parts = append(parts, "todos de "+strings.Join(t.All, ", "))
		}
		if len(t.Any) > 0 {
			parts = append(parts, "um de "+strings.Join(t.Any, ", "))
		}
		out = append(out, strings.Join(parts, " e "))
	}
	return strings.Join(out, " | ")
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

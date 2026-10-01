package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/host"
	"github.com/joaoprofile/gofi/cli/internal/i18n"
	"github.com/joaoprofile/gofi/cli/internal/layout"
	"github.com/joaoprofile/gofi/cli/internal/scaffold"
)

func newUpdateSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: i18n.T("cmd.update.skills.short"),
		Long: `Reinstall .claude/skills/ from the source pinned at 'sources.agents' in
.gofi.yaml — the repo configured during 'gofi init', or the gofi default when
none was given.

The skills are the agents themselves: .claude/skills/<name>/SKILL.md, the slash
commands (/gofi-pd, /gofi-spec, /gofi-eng, …) the editor discovers.

The changes are listed and confirmed before anything is written. A skill you
edited is kept and reported, never overwritten; --force puts upstream back and
copies what it replaced to .gofi/backup/ first.

Nothing outside .claude/skills/ is touched — not the graph either, which is what
separates this from a bare 'gofi update'.`,
		Example: `gofi update skills
gofi update skills --yes
gofi update skills --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			return runSkillsUpdate(yes, force)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().Bool("force", false, "overwrite skills that carry local edits (backed up to .gofi/backup/)")
	return cmd
}

func runSkillsUpdate(autoConfirm, force bool) error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}

	if installed := readInstalledSha(cfg.Project.Root); installed != "" {
		fmt.Printf("Resolving %s … (installed: %s)\n", cfg.Sources.Agents, short(installed))
	} else {
		fmt.Printf("Resolving %s …\n", cfg.Sources.Agents)
	}
	// No "already up to date" short-circuit on that sha: it records what the last
	// install fetched, not what is on disk, and someone running this target
	// explicitly is often the person who just edited a skill and wants it back.
	// The plan below says plainly when nothing would change, which is the better
	// answer anyway.
	t, err := skillsTarget(cfg, force)
	if err != nil {
		return err
	}
	return runTarget(cfg, t, autoConfirm)
}

// skillsTarget plans the skills update.
func skillsTarget(cfg *config.GofiConfig, force bool) (*targetPlan, error) {
	plan, err := planSkills(cfg)
	if err != nil {
		return nil, err
	}
	return &targetPlan{
		name:  "skills",
		title: "Update the skills?",
		scope: plan.scope(force),
		apply: func() error {
			sha, err := plan.install(cfg, force)
			if err != nil {
				return err
			}
			fmt.Printf("\nSkills updated — %s/ now at %s.\n", layout.Skills().Dir, short(sha))
			return nil
		},
	}, nil
}

// skillsPlan is a computed skills update, held between the question and the
// install so that what the user confirmed and what gets written are the same
// thing. Shared by `gofi update` and `gofi update skills`, which is why the two
// cannot drift apart on what a skill update means.
type skillsPlan struct {
	changes []scaffold.Change
	edited  []string
}

// planSkills computes the plan and prints the file-by-file detail, which always
// precedes the scope block — the block summarises, the list is the evidence.
func planSkills(cfg *config.GofiConfig) (*skillsPlan, error) {
	// The model each skill names follows this project's host and ai.tiers —
	// set here, from the config the plan is for, not left to whoever ran first.
	if h, ok := host.Get(cfg.AI.Host); ok {
		scaffold.SetSkillModels(h, cfg.AI.Tiers)
	}
	ref := cfg.Sources.Agents
	srcDir, _, err := fetchSource(cfg.Project.Root, ref)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", ref, err)
	}
	changes, err := scaffold.PlanSkillsUpdate(os.DirFS(srcDir), ".", cfg.Project.Root)
	if err != nil {
		return nil, fmt.Errorf("plan update: %w", err)
	}
	p := &skillsPlan{
		changes: changes,
		edited:  scaffold.PreservedFilesIn(cfg.Project.Root, []string{scaffold.SkillsDir}),
	}
	printUpdatePlan("skill", layout.Skills().Dir, p.changes, p.edited)
	return p, nil
}

func (p *skillsPlan) scope(force bool) updateScope {
	var added, changed int
	for _, c := range p.changes {
		if c.Kind == scaffold.ChangeNew {
			added++
		} else {
			changed++
		}
	}
	s := updateScope{
		Keeps: p.edited,
		Force: force,
		Hint:  keepsHint,
		LeavesAlone: []string{
			".gofi.yaml", "AGENTS.md", "templates/", "sdk/",
			"knowledge/", "memory/", "institutional/", "go.work",
		},
	}
	if len(p.changes) > 0 || force {
		s.write(layout.Skills().Dir+"/", planNote(added, changed))
	}
	return s
}

func (p *skillsPlan) install(cfg *config.GofiConfig, force bool) (string, error) {
	mode := scaffold.InstallUpdate
	if force {
		mode = scaffold.InstallReset
	}
	sha, err := installSkillsFromSource(cfg.Project.Root, cfg.Sources.Agents, mode)
	if err != nil {
		return "", err
	}
	if err := writeInstalledSha(cfg.Project.Root, sha); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", installedFileName, err)
	}
	return sha, nil
}

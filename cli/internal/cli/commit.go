package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/gitops"
)

func newCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "commit <message>",
		Short: "Stage all changes and create a commit",
		Long: `Stage every change in the project worktree (git add .) and create a commit
with the given message.

Useful right after 'gofi init' — the wizard intentionally leaves files unstaged
so you can review them, then run 'gofi commit "chore: gofi init"' (or use git
directly) when you're ready. Author/committer are read from your global git
config; if missing, "gofi <gofi@local>" is used as fallback.

Refuses to run when the worktree is clean.`,
		Example: `gofi commit "chore: gofi init"
gofi commit "feat: add fiscal domain"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommit(args[0])
		},
	}
}

func runCommit(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("commit message cannot be empty")
	}
	_, root, err := loadProjectConfig()
	if err != nil {
		return err
	}
	dirty, err := gitops.HasChanges(root)
	if err != nil {
		return err
	}
	if !dirty {
		fmt.Println("nothing to commit, working tree clean")
		return nil
	}
	if err := gitops.AddAndCommit(root, message); err != nil {
		return err
	}
	fmt.Printf("committed: %s\n", message)
	return nil
}

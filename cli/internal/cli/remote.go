package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/gitops"
)

const remoteName = "origin"

func newRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage the git remote for this project",
		Long: `Configure the git remote for this gofi project.

The wizard step is optional during 'gofi init'; this command lets you add, view or
remove the origin remote afterwards. gofi never pushes automatically.`,
		Example: `gofi remote show
gofi remote add git@github.com:org/my-service.git
gofi remote remove`,
	}
	cmd.AddCommand(newRemoteAddCmd(), newRemoteShowCmd(), newRemoteRemoveCmd())
	return cmd
}

func newRemoteAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <url>",
		Short: "Set the origin remote to the given URL",
		Long: `Run 'git remote add origin <url>' and persist the URL in .gofi.yaml.

Accepted forms:
  https://github.com/org/repo[.git]
  git@github.com:org/repo.git
  github.com/org/repo (shorthand — normalized to https)`,
		Example: `gofi remote add git@github.com:org/my-service.git
gofi remote add github.com/org/my-service`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemoteAdd(args[0])
		},
	}
}

func newRemoteShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Print the configured origin remote",
		Long:    `Print the URL stored under git.remote in .gofi.yaml. Reports "not configured" when empty.`,
		Example: `gofi remote show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemoteShow()
		},
	}
}

func newRemoteRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove",
		Short:   "Remove the configured origin remote",
		Long:    `Run 'git remote remove origin' and clear git.remote in .gofi.yaml.`,
		Example: `gofi remote remove`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemoteRemove()
		},
	}
}

func runRemoteAdd(rawURL string) error {
	url, err := normalizeRemoteURL(rawURL)
	if err != nil {
		return err
	}
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	root := projectRootFromCfg(cfg)

	existing, _ := gitops.GetRemote(root, remoteName)
	if existing != "" {
		return fmt.Errorf("remote %q already configured (%s); run 'gofi remote remove' first", remoteName, existing)
	}
	if err := gitops.AddRemote(root, remoteName, url); err != nil {
		return err
	}
	cfg.Git.Remote = url
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}
	fmt.Printf("Configured %s → %s\n", remoteName, url)
	return nil
}

func runRemoteShow() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	if cfg.Git.Remote == "" {
		fmt.Println("not configured")
		return nil
	}
	fmt.Printf("%s → %s\n", remoteName, cfg.Git.Remote)
	return nil
}

func runRemoteRemove() error {
	cfg, err := config.Load(config.FileName)
	if err != nil {
		return fmt.Errorf("read .gofi.yaml: %w", err)
	}
	root := projectRootFromCfg(cfg)
	existing, _ := gitops.GetRemote(root, remoteName)
	if existing == "" && cfg.Git.Remote == "" {
		fmt.Println("no remote configured")
		return nil
	}
	if existing != "" {
		if err := gitops.RemoveRemote(root, remoteName); err != nil {
			return err
		}
	}
	cfg.Git.Remote = ""
	if err := config.Save(config.FileName, cfg); err != nil {
		return fmt.Errorf("save .gofi.yaml: %w", err)
	}
	fmt.Printf("Removed %s.\n", remoteName)
	return nil
}

// normalizeRemoteURL accepts the three supported forms and returns a
// canonical URL git can clone.
func normalizeRemoteURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("URL is required")
	}
	switch {
	case strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "http://"):
		return s, nil
	case strings.HasPrefix(s, "git@"):
		return s, nil
	case strings.HasPrefix(s, "github.com/"):
		return "https://" + s, nil
	}
	return "", fmt.Errorf("unsupported URL %q (expected https://, git@ or github.com/...)", s)
}

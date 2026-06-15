package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/help"
)

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "help [command]",
		Aliases: []string{"h"},
		Short:   "Show help for any command",
		Long: `Print structured help for gofi or one of its subcommands.

Without arguments, lists every available command with a one-line summary.
With a command name, prints detailed usage, flags and examples for that command.`,
		Example: `gofi help
gofi h
gofi h init
gofi help agent add`,
		Run: func(cmd *cobra.Command, args []string) {
			opts := help.DetectOptions(cmd)
			root := cmd.Root()

			if len(args) == 0 {
				fmt.Print(help.RenderRoot(root, Version, opts))
				return
			}

			target, _, err := root.Find(args)
			if err != nil || target == root {
				fmt.Fprintf(os.Stderr, "unknown command %q for gofi\n", args[0])
				if suggestions := root.SuggestionsFor(args[0]); len(suggestions) > 0 {
					fmt.Fprintf(os.Stderr, "\nDid you mean:\n")
					for _, s := range suggestions {
						fmt.Fprintf(os.Stderr, "  %s\n", s)
					}
				}
				fmt.Fprintln(os.Stderr)
				fmt.Print(help.RenderRoot(root, Version, opts))
				os.Exit(1)
			}
			fmt.Print(help.RenderCommand(target, opts))
		},
	}
}

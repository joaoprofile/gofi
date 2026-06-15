package help

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Lint walks the command tree and ensures every visible command has Short,
// Long, and Example filled in. Returns one error per missing field.
func Lint(root *cobra.Command) []error {
	var errs []error
	walk(root, func(cmd *cobra.Command) {
		if cmd == root || cmd.Hidden || cmd.Name() == "help" {
			return
		}
		if cmd.Short == "" {
			errs = append(errs, fmt.Errorf("%s: Short is empty", cmd.CommandPath()))
		}
		if cmd.Long == "" {
			errs = append(errs, fmt.Errorf("%s: Long is empty", cmd.CommandPath()))
		}
		if cmd.Example == "" {
			errs = append(errs, fmt.Errorf("%s: Example is empty", cmd.CommandPath()))
		}
	})
	return errs
}

func walk(cmd *cobra.Command, fn func(*cobra.Command)) {
	fn(cmd)
	for _, c := range cmd.Commands() {
		walk(c, fn)
	}
}

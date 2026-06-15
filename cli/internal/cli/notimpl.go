package cli

import "fmt"

func notImplemented(commandPath, phase string) error {
	return fmt.Errorf("%s: not implemented yet — planned for %s of the gofi CLI roadmap (see specs/gofi-cli/sdd-gofi-cli.md §10)", commandPath, phase)
}

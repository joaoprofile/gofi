package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/joaoprofile/gofi-cli/internal/help"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build info",
		Long: `Print the gofi CLI version, the git commit it was built from,
the build date, the Go toolchain version and the target platform.

Values for Version, Commit and BuildDate are injected at build time via -ldflags.`,
		Example: `gofi version
gofi version --plain`,
		Run: func(cmd *cobra.Command, args []string) {
			opts := help.DetectOptions(cmd)
			if !opts.Plain {
				fmt.Print(help.RenderSplash(cmd.Root().Short, Version, opts))
				fmt.Println()
			}
			fmt.Printf("  commit:   %s\n", Commit)
			fmt.Printf("  built:    %s\n", BuildDate)
			fmt.Printf("  go:       %s\n", runtime.Version())
			fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			if opts.Plain {
				fmt.Printf("  version:  %s\n", Version)
			}
		},
	}
}

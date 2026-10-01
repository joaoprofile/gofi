package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/joaoprofile/gofi/cli/internal/cli"
)

func main() {
	if err := cli.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var ec *cli.ExitCodeError
		if errors.As(err, &ec) {
			os.Exit(ec.Code)
		}
		os.Exit(1)
	}
}

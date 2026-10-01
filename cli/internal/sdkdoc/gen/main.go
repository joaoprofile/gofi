// Command gen regenerates the Go SDK reference that ships with gofi, for
// projects that have no SDK checkout of their own. Run it from the cli module
// when the SDK releases:
//
//	go run ./internal/sdkdoc/gen <gofi-sdk-go checkout> ../ai/sdk/go/api <version>
//
// Projects with a checkout never read this copy: `gofi update sdk` generates
// the reference from the version they pinned.
package main

import (
	"fmt"
	"os"

	"github.com/gofi-labs/gofi/cli/internal/sdkdoc"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: gen <sdk checkout> <output dir> <version>")
		os.Exit(2)
	}
	src, out, version := os.Args[1], os.Args[2], os.Args[3]
	pkgs, err := sdkdoc.Generate(src, out, version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	// The shipped copy points at the examples in the SDK repository itself:
	// there is no checkout in a project that reads it.
	examples, err := sdkdoc.WriteExamples(src, out, "https://github.com/gofi-labs/gofi-sdk-go/tree/main", version)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %d package references and %d examples to %s\n", len(pkgs), len(examples), out)
}

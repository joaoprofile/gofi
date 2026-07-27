// Command gen packages gofi-ai/ into the .vsix embedded in the gofi binary.
//
// Run it via `go generate ./internal/extensions` from the cli module after
// changing anything under gofi-ai/.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joaoprofile/gofi-cli/internal/vsix"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	// `go generate` runs this with the *generating package's* directory as cwd,
	// while `go run ./gen` uses gen/ — so walk up to the repo root instead of
	// counting levels, and both entry points work.
	srcDir, err := findUpwards(vsix.GofiAIDir)
	if err != nil {
		return err
	}
	outDir, err := findUpwards(filepath.Join("cli", "internal", "extensions"))
	if err != nil {
		return err
	}
	outDir = filepath.Join(outDir, "embedded")

	data, manifest, err := vsix.Build(os.DirFS(srcDir))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	// Exactly one .vsix may live here — the embed glob rejects more. Clear the
	// previous version so a bump doesn't leave two behind.
	stale, err := filepath.Glob(filepath.Join(outDir, "*.vsix"))
	if err != nil {
		return err
	}
	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			return err
		}
	}

	out := filepath.Join(outDir, manifest.VSIXName())
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%s, %d bytes)\n", out, manifest.ID(), len(data))
	return nil
}

// findUpwards walks from the working directory towards the filesystem root
// looking for rel, and returns its absolute path.
func findUpwards(rel string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, rel)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s not found in any parent of the working directory", rel)
		}
		dir = parent
	}
}

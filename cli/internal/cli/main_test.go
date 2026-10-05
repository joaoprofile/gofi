package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/toolchain"
)

// TestMain fences the tests in: the search for .gofi.yaml stops at this
// module's folder. The repository's own .gofi.yaml, one level up, names a
// real project elsewhere on the machine, and a command run from the package's
// folder — a test that forgets to change into its temporary project — would
// otherwise find it and write there.
//
// It also reports Claude Code as present: the pipeline refuses to run without
// it, and CI runners don't have it installed. Go and Node keep their real
// probe; a test about a missing Claude Code overrides detectToolchain itself.
func TestMain(m *testing.M) {
	if module, err := filepath.Abs("../.."); err == nil {
		os.Setenv(EnvCeilingDirs, module)
	}
	detectToolchain = func(needs toolchain.Needs) toolchain.Preflight {
		needs.Claude = false
		return toolchain.Detect(needs)
	}
	os.Exit(m.Run())
}

// From the package's folder, no project is found: the fence holds.
func TestTheTestsCannotReachAProjectAboveTheModule(t *testing.T) {
	if _, err := findProjectRoot(); !errors.Is(err, ErrNotInProject) {
		t.Fatalf("found a project from the package folder: %v", err)
	}
}

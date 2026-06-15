package cli

import (
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/help"
)

func TestRoot_AllCommandsHaveHelp(t *testing.T) {
	root := NewRoot()
	errs := help.Lint(root)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("%v", e)
		}
	}
}

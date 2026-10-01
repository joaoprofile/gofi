package cli

import (
	"context"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/intake"
)

// The init pipeline and `gofi install` install the GOFI AI extension into
// every editor on PATH. That is right for a user and wrong for a test run,
// which must not mutate the developer's editors — so the whole package runs
// against a stub.
func init() {
	installExtensionsOnInit = func(context.Context) string { return "" }
	installExtensionsNow = func(context.Context) installOutcome {
		return installOutcome{installDone, "stub"}
	}
	// Nor may a test spend a model call.
	lightModelFor = func(*config.GofiConfig) intake.Model { return nil }
}

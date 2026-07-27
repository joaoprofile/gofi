package cli

import "context"

// The init pipeline installs the GOFI AI extension into every editor on
// PATH. That is right for a user running `gofi init` and wrong for a test run,
// which must not mutate the developer's editors — so the whole package runs
// against a stub.
func init() {
	installExtensionsOnInit = func(context.Context) string { return "" }
}

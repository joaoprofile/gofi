// Package extensions installs the editor extensions that ship with gofi —
// today just GOFI AI, the chat panel whose sources live in gofi-ai/ at the
// repo root.
//
// The packaged .vsix is embedded in the binary, so installing needs neither a
// network round trip nor a Node toolchain, and the extension a user gets is
// always the build that matches their CLI.
package extensions

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/joaoprofile/gofi-cli/internal/vsix"
)

// embeddedFS carries the packaged GOFI AI extension. Regenerate with
// `go generate ./internal/extensions` after touching anything under gofi-ai/ —
// TestEmbeddedVSIXMatchesSources fails if you forget.
//
//go:generate go run ./gen
//go:embed embedded/*.vsix
var embeddedFS embed.FS

// Embedded returns the packaged extension bytes and its manifest.
func Embedded() ([]byte, vsix.Manifest, error) {
	matches, err := fs.Glob(embeddedFS, "embedded/*.vsix")
	if err != nil {
		return nil, vsix.Manifest{}, fmt.Errorf("locate embedded vsix: %w", err)
	}
	if len(matches) != 1 {
		return nil, vsix.Manifest{}, fmt.Errorf("expected exactly one embedded vsix, found %d", len(matches))
	}
	data, err := embeddedFS.ReadFile(matches[0])
	if err != nil {
		return nil, vsix.Manifest{}, fmt.Errorf("read embedded vsix: %w", err)
	}
	manifest, err := vsix.ManifestFromVSIX(data)
	if err != nil {
		return nil, vsix.Manifest{}, err
	}
	return data, manifest, nil
}

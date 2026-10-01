package help

import (
	"os"

	"github.com/gofi-labs/gofi/cli/internal/tui/banner"
)

// RenderSplash returns the banner that opens `gofi` and `gofi help`: the same
// card every interactive dialogue starts with, so the CLI has a single face.
// Empty string when opts.Plain is true; uncolored when opts.NoColor is.
func RenderSplash(tagline, version string, opts Options) string {
	if opts.Plain {
		return ""
	}
	cwd, _ := os.Getwd()
	return banner.Render(banner.Banner{Version: version, Subtitle: tagline, Dir: cwd}, !opts.NoColor)
}

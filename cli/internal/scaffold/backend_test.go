package scaffold

import (
	"io/fs"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

// TestBackendScaffolds_MatchConfigLanguages locks the pairing backend.go claims
// but cannot express: its keys are plain strings so the package stays
// dependency-light, and a typo there would silently degrade a language to "no
// scaffold" instead of failing the build.
func TestBackendScaffolds_MatchConfigLanguages(t *testing.T) {
	known := map[string]bool{
		config.LanguageGo:     true,
		config.LanguageRust:   true,
		config.LanguageJava:   true,
		config.LanguageCSharp: true,
		config.LanguagePython: true,
		config.LanguageNodeJS: true,
	}
	for language, dir := range backendScaffolds {
		if !known[language] {
			t.Errorf("backendScaffolds key %q is not a config language", language)
		}
		if _, err := fs.Stat(embeddedFS, "embedded/"+dir); err != nil {
			t.Errorf("%s: embedded/%s: %v", language, dir, err)
		}
	}
	// Python is the one language config accepts with no skeleton to write.
	for language := range known {
		if language == config.LanguagePython {
			continue
		}
		if !HasBackendScaffold(language) {
			t.Errorf("HasBackendScaffold(%q) = false, want true", language)
		}
	}
	if HasBackendScaffold(config.LanguagePython) {
		t.Error("HasBackendScaffold(python) = true, want false")
	}
}

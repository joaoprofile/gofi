package scaffold

import (
	"io/fs"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/config"
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
	// Only a language with a gofi SDK gets a skeleton; the others are accepted
	// by config and adopted or started by hand.
	for language := range known {
		want := language == config.LanguageGo
		if HasBackendScaffold(language) != want {
			t.Errorf("HasBackendScaffold(%q) = %v, want %v", language, !want, want)
		}
	}
	if HasBackendScaffold(config.LanguagePython) {
		t.Error("HasBackendScaffold(python) = true, want false")
	}
}

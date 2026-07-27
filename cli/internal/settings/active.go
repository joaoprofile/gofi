package settings

import (
	"os"
	"sync"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

// The active settings for this process: one Use call at startup decides the
// language and the output style for the whole run.
var (
	activeMu sync.RWMutex
	active   = Default()
)

// Use installs s as the active settings and applies its language.
// GOFI_LANG still wins for a single invocation, without touching the file.
func Use(s *Settings) {
	if s == nil {
		return
	}
	activeMu.Lock()
	active = s
	activeMu.Unlock()

	lang := s.Language
	if code, ok := i18n.Normalize(os.Getenv(i18n.EnvLang)); ok {
		lang = code
	}
	i18n.SetLanguage(lang)
}

// Active returns the settings in force. Never nil: Default() before Use.
func Active() *Settings {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return active
}

// ColorMode returns the active color preference (auto | always | never).
func ColorMode() string { return Active().Color }

// PlainOutput reports whether the user asked for text-only rendering.
func PlainOutput() bool { return Active().Output == OutputPlain }

// CheckinEnabled reports whether a bare `gofi` should ping the skills source.
func CheckinEnabled() bool { return Active().Checkin }

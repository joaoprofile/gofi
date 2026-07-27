// Package i18n holds the CLI translation catalogs and the active language.
// The language is picked on first run and persisted in gofi.json.
//
// TODO: Joao — os textos Long dos comandos antigos ainda estão só em inglês;
// basta criar as chaves cmd.<nome>.long nos catálogos para traduzi-los.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Supported language codes.
const (
	LangEN = "en"
	LangPT = "pt"
	LangFR = "fr"
)

// DefaultLang is used when nothing else resolves.
const DefaultLang = LangEN

// EnvLang overrides the persisted language for a single invocation.
const EnvLang = "GOFI_LANG"

// Language is one supported UI language.
type Language struct {
	Code   string // BCP-47 primary subtag stored in gofi.json
	Name   string // English name
	Native string // endonym, shown in the setup wizard
}

// Supported lists the languages the CLI ships with, in display order.
func Supported() []Language {
	return []Language{
		{Code: LangEN, Name: "English", Native: "English"},
		{Code: LangPT, Name: "Portuguese", Native: "Português"},
		{Code: LangFR, Name: "French", Native: "Français"},
	}
}

// SupportedCodes returns just the codes, in display order.
func SupportedCodes() []string {
	langs := Supported()
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		out = append(out, l.Code)
	}
	return out
}

// Lookup returns the Language for a code, or false when unsupported.
func Lookup(code string) (Language, bool) {
	for _, l := range Supported() {
		if l.Code == code {
			return l, true
		}
	}
	return Language{}, false
}

// IsSupported reports whether code is one of the shipped languages.
func IsSupported(code string) bool {
	_, ok := Lookup(code)
	return ok
}

// Normalize reduces a locale ("pt_BR", "pt-BR.UTF-8", "FR") to a supported
// language code. Returns false when the language is not one we ship.
func Normalize(locale string) (string, bool) {
	s := strings.TrimSpace(locale)
	if s == "" {
		return "", false
	}
	// strip encoding and modifier
	s = strings.SplitN(s, ".", 2)[0]
	s = strings.SplitN(s, "@", 2)[0]
	// keep only the primary subtag
	s = strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })[0]
	s = strings.ToLower(s)
	if !IsSupported(s) {
		return "", false
	}
	return s, true
}

// DetectFromEnv guesses the language from the environment, so the setup wizard
// can preselect a sensible option. Falls back to English.
func DetectFromEnv() string {
	for _, env := range []string{EnvLang, "LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		v := os.Getenv(env)
		if v == "" {
			continue
		}
		// LANGUAGE is a colon-separated priority list
		for _, part := range strings.Split(v, ":") {
			if code, ok := Normalize(part); ok {
				return code
			}
		}
	}
	return DefaultLang
}

var (
	mu      sync.RWMutex
	current = DefaultLang
)

// SetLanguage sets the active language. Unsupported codes fall back to English.
func SetLanguage(code string) {
	mu.Lock()
	defer mu.Unlock()
	if IsSupported(code) {
		current = code
		return
	}
	current = DefaultLang
}

// Current returns the active language code.
func Current() string {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// catalogs maps a language code to its key/text table.
var catalogs = map[string]map[string]string{
	LangEN: catalogEN,
	LangPT: catalogPT,
	LangFR: catalogFR,
}

// T translates key into the active language, formatted with args when given.
// A missing key falls back to English, then to the key itself.
func T(key string, args ...any) string {
	return TIn(Current(), key, args...)
}

// TIn is T against an explicit language.
func TIn(lang, key string, args ...any) string {
	s, ok := lookupKey(lang, key)
	if !ok {
		s = key
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

// Has reports whether key exists in that language catalog, with no fallback.
func Has(lang, key string) bool {
	c, ok := catalogs[lang]
	if !ok {
		return false
	}
	_, ok = c[key]
	return ok
}

func lookupKey(lang, key string) (string, bool) {
	if c, ok := catalogs[lang]; ok {
		if s, ok := c[key]; ok && s != "" {
			return s, true
		}
	}
	if s, ok := catalogs[DefaultLang][key]; ok && s != "" {
		return s, true
	}
	return "", false
}

// Keys returns every key of the English catalog, in no particular order.
func Keys() []string {
	out := make([]string, 0, len(catalogEN))
	for k := range catalogEN {
		out = append(out, k)
	}
	return out
}

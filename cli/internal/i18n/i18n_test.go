package i18n

import (
	"regexp"
	"sort"
	"testing"
)

// TestCatalogsAreComplete keeps the translations honest: every key the CLI
// looks up exists in English (the reference) and in each shipped language, and
// no translation carries a key English does not know about (a typo that would
// silently never render).
func TestCatalogsAreComplete(t *testing.T) {
	for _, lang := range SupportedCodes() {
		if lang == DefaultLang {
			continue
		}
		var missing, extra []string
		for key := range catalogEN {
			if !Has(lang, key) {
				missing = append(missing, key)
			}
		}
		for key := range catalogs[lang] {
			if _, ok := catalogEN[key]; !ok {
				extra = append(extra, key)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		if len(missing) > 0 {
			t.Errorf("%s: missing %d keys: %v", lang, len(missing), missing)
		}
		if len(extra) > 0 {
			t.Errorf("%s: keys absent from the English catalog: %v", lang, extra)
		}
	}
}

var verbRe = regexp.MustCompile(`%[#+\-0-9. ]*[a-zA-Z]`)

// TestFormatVerbsMatch guards against a translation dropping or reordering a
// format verb, which would print "%!s(MISSING)" to the user.
func TestFormatVerbsMatch(t *testing.T) {
	for _, lang := range SupportedCodes() {
		if lang == DefaultLang {
			continue
		}
		for key, ref := range catalogEN {
			got, ok := catalogs[lang][key]
			if !ok {
				continue // reported by TestCatalogsAreComplete
			}
			want := verbRe.FindAllString(ref, -1)
			have := verbRe.FindAllString(got, -1)
			if len(want) != len(have) {
				t.Errorf("%s/%s: %d format verbs, English has %d", lang, key, len(have), len(want))
				continue
			}
			for i := range want {
				if want[i] != have[i] {
					t.Errorf("%s/%s: verb %d is %q, English has %q", lang, key, i, have[i], want[i])
				}
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"pt", "pt", true},
		{"PT", "pt", true},
		{"pt_BR", "pt", true},
		{"pt-BR.UTF-8", "pt", true},
		{"fr_FR@euro", "fr", true},
		{" en ", "en", true},
		{"es", "", false},
		{"", "", false},
		{"C.UTF-8", "", false},
	}
	for _, c := range cases {
		got, ok := Normalize(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Normalize(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestDetectFromEnv(t *testing.T) {
	t.Setenv(EnvLang, "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANGUAGE", "")

	t.Setenv("LANG", "fr_FR.UTF-8")
	if got := DetectFromEnv(); got != LangFR {
		t.Errorf("LANG=fr_FR.UTF-8 → %q, want %q", got, LangFR)
	}

	// GOFI_LANG outranks the locale.
	t.Setenv(EnvLang, "pt")
	if got := DetectFromEnv(); got != LangPT {
		t.Errorf("GOFI_LANG=pt → %q, want %q", got, LangPT)
	}

	// An unsupported locale falls back to English rather than failing.
	t.Setenv(EnvLang, "")
	t.Setenv("LANG", "ja_JP.UTF-8")
	if got := DetectFromEnv(); got != DefaultLang {
		t.Errorf("LANG=ja_JP.UTF-8 → %q, want %q", got, DefaultLang)
	}
}

func TestTranslatesAndFallsBack(t *testing.T) {
	defer SetLanguage(DefaultLang)

	SetLanguage(LangPT)
	if got := T("settings.label.lang"); got != "Idioma" {
		t.Errorf("pt settings.label.lang = %q, want %q", got, "Idioma")
	}

	// An unknown key renders as itself, so the gap is visible in output.
	if got := T("no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q, want the key itself", got)
	}

	// An unsupported language falls back to English.
	SetLanguage("es")
	if Current() != DefaultLang {
		t.Errorf("SetLanguage(es) → %q, want %q", Current(), DefaultLang)
	}
	if got := T("settings.label.lang"); got != "Language" {
		t.Errorf("fallback settings.label.lang = %q, want %q", got, "Language")
	}
}

func TestTFormatsArgs(t *testing.T) {
	defer SetLanguage(DefaultLang)
	SetLanguage(LangFR)
	if got := TIn(LangEN, "settings.saved", "/tmp/gofi.json"); got != "Saved /tmp/gofi.json" {
		t.Errorf("TIn = %q", got)
	}
	if got := T("settings.saved", "/tmp/gofi.json"); got != "Enregistré dans /tmp/gofi.json" {
		t.Errorf("T = %q", got)
	}
}

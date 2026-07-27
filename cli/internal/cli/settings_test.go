package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
	"github.com/joaoprofile/gofi-cli/internal/settings"
)

// isolateSettings points gofi.json at a temp file and restores the process-wide
// settings afterwards, so tests never read or write the developer's real file.
func isolateSettings(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), settings.FileName)
	t.Setenv(settings.EnvPath, path)
	t.Setenv(settings.EnvNoSetup, "1")
	t.Setenv(i18n.EnvLang, "")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Cleanup(func() { settings.Use(settings.Default()) })
	return path
}

func TestSettingsSetPersistsAndTranslates(t *testing.T) {
	path := isolateSettings(t)

	root := NewRoot()
	root.SetArgs([]string{"settings", "set", "language", "pt"})
	if err := root.Execute(); err != nil {
		t.Fatalf("settings set: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("gofi.json was not written: %v", err)
	}
	if !strings.Contains(string(data), `"language": "pt"`) {
		t.Errorf("language not persisted:\n%s", data)
	}

	// A CLI started fresh now speaks Portuguese, from the file alone.
	if got := NewRoot().Short; got != i18n.TIn(i18n.LangPT, "root.short") {
		t.Errorf("root Short = %q, want the Portuguese text", got)
	}
}

func TestSettingsSetAcceptsKeyEqualsValue(t *testing.T) {
	path := isolateSettings(t)

	root := NewRoot()
	root.SetArgs([]string{"settings", "set", "output=plain"})
	if err := root.Execute(); err != nil {
		t.Fatalf("settings set: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"output": "plain"`) {
		t.Errorf("output not persisted:\n%s", data)
	}
}

func TestSettingsSetRejectsBadInput(t *testing.T) {
	path := isolateSettings(t)

	cases := [][]string{
		{"settings", "set", "language", "klingon"},
		{"settings", "set", "timezone", "utc"},
		{"settings", "set", "checkin", "maybe"},
	}
	for _, args := range cases {
		root := NewRoot()
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a rejected value still wrote gofi.json")
	}
}

// The first run of a non-interactive session must not leave a settings file
// behind — CI images stay clean, and the wizard still runs for a real user.
func TestBootstrapWritesNothingWhenItCannotAsk(t *testing.T) {
	path := isolateSettings(t)

	NewRoot()
	if _, err := os.Stat(path); err == nil {
		t.Error("bootstrap wrote gofi.json without asking the user")
	}
	if settings.Active().Language != i18n.LangEN {
		t.Errorf("detected language = %q, want en", settings.Active().Language)
	}
}

func TestSettingsResetRestoresDefaults(t *testing.T) {
	path := isolateSettings(t)

	root := NewRoot()
	root.SetArgs([]string{"settings", "set", "color", "never"})
	if err := root.Execute(); err != nil {
		t.Fatalf("settings set: %v", err)
	}

	root = NewRoot()
	root.SetArgs([]string{"settings", "reset"})
	if err := root.Execute(); err != nil {
		t.Fatalf("settings reset: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"color": "auto"`) {
		t.Errorf("reset did not restore defaults:\n%s", data)
	}
}

func TestParseSettingArgs(t *testing.T) {
	cases := []struct {
		args      []string
		key, val  string
		expectErr bool
	}{
		{args: []string{"language", "pt"}, key: "language", val: "pt"},
		{args: []string{"language=fr"}, key: "language", val: "fr"},
		{args: []string{"language"}, expectErr: true},
	}
	for _, c := range cases {
		key, val, err := parseSettingArgs(c.args)
		if c.expectErr {
			if err == nil {
				t.Errorf("%v: expected an error", c.args)
			}
			continue
		}
		if err != nil || key != c.key || val != c.val {
			t.Errorf("%v = (%q, %q, %v)", c.args, key, val, err)
		}
	}
}

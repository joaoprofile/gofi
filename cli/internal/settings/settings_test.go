package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

// isolate points the settings file at a temp dir for the duration of a test.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(EnvPath, filepath.Join(dir, FileName))
	return dir
}

func TestLocateHonorsEnvFile(t *testing.T) {
	dir := isolate(t)
	path, exists, err := Locate()
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if want := filepath.Join(dir, FileName); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if exists {
		t.Error("exists = true for a file that was never written")
	}
}

// A directory in GOFI_SETTINGS means "put gofi.json in here".
func TestLocateAcceptsEnvDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvPath, dir)
	path, _, err := Locate()
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if want := filepath.Join(dir, FileName); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
}

func TestLoadReportsNotConfigured(t *testing.T) {
	isolate(t)
	if _, err := Load(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Load on a fresh machine = %v, want ErrNotConfigured", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	isolate(t)

	s := Default()
	s.Language = i18n.LangPT
	s.Color = ColorNever
	s.Output = OutputPlain
	s.Checkin = false
	if err := Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if s.Path() == "" {
		t.Error("Save left Path empty")
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Language != i18n.LangPT || got.Color != ColorNever || got.Output != OutputPlain || got.Checkin {
		t.Errorf("round trip lost values: %+v", got)
	}
	if got.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", got.Version, CurrentVersion)
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt not stamped")
	}

	// The file is real JSON, human-editable, and carries no internal fields.
	data, err := os.ReadFile(got.Path())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), `"language": "pt"`) {
		t.Errorf("unexpected file contents:\n%s", data)
	}
	if strings.Contains(string(data), "path") {
		t.Errorf("internal path field leaked into the file:\n%s", data)
	}
}

// A hand-edited file with nonsense values must still yield a usable CLI.
func TestLoadRepairsUnknownValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	body := `{"version":0,"language":"klingon","color":"rainbow","output":"fancy"}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if got.Language != i18n.DefaultLang {
		t.Errorf("Language = %q, want %q", got.Language, i18n.DefaultLang)
	}
	if got.Color != ColorAuto {
		t.Errorf("Color = %q, want %q", got.Color, ColorAuto)
	}
	if got.Output != OutputRich {
		t.Errorf("Output = %q, want %q", got.Output, OutputRich)
	}
	if got.Version != CurrentVersion {
		t.Errorf("Version = %d, want %d", got.Version, CurrentVersion)
	}
}

func TestSetValidatesValues(t *testing.T) {
	s := Default()

	if err := s.Set(KeyLanguage, "PT_BR"); err != nil {
		t.Fatalf("Set language: %v", err)
	}
	if s.Language != i18n.LangPT {
		t.Errorf("Language = %q, want pt", s.Language)
	}
	if err := s.Set(KeyCheckin, "false"); err != nil || s.Checkin {
		t.Errorf("Set checkin=false: err=%v value=%v", err, s.Checkin)
	}
	if err := s.Set(KeyColor, "NEVER"); err != nil || s.Color != ColorNever {
		t.Errorf("Set color=NEVER: err=%v value=%q", err, s.Color)
	}

	if err := s.Set(KeyLanguage, "klingon"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Set language=klingon = %v, want ErrInvalidValue", err)
	}
	if err := s.Set(KeyOutput, "fancy"); !errors.Is(err, ErrInvalidValue) {
		t.Errorf("Set output=fancy = %v, want ErrInvalidValue", err)
	}
	if err := s.Set("timezone", "utc"); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("Set timezone = %v, want ErrUnknownKey", err)
	}
	if _, err := s.Get("timezone"); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("Get timezone = %v, want ErrUnknownKey", err)
	}
}

func TestUseAppliesLanguage(t *testing.T) {
	t.Setenv(i18n.EnvLang, "")
	defer Use(Default())

	s := Default()
	s.Language = i18n.LangFR
	Use(s)
	if i18n.Current() != i18n.LangFR {
		t.Errorf("active language = %q, want fr", i18n.Current())
	}
	if !Active().Checkin || Active().Language != i18n.LangFR {
		t.Errorf("Active() = %+v", Active())
	}

	// GOFI_LANG wins for a single invocation without touching the stored value.
	t.Setenv(i18n.EnvLang, "pt")
	Use(s)
	if i18n.Current() != i18n.LangPT {
		t.Errorf("with GOFI_LANG=pt, active language = %q, want pt", i18n.Current())
	}
	if Active().Language != i18n.LangFR {
		t.Errorf("GOFI_LANG changed the stored language to %q", Active().Language)
	}
}

func TestCloneIsIndependent(t *testing.T) {
	s := Default()
	c := s.Clone()
	c.Language = i18n.LangFR
	c.Checkin = !c.Checkin
	if s.Language == c.Language || s.Checkin == c.Checkin {
		t.Error("Clone shares state with the original")
	}
}

func TestValidValues(t *testing.T) {
	for _, key := range Keys() {
		if len(ValidValues(key)) == 0 {
			t.Errorf("no accepted values documented for %q", key)
		}
	}
	if ValidValues("timezone") != nil {
		t.Error("ValidValues returned values for an unknown key")
	}
}

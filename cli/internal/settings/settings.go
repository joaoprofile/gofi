// Package settings holds the CLI persisted context: the preferences that apply
// to gofi everywhere, independent of any project.
//
// The file is gofi.json, stored next to the gofi executable so a portable
// install carries its configuration with it. When that folder is read-only it
// falls back to ~/.gofi/gofi.json; GOFI_SETTINGS pins an explicit path.
// It is created on the first interactive run by the setup wizard.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joaoprofile/gofi-cli/internal/i18n"
)

const (
	// FileName is the settings file, stored beside the gofi executable.
	FileName = "gofi.json"

	// CurrentVersion is the schema version written to new files.
	CurrentVersion = 1

	// EnvPath pins an explicit settings file path (absolute, or a directory).
	EnvPath = "GOFI_SETTINGS"

	// EnvNoSetup suppresses the first-run wizard (CI, scripts, containers).
	EnvNoSetup = "GOFI_NO_SETUP"

	// color modes
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"

	// output modes
	OutputRich  = "rich"
	OutputPlain = "plain"
)

// Setting keys accepted by `gofi settings set`.
const (
	KeyLanguage = "language"
	KeyColor    = "color"
	KeyOutput   = "output"
	KeyCheckin  = "checkin"
)

// ErrNotConfigured is returned by Load when no settings file exists yet.
var ErrNotConfigured = errors.New("gofi.json not found")

// Set returns these so the caller can render a localized message.
var (
	ErrUnknownKey   = errors.New("unknown setting key")
	ErrInvalidValue = errors.New("invalid setting value")
)

// Settings is the persisted CLI context.
type Settings struct {
	Version  int    `json:"version"`
	Language string `json:"language"`
	Color    string `json:"color"`
	Output   string `json:"output"`
	// Checkin: a bare `gofi` inside a project pings the skills repository.
	Checkin   bool   `json:"checkin"`
	UpdatedAt string `json:"updated_at,omitempty"`

	// path is where this value was loaded from. Not serialized.
	path string
}

// Default is what a fresh install starts from.
func Default() *Settings {
	return &Settings{
		Version:  CurrentVersion,
		Language: i18n.DetectFromEnv(),
		Color:    ColorAuto,
		Output:   OutputRich,
		Checkin:  true,
	}
}

// Path is the file this value was loaded from, or "" when never persisted.
func (s *Settings) Path() string { return s.path }

// Clone returns a copy, so a cancelled edit leaves the active settings alone.
func (s *Settings) Clone() *Settings {
	c := *s
	return &c
}

// Keys lists the settable keys, in display order.
func Keys() []string { return []string{KeyLanguage, KeyColor, KeyOutput, KeyCheckin} }

// ValidValues returns the accepted values for a key, or nil when unknown.
func ValidValues(key string) []string {
	switch key {
	case KeyLanguage:
		return i18n.SupportedCodes()
	case KeyColor:
		return []string{ColorAuto, ColorAlways, ColorNever}
	case KeyOutput:
		return []string{OutputRich, OutputPlain}
	case KeyCheckin:
		return []string{"true", "false"}
	}
	return nil
}

// Get returns the current value of a key as a string.
func (s *Settings) Get(key string) (string, error) {
	switch key {
	case KeyLanguage:
		return s.Language, nil
	case KeyColor:
		return s.Color, nil
	case KeyOutput:
		return s.Output, nil
	case KeyCheckin:
		return strconv.FormatBool(s.Checkin), nil
	}
	return "", fmt.Errorf("%w: %s", ErrUnknownKey, key)
}

// Set assigns a key from its string form. The caller persists afterwards.
func (s *Settings) Set(key, value string) error {
	v := strings.ToLower(strings.TrimSpace(value))
	switch key {
	case KeyLanguage:
		code, ok := i18n.Normalize(v)
		if !ok {
			return fmt.Errorf("%w: %s", ErrInvalidValue, value)
		}
		s.Language = code
	case KeyColor:
		if !contains(ValidValues(KeyColor), v) {
			return fmt.Errorf("%w: %s", ErrInvalidValue, value)
		}
		s.Color = v
	case KeyOutput:
		if !contains(ValidValues(KeyOutput), v) {
			return fmt.Errorf("%w: %s", ErrInvalidValue, value)
		}
		s.Output = v
	case KeyCheckin:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidValue, value)
		}
		s.Checkin = b
	default:
		return fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	return nil
}

// normalize repairs missing or unknown fields, so a hand-edited file still
// yields a usable value instead of an error.
func (s *Settings) normalize() {
	if s.Version <= 0 {
		s.Version = CurrentVersion
	}
	if code, ok := i18n.Normalize(s.Language); ok {
		s.Language = code
	} else {
		s.Language = i18n.DefaultLang
	}
	if !contains(ValidValues(KeyColor), s.Color) {
		s.Color = ColorAuto
	}
	if !contains(ValidValues(KeyOutput), s.Output) {
		s.Output = OutputRich
	}
}

// Locate resolves the settings file path and reports whether it exists.
// Order: GOFI_SETTINGS, an existing file beside the executable, an existing
// file under ~/.gofi. When none exists, the first writable location wins.
func Locate() (path string, exists bool, err error) {
	if p := strings.TrimSpace(os.Getenv(EnvPath)); p != "" {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			p = filepath.Join(p, FileName)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", false, fmt.Errorf("resolve %s: %w", EnvPath, err)
		}
		return abs, fileExists(abs), nil
	}

	beside, besideErr := besidePath()
	home, homeErr := homePath()
	if besideErr != nil && homeErr != nil {
		return "", false, fmt.Errorf("resolve settings path: %w", errors.Join(besideErr, homeErr))
	}

	for _, cand := range []string{beside, home} {
		if cand != "" && fileExists(cand) {
			return cand, true, nil
		}
	}
	if beside != "" && dirWritable(filepath.Dir(beside)) {
		return beside, false, nil
	}
	if home != "" {
		return home, false, nil
	}
	return beside, false, nil
}

// besidePath is <dir of the gofi executable>/gofi.json, symlinks resolved.
func besidePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), FileName), nil
}

// homePath is the fallback for read-only install folders.
func homePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gofi", FileName), nil
}

// Load reads the settings from the resolved path.
func Load() (*Settings, error) {
	path, exists, err := Locate()
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w (looked at %s)", ErrNotConfigured, path)
	}
	return LoadFrom(path)
}

// LoadFrom reads and normalizes the settings file at an explicit path.
func LoadFrom(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w (looked at %s)", ErrNotConfigured, path)
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	s.normalize()
	s.path = path
	return &s, nil
}

// Save writes the settings to their resolved path, creating the folder.
func Save(s *Settings) error {
	path := s.path
	if path == "" {
		p, _, err := Locate()
		if err != nil {
			return err
		}
		path = p
	}
	if err := SaveTo(path, s); err != nil {
		return err
	}
	s.path = path
	return nil
}

// SaveTo writes the settings to an explicit path, atomically.
func SaveTo(path string, s *Settings) error {
	s.normalize()
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// dirWritable probes a directory with a temp file: the portable way to ask
// "can I write here?", since mode bits lie on Windows and on read-only mounts.
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".gofi-write-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

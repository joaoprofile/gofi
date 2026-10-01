package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shape the old gofi-ui material documented. A project carrying it could not
// be loaded at all, which meant `gofi update` could not repair it either — the
// command was unreachable exactly where it was needed most.
const brandBlockConfig = `version: 1
project:
  name: lm
  root: /r
  language: go
  path: backend
ui:
  web:
    framework: react
    path: frontend/web
    brand:
      surface: "#dcebfb"
      onBrand: "#0e3a6b"
      action: "#025cb2"
    styling: tailwind
    state: tanstack-query
    testing: vitest
    ds: gofi-ui
ai:
  host: claude-vscode
  model: claude-opus-4-8
agents: [gofi-eng]
sources:
  agents: github.com/joaoprofile/gofi@main
`

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadReadsABrandBlock(t *testing.T) {
	cfg, err := Load(write(t, brandBlockConfig))
	if err != nil {
		t.Fatalf("a config the old material told the project to write must still load: %v", err)
	}
	if cfg.Frontend == nil {
		t.Fatal("ui.web must migrate into frontend:")
	}
	if cfg.Frontend.Brand != "#dcebfb" {
		t.Errorf("brand = %q, want the surface colour", cfg.Frontend.Brand)
	}
	if cfg.Frontend.Styling != "tailwind" {
		t.Errorf("the rest of the surface must survive the fold, styling = %q", cfg.Frontend.Styling)
	}
}

// The seed has to come back quoted: a bare #rrggbb re-read as YAML is a comment,
// which would silently blank the brand on the next load.
func TestFoldedBrandSurvivesARoundTrip(t *testing.T) {
	p := write(t, brandBlockConfig)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	again, err := Load(p)
	if err != nil {
		t.Fatalf("reload after save: %v", err)
	}
	if again.Frontend.Brand != "#dcebfb" {
		t.Errorf("brand after round-trip = %q", again.Frontend.Brand)
	}
}

// withBackoffice is the v1 shape that has no named block in the current schema:
// a third surface alongside web. It has to survive the migration under its own
// name, because dropping a surface a team declared is losing their work.
func withBackoffice(body string) string {
	return strings.Replace(body, "ui:\n  web:",
		"ui:\n  backoffice:\n    framework: react\n    path: frontend/backoffice\n    brand: violet\n  web:", 1)
}

func TestLoadKeepsAThirdSurface(t *testing.T) {
	cfg, err := Load(write(t, withBackoffice(brandBlockConfig)))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	s := cfg.Surfaces["backoffice"]
	if s == nil {
		t.Fatalf("ui.backoffice must land in surfaces:, got %+v", cfg.Surfaces)
	}
	if s.Path != "frontend/backoffice" || s.Framework != "react" || s.Brand != "violet" {
		t.Errorf("the surface must arrive whole, got %+v", s)
	}
	if cfg.Frontend == nil || cfg.Frontend.Path != "frontend/web" {
		t.Errorf("web still belongs in frontend:, got %+v", cfg.Frontend)
	}
}

func TestThirdSurfaceSurvivesARoundTrip(t *testing.T) {
	p := write(t, withBackoffice(brandBlockConfig))
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(p, cfg); err != nil {
		t.Fatal(err)
	}
	again, err := Load(p)
	if err != nil {
		t.Fatalf("reload after save: %v", err)
	}
	if s := again.Surfaces["backoffice"]; s == nil || s.Path != "frontend/backoffice" {
		t.Errorf("surfaces: must round-trip, got %+v", again.Surfaces)
	}
}

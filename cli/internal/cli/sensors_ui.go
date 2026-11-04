package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/pack"
)

// changeSurfaces are what a change touches, as its proposal declares; a
// proposal that declares none touches the project's first surface.
func changeSurfaces(cfg *config.GofiConfig, root, change string) ([]string, error) {
	all, err := contract.Docs(root)
	if err != nil {
		return nil, err
	}
	var surfaces []string
	for _, c := range contract.Changes(all) {
		if c.ID == change {
			surfaces = c.Surfaces
		}
	}
	if len(surfaces) == 0 {
		if project := projectSurfaces(cfg); len(project) > 0 {
			surfaces = project[:1]
		}
	}
	return surfaces, nil
}

// sensorCommandsFor are the sensors of a change by what it touches: the
// backend's pack for the backend, the UI surface's own scripts for web and
// mobile. A change of both runs both builds and both static checks, one after
// the other; its test stage is the backend's. loop.sensors, the project's own
// word, overrides any stage. A change that names no surface is sensed as the
// backend, as before.
func sensorCommandsFor(cfg *config.GofiConfig, work string, surfaces []string) (map[string]pack.Command, error) {
	backend := len(surfaces) == 0 || slices.Contains(surfaces, "backend")
	var ui *config.UISurface
	for _, s := range cfg.UISurfaces() {
		name := "web"
		if s.Name == "mobile" {
			name = "mobile"
		}
		if slices.Contains(surfaces, name) {
			ui = s.Surface
			break
		}
	}
	if ui == nil {
		return sensorCommands(cfg)
	}
	cmds := uiSensors(work, ui)
	if backend {
		be, err := sensorCommands(cfg)
		if err != nil {
			return nil, err
		}
		for stage, c := range be {
			u, both := cmds[stage]
			switch {
			case !both || stage == "test":
				cmds[stage] = c
			case c.Format == pack.FormatLines || c.Format == pack.FormatExit:
				cmds[stage] = pack.Command{Run: "(" + c.Run + ") && (" + u.Run + ")", Format: pack.FormatExit}
			default:
				cmds[stage] = c
			}
		}
		return cmds, nil
	}
	if cfg.Loop != nil {
		for stage, s := range cfg.Loop.Sensors {
			format := s.Format
			if format == "" {
				format = pack.FormatLines
			}
			cmds[stage] = pack.Command{Run: s.Run, Format: format, Report: s.Report}.Expand(ui.Path)
		}
	}
	return cmds, nil
}

// uiSensors are the checks a UI surface's own package.json offers: its type
// check, its lint and its tests, run with its package manager in its folder.
// A stage it offers no command for is left out — recorded as not checked,
// never as passed.
func uiSensors(work string, s *config.UISurface) map[string]pack.Command {
	dir := s.Path
	if dir == "" {
		dir = "."
	}
	abs := filepath.Join(work, filepath.FromSlash(dir))
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if b, err := os.ReadFile(filepath.Join(abs, "package.json")); err == nil {
		_ = json.Unmarshal(b, &pkg)
	}
	pm := "npm"
	switch {
	case fileExists(filepath.Join(abs, "yarn.lock")):
		pm = "yarn"
	case fileExists(filepath.Join(abs, "pnpm-lock.yaml")):
		pm = "pnpm"
	}
	script := func(names ...string) string {
		for _, n := range names {
			if _, ok := pkg.Scripts[n]; ok {
				return n
			}
		}
		return ""
	}
	in := "cd " + dir + " && "
	cmds := map[string]pack.Command{}
	if sc := script("typecheck", "type-check", "tsc"); sc != "" {
		cmds["build"] = pack.Command{Run: in + pm + " run " + sc, Format: pack.FormatExit}
	} else if fileExists(filepath.Join(abs, "tsconfig.json")) {
		cmds["build"] = pack.Command{Run: in + "npx --no-install tsc --noEmit --pretty false", Format: pack.FormatLines}
	}
	if sc := script("lint"); sc != "" {
		cmds["static"] = pack.Command{Run: in + pm + " run " + sc, Format: pack.FormatExit}
	}
	switch {
	case s.Testing == "vitest":
		cmds["test"] = pack.Command{Run: in + "npx --no-install vitest run", Format: pack.FormatExit}
	case script("test") != "":
		cmds["test"] = pack.Command{Run: in + "CI=true " + pm + " run test", Format: pack.FormatExit}
	}
	return cmds
}

// uiDependencies are the dependency folders a worktree links to the
// project's: each UI surface's node_modules, and a Node backend's.
func uiDependencies(cfg *config.GofiConfig, root string) []string {
	var dirs []string
	add := func(path string) {
		if path == "" {
			path = "."
		}
		rel := filepath.ToSlash(filepath.Join(path, "node_modules"))
		if fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			dirs = append(dirs, rel)
		}
	}
	for _, s := range cfg.UISurfaces() {
		add(s.Surface.Path)
	}
	if cfg.Backend != nil && cfg.Backend.Language == config.LanguageNodeJS {
		add(cfg.Backend.Path)
	}
	return dirs
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

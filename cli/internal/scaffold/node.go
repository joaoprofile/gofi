package scaffold

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// commandRunner runs an external command rooted at dir, streaming its I/O to
// the user. It is a package var so tests can swap in a recorder and assert the
// constructed command without executing npm/npx.
type commandRunner func(dir, name string, args ...string) error

func execRunner(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Stdin is intentionally left detached. The official scaffolders
	// (create-vite, create-expo-app) only prompt — and create-vite even offers
	// to "Install with npm and start now?", which boots a dev server — when
	// stdin is an interactive TTY. Without it they run non-interactively with
	// the flags we pass and just scaffold: we create the project, never run it,
	// so the gofi init flow is never blocked waiting on a child process.
	return cmd.Run()
}

// nodeRunner runs the web/mobile create commands. Overridden in tests.
var nodeRunner commandRunner = execRunner

// CreateViteApp scaffolds a Vite + React + TypeScript app at <root>/<path> by
// invoking the official `npm create vite` tool. When useDS is true it also
// installs the gofi-ui design system into the new app. Requires Node.js on PATH
// (the caller gates this on the toolchain preflight).
func CreateViteApp(root, path string, useDS bool) error {
	if path == "" {
		return fmt.Errorf("web path is required")
	}
	if err := nodeRunner(root, "npm", "create", "vite@latest", path, "--", "--template", "react-ts"); err != nil {
		return fmt.Errorf("npm create vite: %w", err)
	}
	if useDS {
		appDir := filepath.Join(root, path)
		if err := nodeRunner(appDir, "npm", "install", DSWeb); err != nil {
			return fmt.Errorf("install %s: %w", DSWeb, err)
		}
		// Replace Vite's default entry (main.tsx / App.tsx / css) with a gofi-ui
		// hello-world: a centered Button that reveals a message on click.
		if err := seedStarter("embedded/web-starter", appDir); err != nil {
			return fmt.Errorf("seed gofi-ui starter: %w", err)
		}
	}
	return nil
}

// CreateExpoApp scaffolds a React Native (Expo) + TypeScript app at
// <root>/<path> via the official `create-expo-app` tool. When useDS is true it
// installs the gofi-ui-native design system. Requires Node.js on PATH.
func CreateExpoApp(root, path string, useDS bool) error {
	if path == "" {
		return fmt.Errorf("mobile path is required")
	}
	if err := nodeRunner(root, "npx", "--yes", "create-expo-app@latest", path, "--template", "blank-typescript"); err != nil {
		return fmt.Errorf("create-expo-app: %w", err)
	}
	if useDS {
		appDir := filepath.Join(root, path)
		if err := nodeRunner(appDir, "npm", "install", DSMobile); err != nil {
			return fmt.Errorf("install %s: %w", DSMobile, err)
		}
		// Replace Expo's default App.tsx with a gofi-ui-native hello-world: a
		// centered Button that reveals a message on press, in native app style.
		if err := seedStarter("embedded/expo-starter", appDir); err != nil {
			return fmt.Errorf("seed gofi-ui-native starter: %w", err)
		}
	}
	return nil
}

// seedStarter overlays an embedded starter tree (rooted at src inside the
// embedded FS) onto the freshly scaffolded app at appDir, overwriting the
// default entry files with a design-system hello-world. Directories are created
// as needed; unrelated generated files are left in place.
func seedStarter(src, appDir string) error {
	_, err := installFS(embeddedFS, src, appDir, TemplateData{}, InstallOptions{})
	return err
}

// DSWebConst / DSMobileConst mirror config.DSWeb/DSMobile without importing the
// config package (scaffold stays dependency-light). Kept in sync by tests.
const (
	DSWeb    = "gofi-ui"
	DSMobile = "gofi-ui-native"
)

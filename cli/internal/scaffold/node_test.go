package scaffold

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type recordedCmd struct {
	dir, name string
	args      []string
}

func captureNodeRunner(t *testing.T) *[]recordedCmd {
	t.Helper()
	var calls []recordedCmd
	orig := nodeRunner
	nodeRunner = func(dir, name string, args ...string) error {
		calls = append(calls, recordedCmd{dir: dir, name: name, args: args})
		return nil
	}
	t.Cleanup(func() { nodeRunner = orig })
	return &calls
}

func TestCreateViteApp_WithDS(t *testing.T) {
	calls := captureNodeRunner(t)
	root := t.TempDir()
	if err := CreateViteApp(root, "web", true); err != nil {
		t.Fatalf("CreateViteApp: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected create + install, got %d calls: %+v", len(*calls), *calls)
	}
	create := (*calls)[0]
	if create.dir != root || create.name != "npm" {
		t.Errorf("create ran in %s via %s", create.dir, create.name)
	}
	wantCreate := []string{"create", "vite@latest", "web", "--", "--template", "react-ts"}
	if !reflect.DeepEqual(create.args, wantCreate) {
		t.Errorf("create args = %v, want %v", create.args, wantCreate)
	}
	install := (*calls)[1]
	if install.dir != filepath.Join(root, "web") {
		t.Errorf("install ran in %s", install.dir)
	}
	if !reflect.DeepEqual(install.args, []string{"install", DSWeb}) {
		t.Errorf("install args = %v", install.args)
	}
	// The gofi-ui hello-world starter overwrites Vite's defaults.
	for _, f := range []string{"src/main.tsx", "src/App.tsx", "src/App.css"} {
		if _, err := os.Stat(filepath.Join(root, "web", f)); err != nil {
			t.Errorf("starter file %s not seeded: %v", f, err)
		}
	}
}

func TestCreateViteApp_NoDS(t *testing.T) {
	calls := captureNodeRunner(t)
	if err := CreateViteApp("/root", "web", false); err != nil {
		t.Fatalf("CreateViteApp: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected only create, got %d calls", len(*calls))
	}
}

func TestCreateExpoApp_WithDS(t *testing.T) {
	calls := captureNodeRunner(t)
	root := t.TempDir()
	if err := CreateExpoApp(root, "mobile", true); err != nil {
		t.Fatalf("CreateExpoApp: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected create + install, got %d", len(*calls))
	}
	create := (*calls)[0]
	if create.name != "npx" {
		t.Errorf("expected npx, got %s", create.name)
	}
	wantCreate := []string{"--yes", "create-expo-app@latest", "mobile", "--template", "blank-typescript"}
	if !reflect.DeepEqual(create.args, wantCreate) {
		t.Errorf("create args = %v, want %v", create.args, wantCreate)
	}
	if !reflect.DeepEqual((*calls)[1].args, []string{"install", DSMobile}) {
		t.Errorf("install args = %v", (*calls)[1].args)
	}
	// The gofi-ui-native hello-world starter overwrites Expo's default App.tsx.
	if _, err := os.Stat(filepath.Join(root, "mobile", "App.tsx")); err != nil {
		t.Errorf("starter App.tsx not seeded: %v", err)
	}
}

func TestCreateApp_EmptyPath(t *testing.T) {
	captureNodeRunner(t)
	if err := CreateViteApp("/root", "", false); err == nil {
		t.Error("expected error for empty web path")
	}
	if err := CreateExpoApp("/root", "", false); err == nil {
		t.Error("expected error for empty mobile path")
	}
}

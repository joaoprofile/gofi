package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func TestRunTestList(t *testing.T) {
	setupProject(t)
	if err := runTestList(); err != nil {
		t.Errorf("list: %v", err)
	}
}

func TestRunTest_NoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runTest(nil, nil); err == nil {
		t.Error("expected error without .gofi.yaml")
	}
}

func TestRunTest_DefaultTask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required")
	}
	root := setupProject(t)

	// Replace the seeded test section with a task that creates a marker file.
	marker := filepath.Join(root, "ran")
	cfg, err := config.Load(config.FileName)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Test.Default = "touch"
	cfg.Test.Tasks = map[string]config.TestTask{
		"touch": {Desc: "create marker", Run: "touch " + marker},
	}
	if err := config.Save(config.FileName, cfg); err != nil {
		t.Fatal(err)
	}

	if err := runTest(nil, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker created: %v", err)
	}
}

func TestRunTest_NamedTaskWithExtras(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required")
	}
	root := setupProject(t)

	marker := filepath.Join(root, "captured")
	cfg, _ := config.Load(config.FileName)
	cfg.Test.Default = "noop"
	cfg.Test.Tasks = map[string]config.TestTask{
		"noop": {Run: "true"},
		// Captures any extra args appended after the run string.
		"echo": {Run: "printf '%s\\n' > " + marker},
	}
	if err := config.Save(config.FileName, cfg); err != nil {
		t.Fatal(err)
	}

	if err := runTest([]string{"echo"}, []string{"hello"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("expected marker: %v", err)
	}
	if string(body) != "hello\n" {
		t.Errorf("expected 'hello\\n' in marker, got %q", body)
	}
}

func TestRunTest_TaskFailurePropagatesError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required")
	}
	setupProject(t)
	cfg, _ := config.Load(config.FileName)
	cfg.Test.Default = "fail"
	cfg.Test.Tasks = map[string]config.TestTask{
		"fail": {Run: "exit 7"},
	}
	if err := config.Save(config.FileName, cfg); err != nil {
		t.Fatal(err)
	}
	if err := runTest(nil, nil); err == nil {
		t.Error("expected non-zero exit to propagate as error")
	}
}

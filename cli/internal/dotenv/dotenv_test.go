package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileIsNoOp(t *testing.T) {
	set, err := Load(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("expected no keys set, got %v", set)
	}
}

func TestLoad_ParsesAndExports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := `# comment line
export SONAR_HOST_URL=http://localhost:9000
SONAR_TOKEN="secret-token"
QUOTED='single'
INLINE=value # trailing comment
EMPTY=

not_a_pair
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, k := range []string{"SONAR_HOST_URL", "SONAR_TOKEN", "QUOTED", "INLINE", "EMPTY"} {
		t.Setenv(k, "") // register for cleanup
		os.Unsetenv(k)
	}

	if _, err := Load(path); err != nil {
		t.Fatalf("load: %v", err)
	}

	want := map[string]string{
		"SONAR_HOST_URL": "http://localhost:9000",
		"SONAR_TOKEN":    "secret-token",
		"QUOTED":         "single",
		"INLINE":         "value",
		"EMPTY":          "",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}

func TestLoad_DoesNotOverrideExistingEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("SONAR_HOST_URL=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SONAR_HOST_URL", "from-shell")
	set, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := os.Getenv("SONAR_HOST_URL"); got != "from-shell" {
		t.Errorf("explicit env should win: got %q", got)
	}
	for _, k := range set {
		if k == "SONAR_HOST_URL" {
			t.Errorf("should not report overriding an already-set key")
		}
	}
}

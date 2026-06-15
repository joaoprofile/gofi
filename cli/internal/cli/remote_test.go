package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
	"github.com/joaoprofile/gofi-cli/internal/gitops"
)

func TestNormalizeRemoteURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"https://github.com/x/y", "https://github.com/x/y", false},
		{"https://github.com/x/y.git", "https://github.com/x/y.git", false},
		{"git@github.com:x/y.git", "git@github.com:x/y.git", false},
		{"github.com/x/y", "https://github.com/x/y", false},
		{"github.com/x/y.git", "https://github.com/x/y.git", false},
		{"  github.com/x/y  ", "https://github.com/x/y", false},
		{"", "", true},
		{"ftp://x", "", true},
		{"random", "", true},
	}
	for _, c := range cases {
		got, err := normalizeRemoteURL(c.in)
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected err %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

// setupProject creates a fresh project with executePipeline rooted at a temp
// dir, then chdirs into it so command helpers can find .gofi.yaml. Uses a
// local fixture repo via GOFI_AGENTS_LOCAL_DIR to avoid hitting GitHub.
func setupProject(t *testing.T) string {
	t.Helper()
	t.Setenv("GOFI_AGENTS_LOCAL_DIR", writeFixtureRepo(t))
	root := filepath.Join(t.TempDir(), "proj")
	r := goWizardResult(root)
	if err := executePipeline(r); err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	t.Chdir(root)
	return root
}

func TestRunRemoteAdd_AndShow_AndRemove(t *testing.T) {
	root := setupProject(t)

	url := "git@github.com:acme/foo.git"
	if err := runRemoteAdd(url); err != nil {
		t.Fatalf("add: %v", err)
	}

	// .gofi.yaml has the URL
	cfg, err := config.Load(config.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Git.Remote != url {
		t.Errorf("expected URL in .gofi.yaml, got %q", cfg.Git.Remote)
	}
	// git remote also configured
	if got, _ := gitops.GetRemote(root, "origin"); got != url {
		t.Errorf("expected origin %q, got %q", url, got)
	}

	if err := runRemoteShow(); err != nil {
		t.Errorf("show: %v", err)
	}

	if err := runRemoteAdd("https://x"); err == nil {
		t.Errorf("expected error on add when remote exists")
	}

	if err := runRemoteRemove(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	cfg2, _ := config.Load(config.FileName)
	if cfg2.Git.Remote != "" {
		t.Errorf("expected empty remote after remove")
	}
	if got, _ := gitops.GetRemote(root, "origin"); got != "" {
		t.Errorf("expected git origin removed, got %q", got)
	}
}

func TestRunRemote_NoConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := os.Stat(config.FileName); !os.IsNotExist(err) {
		t.Skip("temp dir somehow has .gofi.yaml")
	}
	if err := runRemoteShow(); err == nil {
		t.Error("expected error without .gofi.yaml")
	}
	if err := runRemoteAdd("git@github.com:x/y.git"); err == nil {
		t.Error("expected error without .gofi.yaml")
	}
}

package train

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTopic(t *testing.T) {
	for _, ok := range []string{"x", "foo", "foo-bar", "x9", "a-b-c", "a"} {
		if err := ValidateTopic(ok); err != nil {
			t.Errorf("expected %q valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Foo", "9bad", "a_b", "with space", "trailing-", "-leading"} {
		if err := ValidateTopic(bad); err == nil {
			t.Errorf("expected %q invalid", bad)
		}
	}
}

func TestInstall_WritesHeaderedFile(t *testing.T) {
	root := t.TempDir()
	item, err := Install(root, "pd", "domain", "./docs/x.md", []byte("# hello\n"), "2026-04-25", false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if item.Topic != "domain" || item.Source != "./docs/x.md" || item.InstalledAt != "2026-04-25" {
		t.Errorf("unexpected item: %+v", item)
	}
	if !strings.HasPrefix(item.Hash, "sha256:") || len(item.Hash) != len("sha256:")+64 {
		t.Errorf("unexpected hash format: %s", item.Hash)
	}

	body, err := os.ReadFile(TopicPath(root, "pd", "domain"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.HasPrefix(s, "<!-- gofi-train") {
		t.Errorf("missing header in:\n%s", s)
	}
	if !strings.Contains(s, "source: ./docs/x.md") {
		t.Errorf("missing source line")
	}
	if !strings.Contains(s, "# hello") {
		t.Errorf("missing original content")
	}
}

func TestInstall_RejectsEmpty(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "pd", "x", "src", []byte("   \n  \n"), "2026-04-25", false); err == nil {
		t.Error("expected error on empty content")
	}
}

func TestInstall_RejectsBadTopic(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "pd", "Bad-Topic", "src", []byte("x"), "2026-04-25", false); err == nil {
		t.Error("expected slug error")
	}
}

func TestInstall_NoOverwriteWithoutReplace(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "pd", "x", "src1", []byte("a"), "d", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, "pd", "x", "src2", []byte("b"), "d", false); err == nil {
		t.Error("expected error on duplicate topic")
	}
}

func TestInstall_ReplaceOverwrites(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "pd", "x", "src1", []byte("a"), "d", false); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(root, "pd", "x", "src2", []byte("brand new"), "d", true); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(TopicPath(root, "pd", "x"))
	if !strings.Contains(string(body), "brand new") {
		t.Errorf("replace did not overwrite content:\n%s", body)
	}
}

func TestList(t *testing.T) {
	root := t.TempDir()
	for _, top := range []string{"a", "b", "c"} {
		if _, err := Install(root, "pd", top, "x", []byte("ok"), "d", false); err != nil {
			t.Fatal(err)
		}
	}
	got, err := List(root, "pd")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 topics, got %v", got)
	}
}

func TestList_EmptyScopeReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := List(root, "spec")
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty: %v", got)
	}
}

func TestRemove(t *testing.T) {
	root := t.TempDir()
	if _, err := Install(root, "pd", "x", "src", []byte("ok"), "d", false); err != nil {
		t.Fatal(err)
	}
	if err := Remove(root, "pd", "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(TopicPath(root, "pd", "x")); !os.IsNotExist(err) {
		t.Errorf("expected topic file removed")
	}
	// idempotent
	if err := Remove(root, "pd", "x"); err != nil {
		t.Errorf("expected idempotent remove: %v", err)
	}
}

func TestScopeDir(t *testing.T) {
	got := ScopeDir("/proj", "pd")
	want := filepath.Join("/proj", ".claude", "knowledge", "pd")
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

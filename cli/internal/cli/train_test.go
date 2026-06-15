package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func writeFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunTrain_FileMode(t *testing.T) {
	root := setupProject(t)
	src := writeFixture(t, t.TempDir(), "dominio-fiscal.md", "# Domínio fiscal\n\nregra X\n")

	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatalf("train: %v", err)
	}

	// File installed with header
	body, err := os.ReadFile(filepath.Join(root, ".claude/knowledge/pd/dominio-fiscal.md"))
	if err != nil {
		t.Fatalf("expected installed file: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "<!-- gofi-train") {
		t.Errorf("expected header, got:\n%s", s)
	}
	if !strings.Contains(s, "Domínio fiscal") {
		t.Errorf("expected original content")
	}

	// .gofi.yaml has the entry
	cfg, err := config.Load(config.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Training.PD) != 1 || cfg.Training.PD[0].Topic != "dominio-fiscal" {
		t.Errorf("expected training entry, got %+v", cfg.Training)
	}
}

func TestRunTrain_DuplicateWithoutReplace(t *testing.T) {
	setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "first\n")
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err == nil {
		t.Error("expected duplicate error without --replace")
	}
}

func TestRunTrain_ReplaceOverwrites(t *testing.T) {
	root := setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "first\n")
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	src2 := writeFixture(t, t.TempDir(), "x.md", "second content\n")
	if err := runTrain([]string{src2}, "gofi-pd", false, "", true, "", "", true); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, ".claude/knowledge/pd/x.md"))
	if !strings.Contains(string(body), "second content") {
		t.Errorf("replace did not overwrite:\n%s", body)
	}
}

func TestRunTrain_Shared(t *testing.T) {
	root := setupProject(t)
	src := writeFixture(t, t.TempDir(), "glossario.md", "shared content\n")
	if err := runTrain([]string{src}, "", true, "", false, "", "", true); err != nil {
		t.Fatalf("train shared: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/shared/glossario.md")); err != nil {
		t.Errorf("expected shared topic: %v", err)
	}
	cfg, _ := config.Load(config.FileName)
	if len(cfg.Training.Shared) != 1 {
		t.Errorf("expected 1 shared entry, got %d", len(cfg.Training.Shared))
	}
}

func TestRunTrain_Multiple(t *testing.T) {
	root := setupProject(t)
	dir := t.TempDir()
	a := writeFixture(t, dir, "a.md", "content a")
	b := writeFixture(t, dir, "b.md", "content b")

	if err := runTrain([]string{a, b}, "gofi-spec", false, "", false, "", "", true); err != nil {
		t.Fatalf("train: %v", err)
	}
	for _, name := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/spec/"+name+".md")); err != nil {
			t.Errorf("expected %s topic: %v", name, err)
		}
	}
	cfg, _ := config.Load(config.FileName)
	if len(cfg.Training.Spec) != 2 {
		t.Errorf("expected 2 spec entries, got %d", len(cfg.Training.Spec))
	}
}

func TestRunTrain_AgentNotActive(t *testing.T) {
	setupProject(t)
	if err := runAgentRemove("gofi-qa", true); err != nil {
		t.Fatalf("pre-remove: %v", err)
	}
	src := writeFixture(t, t.TempDir(), "x.md", "x")
	if err := runTrain([]string{src}, "gofi-qa", false, "", false, "", "", true); err == nil {
		t.Error("expected error: agent not active")
	}
}

func TestRunTrain_AgentAndSharedExclusive(t *testing.T) {
	setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "x")
	if err := runTrain([]string{src}, "gofi-pd", true, "", false, "", "", true); err == nil {
		t.Error("expected mutually-exclusive error")
	}
}

func TestRunTrain_TopicOverride(t *testing.T) {
	root := setupProject(t)
	src := writeFixture(t, t.TempDir(), "weird_name.md", "x")
	if err := runTrain([]string{src}, "gofi-pd", false, "custom-topic", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/pd/custom-topic.md")); err != nil {
		t.Errorf("expected custom-topic: %v", err)
	}
}

func TestRunTrainList_Empty(t *testing.T) {
	setupProject(t)
	if err := runTrainList("gofi-pd", false); err != nil {
		t.Errorf("list empty: %v", err)
	}
}

func TestRunTrainList_AfterInstall(t *testing.T) {
	setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "x")
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	if err := runTrainList("gofi-pd", false); err != nil {
		t.Errorf("list: %v", err)
	}
}

func TestRunTrainRemove(t *testing.T) {
	root := setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "x")
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	if err := runTrainRemove("gofi-pd", false, "x"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/pd/x.md")); !os.IsNotExist(err) {
		t.Error("expected file removed")
	}
	cfg, _ := config.Load(config.FileName)
	if len(cfg.Training.PD) != 0 {
		t.Errorf("expected empty training list, got %+v", cfg.Training.PD)
	}
}

func TestRunTrainShow(t *testing.T) {
	setupProject(t)
	src := writeFixture(t, t.TempDir(), "x.md", "show me")
	if err := runTrain([]string{src}, "gofi-pd", false, "", false, "", "", true); err != nil {
		t.Fatal(err)
	}
	if err := runTrainShow("gofi-pd", false, "x"); err != nil {
		t.Errorf("show: %v", err)
	}
}

func TestTopicFromBasename(t *testing.T) {
	cases := map[string]string{
		"foo.md":          "foo",
		"FOO.md":          "foo",
		"foo bar.md":      "foo-bar",
		"foo_bar.md":      "foo-bar",
		"already-slug.md": "already-slug",
	}
	for in, want := range cases {
		if got := topicFromBasename(in); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}

func TestStripGofiHeader(t *testing.T) {
	body := `<!-- gofi-train
  source: x
  installed_at: 2026-04-25
  hash: sha256:abc
-->

# real content`
	got := stripGofiHeader(body)
	if !strings.HasPrefix(got, "# real content") {
		t.Errorf("expected stripped, got:\n%s", got)
	}
}

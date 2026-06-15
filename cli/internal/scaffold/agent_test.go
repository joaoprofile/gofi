package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidAgent(t *testing.T) {
	for _, ok := range []string{"gofi-pd", "gofi-spec", "gofi-eng", "gofi-qa"} {
		if !IsValidAgent(ok) {
			t.Errorf("expected %s to be valid", ok)
		}
	}
	for _, bad := range []string{"", "pd", "gofi-foo", "gofi-spec ", "GOFI-PD"} {
		if IsValidAgent(bad) {
			t.Errorf("expected %q to be invalid", bad)
		}
	}
}

func TestInstallAgentFromFS(t *testing.T) {
	root := t.TempDir()
	if err := InstallAgentFromFS(fixtureFS(), ".", root, "gofi-pd"); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/gofi-pd.md")); err != nil {
		t.Errorf("expected skills/gofi-pd.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/pd")); err != nil {
		t.Errorf("expected knowledge/pd/: %v", err)
	}
}

func TestInstallAgentFromFS_Unknown(t *testing.T) {
	root := t.TempDir()
	if err := InstallAgentFromFS(fixtureFS(), ".", root, "gofi-foo"); err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

func TestRemoveAgent_KeepsKnowledge(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())
	// Write a user training topic.
	topic := filepath.Join(root, ".claude/knowledge/pd/x.md")
	if err := os.WriteFile(topic, []byte("user content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAgent(root, "gofi-pd", false); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/skills/gofi-pd.md")); !os.IsNotExist(err) {
		t.Errorf("expected skill to be removed")
	}
	if _, err := os.Stat(topic); err != nil {
		t.Errorf("expected knowledge preserved: %v", err)
	}
}

func TestRemoveAgent_DropsKnowledge(t *testing.T) {
	root := t.TempDir()
	installAgentsFromFixture(t, root, sampleData())
	if err := RemoveAgent(root, "gofi-pd", true); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude/knowledge/pd")); !os.IsNotExist(err) {
		t.Errorf("expected knowledge/pd to be removed")
	}
}

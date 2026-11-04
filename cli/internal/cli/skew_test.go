package cli

import (
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/engine"
)

func withCommit(t *testing.T, c string) {
	t.Helper()
	prev := Commit
	Commit = c
	t.Cleanup(func() { Commit = prev })
}

// Skills installed from one commit of gofi and a binary built from another
// name different folders: the binary wrote changes in specs/changes/ while
// the skills sent the agents to specs/<context>/changes/. The pair is
// refused, unless the build says no commit or the person allows it.
func TestSkillsOfAnotherCommitThanTheBinaryAreRefused(t *testing.T) {
	root := t.TempDir()
	if err := writeInstalledSha(root, "a07a9bf87f25bcfaa2021d0f982e254cf770668a"); err != nil {
		t.Fatal(err)
	}
	withCommit(t, "f76cf76")
	err := checkAgentsSkew(root)
	if err == nil || !strings.Contains(err.Error(), "a07a9bf") || !strings.Contains(err.Error(), "f76cf76") {
		t.Fatalf("skew of a07a9bf skills and an f76cf76 binary: %v", err)
	}
	t.Setenv(EnvAllowSkew, "1")
	if err := checkAgentsSkew(root); err != nil {
		t.Errorf("%s=1 must allow it: %v", EnvAllowSkew, err)
	}
}

func TestSkillsOfTheBinarysCommitAreAccepted(t *testing.T) {
	root := t.TempDir()
	if err := writeInstalledSha(root, "a07a9bf87f25bcfaa2021d0f982e254cf770668a"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"a07a9bf", "a07a9bf-dirty", "A07A9BF87F25", "none", ""} {
		withCommit(t, c)
		if err := checkAgentsSkew(root); err != nil {
			t.Errorf("binary %q: %v", c, err)
		}
	}
	if rec := loadInstalled(root); rec.CLI != "" {
		t.Errorf("installed by a build with no commit, cli = %q", rec.CLI)
	}
	if err := checkSourceSkew("local"); err != nil {
		t.Errorf("skills from a local folder: %v", err)
	}
}

func TestInstallingSkillsOfAnotherCommitIsRefused(t *testing.T) {
	withCommit(t, "2698456")
	if err := checkSourceSkew("a07a9bf87f25bcfaa2021d0f982e254cf770668a"); err == nil {
		t.Fatal("a source of another commit was accepted")
	}
	if err := checkSourceSkew("2698456aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Errorf("the binary's own commit: %v", err)
	}
}

// gofi run refuses a project whose skills are of another commit before any
// turn is spent.
func TestARunRefusesSkillsOfAnotherCommit(t *testing.T) {
	root := committedChangeProject(t, "true")
	var prompts []string
	runPorts.open = func(dir string) engine.Session { return fakeSession{root: dir, prompts: &prompts} }
	if err := writeInstalledSha(root, "a07a9bf87f25bcfaa2021d0f982e254cf770668a"); err != nil {
		t.Fatal(err)
	}
	withCommit(t, "f76cf76")
	out, err := execCmd(newRunCmd, "criar-pedidos")
	if err == nil || len(prompts) != 0 {
		t.Fatalf("run with skewed skills: err %v, %d turns\n%s", err, len(prompts), out)
	}
}

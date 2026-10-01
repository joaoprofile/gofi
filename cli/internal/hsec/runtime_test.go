package hsec

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gofi-labs/gofi/cli/internal/config"
)

// Regression: horusec v2.8.0 slices the daemon version by fixed offsets, so
// any single-digit minor (Docker 24.0, 29.4 …) is rejected as "below 19.3".
func TestHorusecAcceptsDockerVersion(t *testing.T) {
	cases := map[string]bool{
		"29.4.0":   false,
		"24.0.7":   false,
		"20.10.24": true,
		"19.03.15": true,
		"18.09.1":  false,
		"":         false,
	}
	for v, want := range cases {
		if got := horusecAcceptsDockerVersion(v); got != want {
			t.Errorf("%q: got %v want %v", v, got, want)
		}
	}
}

func isolatedHsec() config.HsecConfig {
	c := config.DefaultHsec()
	c.UseDocker = true
	c.DockerRuntime = config.HsecDockerRuntimeIsolated
	return c
}

func TestCommand_HostRunsLocalBinary(t *testing.T) {
	c := config.DefaultHsec()
	c.UseDocker = true
	name, args := command(c, RunOptions{ProjectRoot: "/w/proj", ConfigPath: "/w/proj/.gofi/horusec-config.json"})
	if name != "horusec" || slices.Contains(args, "-P") {
		t.Fatalf("host runtime must call the local binary without -P, got %s %v", name, args)
	}
}

func TestCommand_IsolatedRunsOfficialImageAgainstPinnedDaemon(t *testing.T) {
	root := "/w/proj"
	name, args := command(isolatedHsec(), RunOptions{ProjectRoot: root, ConfigPath: root + "/.gofi/horusec-config.json"})
	joined := strings.Join(args, " ")
	container, _ := IsolatedNames(root)
	if name != "docker" {
		t.Fatalf("isolated runtime runs through docker, got %s", name)
	}
	for _, want := range []string{
		"--network container:" + container,
		"DOCKER_HOST=" + isolatedDaemonHost,
		"-v " + root + ":" + root,
		IsolatedCLIImage + " horusec start --config-file-path " + root + "/.gofi/horusec-config.json",
		"-P " + root,
		"--ulimit nofile=",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in: %s", want, joined)
		}
	}
	if strings.Contains(joined, "--disable-docker") {
		t.Error("isolated runtime exists to run the docker tools")
	}
	if slices.Contains(args, "-e") && strings.Contains(joined, horusecAuthEnv) {
		t.Error("local isolated run must not forward the token variable")
	}
}

func TestCommand_IsolatedPublishForwardsTokenByNameOnly(t *testing.T) {
	name, args := command(isolatedHsec(), RunOptions{ProjectRoot: "/w/p", ConfigPath: "/w/p/c.json", Publish: true, AuthToken: fakeToken})
	joined := strings.Join(args, " ")
	if name != "docker" || !strings.Contains(joined, "-e "+horusecAuthEnv+" ") {
		t.Fatalf("publish must forward %s by name (value from env), got %s", horusecAuthEnv, joined)
	}
	if strings.Contains(joined, fakeToken) {
		t.Fatalf("token value leaked into docker argv: %s", joined)
	}
}

func TestIsolatedNames_StablePerProject(t *testing.T) {
	a1, v1 := IsolatedNames("/w/a")
	a2, _ := IsolatedNames("/w/a")
	b, _ := IsolatedNames("/w/b")
	if a1 != a2 || a1 == b {
		t.Fatalf("names must be stable per root and differ across roots: %s %s %s", a1, a2, b)
	}
	if !strings.HasPrefix(v1, a1) {
		t.Errorf("volume should be tied to its daemon: %s / %s", a1, v1)
	}
}

// Regression: the privileged isolated daemon listened on 0.0.0.0:2375 without
// TLS, reachable from the host and every container on the bridge.
func TestIsolatedDaemonRunArgs_ListensOnlyOnLoopback(t *testing.T) {
	args := isolatedDaemonRunArgs("/w/p", "c", "v")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--host=tcp://127.0.0.1:2375") {
		t.Fatalf("daemon must listen on loopback only: %s", joined)
	}
	if strings.Contains(joined, "0.0.0.0") {
		t.Fatalf("daemon must not listen on all interfaces: %s", joined)
	}
	i := slices.Index(args, IsolatedDaemonImage)
	if i < 0 || i+1 >= len(args) || args[i+1] != "dockerd" {
		t.Fatalf("an explicit dockerd command keeps the image entrypoint from adding its default 0.0.0.0 host: %v", args)
	}
	if !strings.Contains(joined, isolatedDaemonLabel+"="+isolatedDaemonRevision) {
		t.Fatalf("daemon must carry its setup revision: %s", joined)
	}
}

func TestOutdatedDaemon_RecreatesExposedOnes(t *testing.T) {
	if !outdatedDaemon("") {
		t.Fatal("a daemon created before the revision label (exposed setup) must be recreated")
	}
	if outdatedDaemon(isolatedDaemonRevision) {
		t.Fatal("a current daemon must be reused")
	}
}

// Regression: horusec ran as root in its container, so a killed run left a
// work dir the developer could only delete with sudo.
func TestCommand_IsolatedRunsAsCallingUser(t *testing.T) {
	if os.Getuid() < 0 {
		t.Skip("no uid on this platform")
	}
	_, args := command(isolatedHsec(), RunOptions{ProjectRoot: "/w/p", ConfigPath: "/w/p/c.json"})
	want := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	i := slices.Index(args, "--user")
	if i < 0 || args[i+1] != want {
		t.Fatalf("expected --user %s, got %v", want, args)
	}
	if img := slices.Index(args, IsolatedCLIImage); img < i {
		t.Fatalf("--user must be a docker run flag, before the image: %v", args)
	}
}

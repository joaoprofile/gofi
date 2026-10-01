package hsec

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joaoprofile/gofi/cli/internal/config"
)

// Pinned pair for the isolated runtime: horusec v2.8.0 reads the daemon
// version by fixed string offsets and rejects any Docker whose minor version
// has one digit (e.g. 29.4.0), so it gets a 20.10 daemon of its own.
const (
	IsolatedCLIImage    = "horuszup/horusec-cli:v2.8.0"
	IsolatedDaemonImage = "docker:20.10-dind"
	isolatedDaemonHost  = "tcp://localhost:2375"
	// The daemon is privileged and has no TLS: it must listen only inside its
	// own network namespace, which the horusec container joins. Bumping the
	// revision recreates daemons created with an older (exposed) setup.
	isolatedDaemonListen   = "tcp://127.0.0.1:2375"
	isolatedDaemonLabel    = "gofi.hsec.dind"
	isolatedDaemonRevision = "2"
	isolatedReadyWait      = 90 * time.Second
	// The native engine opens every project file at once; the container
	// default (1024) aborts it with "too many open files" on a monorepo.
	isolatedNoFile = "65536"
)

func isIsolated(cfg config.HsecConfig) bool {
	return cfg.UseDocker && cfg.DockerRuntime == config.HsecDockerRuntimeIsolated
}

// IsolatedNames returns the daemon container and its image-cache volume for a
// project. One daemon per project root: it bind-mounts the root at the same
// path, which is what horusec's tool containers mount in turn.
func IsolatedNames(projectRoot string) (container, volume string) {
	sum := sha1.Sum([]byte(projectRoot))
	id := hex.EncodeToString(sum[:])[:10]
	return "gofi-hsec-dind-" + id, "gofi-hsec-dind-" + id + "-data"
}

func command(cfg config.HsecConfig, opts RunOptions) (string, []string) {
	hz := buildArgs(cfg, opts)
	if !isIsolated(cfg) {
		return "horusec", hz
	}
	container, _ := IsolatedNames(opts.ProjectRoot)
	args := []string{
		"run", "--rm",
		"--ulimit", "nofile=" + isolatedNoFile + ":" + isolatedNoFile,
	}
	// Files horusec writes into the mounted project (its work dir, the output)
	// belong to the developer, not root — a killed run leaves nothing only
	// sudo can delete.
	if uid, gid := os.Getuid(), os.Getgid(); uid >= 0 && gid >= 0 {
		args = append(args, "--user", strconv.Itoa(uid)+":"+strconv.Itoa(gid))
	}
	args = append(args,
		"--network", "container:"+container,
		"-e", "DOCKER_HOST="+isolatedDaemonHost,
	)
	if opts.Publish && opts.AuthToken != "" {
		args = append(args, "-e", horusecAuthEnv)
	}
	args = append(args,
		"-v", opts.ProjectRoot+":"+opts.ProjectRoot,
		"-w", opts.ProjectRoot,
		IsolatedCLIImage, "horusec",
	)
	return "docker", append(args, hz...)
}

// EnsureIsolatedDaemon starts (or reuses) the project's pinned Docker daemon
// and waits until it answers. The image-cache volume survives between runs;
// the container is stopped after each scan (StopIsolatedDaemon).
func EnsureIsolatedDaemon(projectRoot string, progress io.Writer) error {
	container, volume := IsolatedNames(projectRoot)
	rev, err := dockerOutput("inspect", "-f", "{{index .Config.Labels \""+isolatedDaemonLabel+"\"}}", container)
	exists := err == nil
	if exists && outdatedDaemon(rev) {
		fmt.Fprintf(progress, "Recreating isolated Docker daemon %s (outdated setup) …\n", container)
		if _, err := dockerOutput("rm", "-f", container); err != nil {
			return fmt.Errorf("remove outdated isolated docker daemon %s: %w", container, err)
		}
		exists = false
	}
	if exists {
		if _, err := dockerOutput("start", container); err != nil {
			return fmt.Errorf("start isolated docker daemon %s: %w", container, err)
		}
	} else {
		fmt.Fprintf(progress, "Starting isolated Docker daemon %s (%s) …\n", container, IsolatedDaemonImage)
		if _, err := dockerOutput(isolatedDaemonRunArgs(projectRoot, container, volume)...); err != nil {
			return fmt.Errorf("create isolated docker daemon %s: %w", container, err)
		}
	}
	deadline := time.Now().Add(isolatedReadyWait)
	for {
		if _, err := dockerOutput("exec", container, "docker", "version", "--format", "{{.Server.Version}}"); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("isolated docker daemon %s did not become ready within %s", container, isolatedReadyWait)
		}
		time.Sleep(time.Second)
	}
}

func outdatedDaemon(revisionLabel string) bool {
	return revisionLabel != isolatedDaemonRevision
}

func isolatedDaemonRunArgs(projectRoot, container, volume string) []string {
	return []string{
		"run", "-d", "--privileged",
		"--name", container,
		"--label", isolatedDaemonLabel + "=" + isolatedDaemonRevision,
		"-e", "DOCKER_TLS_CERTDIR=",
		"-v", projectRoot + ":" + projectRoot,
		"-v", volume + ":/var/lib/docker",
		IsolatedDaemonImage,
		"dockerd", "--host=unix:///var/run/docker.sock", "--host=" + isolatedDaemonListen,
	}
}

// StopIsolatedDaemon stops the project's daemon, keeping its cache volume, so
// no privileged daemon stays up between scans.
func StopIsolatedDaemon(projectRoot string) error {
	container, _ := IsolatedNames(projectRoot)
	if _, err := dockerOutput("stop", container); err != nil {
		return fmt.Errorf("stop isolated docker daemon %s: %w", container, err)
	}
	return nil
}

// RemoveIsolatedDaemon drops the project's daemon container and its cache,
// and a leftover horusec work dir — through a container when a run from
// before the non-root fix left it owned by root.
func RemoveIsolatedDaemon(projectRoot string) error {
	if removeWorkDir(projectRoot) != nil {
		if _, err := dockerOutput("run", "--rm", "--entrypoint", "rm",
			"-v", projectRoot+":"+projectRoot,
			IsolatedCLIImage, "-rf", filepath.Join(projectRoot, WorkDirName)); err != nil {
			return fmt.Errorf("remove leftover %s: %w", WorkDirName, err)
		}
	}
	container, volume := IsolatedNames(projectRoot)
	_, _ = dockerOutput("rm", "-f", container)
	if _, err := dockerOutput("volume", "rm", "-f", volume); err != nil {
		return fmt.Errorf("remove volume %s: %w", volume, err)
	}
	return nil
}

// ErrHostDockerIncompatible means horusec v2 cannot parse the host daemon's
// version and would abort with a misleading "docker not found".
var ErrHostDockerIncompatible = errors.New("horusec v2 cannot run tools on this Docker daemon")

// CheckHostDockerCompatible reproduces horusec's own version gate on the host
// daemon so the failure is explained before the scan starts.
func CheckHostDockerCompatible() error {
	v, err := dockerOutput("version", "--format", "{{.Server.Version}}")
	if err != nil {
		return fmt.Errorf("read docker server version: %w", err)
	}
	if !horusecAcceptsDockerVersion(v) {
		return fmt.Errorf("%w (server %s); set hsec.docker_runtime: isolated", ErrHostDockerIncompatible, v)
	}
	return nil
}

// horusecAcceptsDockerVersion mirrors horusec v2.8.0: major from [0:2],
// minor from [3:5], minimum 19.03.
func horusecAcceptsDockerVersion(v string) bool {
	if len(v) < 5 {
		return false
	}
	major, err1 := strconv.Atoi(v[0:2])
	minor, err2 := strconv.Atoi(v[3:5])
	if err1 != nil || err2 != nil {
		return false
	}
	return major > 19 || (major == 19 && minor >= 3)
}

func dockerOutput(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

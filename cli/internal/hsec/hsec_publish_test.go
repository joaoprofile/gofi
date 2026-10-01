package hsec

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofi-labs/gofi/cli/internal/config"
)

const fakeToken = "11111111-2222-3333-4444-555555555555"

// Lines captured from horusec v2.8.0 when POST /api/analysis fails. horusec
// still exits 0 and writes a clean JSON output in that case.
const (
	horusecTransportErrLine = `time="2026-09-18T06:47:51-03:00" level=error msg="{ERROR_HTTP} failed to make request" error="Post \"http://127.0.0.1:1/api/analysis\": dial tcp 127.0.0.1:1: connect: connection refused"`
	horusecSendErrLine      = `time="2026-09-18T06:47:51-03:00" level=error msg="[HORUSEC] Post \"http://127.0.0.1:1/api/analysis\": dial tcp 127.0.0.1:1: connect: connection refused"`
	horusecNoTokenLine      = `time="2026-09-18T06:47:51-03:00" level=warning msg="{HORUSEC_CLI} No authorization token was found, your code it is not going to be sent to horusec."`
	horusecToolErrLine      = `time="2026-09-18T06:47:51-03:00" level=error msg="{HORUSEC_CLI} Something went wrong in tool GoSec"`
)

func managerHsec() config.HsecConfig {
	c := config.DefaultHsec()
	c.EnableGitHistory = true
	c.EnableCommitAuthor = true
	c.EnableOwaspDependencyCheck = true
	c.Manager = &config.HsecManagerConfig{
		URL:            "https://horusec.example.com/",
		RepositoryName: "MY-REPO",
		TimeoutSeconds: 300,
	}
	return c
}

func TestBuildHorusecConfig_RendersManagerAndAnalysisFlags(t *testing.T) {
	body, err := BuildHorusecConfig(managerHsec(), "/tmp/out.json")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var got horusecJSON
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.HorusecApiUri != "https://horusec.example.com" {
		t.Errorf("manager url must be the trimmed base (horusec appends /api/analysis), got %q", got.HorusecApiUri)
	}
	if got.RepositoryName != "MY-REPO" || got.TimeoutInSecondsRequest != 300 {
		t.Errorf("manager identity not rendered: %+v", got)
	}
	if !got.EnableGitHistoryAnalysis || !got.EnableCommitAuthor || !got.EnableOwaspDependencyCheck {
		t.Errorf("analysis flags not rendered: %+v", got)
	}
}

func TestBuildHorusecConfig_NeverRendersRepositoryToken(t *testing.T) {
	t.Setenv(AuthTokenEnv, fakeToken)
	body, err := BuildHorusecConfig(managerHsec(), "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(string(body), "RepositoryAuthorization") || strings.Contains(string(body), fakeToken) {
		t.Fatalf("the rendered config may be tracked by git — it must never carry the token:\n%s", body)
	}
}

func TestBuildHorusecConfig_NoManagerRendersNoManagerKeys(t *testing.T) {
	body, err := BuildHorusecConfig(config.DefaultHsec(), "")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, k := range []string{"horusecCliHorusecApiUri", "horusecCliRepositoryName", "horusecCliTimeoutInSecondsRequest"} {
		if strings.Contains(string(body), k) {
			t.Errorf("default config must stay unchanged, found %q:\n%s", k, body)
		}
	}
}

// Regression: horusec v2 maps -c to --custom-rules-path, so passing the
// rendered file with -c left the whole hsec: block unapplied.
func TestBuildArgs_PassesRenderedFileAsConfigNotCustomRules(t *testing.T) {
	args := buildArgs(config.DefaultHsec(), RunOptions{ProjectRoot: "/p", ConfigPath: "/p/.gofi/horusec-config.json"})
	i := slices.Index(args, "--config-file-path")
	if i < 0 || i+1 >= len(args) || args[i+1] != "/p/.gofi/horusec-config.json" {
		t.Fatalf("rendered config must go through --config-file-path, got %v", args)
	}
	if slices.Contains(args, "-c") {
		t.Fatalf("-c is --custom-rules-path in horusec v2; it must not carry the config: %v", args)
	}
}

func TestBuildArgs_DisableDockerOnlyWhenUseDockerIsFalse(t *testing.T) {
	opts := RunOptions{ProjectRoot: "/p", ConfigPath: "/p/.gofi/horusec-config.json"}
	c := config.DefaultHsec()
	if !slices.Contains(buildArgs(c, opts), "--disable-docker") {
		t.Error("default must keep --disable-docker (previous behaviour)")
	}
	c.UseDocker = true
	if slices.Contains(buildArgs(c, opts), "--disable-docker") {
		t.Error("use_docker: true must drop --disable-docker, otherwise GoSec/NpmAudit/etc never run")
	}
}

func TestBuildArgs_NeverCarriesToken(t *testing.T) {
	args := buildArgs(managerHsec(), RunOptions{ProjectRoot: "/p", ConfigPath: "/c", Publish: true, AuthToken: fakeToken})
	for _, a := range args {
		if strings.Contains(a, fakeToken) || a == "-a" || a == "--authorization" {
			t.Fatalf("token must not travel in argv (visible in ps): %v", args)
		}
	}
}

func TestBuildEnv_LocalRunStripsInheritedHorusecToken(t *testing.T) {
	base := []string{"PATH=/usr/bin", horusecAuthEnv + "=" + fakeToken, AuthTokenEnv + "=" + fakeToken}
	env := buildEnv(base, RunOptions{Publish: false})
	for _, kv := range env {
		if strings.Contains(kv, fakeToken) {
			t.Fatalf("an exported token must not make a local scan publish: %v", env)
		}
	}
	if !slices.Contains(env, "PATH=/usr/bin") {
		t.Errorf("unrelated env must be preserved: %v", env)
	}
}

func TestBuildEnv_PublishInjectsTokenForHorusec(t *testing.T) {
	env := buildEnv([]string{"PATH=/usr/bin"}, RunOptions{Publish: true, AuthToken: fakeToken})
	if !slices.Contains(env, horusecAuthEnv+"="+fakeToken) {
		t.Fatalf("publish must hand the token to horusec via %s: %v", horusecAuthEnv, env)
	}
}

func TestBuildEnv_PublishWithoutTokenInjectsNothing(t *testing.T) {
	env := buildEnv([]string{"PATH=/usr/bin"}, RunOptions{Publish: true})
	for _, kv := range env {
		if strings.HasPrefix(kv, horusecAuthEnv+"=") {
			t.Fatalf("empty token must not be injected: %v", env)
		}
	}
}

func TestPublishFailureDetector_FlagsHorusecSendErrors(t *testing.T) {
	for name, line := range map[string]string{"transport": horusecTransportErrLine, "send": horusecSendErrLine} {
		t.Run(name, func(t *testing.T) {
			d := &publishFailureDetector{}
			_, _ = io.WriteString(d, line+"\n")
			if !d.failed() {
				t.Fatalf("expected publish failure for: %s", line)
			}
		})
	}
}

func TestPublishFailureDetector_LineSplitAcrossWrites(t *testing.T) {
	d := &publishFailureDetector{}
	half := len(horusecSendErrLine) / 2
	_, _ = io.WriteString(d, horusecSendErrLine[:half])
	if d.failed() {
		t.Fatal("must not decide on a partial line")
	}
	_, _ = io.WriteString(d, horusecSendErrLine[half:])
	d.flush()
	if !d.failed() {
		t.Fatal("expected detection once the line completes")
	}
}

func TestPublishFailureDetector_IgnoresUnrelatedLines(t *testing.T) {
	d := &publishFailureDetector{}
	_, _ = io.WriteString(d, horusecNoTokenLine+"\n"+horusecToolErrLine+"\nplain output line\n")
	d.flush()
	if d.failed() {
		t.Fatal("tool errors and warnings are not publish failures")
	}
}

func TestResolveAuthToken(t *testing.T) {
	t.Setenv(AuthTokenEnv, "")
	_, err := ResolveAuthToken()
	if err == nil || !strings.Contains(err.Error(), AuthTokenEnv) {
		t.Fatalf("expected actionable error naming %s, got %v", AuthTokenEnv, err)
	}
	t.Setenv(AuthTokenEnv, "  "+fakeToken+"  ")
	got, err := ResolveAuthToken()
	if err != nil || got != fakeToken {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCheckManager(t *testing.T) {
	var gotPath string
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()
	if err := CheckManager(healthy.URL+"/", time.Second); err != nil {
		t.Fatalf("healthy manager: %v", err)
	}
	if gotPath != "/api/health" {
		t.Errorf("healthcheck must hit /api/health on the base url, hit %q", gotPath)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	if err := CheckManager(down.URL, time.Second); err == nil {
		t.Fatal("expected error on 503")
	}

	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	if err := CheckManager(deadURL, time.Second); err == nil {
		t.Fatal("expected error when the manager is unreachable")
	}
}

// fakeHorusec puts a `horusec` script on PATH that records argv and the token
// it received, prints stdoutLines, and exits with exitCode.
func fakeHorusec(t *testing.T, stdoutLines string, exitCode int) (recordDir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake horusec is a POSIX shell script")
	}
	bin := t.TempDir()
	rec := t.TempDir()
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + filepath.Join(rec, "args") + "\n" +
		"printf '%s' \"$" + horusecAuthEnv + "\" > " + filepath.Join(rec, "token") + "\n" +
		"cat <<'EOF'\n" + stdoutLines + "\nEOF\n" +
		"exit " + string(rune('0'+exitCode)) + "\n"
	if err := os.WriteFile(filepath.Join(bin, "horusec"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return rec
}

func runOpts(publish bool) RunOptions {
	return RunOptions{ProjectRoot: ".", ConfigPath: "cfg.json", Publish: publish, AuthToken: fakeToken, Stdout: io.Discard, Stderr: io.Discard}
}

func TestRun_PublishFailureWithCleanExitIsNotSilent(t *testing.T) {
	fakeHorusec(t, horusecNoTokenLine+"\n"+horusecTransportErrLine+"\n"+horusecSendErrLine, 0)
	err := Run(context.Background(), managerHsec(), runOpts(true))
	if !errors.Is(err, ErrPublishFailed) {
		t.Fatalf("horusec exits 0 when the send fails; gofi must still report it, got %v", err)
	}
}

func TestRun_FindingsVerdictWinsOverPublishFailure(t *testing.T) {
	fakeHorusec(t, horusecSendErrLine, 1)
	err := Run(context.Background(), managerHsec(), runOpts(true))
	if err == nil || errors.Is(err, ErrPublishFailed) {
		t.Fatalf("a failing scan must surface as the scan verdict, got %v", err)
	}
}

func TestRun_LocalRunIgnoresSendLinesAndNeverPassesToken(t *testing.T) {
	rec := fakeHorusec(t, horusecSendErrLine, 0)
	t.Setenv(horusecAuthEnv, fakeToken)
	if err := Run(context.Background(), managerHsec(), runOpts(false)); err != nil {
		t.Fatalf("local run: %v", err)
	}
	tok, _ := os.ReadFile(filepath.Join(rec, "token"))
	if len(tok) != 0 {
		t.Fatalf("local run leaked token %q to horusec", tok)
	}
}

func TestRun_PublishHandsTokenThroughEnvOnly(t *testing.T) {
	rec := fakeHorusec(t, "ok", 0)
	if err := Run(context.Background(), managerHsec(), runOpts(true)); err != nil {
		t.Fatalf("publish run: %v", err)
	}
	tok, _ := os.ReadFile(filepath.Join(rec, "token"))
	if string(tok) != fakeToken {
		t.Fatalf("horusec did not receive the token via env, got %q", tok)
	}
	args, _ := os.ReadFile(filepath.Join(rec, "args"))
	if strings.Contains(string(args), fakeToken) {
		t.Fatalf("token leaked into argv: %s", args)
	}
}

// Regression: a stale output from the previous scan was read as the result of
// a scan that wrote nothing.
func TestRun_RemovesPreviousOutput(t *testing.T) {
	fakeHorusec(t, "ok", 0)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gofi"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, OutputFileName)
	if err := os.WriteFile(stale, []byte(`{"status":"success"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := runOpts(false)
	opts.ProjectRoot = root
	if err := Run(context.Background(), config.DefaultHsec(), opts); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous output must be removed before the scan, stat err = %v", err)
	}
}

// Regression: Ctrl+C killed the CLI outright — the privileged isolated daemon
// stayed up and horusec's work dir was left behind (owned by root).
func TestRun_InterruptSignalsChildAndCleansWorkDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signals")
	}
	root := t.TempDir()
	rec := filepath.Join(t.TempDir(), "signal")
	bin := t.TempDir()
	script := "#!/bin/sh\nmkdir -p " + filepath.Join(root, WorkDirName, "copy") + "\n" +
		"trap 'echo INT > " + rec + "; exit 130' INT\n" +
		"while true; do sleep 0.1; done\n"
	if err := os.WriteFile(filepath.Join(bin, "horusec"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(500 * time.Millisecond); cancel() }()
	opts := runOpts(false)
	opts.ProjectRoot = root
	start := time.Now()
	err := Run(ctx, config.DefaultHsec(), opts)

	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("expected ErrInterrupted, got %v", err)
	}
	if got, _ := os.ReadFile(rec); strings.TrimSpace(string(got)) != "INT" {
		t.Fatal("the child must get SIGINT (to clean up), not be killed")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("interrupt took too long: %s", time.Since(start))
	}
	if _, statErr := os.Stat(filepath.Join(root, WorkDirName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("leftover work dir must be removed, stat err = %v", statErr)
	}
}

func TestRun_LeavesNoWorkDirBehind(t *testing.T) {
	fakeHorusec(t, "ok", 0)
	root := t.TempDir()
	stale := filepath.Join(root, WorkDirName, "old-run")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := runOpts(false)
	opts.ProjectRoot = root
	if err := Run(context.Background(), config.DefaultHsec(), opts); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, WorkDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale work dir must be gone, stat err = %v", err)
	}
}

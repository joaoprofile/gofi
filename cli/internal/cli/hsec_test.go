package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/config"
	"github.com/joaoprofile/gofi/cli/internal/hsec"
)

// hsecProject writes a loadable .gofi.yaml into a temp dir, chdirs into it
// and puts a no-op `horusec` on PATH so the install check passes.
func hsecProject(t *testing.T, h config.HsecConfig) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake horusec is a POSIX shell script")
	}
	root := t.TempDir()
	cfg := &config.GofiConfig{
		Version: config.CurrentVersion,
		Project: config.Project{Name: "my-service", Root: root},
		Backend: &config.Backend{Language: config.LanguageGo, Path: "src"},
		AI:      config.AI{Host: config.AIHostClaudeVSCode, Model: config.ModelOpus47},
		Sources: config.Sources{Agents: "github.com/joaoprofile/gofi@v0.1.0"},
		Test: config.TestSection{
			Default: "unit",
			Tasks:   map[string]config.TestTask{"unit": {Desc: "unit tests", Run: "go test ./..."}},
		},
		Hsec: h,
	}
	if err := config.Save(filepath.Join(root, config.FileName), cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "horusec"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(root)
	return root
}

func publishableHsec(managerURL string) config.HsecConfig {
	h := config.DefaultHsec()
	h.Manager = &config.HsecManagerConfig{URL: managerURL, RepositoryName: "MY-REPO"}
	return h
}

func TestHsecStart_PublishWithoutManagerBlockFails(t *testing.T) {
	hsecProject(t, config.DefaultHsec())
	t.Setenv(hsec.AuthTokenEnv, "tok")
	err := runHsecStart(true)
	if err == nil || !strings.Contains(err.Error(), "hsec.manager") {
		t.Fatalf("expected missing manager error, got %v", err)
	}
}

func TestHsecStart_PublishWithoutTokenFailsBeforeScanning(t *testing.T) {
	root := hsecProject(t, publishableHsec("https://horusec.example.com"))
	t.Setenv(hsec.AuthTokenEnv, "")
	err := runHsecStart(true)
	if err == nil || !strings.Contains(err.Error(), hsec.AuthTokenEnv) {
		t.Fatalf("expected error naming %s, got %v", hsec.AuthTokenEnv, err)
	}
	if _, statErr := os.Stat(filepath.Join(root, hsec.ConfigFileName)); statErr == nil {
		t.Error("the scan must not start (config rendered) when the token is missing")
	}
}

func TestHsecStart_ManagerDownExitsWithCode3(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()
	hsecProject(t, publishableHsec(down.URL))
	t.Setenv(hsec.AuthTokenEnv, "tok")

	err := runHsecStart(true)
	var ec *ExitCodeError
	if !errors.As(err, &ec) || ec.Code != exitCodeManager {
		t.Fatalf("expected ExitCodeError code %d, got %v", exitCodeManager, err)
	}
	if strings.Contains(err.Error(), "tok") {
		t.Errorf("error must not echo the token: %v", err)
	}
}

func TestHsecStart_LocalRunNeedsNoManagerNorToken(t *testing.T) {
	hsecProject(t, config.DefaultHsec())
	t.Setenv(hsec.AuthTokenEnv, "")
	if err := runHsecStart(false); err != nil {
		t.Fatalf("local scan must not depend on manager/token: %v", err)
	}
}

// fakeHorusecWriting replaces the no-op horusec with one that writes the
// given output JSON where gofi reads it.
func fakeHorusecWriting(t *testing.T, root, outputJSON string) {
	t.Helper()
	bin := t.TempDir()
	out := filepath.Join(root, hsec.OutputFileName)
	script := "#!/bin/sh\nmkdir -p " + filepath.Dir(out) + "\ncat > " + out + " <<'EOF'\n" + outputJSON + "\nEOF\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "horusec"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fpProjectHsec() config.HsecConfig {
	h := config.DefaultHsec()
	h.ReturnErrorOnFinding = true
	h.FalsePositives = []config.HsecSuppression{{Rule: "HS-LEAKS-26", Path: "**/locales/*.json", Reason: "i18n labels"}}
	return h
}

const (
	suppressedVuln = `{"vulnerabilities":{"rule_id":"HS-LEAKS-26","file":"fe/locales/en.json","severity":"CRITICAL","vulnHash":"h1"}}`
	realVuln       = `{"vulnerabilities":{"rule_id":"G101","file":"backend/x.go","severity":"HIGH","vulnHash":"h2"}}`
)

func TestHsecStart_OnlySuppressedFindingsPass(t *testing.T) {
	root := hsecProject(t, fpProjectHsec())
	fakeHorusecWriting(t, root, `{"status":"success","analysisVulnerabilities":[`+suppressedVuln+`]}`)
	if err := runHsecStart(false); err != nil {
		t.Fatalf("all findings suppressed must pass, got %v", err)
	}
}

func TestHsecStart_RemainingFindingFailsEvenThoughHorusecExitedZero(t *testing.T) {
	root := hsecProject(t, fpProjectHsec())
	fakeHorusecWriting(t, root, `{"status":"success","analysisVulnerabilities":[`+suppressedVuln+`,`+realVuln+`]}`)
	if err := runHsecStart(false); !errors.Is(err, hsec.ErrFindings) {
		t.Fatalf("expected ErrFindings, got %v", err)
	}
}

func TestHsecStart_PublishWithFalsePositivesNeedsLocalScanFirst(t *testing.T) {
	h := fpProjectHsec()
	h.Manager = &config.HsecManagerConfig{URL: "https://horusec.example.com", RepositoryName: "MY-REPO"}
	hsecProject(t, h)
	t.Setenv(hsec.AuthTokenEnv, "tok")
	if err := runHsecStart(true); !errors.Is(err, hsec.ErrNoLocalScan) {
		t.Fatalf("expected ErrNoLocalScan, got %v", err)
	}
}

func TestHsecStart_StaleOutputIsNotReusedAsVerdict(t *testing.T) {
	root := hsecProject(t, fpProjectHsec())
	if err := os.MkdirAll(filepath.Join(root, ".gofi"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := `{"status":"success","analysisVulnerabilities":[` + suppressedVuln + `]}`
	if err := os.WriteFile(filepath.Join(root, hsec.OutputFileName), []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runHsecStart(false); !errors.Is(err, hsec.ErrNoOutput) {
		t.Fatalf("a scan that wrote no output must fail, not reuse the previous one; got %v", err)
	}
}

func TestHsecStart_IncompleteAnalysisFailsWithoutRules(t *testing.T) {
	root := hsecProject(t, config.DefaultHsec())
	fakeHorusecWriting(t, root, `{"status":"error","errors":"tool crashed","analysisVulnerabilities":[]}`)
	if err := runHsecStart(false); !errors.Is(err, hsec.ErrAnalysisIncomplete) {
		t.Fatalf("expected ErrAnalysisIncomplete, got %v", err)
	}
}

func TestExitCodeError_UnwrapsAndKeepsCode(t *testing.T) {
	var wrapped error = &ExitCodeError{Code: 3, Err: hsec.ErrPublishFailed}
	var ec *ExitCodeError
	if !errors.As(wrapped, &ec) || ec.Code != 3 {
		t.Fatal("errors.As must find the exit code")
	}
	if !errors.Is(wrapped, hsec.ErrPublishFailed) {
		t.Fatal("the cause must stay reachable")
	}
}

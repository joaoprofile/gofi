package doctor

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/config"
)

func mockLookup(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if path, ok := found[name]; ok {
			return path, nil
		}
		return "", errors.New("not found")
	}
}

func TestRun_AllOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &config.GofiConfig{Backend: &config.Backend{Language: "go"}}
	checks := Run(cfg, Options{
		HTTPBaseURL: srv.URL,
		HTTPClient:  srv.Client(),
		Lookup:      mockLookup(map[string]string{"git": "/usr/bin/git", "claude": "/usr/bin/claude", "go": "/usr/bin/go"}),
	})

	for _, c := range checks {
		if c.Status != StatusOK {
			t.Errorf("expected ok for %s, got %s (%s)", c.Name, c.Status, c.Detail)
		}
	}
}

func TestRun_MissingBinariesWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	checks := Run(nil, Options{
		HTTPBaseURL: srv.URL,
		HTTPClient:  srv.Client(),
		Lookup:      mockLookup(nil),
	})

	for _, c := range checks {
		if (c.Name == "git on PATH" || c.Name == "claude on PATH") && c.Status != StatusWarn {
			t.Errorf("expected warn for %s, got %s", c.Name, c.Status)
		}
	}
}

func TestRun_MissingToolchainFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &config.GofiConfig{Backend: &config.Backend{Language: "go"}}
	checks := Run(cfg, Options{
		HTTPBaseURL: srv.URL,
		HTTPClient:  srv.Client(),
		Lookup:      mockLookup(nil),
	})

	var foundGo bool
	for _, c := range checks {
		if c.Name == "go on PATH" {
			foundGo = true
			if c.Status != StatusFail {
				t.Errorf("expected fail for missing go, got %s", c.Status)
			}
		}
	}
	if !foundGo {
		t.Error("expected go check in results")
	}
}

func TestRun_GitHubServer500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	checks := Run(nil, Options{
		HTTPBaseURL: srv.URL,
		HTTPClient:  srv.Client(),
		Lookup:      mockLookup(map[string]string{"git": "/x/git", "claude": "/x/claude"}),
	})

	for _, c := range checks {
		if c.Name == "GitHub API reachable" && c.Status != StatusWarn {
			t.Errorf("expected warn on 5xx, got %s (%s)", c.Status, c.Detail)
		}
	}
}

func TestRun_GitHubUnreachable(t *testing.T) {
	checks := Run(nil, Options{
		HTTPBaseURL: "http://127.0.0.1:1", // refused
		HTTPClient:  &http.Client{},
		Lookup:      mockLookup(nil),
	})

	for _, c := range checks {
		if c.Name == "GitHub API reachable" && c.Status != StatusFail {
			t.Errorf("expected fail on unreachable, got %s (%s)", c.Status, c.Detail)
		}
	}
}

func TestRun_RustToolchain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &config.GofiConfig{Backend: &config.Backend{Language: "rust"}}
	checks := Run(cfg, Options{
		HTTPBaseURL: srv.URL,
		HTTPClient:  srv.Client(),
		Lookup:      mockLookup(map[string]string{"git": "/x", "claude": "/x", "cargo": "/x/cargo"}),
	})

	for _, c := range checks {
		if c.Name == "cargo on PATH" && c.Status != StatusOK {
			t.Errorf("expected ok for cargo, got %s", c.Status)
		}
	}
}

func TestStatus_String(t *testing.T) {
	if !strings.HasPrefix(StatusOK.String(), "ok") {
		t.Error("expected ok")
	}
	if StatusWarn.String() != "warn" || StatusFail.String() != "fail" {
		t.Error("unexpected status string")
	}
}

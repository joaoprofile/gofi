// Package sources fetches gofi agents and SDKs from GitHub, with on-disk
// caching. Sources are referenced as `github.com/<owner>/<repo>@<ref>` where
// <ref> is a tag, branch or commit SHA.
package sources

import (
	"errors"
	"strings"
)

// Ref identifies a versioned source on GitHub.
type Ref struct {
	Host  string // always "github.com" in v1
	Owner string
	Repo  string
	Ref   string // tag, branch, or commit SHA
}

// String returns the canonical "github.com/owner/repo@ref" form.
func (r Ref) String() string {
	return r.Host + "/" + r.Owner + "/" + r.Repo + "@" + r.Ref
}

// Parse splits "github.com/owner/repo@ref" into a Ref. The host must be
// github.com in v1; other hosts return an error.
func Parse(s string) (Ref, error) {
	at := strings.LastIndex(s, "@")
	if at < 0 {
		return Ref{}, errors.New("source must contain @<ref>")
	}
	hostPath := s[:at]
	ref := s[at+1:]
	if ref == "" {
		return Ref{}, errors.New("empty ref")
	}
	parts := strings.SplitN(hostPath, "/", 3)
	if len(parts) != 3 {
		return Ref{}, errors.New("source must be host/owner/repo@ref")
	}
	if parts[0] != "github.com" {
		return Ref{}, errors.New("only github.com is supported in v1")
	}
	if parts[1] == "" || parts[2] == "" {
		return Ref{}, errors.New("owner and repo are required")
	}
	return Ref{
		Host:  parts[0],
		Owner: parts[1],
		Repo:  parts[2],
		Ref:   ref,
	}, nil
}

// WithRef returns a copy of r with a different ref (used after resolving a
// tag to a SHA, etc.).
func (r Ref) WithRef(ref string) Ref {
	r.Ref = ref
	return r
}

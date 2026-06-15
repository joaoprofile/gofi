package sources

import "testing"

func TestParse_Valid(t *testing.T) {
	r, err := Parse("github.com/joaoprofile/gofi-agents@v0.1.0")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Owner != "joaoprofile" || r.Repo != "gofi-agents" || r.Ref != "v0.1.0" {
		t.Errorf("unexpected ref: %+v", r)
	}
	if r.String() != "github.com/joaoprofile/gofi-agents@v0.1.0" {
		t.Errorf("unexpected String(): %s", r.String())
	}
}

func TestParse_Invalid(t *testing.T) {
	cases := []string{
		"",
		"github.com/joaoprofile",
		"github.com/joaoprofile/repo",
		"github.com/joaoprofile/repo@",
		"@v1",
		"gitlab.com/x/y@v1",
		"github.com//repo@v1",
		"github.com/owner/@v1",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := Parse(in); err == nil {
				t.Errorf("expected error for %q", in)
			}
		})
	}
}

func TestRef_WithRef(t *testing.T) {
	r := Ref{Host: "github.com", Owner: "a", Repo: "b", Ref: "main"}
	r2 := r.WithRef("abc123")
	if r2.Ref != "abc123" {
		t.Errorf("expected ref abc123, got %s", r2.Ref)
	}
	if r.Ref != "main" {
		t.Errorf("original mutated: %s", r.Ref)
	}
}

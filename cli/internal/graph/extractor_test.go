package graph

import (
	"context"
	"slices"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

type stubExtractor struct{ lang string }

func (s stubExtractor) Language() string { return s.lang }
func (stubExtractor) Extract(context.Context, string, BuildOptions) (*Extraction, error) {
	return &Extraction{Graph: model.New("stub", ".", "fast")}, nil
}

func TestRegistryPrefersNative(t *testing.T) {
	r := NewRegistry(goExtractor{}, stubExtractor{lang: "rust"})

	if _, ok := r.For(LangGo).(goExtractor); !ok {
		t.Errorf("go resolved to %T", r.For(LangGo))
	}
	if _, ok := r.For("rust").(stubExtractor); !ok {
		t.Errorf("rust resolved to %T", r.For("rust"))
	}
	if got := r.Native(); !slices.Equal(got, []string{"go", "rust"}) {
		t.Errorf("Native() = %v", got)
	}
}

// An unregistered language is not an error: it falls through to the external
// protocol, which is how a language gofi cannot read natively is meant to
// arrive. Registering one later replaces that fallback with no change to Build.
func TestRegistryFallsBackToExternal(t *testing.T) {
	r := NewRegistry(goExtractor{})

	ex := r.For("java")
	if _, ok := ex.(externalExtractor); !ok {
		t.Fatalf("java resolved to %T, want the external protocol", ex)
	}
	if ex.Language() != "java" {
		t.Errorf("Language() = %q", ex.Language())
	}

	r.Register(stubExtractor{lang: "java"})
	if _, ok := r.For("java").(stubExtractor); !ok {
		t.Error("registering a native extractor did not replace the fallback")
	}
}

// Only an extractor that can compare cheaply gets to answer --update. The
// external one deliberately cannot: fingerprinting is a Go file hash, and only
// the extractor knows which files its own language compiles.
func TestIncrementalIsGoOnly(t *testing.T) {
	if _, ok := any(goExtractor{}).(Incremental); !ok {
		t.Error("goExtractor should be Incremental")
	}
	if _, ok := any(externalExtractor{lang: "java"}).(Incremental); ok {
		t.Error("externalExtractor must not claim to be Incremental")
	}
}

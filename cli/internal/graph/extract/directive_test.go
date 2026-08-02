package extract_test

import (
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/extract"
	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

// The context tag is what lets an agent go from a context name in
// specs/{context}/ to the code implementing it, instead of guessing from
// package names. The fixture tags the package "storage" and overrides one type
// with "audit", which is the whole contract: a default plus an override.
func TestContextDirective(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		id   string
		want string
	}{
		{model.PkgID("sample/store"), "storage"},
		{model.TypeID("sample/store", "Memory"), "storage"},
		{model.FuncID("sample/store", "NewMemory"), "storage"},
		{model.TypeID("sample/store", "Audit"), "audit"},
		// A package with no directive tags nothing: the field is opt-in, and an
		// empty value has to stay empty or every graph would claim a context.
		{model.PkgID("sample/api"), ""},
		{model.MethodID("sample/api", "Server", "Start"), ""},
	}
	for _, tt := range tests {
		n := g.Get(tt.id)
		if n == nil {
			t.Errorf("%s: node missing", tt.id)
			continue
		}
		if n.Context != tt.want {
			t.Errorf("%s: context = %q, want %q", tt.id, n.Context, tt.want)
		}
	}
}

// Visibility is stored as the language's own word rather than a bool, so a
// language with more than two levels does not have to lie.
func TestVisibility(t *testing.T) {
	g, err := extract.Fast(opts())
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Get(model.TypeID("sample/store", "Repo")).Vis; got != model.VisPublic {
		t.Errorf("Repo vis = %q, want public", got)
	}
	if got := g.Get(model.FuncID("sample/api", "par")).Vis; got != model.VisPrivate {
		t.Errorf("par vis = %q, want private", got)
	}
	if !g.Get(model.TypeID("sample/store", "Repo")).IsPublic() {
		t.Error("IsPublic disagrees with Vis")
	}
}

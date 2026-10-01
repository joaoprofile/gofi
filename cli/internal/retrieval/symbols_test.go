package retrieval

import (
	"slices"
	"testing"
)

func TestNamedSymbols(t *testing.T) {
	got := namedSymbols("como usar `netx.Response` e FindFromCriteria no handler?")
	if want := []string{"netx.Response", "FindFromCriteria"}; !slices.Equal(got, want) {
		t.Fatalf("namedSymbols = %v, want %v", got, want)
	}
	if got := namedSymbols("quando logar e em que nível"); len(got) != 0 {
		t.Fatalf("prose names no symbol, got %v", got)
	}
}

func TestNames(t *testing.T) {
	for _, c := range []struct {
		symbols []string
		title   string
		want    bool
	}{
		{[]string{"sqln.NewTransaction"}, "transaction.NewTransaction", true},
		{[]string{"FindFromCriteria"}, "sqln.FindFromCriteria", true},
		{[]string{"netx.Response"}, "netx.ErrorResponse", false},
		{nil, "netx.Response", false},
	} {
		if got := names(c.symbols, c.title); got != c.want {
			t.Errorf("names(%v, %q) = %v, want %v", c.symbols, c.title, got, c.want)
		}
	}
}

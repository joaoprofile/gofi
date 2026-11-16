package guard

import (
	"slices"
	"testing"
)

// The PRD's check only reads: it runs without a permission prompt, alone or
// after a cd, like a query of the index — and nothing chained to it does.
func TestThePRDCheckIsReadOnly(t *testing.T) {
	for cmd, want := range map[string]bool{
		"gofi prd check":                    true,
		"gofi prd check tenant":             true,
		"cd backend && gofi prd check a b":  true,
		"gofi prd check tenant > out.txt":   false,
		"gofi prd check tenant && rm -rf x": false,
		"gofi prd":                          false,
	} {
		if got := ReadOnlyCommand(cmd); got != want {
			t.Errorf("ReadOnlyCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
	if !slices.Contains(ReadOnlyRules, "Bash(gofi prd check *)") || !slices.Contains(ReadOnlyRules, "Bash(gofi prd check)") {
		t.Error("the PRD's check should be allowed ahead of time, like the specs'")
	}
}

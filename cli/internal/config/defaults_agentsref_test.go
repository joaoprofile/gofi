package config

import "testing"

// A released CLI pins the content released with it; anything else — a dev
// build, a commit, garbage — falls back to the development pin.
func TestAgentsRefFor(t *testing.T) {
	cases := map[string]string{
		"0.3.0":        AgentsRepo + "@v0.3.0",
		"v0.3.0":       AgentsRepo + "@v0.3.0",
		"1.2.3-rc.1":   AgentsRepo + "@v1.2.3-rc.1",
		"dev":          DefaultAgentsRef,
		"":             DefaultAgentsRef,
		"6a034fd":      DefaultAgentsRef,
		"0.3.0-dirty ": AgentsRepo + "@v0.3.0-dirty",
	}
	for in, want := range cases {
		if got := AgentsRefFor(in); got != want {
			t.Errorf("AgentsRefFor(%q) = %q, want %q", in, got, want)
		}
	}
}

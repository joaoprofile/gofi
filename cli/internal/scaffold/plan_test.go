package scaffold

import (
	"sort"
)

func planPaths(p []Change) []string {
	out := make([]string, len(p))
	for i, c := range p {
		out[i] = c.RelPath
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

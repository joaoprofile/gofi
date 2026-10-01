package docs

import (
	"sort"
	"strings"
)

// CheckOverrides reports every correction of the team's that points at a gofi
// rule the project no longer has. Packs and the SDK knowledge are rewritten by
// each update, and a correction whose section was renamed stops being shown
// beside it without a word — the one failure a correction must not have.
func CheckOverrides(root string) []Finding {
	idx, _, _ := Load(root)
	if idx == nil {
		return nil
	}
	byPath := make(map[string]Doc, len(idx.Docs))
	for _, d := range idx.Docs {
		byPath[d.Path] = d
	}
	var out []Finding
	for _, d := range idx.Docs {
		for _, t := range d.Overrides {
			file, sec, _ := strings.Cut(t, "#")
			sec = strings.TrimSpace(sec)
			target, ok := byPath[file]
			if !ok {
				out = append(out, Finding{Path: d.Path, Message: "overrides " + t + ": no such document — the rule moved or was removed; point overrides: at where it lives now", Error: true})
				continue
			}
			if sec != "" && !hasSection(target, sec) {
				out = append(out, Finding{Path: d.Path, Message: "overrides " + t + ": no such section in " + file + " — sections: " + strings.Join(headings(target), " | "), Error: true})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func hasSection(d Doc, heading string) bool {
	for _, s := range d.Sections {
		if s.Heading == heading {
			return true
		}
	}
	return false
}

func headings(d Doc) []string {
	out := make([]string, 0, len(d.Sections))
	for _, s := range d.Sections {
		out = append(out, s.Heading)
	}
	return out
}

package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// SurfaceFields are the UISurface keys the schema declares as strings. Exported
// so the audit reports exactly the set the parser accepts.
var SurfaceFields = map[string]bool{
	"framework": true, "path": true, "brand": true, "styling": true,
	"state": true, "forms": true, "i18n": true, "testing": true,
	"ds": true, "legacy": true,
}

// surfaceBlocks are the document keys that hold a UI surface — or, for
// `surfaces:` and the v1 `ui:` spelling, a map of them.
var surfaceBlocks = []string{"frontend", "mobile", "surfaces", "ui"}

// normalizeLegacy rewrites shapes older gofi material told projects to write but
// the schema never accepted, so a project scaffolded months ago can still be
// read — and therefore repaired by `gofi update`.
//
// Today that is one shape: `brand:` as a token block. The gofi-ui skill used to
// document it as surface/onBrand/action/accent, so real projects carry it, and a
// file carrying it fails to unmarshal — which takes down every gofi command,
// including the update that would fix it. The block collapses to its `surface`
// value, because that is the same role the single string names.
//
// Returns the normalised document and one note per rewrite. A document that does
// not parse is returned untouched: reporting that is the caller's job, and it
// says it better.
func normalizeLegacy(data []byte) ([]byte, []string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return data, nil, nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return data, nil, nil
	}

	var notes []string
	for _, block := range surfaceBlocks {
		if n := mapValue(doc.Content[0], block); n != nil {
			notes = append(notes, normalizeSurface(block, n)...)
		}
	}
	if len(notes) == 0 {
		return data, nil, nil
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, nil, err
	}
	return out, notes, nil
}

func normalizeSurface(where string, n *yaml.Node) []string {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	var notes []string
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i].Value, n.Content[i+1]
		if isSurfaceNode(val) {
			notes = append(notes, normalizeSurface(where+"."+key, val)...)
			continue
		}
		if key != "brand" || val.Kind != yaml.MappingNode {
			continue
		}
		seed := brandSeed(val)
		if seed == "" {
			continue
		}
		// Quoted on purpose: a bare #rrggbb would come back as a comment.
		*val = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seed, Style: yaml.DoubleQuotedStyle}
		notes = append(notes, fmt.Sprintf("%s.brand: token block read as %s", where, seed))
	}
	return notes
}

// brandSeed picks the value the single string stands for. `surface` is the
// dominant brand colour — the role the scalar names — so it wins; otherwise the
// first colour in the block keeps the identity rather than dropping it.
func brandSeed(n *yaml.Node) string {
	if s := mapValue(n, "surface"); s != nil && s.Kind == yaml.ScalarNode {
		return s.Value
	}
	for i := 1; i < len(n.Content); i += 2 {
		if n.Content[i].Kind == yaml.ScalarNode && n.Content[i].Value != "" {
			return n.Content[i].Value
		}
	}
	return ""
}

// isSurfaceNode tells a nested surface (ui.web, ui.mobile, …) from a field that
// was mistakenly written as a block.
func isSurfaceNode(n *yaml.Node) bool {
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	return mapValue(n, "framework") != nil || mapValue(n, "path") != nil
}

func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// LegacyShapes reports what Load had to read differently from what is on disk,
// so `gofi update` can say which values it is about to write in the current
// shape instead of changing them silently.
func LegacyShapes(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	norm, notes, err := normalizeLegacy(data)
	if err != nil {
		return nil
	}
	for name := range extraSurfaces(norm) {
		notes = append(notes, fmt.Sprintf("ui.%s: surface kept as surfaces.%s", name, name))
	}
	sort.Strings(notes)
	return notes
}

// extraSurfaces returns the v1 `ui:` sub-surfaces that are neither web nor
// mobile, keyed by the name the project gave them. Migration lifts ui.web into
// frontend: and ui.mobile into mobile:, and those are the only two named blocks
// — so anything else has to land in surfaces: or the rewrite would delete a
// surface the team declared.
//
// The document reaches here already normalised, so a legacy shape inside one of
// these surfaces (brand as a token block) decodes like any other.
func extraSurfaces(data []byte) map[string]*UISurface {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	ui := mapValue(doc.Content[0], "ui")
	if ui == nil || ui.Kind != yaml.MappingNode {
		return nil
	}

	out := map[string]*UISurface{}
	for i := 0; i+1 < len(ui.Content); i += 2 {
		key, val := ui.Content[i].Value, ui.Content[i+1]
		if key == "web" || key == "mobile" || !isSurfaceNode(val) {
			continue
		}
		var s UISurface
		if err := val.Decode(&s); err != nil {
			continue
		}
		out[key] = &s
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

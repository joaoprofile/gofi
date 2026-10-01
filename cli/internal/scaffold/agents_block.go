package scaffold

import "bytes"

// Markers delimit the project's block in AGENTS.md. Everything between them is
// the team's and survives every update; everything outside is gofi's. HTML
// comments, so a rendered AGENTS.md shows neither.
const (
	ProjectBegin = "<!-- gofi:project:begin -->"
	ProjectEnd   = "<!-- gofi:project:end -->"
)

// projectBlock returns the text between the markers, and whether the file has
// them in order.
func projectBlock(b []byte) ([]byte, bool) {
	i := bytes.Index(b, []byte(ProjectBegin))
	j := bytes.Index(b, []byte(ProjectEnd))
	if i < 0 || j < i {
		return nil, false
	}
	return b[i+len(ProjectBegin) : j], true
}

// gofiPart is the file with the project's block emptied: what gofi wrote and
// what an update compares, so an edit inside the block never reads as an edit
// to gofi's text.
func gofiPart(b []byte) []byte {
	block, ok := projectBlock(b)
	if !ok {
		return b
	}
	return withBlock(b, block, nil)
}

// withProject puts a project block into gofi's text: the upstream file with its
// block replaced by the one given.
func withProject(upstream, block []byte) []byte {
	upstream = ensureBlock(upstream)
	cur, _ := projectBlock(upstream)
	return withBlock(upstream, cur, block)
}

// ensureBlock gives gofi's text a project section when the source has none —
// a release from before the block, pinned by the project — so there is always
// somewhere for the team's instructions to live.
func ensureBlock(upstream []byte) []byte {
	if _, ok := projectBlock(upstream); ok {
		return upstream
	}
	out := append([]byte{}, bytes.TrimRight(upstream, "\n")...)
	return append(out, []byte("\n\n## Projeto\n\n"+ProjectBegin+"\n"+ProjectEnd+"\n")...)
}

func withBlock(b, old, block []byte) []byte {
	i := bytes.Index(b, []byte(ProjectBegin)) + len(ProjectBegin)
	out := make([]byte, 0, len(b)-len(old)+len(block))
	out = append(out, b[:i]...)
	out = append(out, block...)
	return append(out, b[i+len(old):]...)
}

// adopt is the AGENTS.md a project gets when it already had one: gofi's text,
// with what was there kept as the project's block — its own block when it had
// one, the whole file when it did not.
func adopt(upstream, existing []byte) []byte {
	if block, ok := projectBlock(existing); ok {
		return withProject(upstream, block)
	}
	return withProject(upstream, append(append([]byte("\n"), bytes.TrimSpace(existing)...), '\n'))
}

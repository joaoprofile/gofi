package tsjs

import (
	"bytes"
	"slices"
	"strings"
)

// A single-file component keeps its module code in a script block and the tree
// it renders in the markup beside it. Vue, Svelte and Astro are TypeScript or
// JavaScript underneath, so once the two halves are separated the rest of the
// scanner reads them with no special case: the script goes through the same
// blanking and the same declaration patterns as a .ts file, and the markup is
// read only for the elements it renders.
//
// Both halves keep the offsets they had in the file, so a line number taken
// from either still points where it did.

// span is a byte range of a file.
type span struct{ start, end int }

// sfcExts are the single-file component extensions.
var sfcExts = map[string]bool{".vue": true, ".svelte": true, ".astro": true}

// splitSFC returns src with everything outside the script erased, the spans
// that held it, and the spans that hold the markup. Erased bytes become spaces
// and every newline survives, which is what keeps the offsets aligned.
func splitSFC(ext string, src []byte) (code []byte, script, markup []span) {
	if ext == ".astro" {
		// Astro writes its module code between two fences at the top of the file
		// instead of inside a tag.
		script = astroFence(src)
	} else {
		script = tagSpans(src, "script")
	}

	code = make([]byte, len(src))
	erase(src, code, 0, len(src))
	for _, s := range script {
		copy(code[s.start:s.end], src[s.start:s.end])
	}
	return code, script, complement(len(src), append(slices.Clone(script), tagSpans(src, "style")...))
}

// tagSpans returns the content of every <name ...>...</name> block.
func tagSpans(src []byte, name string) []span {
	open, closing := []byte("<"+name), []byte("</"+name)
	var out []span
	for i := 0; i < len(src); {
		j := bytes.Index(src[i:], open)
		if j < 0 {
			break
		}
		after := i + j + len(open)
		// <scriptlet> is not <script>: the tag name has to end here.
		if after < len(src) && !isTagBreak(src[after]) {
			i = after
			continue
		}
		gt := bytes.IndexByte(src[after:], '>')
		if gt < 0 {
			break
		}
		start := after + gt + 1
		if src[start-2] == '/' {
			i = start // self-closing, so it has no content
			continue
		}
		k := bytes.Index(src[start:], closing)
		if k < 0 {
			out = append(out, span{start, len(src)})
			break
		}
		out = append(out, span{start, start + k})
		i = start + k + len(closing)
	}
	return out
}

func isTagBreak(c byte) bool {
	return c == '>' || c == '/' || c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// astroFence is the code between the two --- lines an Astro file opens with.
func astroFence(src []byte) []span {
	i := 0
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n') {
		i++
	}
	if !bytes.HasPrefix(src[i:], []byte("---")) {
		return nil // a file with no frontmatter is all markup
	}
	start := i + 3
	if j := bytes.Index(src[start:], []byte("\n---")); j >= 0 {
		return []span{{start, start + j}}
	}
	return []span{{start, len(src)}}
}

// complement returns the ranges of a file that the given spans leave out.
func complement(n int, spans []span) []span {
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	var out []span
	prev := 0
	for _, s := range spans {
		if s.start > prev {
			out = append(out, span{prev, s.start})
		}
		prev = max(prev, s.end)
	}
	if prev < n {
		out = append(out, span{prev, n})
	}
	return out
}

// pascal turns a hyphenated name into the one the component is declared and
// imported with: card-list and card_list are both CardList. A name that cannot
// be a component's — starting with a digit, or holding a character no
// identifier may — returns empty rather than something approximate.
func pascal(s string) string {
	var b strings.Builder
	upper := true
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '-' || c == '_' || c == '.':
			upper = true
		case upper:
			if c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			b.WriteByte(c)
			upper = false
		default:
			b.WriteByte(c)
		}
	}
	out := b.String()
	if !isIdent(out) || !isPascal(out) {
		return ""
	}
	return out
}

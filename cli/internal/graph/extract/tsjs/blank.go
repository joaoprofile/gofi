package tsjs

import "strings"

// blank returns src with the contents of comments, string literals and regular
// expressions replaced by spaces, together with the literal values indexed by
// the offset of their opening quote.
//
// Every byte keeps its position and every newline survives, so an offset into
// the result still points at the same line and column of the original. That is
// what lets the rest of the scanner match plain patterns without ever finding a
// keyword that only appeared inside a comment, or an `import` that was really a
// sentence in a doc block.
func blank(src []byte) ([]byte, map[int]string) {
	out := make([]byte, len(src))
	lits := map[int]string{}
	// prev is the last significant code byte, which is the only thing that
	// distinguishes a regular expression from a division.
	var prev byte

	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			i = blankLineComment(src, out, i)
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i = blankBlockComment(src, out, i)
		case c == '"' || c == '\'' || c == '`':
			start := i
			end, val := blankString(src, out, i)
			lits[start] = val
			i = end
			prev = '"'
		case c == '/' && regexCanStart(prev):
			i = blankRegex(src, out, i)
			prev = '/'
		default:
			out[i] = c
			if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
				prev = c
			}
			i++
		}
	}
	return out, lits
}

// erase copies src[i:end] into out as spaces, keeping newlines.
func erase(src, out []byte, i, end int) {
	for ; i < end && i < len(src); i++ {
		if src[i] == '\n' {
			out[i] = '\n'
		} else {
			out[i] = ' '
		}
	}
}

func blankLineComment(src, out []byte, i int) int {
	end := i
	for end < len(src) && src[end] != '\n' {
		end++
	}
	erase(src, out, i, end)
	return end
}

func blankBlockComment(src, out []byte, i int) int {
	end := i + 2
	for end < len(src) {
		if src[end] == '*' && end+1 < len(src) && src[end+1] == '/' {
			end += 2
			break
		}
		end++
	}
	erase(src, out, i, end)
	return end
}

// blankString consumes a quoted or template literal and returns the offset past
// it plus the raw text it held.
//
// Inside a template, `${` opens an expression that may itself contain a
// backtick, so only a backtick at substitution depth zero closes the literal.
// Without that, a single styled-component would swallow the rest of the file.
func blankString(src, out []byte, i int) (int, string) {
	quote := src[i]
	out[i] = quote
	end := i + 1
	depth := 0
	for end < len(src) {
		c := src[end]
		switch {
		case c == '\\':
			end += 2
			continue
		case quote == '`' && c == '$' && end+1 < len(src) && src[end+1] == '{':
			depth++
			end += 2
			continue
		case quote == '`' && c == '}' && depth > 0:
			depth--
		case c == quote && depth == 0:
			erase(src, out, i+1, end)
			out[end] = quote
			return end + 1, string(src[i+1 : end])
		case c == '\n' && quote != '`':
			// An unterminated single-line string ends at the newline rather than
			// running to the end of the file.
			erase(src, out, i+1, end)
			return end, string(src[i+1 : end])
		}
		end++
	}
	erase(src, out, i+1, len(src))
	return len(src), string(src[i+1:])
}

// blankRegex consumes a regular expression literal, including a character class
// that may hold an unescaped slash.
func blankRegex(src, out []byte, i int) int {
	end := i + 1
	inClass := false
	for end < len(src) {
		c := src[end]
		switch {
		case c == '\\':
			end++
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case c == '\n':
			// No literal spans a line: this was a division after all, so nothing
			// is blanked and the slash stays code.
			out[i] = '/'
			return i + 1
		case c == '/' && !inClass:
			end++
			for end < len(src) && isFlagByte(src[end]) {
				end++
			}
			erase(src, out, i, end)
			return end
		}
		end++
	}
	out[i] = '/'
	return i + 1
}

func isFlagByte(c byte) bool { return c >= 'a' && c <= 'z' }

// regexCanStart reports whether a slash at this position opens a regular
// expression rather than dividing. A value cannot be divided by nothing, so
// after an operator, an opening bracket or the start of the file the slash can
// only begin a literal.
func regexCanStart(prev byte) bool {
	if prev == 0 {
		return true
	}
	return strings.IndexByte("(,=:[!&|?{};+-*%<>~^", prev) >= 0
}

// stripJSONComments removes the comments and trailing commas that tsconfig.json
// is allowed to carry and encoding/json is not willing to read. String contents
// survive: unlike blank, this exists to make the document parseable, not to hide
// what it says.
func stripJSONComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i < len(src) && !(src[i] == '*' && i+1 < len(src) && src[i+1] == '/') {
				i++
			}
			i += 2
		case c == '"':
			out = append(out, c)
			i++
			for i < len(src) {
				if src[i] == '\\' && i+1 < len(src) {
					out = append(out, src[i], src[i+1])
					i += 2
					continue
				}
				out = append(out, src[i])
				if src[i] == '"' {
					i++
					break
				}
				i++
			}
		case c == ',':
			if j := nextSignificant(src, i+1); j < len(src) && (src[j] == '}' || src[j] == ']') {
				i++ // a trailing comma, which JSON does not allow
				continue
			}
			out = append(out, c)
			i++
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}

func nextSignificant(src []byte, i int) int {
	for i < len(src) {
		switch src[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '/':
			if i+1 < len(src) && src[i+1] == '/' {
				for i < len(src) && src[i] != '\n' {
					i++
				}
				continue
			}
			if i+1 < len(src) && src[i+1] == '*' {
				i += 2
				for i < len(src) && !(src[i] == '*' && i+1 < len(src) && src[i+1] == '/') {
					i++
				}
				i += 2
				continue
			}
			return i
		default:
			return i
		}
	}
	return i
}

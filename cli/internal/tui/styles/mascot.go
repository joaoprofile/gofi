package styles

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// mascotPixels is the gofi bear from the GOFI AI panel, one character per
// pixel: ears, big eyes, teeth, the steel armour and its gold chest plate.
// '.' is transparent, and '1'–'4' mark the cells that carry the letters of
// mascotText — real letters in the terminal's font, since no arrangement of
// blocks this small reads as a word.
//
// Each terminal cell holds 2x2 of these pixels, drawn with the quadrant block
// characters, so a pixel is half a cell wide and half a cell tall. That makes
// it taller than wide, and the art is laid out for it: it was traced from the
// panel's SVG at 1.22 pixels per unit across and 0.55 down, which is what keeps
// the bear in proportion on screen. 28x20 pixels is 14 columns by 10 lines.
var mascotPixels = []string{
	"....BBB....BBBBB....BBB.....",
	"...BBBKKBBBBBBBBBBBBBKKBBB..",
	"...BBKBBBBBBBBBBBBBBBBBKBB..",
	"....BBBWWWWWWBBBWWWWWWBB....",
	"....BBWWWKKKWWBWWWKKKWWBB...",
	"....BBWWWKKKWWBWWWKKKWWBB...",
	"....BBBWWWWBKKKKKBWWWWBBB...",
	".....BBBBBBBWWBWWBBBBBBB....",
	"......BBBBBBWWBWWBBBBBB.....",
	".........BBBBBBBBBBB........",
	".....GGGGGGGGGGGGGGGGGG.....",
	".....GGGGGGGGGGGGGGGGGG.....",
	"....GGGGYY11223344YYGGGGG...",
	"..GGGGGGYY11223344YYGGG.GGG.",
	"GGG..GGGGGGGGGGGGGGGGGG...GG",
	".....GGGGGGGGGGGGGGGGGG.....",
	".....gggggggggggggggggg.....",
	"........GGGGG...GGGGG.......",
	"........GGGGG...GGGGG.......",
	"........ggggg...ggggg.......",
}

// mascotText is written across the chest plate, one letter per marked cell.
const mascotText = "GOFI"

// mascotPalette uses the panel's own colors, so the bear looks the same in the
// terminal and in the editor.
var mascotPalette = map[byte]lipgloss.Color{
	'B': "#7DD3FC", // fur (gofi sky)
	'K': "#0B2B3A", // ear line, pupils, nose, the G (gofi navy)
	'W': "#FFFFFF", // eyes, teeth
	'G': "#94A3B8", // armour (steel)
	'g': "#475569", // armour shade
	'Y': "#EAB308", // chest plate (gold)
}

// quadrants maps which of a cell's four pixels are painted — bit 0 top left,
// 1 top right, 2 bottom left, 3 bottom right — to the block character that
// paints exactly those.
var quadrants = [16]string{
	" ", "▘", "▝", "▀", "▖", "▌", "▞", "▛",
	"▗", "▚", "▐", "▜", "▄", "▙", "▟", "█",
}

// MascotLines renders the mascot, one string per terminal line. With color off
// only the silhouette is drawn, since the features are told apart by color.
func MascotLines(color bool) []string {
	lines := make([]string, 0, (len(mascotPixels)+1)/2)
	for y := 0; y < len(mascotPixels); y += 2 {
		var b strings.Builder
		for x := 0; x < len(mascotPixels[0]); x += 2 {
			b.WriteString(mascotCell([4]byte{
				mascotPixel(x, y), mascotPixel(x+1, y),
				mascotPixel(x, y+1), mascotPixel(x+1, y+1),
			}, color))
		}
		lines = append(lines, b.String())
	}
	return lines
}

func mascotPixel(x, y int) byte {
	if y >= len(mascotPixels) || x >= len(mascotPixels[y]) {
		return '.'
	}
	return mascotPixels[y][x]
}

// mascotCell draws four pixels in one cell. A cell has a foreground and a
// background and nothing else, so when three colors meet the least used one
// takes the nearer of the two that stay — by count, which keeps outlines and
// pupils, the pixels that carry the features, from being voted away by the
// fill around them only when they are the majority of the cell.
func mascotCell(px [4]byte, color bool) string {
	if n := int(px[0] - '1'); n >= 0 && n < len(mascotText) {
		letter := mascotText[n : n+1]
		if !color {
			return letter
		}
		return lipgloss.NewStyle().Bold(true).
			Foreground(mascotPalette['K']).Background(mascotPalette['Y']).Render(letter)
	}
	if !color {
		var mask int
		for i, p := range px {
			if p != '.' {
				mask |= 1 << i
			}
		}
		return quadrants[mask]
	}

	count := map[byte]int{}
	var order []byte
	for _, p := range px {
		if count[p] == 0 {
			order = append(order, p)
		}
		count[p]++
	}
	sort.SliceStable(order, func(i, j int) bool { return count[order[i]] > count[order[j]] })
	keep := order
	if len(keep) > 2 {
		keep = keep[:2]
	}

	// Transparency is the terminal's own background, so when it stays the
	// painted color is the foreground and no background is set.
	fg, bg := keep[0], byte('.')
	if len(keep) == 2 {
		fg, bg = keep[0], keep[1]
		if fg == '.' {
			fg, bg = bg, fg
		}
	}
	if fg == '.' {
		return " "
	}

	var mask int
	for i, p := range px {
		if p != fg && p != bg {
			p = nearer(p, fg, bg)
		}
		if p == fg {
			mask |= 1 << i
		}
	}
	st := lipgloss.NewStyle().Foreground(mascotPalette[fg])
	if bg != '.' {
		st = st.Background(mascotPalette[bg])
	}
	return st.Render(quadrants[mask])
}

// nearer picks whichever of a and b is closer in color to p. A transparent p
// takes the darker of the two: next to the terminal's dark background that is
// the one that reads as the edge rather than as a stray light pixel.
func nearer(p, a, b byte) byte {
	if b == '.' {
		return a
	}
	if a == '.' {
		return b
	}
	if p == '.' {
		if luminance(a) <= luminance(b) {
			return a
		}
		return b
	}
	if colorDistance(p, a) <= colorDistance(p, b) {
		return a
	}
	return b
}

func colorDistance(a, b byte) int {
	ra, ga, ba := hexRGB(string(mascotPalette[a]))
	rb, gb, bb := hexRGB(string(mascotPalette[b]))
	return (ra-rb)*(ra-rb) + (ga-gb)*(ga-gb) + (ba-bb)*(ba-bb)
}

func luminance(c byte) int {
	r, g, b := hexRGB(string(mascotPalette[c]))
	return 299*r + 587*g + 114*b
}

func hexRGB(s string) (int, int, int) {
	var v [3]int
	for i := range v {
		v[i] = hexByte(s[1+2*i])*16 + hexByte(s[2+2*i])
	}
	return v[0], v[1], v[2]
}

func hexByte(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}

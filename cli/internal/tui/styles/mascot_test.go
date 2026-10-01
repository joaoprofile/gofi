package styles

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every pixel must map to a color, or that cell renders with the terminal
// default and the art silently loses a feature.
func TestMascotPixelsHavePalette(t *testing.T) {
	width := len(mascotPixels[0])
	for y, row := range mascotPixels {
		if len(row) != width {
			t.Errorf("row %d is %d wide, want %d", y, len(row), width)
		}
		for x := 0; x < len(row); x++ {
			if c := row[x]; c != '.' && (c < '1' || c > '4') {
				if _, ok := mascotPalette[c]; !ok {
					t.Errorf("pixel %q at %d,%d has no color", c, x, y)
				}
			}
		}
	}
}

func TestMascotLinesShape(t *testing.T) {
	for _, color := range []bool{true, false} {
		lines := MascotLines(color)
		if len(lines) != (len(mascotPixels)+1)/2 {
			t.Fatalf("color=%v: %d lines", color, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w != (len(mascotPixels[0])+1)/2 {
				t.Errorf("color=%v line %d is %d cells wide", color, i, w)
			}
			if !color && strings.Contains(l, "\x1b") {
				t.Errorf("uncolored line %d carries escape codes", i)
			}
		}
	}
}

func TestAnnouncesTrueColor(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	cases := []struct {
		vars map[string]string
		want bool
	}{
		{map[string]string{"WT_SESSION": "abc"}, true},
		{map[string]string{"TERM_PROGRAM": "vscode"}, true},
		{map[string]string{"WSL_DISTRO_NAME": "Ubuntu-22.04"}, true},
		{map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, false},
		{map[string]string{}, false},
	}
	for _, c := range cases {
		if got := announcesTrueColor(env(c.vars)); got != c.want {
			t.Errorf("%v → %v, want %v", c.vars, got, c.want)
		}
	}
}

// A cell holds two colors; the third one in it must fold into the nearer of
// the two instead of disappearing, and transparency must never become a color.
func TestMascotCellFoldsToTwoColors(t *testing.T) {
	if got := mascotCell([4]byte{'.', '.', '.', '.'}, true); got != " " {
		t.Errorf("empty cell = %q", got)
	}
	if got := mascotCell([4]byte{'B', 'B', '.', '.'}, false); got != "▀" {
		t.Errorf("silhouette = %q", got)
	}
	// white and fur stay; the lone navy pixel folds into the nearer, fur
	got := mascotCell([4]byte{'W', 'W', 'B', 'K'}, true)
	if !strings.Contains(got, "▀") {
		t.Errorf("W W / B K should paint the top half, got %q", got)
	}
}

// The plate spells the name, in order, in the plain rendering as well.
func TestMascotSpellsName(t *testing.T) {
	found := false
	for _, l := range MascotLines(false) {
		if strings.Contains(l, mascotText) {
			found = true
		}
	}
	if !found {
		t.Errorf("no line spells %q:\n%s", mascotText, strings.Join(MascotLines(false), "\n"))
	}
}

// The panel draws the same bear: its grid in the extension's webview is this
// one, pixel for pixel.
func TestThePanelDrawsTheSameMascot(t *testing.T) {
	b, err := os.ReadFile("../../../../vscode/media/main.js")
	if err != nil {
		t.Skip("the extension is not beside the CLI:", err)
	}
	src := string(b)
	start := strings.Index(src, "const MASCOT = [")
	if start < 0 {
		t.Fatal("main.js has no MASCOT grid")
	}
	end := strings.Index(src[start:], "];")
	var grid []string
	for _, line := range strings.Split(src[start:start+end], "\n")[1:] {
		if row := strings.Trim(strings.TrimSpace(line), "',"); row != "" {
			grid = append(grid, row)
		}
	}
	if strings.Join(grid, "\n") != strings.Join(mascotPixels, "\n") {
		t.Errorf("the panel's bear differs from the terminal's:\n%s", strings.Join(grid, "\n"))
	}
}

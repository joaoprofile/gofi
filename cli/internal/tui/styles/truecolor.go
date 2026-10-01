package styles

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Some terminals render 24-bit color but never say so: Windows Terminal (the
// usual home of WSL) sets WT_SESSION and leaves COLORTERM empty, so detection
// falls back to 256 colors and the palette drifts — navy turns bright blue,
// steel turns green. Trust those terminals when they are the one we write to.
func init() {
	if os.Getenv("COLORTERM") != "" || !term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	if announcesTrueColor(os.Getenv) {
		lipgloss.SetColorProfile(termenv.TrueColor)
	}
}

// announcesTrueColor reports whether the environment identifies a terminal
// known to render 24-bit color.
func announcesTrueColor(getenv func(string) string) bool {
	// Windows Terminal; and any WSL session, whose host terminals (Windows
	// Terminal, VS Code, the Windows 10+ console) all render 24-bit color but
	// do not all forward a variable that says so.
	if getenv("WT_SESSION") != "" || getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	switch getenv("TERM_PROGRAM") {
	case "vscode", "iTerm.app", "WezTerm", "ghostty", "Hyper":
		return true
	}
	return false
}

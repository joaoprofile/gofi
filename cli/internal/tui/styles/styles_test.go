package styles

import "testing"

func TestEnabled_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Enabled() {
		t.Error("NO_COLOR should disable colored output")
	}
}

func TestFormTheme_NonNil(t *testing.T) {
	if FormTheme() == nil {
		t.Error("FormTheme must never be nil")
	}
}

func TestHelpers_PlainWhenDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Header("x") != "x" || Panel("y") != "y" {
		t.Error("styles must pass content through unchanged when disabled")
	}
}

package styles

import "testing"

func TestEnabled_NoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Enabled() {
		t.Error("NO_COLOR should disable colored output")
	}
}

func TestHelpers_PlainWhenDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Header("x") != "x" || Note("y") != "y" {
		t.Error("styles must pass content through unchanged when disabled")
	}
	if got := Bullet(Done, "ok"); got != "● ok" {
		t.Errorf("Bullet = %q", got)
	}
	if got := Detail("why"); got != "  ⎿  why" {
		t.Errorf("Detail = %q", got)
	}
}

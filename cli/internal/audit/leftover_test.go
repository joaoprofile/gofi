package audit

import (
	"os"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/layout"
)

// An internal role left under skills/ — edited, so the update did not move
// it — is reported with what to do about it.
func TestAnInternalRoleLeftAsASkillIsReported(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{layout.Roles().Abs(root, "gofi-eng"), layout.Skills().Abs(root, "gofi-eng"), layout.Skills().Abs(root, "gofi-pd")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := leftoverRoles(root)
	if len(got) != 1 || !strings.Contains(got[0].Item, "gofi-eng") || !strings.Contains(got[0].Hint, "knowledge") {
		t.Fatalf("findings = %+v", got)
	}
}

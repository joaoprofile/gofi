package layout

import "testing"

func TestHomeFollowsTheHost(t *testing.T) {
	for host, want := range map[string]string{
		"":              ".claude",
		"claude-vscode": ".claude",
		"claude-code":   ".claude",
		"codex":         ".agents",
		"copilot":       ".agents",
	} {
		if got := HomeFor(host); got != want {
			t.Errorf("HomeFor(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestAreasFollowTheHomeAndDeclareOwners(t *testing.T) {
	t.Cleanup(func() { SetHome(DefaultHome) })
	SetHome(".agents")
	if Skills().Dir != ".agents/skills" || Contexts().Dir != ".agents/memory/contexts" || Specs().Dir != "specs" {
		t.Fatalf("areas did not follow the home: %s %s %s", Skills().Dir, Contexts().Dir, Specs().Dir)
	}
	for rel, want := range map[string]Owner{
		".agents/skills/gofi-eng/SKILL.md": OwnerGofi,
		".agents/sdk/go/knowledge/x.md":    OwnerGofi,
		".agents/knowledge/eng/x.md":       OwnerProject,
		".agents/memory/contexts/order.md": OwnerProject,
		"specs/order/sdd-order.md":         OwnerProject,
	} {
		if got, ok := OwnerOf(rel); !ok || got != want {
			t.Errorf("OwnerOf(%s) = %v, %v; want %v", rel, got, ok, want)
		}
	}
	if _, ok := OwnerOf("src/main.go"); ok {
		t.Error("code outside the harness has no owner here")
	}
}

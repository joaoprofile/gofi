package cli

import (
	"context"

	"github.com/joaoprofile/gofi/cli/internal/loop"
	"github.com/joaoprofile/gofi/cli/internal/worktree"
)

// settleLandings records the landings a person finished by hand: a run left
// in conflict whose branch, or whose archived change, the base now holds.
func settleLandings(ctx context.Context, root string) {
	infos, _ := loop.List(root)
	for _, i := range infos {
		s := i.State
		if i.Liveness != loop.Ended || s.Landing != conflict || s.Base == "" || s.Branch == "" {
			continue
		}
		if !worktree.Integrated(ctx, root, s.Base, s.Branch) && !worktree.Landed(ctx, root, s.Base, s.Change) {
			continue
		}
		s.Landing = landed
		if loop.SaveState(root, s) == nil {
			loop.AppendEvent(root, s.Run, loop.Event{Ev: "land.done", Node: s.Run, Why: s.Base + " (integrada à mão)"})
		}
	}
}

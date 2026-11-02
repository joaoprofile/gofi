package intake

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joaoprofile/gofi/cli/internal/contract"
)

// SealContract approves what the person just approved at a review stop: when
// phase i wrote the contract (a spec), every open change touching the
// context and its spec, if not sealed yet (0003, S4). It is the conductor's to
// do — the agent is never allowed to approve — and it runs the same check as
// gofi spec approve: a document with an error seals nothing, and the error is
// returned for the plan to stop on. It returns the paths sealed.
func SealContract(root string, r *Result, i int) ([]string, error) {
	if r.Context == nil || i < 0 || i >= len(r.Plan) || r.Plan[i].Produces != "spec" {
		return nil, nil
	}
	all, err := contract.Docs(root)
	if err != nil {
		return nil, err
	}
	picked, _ := contract.Select(all, []string{r.Context.Name})
	var todo []contract.Doc
	for _, d := range picked {
		if state, _ := contract.SealOf(d); state != contract.Sealed {
			todo = append(todo, d)
		}
	}
	placeholders, _ := contract.Placeholders(root)
	var blocked []string
	for _, d := range todo {
		for _, f := range contract.Blocking(root, all, d, placeholders) {
			blocked = append(blocked, f.Path+": "+f.Message)
		}
	}
	if len(blocked) > 0 {
		return nil, fmt.Errorf("o contrato de %s não pode ser aprovado como está — corrija e rode `gofi spec check %s`:\n  %s",
			r.Context.Name, r.Context.Name, strings.Join(blocked, "\n  "))
	}
	var sealed []string
	for _, d := range todo {
		if _, err := contract.Approve(root, d); err != nil {
			return sealed, err
		}
		sealed = append(sealed, d.Path)
	}
	return sealed, nil
}

// contractState reads what the contract of a context is now: its open sealed
// changes, and whether its spec is still in format 2.
func contractState(root string, c *Context) {
	if c == nil || c.New {
		return
	}
	all, err := contract.Docs(root)
	if err != nil {
		return
	}
	for _, d := range all {
		if d.Kind == contract.KindSpec && d.Change == "" && d.Context == c.Name && d.Format < contract.FormatSplit {
			c.FormatTwo = true
		}
	}
	for _, ch := range contract.Changes(all) {
		if ch.Sealed && slices.Contains(ch.Contexts, c.Name) {
			c.Sealed = append(c.Sealed, ch.ID)
		}
	}
}

package query

import (
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/graph/model"
)

// shop is a handler that calls a service that calls a repository, a type
// the service uses, a second caller of the repository and an external call.
func shop() *model.Graph {
	g := model.New("m", "", "fast")
	for _, n := range []*model.Node{
		{ID: "pkg:m/h", Kind: model.KindPackage, Unit: "m/h"},
		{ID: "method:m/h.H.Create", Kind: model.KindMethod, Unit: "m/h", File: "h/h.go", Line: 10, Lines: 8},
		{ID: "method:m/s.S.Create", Kind: model.KindMethod, Unit: "m/s", File: "s/s.go", Line: 20, Lines: 12},
		{ID: "type:m/s.S", Kind: model.KindStruct, Unit: "m/s", File: "s/s.go", Line: 5, Lines: 40},
		{ID: "method:m/r.R.Save", Kind: model.KindMethod, Unit: "m/r", File: "r/r.go", Line: 3, Lines: 5},
		{ID: "func:m/job.Run", Kind: model.KindFunc, Unit: "m/job", File: "job/job.go", Line: 1, Lines: 4},
		{ID: "type:m/model.Order", Kind: model.KindStruct, Unit: "m/model", File: "model/order.go", Line: 1, Lines: 6},
		{ID: "func:ext.Do", Kind: model.KindFunc, External: true},
	} {
		g.AddNode(n)
	}
	for _, e := range []*model.Edge{
		{From: "pkg:m/h", To: "method:m/h.H.Create", Rel: model.RelContains, Conf: 1},
		{From: "method:m/h.H.Create", To: "method:m/s.S.Create", Rel: model.RelCalls, Conf: 1},
		{From: "method:m/s.S.Create", To: "method:m/r.R.Save", Rel: model.RelCalls, Conf: 1},
		{From: "func:m/job.Run", To: "method:m/r.R.Save", Rel: model.RelCalls, Conf: 0.5},
		{From: "method:m/s.S.Create", To: "type:m/model.Order", Rel: model.RelUses, Conf: 1},
		{From: "func:ext.Do", To: "method:m/r.R.Save", Rel: model.RelCalls, Conf: 1},
	} {
		g.AddEdge(e)
	}
	g.Index()
	return g
}

func TestImpact(t *testing.T) {
	g := shop()
	got := Impact(g, []string{"method:m/r.R.Save"}, 0, 0)
	want := []Impacted{
		{ID: "func:m/job.Run", Depth: 1, Rel: "calls", Via: "method:m/r.R.Save", File: "job/job.go", Line: 1, Unit: "m/job"},
		{ID: "method:m/s.S.Create", Depth: 1, Rel: "calls", Via: "method:m/r.R.Save", File: "s/s.go", Line: 20, Unit: "m/s"},
		{ID: "method:m/h.H.Create", Depth: 2, Rel: "calls", Via: "method:m/s.S.Create", File: "h/h.go", Line: 10, Unit: "m/h"},
	}
	if len(got) != len(want) {
		t.Fatalf("Impact = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Impact[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	// An edge under the confidence asked for is not followed; depth bounds
	// the hops.
	if got := Impact(g, []string{"method:m/r.R.Save"}, 0.9, 1); len(got) != 1 || got[0].ID != "method:m/s.S.Create" {
		t.Errorf("Impact(conf 0.9, depth 1) = %+v", got)
	}
	// A type is reached by what uses it.
	if got := Impact(g, []string{"type:m/model.Order"}, 0, 0); len(got) != 2 || got[0].ID != "method:m/s.S.Create" || got[1].ID != "method:m/h.H.Create" {
		t.Errorf("Impact(Order) = %+v", got)
	}
}

func TestInFileAndAt(t *testing.T) {
	g := shop()
	if got := InFile(g, "s/s.go"); len(got) != 2 || got[0] != "method:m/s.S.Create" || got[1] != "type:m/s.S" {
		t.Errorf("InFile = %v", got)
	}
	for line, want := range map[int]string{25: "method:m/s.S.Create", 6: "type:m/s.S", 99: ""} {
		if got := At(g, "s/s.go", line); got != want {
			t.Errorf("At(s/s.go:%d) = %q, want %q", line, got, want)
		}
	}
}

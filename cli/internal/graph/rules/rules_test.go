package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/graph/diff"
	"github.com/joaoprofile/gofi/cli/internal/graph/model"
	"github.com/joaoprofile/gofi/cli/internal/pack"
	"github.com/joaoprofile/gofi/cli/internal/role"
)

var layers = []string{"handler", "service", "repository"}

func layerOf(file string) string {
	for _, l := range layers {
		if strings.Contains(file, "/"+l+"/") {
			return l
		}
	}
	return ""
}

// tree is a handler, a service and a repository of one context; edges are
// added to it as a change would.
func tree(edges ...*model.Edge) *model.Graph {
	g := model.New("m", "", "fast")
	for _, n := range []*model.Node{
		{ID: "pkg:m/d/x/handler", Kind: model.KindPackage, Unit: "m/d/x/handler"},
		{ID: "pkg:m/d/x/service", Kind: model.KindPackage, Unit: "m/d/x/service"},
		{ID: "pkg:m/d/x/repository", Kind: model.KindPackage, Unit: "m/d/x/repository"},
		{ID: "pkg:net/http", Kind: model.KindPackage, Unit: "net/http", External: true},
		{ID: "func:m/d/x/handler.Create", Kind: model.KindFunc, Unit: "m/d/x/handler", File: "d/x/handler/h.go", Line: 5, Lines: 9},
		{ID: "func:m/d/x/service.Create", Kind: model.KindFunc, Unit: "m/d/x/service", File: "d/x/service/s.go", Line: 3, Lines: 9},
		{ID: "func:m/d/x/repository.Save", Kind: model.KindFunc, Unit: "m/d/x/repository", File: "d/x/repository/r.go", Line: 7, Lines: 9},
	} {
		g.AddNode(n)
	}
	g.Files = map[string]model.FileInfo{
		"d/x/handler/h.go":    {Path: "d/x/handler/h.go", Hash: "1", Unit: "m/d/x/handler"},
		"d/x/service/s.go":    {Path: "d/x/service/s.go", Hash: "1", Unit: "m/d/x/service"},
		"d/x/repository/r.go": {Path: "d/x/repository/r.go", Hash: "1", Unit: "m/d/x/repository"},
	}
	g.AddEdge(&model.Edge{From: "func:m/d/x/handler.Create", To: "func:m/d/x/service.Create", Rel: model.RelCalls, File: "d/x/handler/h.go", Line: 8, Conf: 1})
	for _, e := range edges {
		g.AddEdge(e)
	}
	g.Index()
	return g
}

func TestParse(t *testing.T) {
	if _, err := Parse([]string{"deny: handler -> repository", `deny: import "net/http" from service`, "deny: new-cycle"}, layers); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"allow: handler -> service", "deny: handler -> dao", "deny: handler", `deny: import from service`} {
		if _, err := Parse([]string{bad}, layers); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

func TestCheckHoldsTheChangeNotTheCodeItStartedFrom(t *testing.T) {
	rs, err := Parse([]string{"deny: handler -> repository", `deny: import "net/http" from service`, "deny: new-cycle"}, layers)
	if err != nil {
		t.Fatal(err)
	}
	reachRepo := &model.Edge{From: "func:m/d/x/handler.Create", To: "func:m/d/x/repository.Save", Rel: model.RelCalls, File: "d/x/handler/h.go", Line: 10, Conf: 1}
	importHTTP := &model.Edge{From: "pkg:m/d/x/service", To: "pkg:net/http", Rel: model.RelImports, File: "d/x/service/s.go", Line: 2, Conf: 1}
	old := tree()
	now := tree(reachRepo, importHTTP)
	now.Files["d/x/handler/h.go"] = model.FileInfo{Path: "d/x/handler/h.go", Hash: "2", Unit: "m/d/x/handler"}
	now.Cycles = [][]string{{"func:m/d/x/service.Create", "func:m/d/x/handler.Create"}}
	d := diff.Of(old, now)
	if strings.Join(d.Files, ",") != "d/x/handler/h.go" || strings.Join(d.Changed, ",") != "func:m/d/x/handler.Create" || len(d.Edges) != 2 {
		t.Fatalf("delta = %+v", d)
	}
	vs := Check(rs, d, now, Scope{LayerOf: layerOf})
	if len(vs) != 3 {
		t.Fatalf("violations = %+v", vs)
	}
	if vs[0].Symbol != "func:m/d/x/handler.Create" || vs[0].Rule != "deny: new-cycle" && vs[0].Rule != "deny: handler -> repository" {
		t.Errorf("first = %+v", vs[0])
	}
	var got []string
	for _, v := range vs {
		got = append(got, v.Rule)
	}
	for _, want := range []string{"deny: handler -> repository", `deny: import "net/http" from service`, "deny: new-cycle"} {
		if !strings.Contains(strings.Join(got, "|"), want) {
			t.Errorf("no violation of %s in %v", want, got)
		}
	}
	// The same violations already in the code a change starts from are not
	// the change's.
	if vs := Check(rs, diff.Of(now, now), now, Scope{LayerOf: layerOf}); len(vs) != 0 {
		t.Errorf("unchanged code: %+v", vs)
	}
}

// Every pack's rules name its own layers: a misspelt one would hold nothing.
func TestThePacksRulesParse(t *testing.T) {
	for _, lang := range pack.Languages() {
		p, err := pack.Load(lang)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, l := range p.Layers {
			names = append(names, l.Name)
			if !slices.Contains(role.Layers, l.Plan) {
				t.Errorf("pack %s: layer %s is part of plan layer %q — %v", lang, l.Name, l.Plan, role.Layers)
			}
		}
		if _, err := Parse(p.Graph.Rules, names); err != nil {
			t.Errorf("pack %s: %v", lang, err)
		}
	}
}

// A new edge from one context's code to another's is refused unless the
// change declares both: an undeclared dependency is the contract's, for the
// person to decide.
func TestACrossContextEdgeNeedsTheChangeToDeclareIt(t *testing.T) {
	rs, err := Parse([]string{"deny: new-cross-context-edge"}, layers)
	if err != nil {
		t.Fatal(err)
	}
	g := model.New("m", "", "fast")
	for _, n := range []*model.Node{
		{ID: "func:m/d/pedidos/service.Fechar", Kind: model.KindFunc, Unit: "m/d/pedidos/service", File: "d/pedidos/service/s.go", Line: 3},
		{ID: "func:m/d/estoque/service.Reservar", Kind: model.KindFunc, Unit: "m/d/estoque/service", File: "d/estoque/service/s.go", Line: 3},
	} {
		g.AddNode(n)
	}
	g.AddEdge(&model.Edge{From: "func:m/d/pedidos/service.Fechar", To: "func:m/d/estoque/service.Reservar", Rel: model.RelCalls, File: "d/pedidos/service/s.go", Line: 5, Conf: 1})
	g.Index()
	old := model.New("m", "", "fast")
	old.Index()
	ctxOf := func(f string) string {
		if p := strings.Split(f, "/"); len(p) > 2 {
			return p[1]
		}
		return ""
	}
	d := diff.Of(old, g)
	if vs := Check(rs, d, g, Scope{ContextOf: ctxOf, Declared: []string{"pedidos"}}); len(vs) != 1 || !strings.Contains(vs[0].Message, "não declara estoque") {
		t.Errorf("undeclared: %+v", vs)
	}
	if vs := Check(rs, d, g, Scope{ContextOf: ctxOf, Declared: []string{"pedidos", "estoque"}}); len(vs) != 0 {
		t.Errorf("declared: %+v", vs)
	}
}

package tsjs

import (
	"slices"
	"testing"

	"github.com/joaoprofile/gofi-cli/internal/graph/model"
)

func build(t *testing.T, root, lang string) *model.Graph {
	t.Helper()
	g, err := Fast(Options{Root: root, Language: lang})
	if err != nil {
		t.Fatalf("Fast: %v", err)
	}
	return g
}

// node finds a node by ID and fails with the available IDs, which is the only
// way a mismatch here is readable.
func node(t *testing.T, g *model.Graph, id string) *model.Node {
	t.Helper()
	if n := g.Get(id); n != nil {
		return n
	}
	var ids []string
	for _, n := range g.Nodes {
		ids = append(ids, n.ID)
	}
	slices.Sort(ids)
	t.Fatalf("node %q not found; graph has %v", id, ids)
	return nil
}

func hasEdge(g *model.Graph, from string, rel model.Rel, to string) bool {
	for _, e := range g.Edges {
		if e.From == from && e.Rel == rel && e.To == to {
			return true
		}
	}
	return false
}

func TestReactKinds(t *testing.T) {
	g := build(t, "testdata/react", "typescript")

	if g.Module != "shop-web" {
		t.Errorf("module = %q, want shop-web (from package.json)", g.Module)
	}
	if g.Language != "typescript" {
		t.Errorf("language = %q", g.Language)
	}

	for _, tc := range []struct {
		id   string
		kind model.Kind
	}{
		{"func:shop-web/src/components/Button.Button", model.KindComponent},
		{"func:shop-web/src/components/Card.Card", model.KindComponent},
		{"func:shop-web/src/pages/Home.Home", model.KindComponent},
		{"func:shop-web/src/hooks/useCart.useCart", model.KindHook},
		{"func:shop-web/src/api/client.fetchCart", model.KindFunc},
		{"type:shop-web/src/components/Button.ButtonProps", model.KindInterface},
		{"type:shop-web/src/api/client.Cart", model.KindInterface},
	} {
		if got := node(t, g, tc.id).Kind; got != tc.kind {
			t.Errorf("%s kind = %q, want %q", tc.id, got, tc.kind)
		}
	}
}

func TestReactEdges(t *testing.T) {
	g := build(t, "testdata/react", "typescript")

	// A relative import, resolved by adding the extension.
	if !hasEdge(g, "pkg:shop-web/src/components/Card", model.RelImports, "pkg:shop-web/src/components/Button") {
		t.Error("Card does not import Button")
	}
	// A tsconfig alias declared in the config the project extends.
	if !hasEdge(g, "pkg:shop-web/src/hooks/useCart", model.RelImports, "pkg:shop-web/src/api/client") {
		t.Error("useCart does not import client through @app/*")
	}
	// A dependency outside the tree stays a single external node.
	ext := node(t, g, "pkg:react")
	if !ext.External {
		t.Error("react is not marked external")
	}

	// The component tree: a rendered child is a use, never a call.
	if !hasEdge(g, "func:shop-web/src/components/Card.Card", model.RelUses, "func:shop-web/src/components/Button.Button") {
		t.Error("Card does not render Button")
	}
	if !hasEdge(g, "func:shop-web/src/pages/Home.Home", model.RelUses, "func:shop-web/src/components/Card.Card") {
		t.Error("Home does not render Card")
	}
	// A hook is called, and so is a function reached through an alias.
	if !hasEdge(g, "func:shop-web/src/pages/Home.Home", model.RelCalls, "func:shop-web/src/hooks/useCart.useCart") {
		t.Error("Home does not call useCart")
	}
	if !hasEdge(g, "func:shop-web/src/hooks/useCart.useCart", model.RelCalls, "func:shop-web/src/api/client.fetchCart") {
		t.Error("useCart does not call fetchCart")
	}
	// A parameter annotation is the props contract of a component.
	if !hasEdge(g, "func:shop-web/src/components/Button.Button", model.RelUses, "type:shop-web/src/components/Button.ButtonProps") {
		t.Error("Button does not use its props type")
	}
	// Every declaration belongs to the module that declares it.
	if !hasEdge(g, "pkg:shop-web/src/api/client", model.RelContains, "func:shop-web/src/api/client.fetchCart") {
		t.Error("client does not contain fetchCart")
	}
}

// A comment and a string literal must never produce an edge: that is the whole
// reason the source is blanked before any pattern runs.
func TestReactIgnoresCommentsAndStrings(t *testing.T) {
	g := build(t, "testdata/react", "typescript")
	for _, e := range g.Edges {
		if e.Rel != model.RelImports {
			continue
		}
		if e.To == "pkg:./ghost" || e.To == "pkg:./nowhere" {
			t.Errorf("edge from a comment or string literal: %s -> %s", e.From, e.To)
		}
	}
}

// A React function component or hook has no members. Reading its body for them
// turns every `useEffect(() => {` into a method of it — in a real 370-file app
// that was 101 invented nodes.
func TestFunctionBodiesHaveNoMembers(t *testing.T) {
	g := build(t, "testdata/react", "typescript")
	for _, n := range g.Nodes {
		if n.Kind == model.KindMethod {
			t.Errorf("method node %q in a tree with no classes", n.ID)
		}
	}
}

// Unresolved counts holes in the graph, not names that were never going to be
// nodes. Locals, parameters and browser globals resolve to nothing by design; if
// they are counted the number stops meaning anything and, since the graph is
// committed, churns in every diff.
func TestUnresolvedCountsOnlyRealHoles(t *testing.T) {
	g := build(t, "testdata/react", "typescript")
	if g.Stats.Unresolved != 0 {
		t.Errorf("unresolved = %d, want 0: every import in this fixture lands on a declaration",
			g.Stats.Unresolved)
	}
}

func TestAngularKinds(t *testing.T) {
	g := build(t, "testdata/angular", "typescript")

	if g.Module != "admin-console" {
		t.Errorf("module = %q", g.Module)
	}
	for _, tc := range []struct {
		id   string
		kind model.Kind
	}{
		// The decorator is authoritative: neither name would give this away.
		{"type:admin-console/src/app/core/user.service.UserService", model.KindService},
		{"type:admin-console/src/app/user-list/user-list.component.UserListComponent", model.KindComponent},
		{"type:admin-console/src/app/app.module.AppModule", model.KindClass},
		{"type:admin-console/src/app/core/user.service.User", model.KindInterface},
		{"method:admin-console/src/app/core/user.service.UserService.list", model.KindMethod},
	} {
		if got := node(t, g, tc.id).Kind; got != tc.kind {
			t.Errorf("%s kind = %q, want %q", tc.id, got, tc.kind)
		}
	}

	if v := node(t, g, "method:admin-console/src/app/core/user.service.UserService.endpoint").Vis; v != model.VisPrivate {
		t.Errorf("private method vis = %q", v)
	}
	if v := node(t, g, "method:admin-console/src/app/core/user.service.UserService.list").Vis; v != model.VisPublic {
		t.Errorf("undecorated method vis = %q, want public", v)
	}
}

func TestAngularEdges(t *testing.T) {
	g := build(t, "testdata/angular", "typescript")

	const component = "type:admin-console/src/app/user-list/user-list.component.UserListComponent"
	const service = "type:admin-console/src/app/core/user.service.UserService"

	// A constructor parameter is the dependency list in an injected framework.
	if !hasEdge(g, component, model.RelUses, service) {
		t.Error("component does not depend on the injected service")
	}
	// A field annotation is a dependency too.
	if !hasEdge(g, component, model.RelUses, "type:admin-console/src/app/core/user.service.User") {
		t.Error("component does not use the User type")
	}
	// A class owns its methods, on top of the module owning them.
	if !hasEdge(g, service, model.RelContains, "method:admin-console/src/app/core/user.service.UserService.list") {
		t.Error("service does not contain its method")
	}
	// A baseUrl pointing at src makes @core/* resolve.
	if !hasEdge(g, "pkg:admin-console/src/app/user-list/user-list.component", model.RelImports, "pkg:admin-console/src/app/core/user.service") {
		t.Error("@core/user.service did not resolve")
	}
}

// Where the markup lives in a file of its own, the component tree is written
// nowhere else: user-list.component.ts never names UserCardComponent, and the
// module that declares both names them by class, not by the tag the template
// spells. Reading the template is the only way the edge exists at all.
func TestAngularTemplates(t *testing.T) {
	g := build(t, "testdata/angular", "typescript")

	const list = "type:admin-console/src/app/user-list/user-list.component.UserListComponent"
	const card = "type:admin-console/src/app/user-card/user-card.component.UserCardComponent"
	const shell = "type:admin-console/src/app/shell/shell.component.ShellComponent"

	if !hasEdge(g, list, model.RelUses, card) {
		t.Error("the template's <app-user-card> did not become an edge")
	}
	// A template written inline in the decorator is read the same way.
	if !hasEdge(g, shell, model.RelUses, list) {
		t.Error("the inline template's <app-user-list> did not become an edge")
	}
	// The template belongs to the module beside it: they are one component split
	// across two files.
	tmpl, ok := g.Files["src/app/user-list/user-list.component.html"]
	if !ok {
		t.Fatal("the template was not recorded as a scanned file")
	}
	if want := "admin-console/src/app/user-list/user-list.component"; tmpl.Unit != want {
		t.Errorf("template unit = %q, want %q", tmpl.Unit, want)
	}
	// A page shell is markup with no module beside it, so it is not a template
	// and its <app-shell> is nobody's child.
	if _, scanned := g.Files["src/index.html"]; scanned {
		t.Error("an unpaired .html was read as a template")
	}
	// Markup never declares anything: only the modules do.
	for _, n := range g.Nodes {
		if n.File == "src/app/user-list/user-list.component.html" {
			t.Errorf("the template produced a node: %q", n.ID)
		}
	}
}

// A single-file component is a module like any other: the file is the unit, and
// the file is also the component, because that is what importing it binds to.
func TestVueSingleFileComponents(t *testing.T) {
	g := build(t, "testdata/vue", "typescript")

	const home = "func:shop-vue/src/pages/Home.Home"
	const card = "func:shop-vue/src/components/Card.Card"

	if got := node(t, g, home).Kind; got != model.KindComponent {
		t.Errorf("Home kind = %q, want component", got)
	}
	// A .vue file with only a template still declares its component.
	if got := node(t, g, "func:shop-vue/src/components/EmptyState.EmptyState").Kind; got != model.KindComponent {
		t.Errorf("EmptyState kind = %q, want component", got)
	}

	// The component tree lives in the template, which sits outside every
	// declaration and is the only place it is written down.
	if !hasEdge(g, home, model.RelUses, card) {
		t.Error("Home does not render Card")
	}
	// Vue lets the same component be spelled in kebab-case.
	if !hasEdge(g, home, model.RelUses, "func:shop-vue/src/components/EmptyState.EmptyState") {
		t.Error("<empty-state /> did not resolve to the EmptyState component")
	}
	// A setup script runs on every render, so what it calls the component depends on.
	if !hasEdge(g, home, model.RelCalls, "func:shop-vue/src/composables/useCart.useCart") {
		t.Error("Home does not call useCart")
	}
	// An alias declared in tsconfig resolves from inside a .vue script too.
	if !hasEdge(g, "pkg:shop-vue/src/components/Card", model.RelImports, "pkg:shop-vue/src/composables/useCart") {
		t.Error("@app/composables/useCart did not resolve from a .vue file")
	}
}

// The script of a single-file component holds declarations of its own, and a
// call inside one of them belongs to it — not to the component that encloses it.
func TestSFCScriptCallsAreNotAttributedTwice(t *testing.T) {
	g := build(t, "testdata/vue", "typescript")

	const home = "func:shop-vue/src/pages/Home.Home"
	const fetchCart = "func:shop-vue/src/composables/useCart.fetchCart"

	if !hasEdge(g, "func:shop-vue/src/pages/Home.reload", model.RelCalls, fetchCart) {
		t.Error("reload does not call fetchCart")
	}
	if hasEdge(g, home, model.RelCalls, fetchCart) {
		t.Error("the call inside reload was also recorded against the component")
	}
}

func TestSvelteSingleFileComponents(t *testing.T) {
	g := build(t, "testdata/svelte", "typescript")

	const page = "func:shop-svelte/src/routes/Page.Page"

	if got := node(t, g, page).Kind; got != model.KindComponent {
		t.Errorf("Page kind = %q, want component", got)
	}
	if !hasEdge(g, page, model.RelUses, "func:shop-svelte/src/lib/Counter.Counter") {
		t.Error("Page does not render Counter")
	}
	// A relative import that spells the .svelte extension.
	if !hasEdge(g, "pkg:shop-svelte/src/routes/Page", model.RelImports, "pkg:shop-svelte/src/lib/Counter") {
		t.Error("Page does not import Counter")
	}
	if !hasEdge(g, page, model.RelCalls, "func:shop-svelte/src/lib/cart.addLine") {
		t.Error("Page does not call addLine")
	}
	// The markup is read for elements, never for declarations: a <button> is not
	// a node and a <style> block holds no code.
	for _, n := range g.Nodes {
		if n.Name == "button" || n.Name == "style" {
			t.Errorf("markup produced a node: %q", n.ID)
		}
	}
}

// Astro puts the module code between two fences instead of inside a tag, which
// is the only thing that differs from the other single-file formats.
func TestAstroFrontmatter(t *testing.T) {
	g := build(t, "testdata/astro", "typescript")

	const index = "func:shop-astro/src/pages/index.Index"

	if got := node(t, g, index).Kind; got != model.KindComponent {
		t.Errorf("index kind = %q, want component", got)
	}
	if !hasEdge(g, index, model.RelUses, "func:shop-astro/src/components/Hero.Hero") {
		t.Error("index does not render Hero")
	}
	if !hasEdge(g, index, model.RelCalls, "func:shop-astro/src/lib/greet.greet") {
		t.Error("index does not call greet")
	}
	// The frontmatter is TypeScript, so its interfaces are declarations.
	if got := node(t, g, "type:shop-astro/src/components/Hero.Props").Kind; got != model.KindInterface {
		t.Errorf("Props kind = %q, want interface", got)
	}
}

// The graph is committed to git, so two scans of the same tree must produce the
// same bytes.
func TestDeterministic(t *testing.T) {
	for _, root := range []string{"testdata/react", "testdata/angular", "testdata/vue", "testdata/svelte", "testdata/astro"} {
		a, b := build(t, root, "typescript"), build(t, root, "typescript")
		a.Sort()
		b.Sort()
		if len(a.Nodes) != len(b.Nodes) || len(a.Edges) != len(b.Edges) {
			t.Fatalf("%s: node/edge counts differ between runs", root)
		}
		for i := range a.Nodes {
			if a.Nodes[i].ID != b.Nodes[i].ID {
				t.Fatalf("%s: node %d differs: %s vs %s", root, i, a.Nodes[i].ID, b.Nodes[i].ID)
			}
		}
		for i := range a.Edges {
			if a.Edges[i].Key() != b.Edges[i].Key() {
				t.Fatalf("%s: edge %d differs", root, i)
			}
		}
	}
}

func TestFingerprintMatchesScan(t *testing.T) {
	// The Angular tree is here because its graph is built from templates as well
	// as from modules: a fingerprint that left them out would let --update call a
	// graph current after its markup had been rewritten.
	for _, root := range []string{"testdata/react", "testdata/angular"} {
		t.Run(root, func(t *testing.T) {
			fp, err := Fingerprint(Options{Root: root})
			if err != nil {
				t.Fatalf("Fingerprint: %v", err)
			}
			g := build(t, root, "typescript")
			if len(fp) != len(g.Files) {
				t.Fatalf("fingerprint has %d files, graph has %d", len(fp), len(g.Files))
			}
			for path, info := range fp {
				got, ok := g.Files[path]
				if !ok {
					t.Fatalf("%s missing from the graph", path)
				}
				if got.Hash != info.Hash || got.Unit != info.Unit {
					t.Errorf("%s: fingerprint %+v, graph %+v", path, info, got)
				}
			}
		})
	}
}

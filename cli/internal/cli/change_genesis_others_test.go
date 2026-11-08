package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/contract"
	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// changeTemplates writes the project's change templates.
func changeTemplates(t *testing.T, root string) {
	t.Helper()
	tmpl := filepath.Join(root, ".claude", "templates")
	writeFile(t, filepath.Join(tmpl, "proposal-template.md"), "---\ntipo: mudanca\nformato: 3\ncontextos: [{contextos}]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Mudança — {mudanca}\n\n## Por quê\n\n## O que muda\n")
	writeFile(t, filepath.Join(tmpl, "delta-template.md"), "---\ntipo: delta\nformato: 3\ncontexto: {contexto}\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Delta — {contexto}\n\n## ADICIONADO\n\n### Requisito: REQ-01 — {Título}\n")
	writeFile(t, filepath.Join(tmpl, "design-template.md"), "---\ntipo: design\nformato: 3\ncontexto: {contexto}\nversao: \"1.0\"\nstatus: proposto\nkeywords: [{Termo}]\n---\n# Design\n\n## Abordagem\n\n## Mapa de requisitos\n")
}

func genesisProposal(contexts string) string {
	return "---\ntipo: mudanca\nformato: 3\ncontextos: [" + contexts + "]\nversao: \"1.0\"\nstatus: proposto\nkeywords: [conta]\n---\n# Mudança\n\n## Por quê\n\nNão há.\n\n## O que muda\n\nREQ novo.\n"
}

func genesisDelta(ctx, req string, purpose bool) string {
	p := ""
	if purpose {
		p = "## Propósito\n\nO contexto " + ctx + ".\n\n"
	}
	return "---\ntipo: delta\nformato: 3\ncontexto: " + ctx + "\nversao: \"1.0\"\nstatus: proposto\nkeywords: [conta]\n---\n# Delta — " + ctx + "\n\n" + p +
		"## ADICIONADO\n\n### Requisito: " + req + " — Regra de " + ctx + "\nO sistema DEVE recusar o vazio.\n\n#### Cenário: " + req + ".1 — Vazio\n- DADO um pedido vazio\n- QUANDO chega\n- ENTÃO é recusado\n"
}

func genesisDesign(ctx, reqs string) string {
	return "---\ntipo: design\nformato: 3\ncontexto: " + ctx + "\nversao: \"1.0\"\nstatus: proposto\nkeywords: [conta]\n---\n# Design\n\n## Abordagem\n\nUm serviço.\n\n## Mapa de requisitos\n\n" + reqs + "\n"
}

// userLiving gives the project a living spec of user, born and archived as
// any context is.
func userLiving(t *testing.T, root string) {
	t.Helper()
	if out, err := runChange(t, root, "new", "criar-user", "--context", "user"); err != nil {
		t.Fatalf("new user: %v\n%s", err, out)
	}
	dir := filepath.Join(root, "specs", "user")
	writeFile(t, filepath.Join(dir, "proposal.md"), genesisProposal("user"))
	writeFile(t, filepath.Join(dir, "deltas", "user.md"), genesisDelta("user", "REQ-01", true))
	writeFile(t, filepath.Join(dir, "design.md"), genesisDesign("user", "| REQ-01 | User.Create |"))
	if out, err := runChange(t, root, "approve", "user"); err != nil {
		t.Fatalf("approve user: %v\n%s", err, out)
	}
	if out, err := runChange(t, root, "archive", "user"); err != nil {
		t.Fatalf("archive user: %v\n%s", err, out)
	}
}

// A new context that also changes one that exists is born in its own folder:
// the genesis of signup, with a delta for user beside its own — not in
// specs/changes/, which is for a change among contexts that exist. Checked,
// reviewed, approved and archived by its id, the context's name: signup gets
// its spec, user gets the requirement, and the change goes to signup's
// archive.
func TestAGenesisThatAlsoChangesAnotherContext(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	userLiving(t, root)

	out, err := runChange(t, root, "new", "signup", "--context", "signup", "--context", "user")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	for _, f := range []string{"proposal.md", "design.md", "deltas/signup.md", "deltas/user.md"} {
		if !exists(filepath.Join(root, "specs", "signup", filepath.FromSlash(f))) {
			t.Errorf("specs/signup/%s was not written:\n%s", f, out)
		}
	}
	if exists(filepath.Join(root, "specs", "changes")) {
		t.Errorf("the genesis went to specs/changes/:\n%s", out)
	}
	if !strings.Contains(out, "gofi spec check signup") {
		t.Errorf("the id signup was not said:\n%s", out)
	}
	if out, _ := runChange(t, root, "list"); !strings.Contains(out, "signup  proposto · signup, user · ") || !strings.Contains(out, "· specs/signup/") {
		t.Errorf("list:\n%s", out)
	}

	dir := filepath.Join(root, "specs", "signup")
	writeFile(t, filepath.Join(dir, "proposal.md"), genesisProposal("signup, user"))
	writeFile(t, filepath.Join(dir, "deltas", "signup.md"), genesisDelta("signup", "REQ-01", true))
	writeFile(t, filepath.Join(dir, "deltas", "user.md"), genesisDelta("user", "REQ-02", false))
	writeFile(t, filepath.Join(dir, "design.md"), genesisDesign("signup", "| REQ-01 | Signup.Create |\n| REQ-02 | User.Refuse |"))

	if out, err := runSpec(t, root, "check", "signup"); err != nil {
		t.Fatalf("spec check signup: %v\n%s", err, out)
	}
	if got, err := changeDirOf(root, "signup"); err != nil || got != "specs/signup" {
		t.Errorf("review finds the genesis at %q: %v", got, err)
	}
	if out, err := runChange(t, root, "approve", "signup"); err != nil || strings.Count(out, "sha256:") != 4 {
		t.Fatalf("approve: %v\n%s", err, out)
	}

	// The index files the proposal under each of its contexts, and the
	// genesis's deltas/ is no drift.
	b := &docs.Builder{Root: root}
	if _, _, drift, err := b.Build(); err != nil {
		t.Fatal(err)
	} else {
		for _, d := range drift {
			if strings.Contains(string(d), "serve só o contexto") {
				t.Errorf("drift: %s", d)
			}
		}
	}
	router, _ := os.ReadFile(filepath.Join(root, "specs", "INDEX.md"))
	if strings.Contains(string(router), "sem-contexto") {
		t.Errorf("a proposal was filed under sem-contexto:\n%s", router)
	}

	out, err = runChange(t, root, "archive", "signup")
	if err != nil {
		t.Fatalf("archive: %v\n%s", err, out)
	}
	if !strings.Contains(out, "specs/signup/spec.md") || !strings.Contains(out, "specs/user/spec.md") || !strings.Contains(out, "specs/signup/changes/archive/") {
		t.Errorf("archive said:\n%s", out)
	}
	user, _ := os.ReadFile(filepath.Join(root, "specs", "user", "spec.md"))
	if !strings.Contains(string(user), "REQ-01") || !strings.Contains(string(user), "REQ-02") {
		t.Errorf("user's spec did not get the requirement:\n%s", user)
	}
	for _, f := range []string{"proposal.md", "design.md", "deltas"} {
		if exists(filepath.Join(dir, f)) {
			t.Errorf("specs/signup/%s was left after the archive", f)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "changes", "archive", "*-signup", "deltas", "user.md")); len(matches) != 1 {
		t.Error("the delta of user was not archived with the genesis")
	}
	for _, ctx := range []string{"signup", "user"} {
		if out, err := runSpec(t, root, "check", ctx); err != nil {
			t.Errorf("the spec of %s does not pass the check: %v\n%s", ctx, err, out)
		}
	}
}

// The first context owns the change, alone or with others: nothing is born in
// specs/changes/.
func TestAChangeOfContextsThatExistIsNoGenesis(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	writeFile(t, filepath.Join(root, "specs/tenant/spec.md"), "---\ntipo: spec\nformato: 3\ncontexto: tenant\n---\n# Spec\n")
	writeFile(t, filepath.Join(root, "specs/user/spec.md"), "---\ntipo: spec\nformato: 3\ncontexto: user\n---\n# Spec\n")
	for _, tc := range []struct {
		args []string
		dir  string
	}{
		{[]string{"new", "criar-x", "--context", "tenant"}, "specs/tenant/changes/criar-x"},
		{[]string{"new", "criar-y", "--context", "tenant", "--context", "user"}, "specs/tenant/changes/criar-y"},
		// A context with a spec first: the other's lack of one makes no genesis.
		{[]string{"new", "criar-z", "--context", "tenant", "--context", "novo"}, "specs/tenant/changes/criar-z"},
	} {
		if out, err := runChange(t, root, tc.args...); err != nil || !strings.Contains(out, tc.dir+"/proposal.md") {
			t.Errorf("%v: %v\n%s", tc.args, err, out)
		}
	}
	if exists(filepath.Join(root, "specs", "changes")) {
		t.Error("a change was born in specs/changes/")
	}
	if out, _ := runChange(t, root, "list"); strings.Contains(out, "!") {
		t.Errorf("a change of several contexts in its owner's folder was said out of place:\n%s", out)
	}
}

// A change among contexts that exist, in its owner's folder, carries the
// other's delta: archived, each spec gets its requirement and the change
// stays with its owner.
func TestAChangeOfSeveralContextsIsArchivedByItsOwner(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	userLiving(t, root)
	if out, err := runChange(t, root, "new", "tenant", "--context", "tenant"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	tenant := filepath.Join(root, "specs", "tenant")
	writeFile(t, filepath.Join(tenant, "proposal.md"), genesisProposal("tenant"))
	writeFile(t, filepath.Join(tenant, "deltas", "tenant.md"), genesisDelta("tenant", "REQ-01", true))
	writeFile(t, filepath.Join(tenant, "design.md"), genesisDesign("tenant", "| REQ-01 | Tenant.Create |"))
	for _, verb := range []string{"approve", "archive"} {
		if out, err := runChange(t, root, verb, "tenant"); err != nil {
			t.Fatalf("%s: %v\n%s", verb, err, out)
		}
	}

	if out, err := runChange(t, root, "new", "ligar-user", "--context", "tenant", "--context", "user"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	dir := filepath.Join(tenant, "changes", "ligar-user")
	writeFile(t, filepath.Join(dir, "proposal.md"), genesisProposal("tenant, user"))
	writeFile(t, filepath.Join(dir, "deltas", "tenant.md"), genesisDelta("tenant", "REQ-02", false))
	writeFile(t, filepath.Join(dir, "deltas", "user.md"), genesisDelta("user", "REQ-02", false))
	writeFile(t, filepath.Join(dir, "design.md"), genesisDesign("tenant", "| tenant/REQ-02 | Tenant.Link |\n| user/REQ-02 | User.Link |"))
	if out, err := runChange(t, root, "approve", "ligar-user"); err != nil {
		t.Fatalf("approve: %v\n%s", err, out)
	}
	out, err := runChange(t, root, "archive", "ligar-user")
	if err != nil || !strings.Contains(out, "specs/tenant/changes/archive/") {
		t.Fatalf("archive: %v\n%s", err, out)
	}
	for _, ctx := range []string{"tenant", "user"} {
		if b, _ := os.ReadFile(filepath.Join(root, "specs", ctx, "spec.md")); !strings.Contains(string(b), "REQ-02") {
			t.Errorf("the spec of %s did not get the requirement:\n%s", ctx, b)
		}
	}
}

// specs/changes/ holds no change: a proposal written there by hand is not
// read, list says so, and no context can take the folder's name.
func TestTheSharedFolderHoldsNoChange(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	writeFile(t, filepath.Join(root, "specs/changes/ligar/proposal.md"), genesisProposal("tenant, user"))
	if _, ok := contract.FindChange(root, "ligar"); ok {
		t.Error("a folder in specs/changes/ was read as a change")
	}
	out, err := runChange(t, root, "list")
	if err != nil || !strings.Contains(out, "specs/changes/ligar") || strings.Contains(out, "specs/changes/<id>") {
		t.Errorf("list: %v\n%s", err, out)
	}
	if _, err := runChange(t, root, "new", "criar-x", "--context", "changes"); err == nil || !strings.Contains(err.Error(), "reservado") {
		t.Errorf("a context named changes: %v", err)
	}
}

// A genesis carrying a delta of a context still being born is archived after
// that context's own genesis, not before it.
func TestAGenesisWaitsForTheOtherContextsGenesis(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	if out, err := runChange(t, root, "new", "user", "--context", "user"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, err := runChange(t, root, "new", "signup", "--context", "signup", "--context", "user"); err != nil || !strings.Contains(out, "specs/signup/deltas/user.md") {
		t.Fatalf("%v\n%s", err, out)
	}
	dir := filepath.Join(root, "specs", "signup")
	writeFile(t, filepath.Join(dir, "proposal.md"), genesisProposal("signup, user"))
	writeFile(t, filepath.Join(dir, "deltas", "signup.md"), genesisDelta("signup", "REQ-01", true))
	writeFile(t, filepath.Join(dir, "deltas", "user.md"), genesisDelta("user", "REQ-02", true))
	writeFile(t, filepath.Join(dir, "design.md"), genesisDesign("signup", "| REQ-01 | Signup.Create |\n| REQ-02 | User.Refuse |"))
	if out, err := runChange(t, root, "approve", "signup"); err != nil {
		t.Fatalf("approve: %v\n%s", err, out)
	}
	out, err := runChange(t, root, "archive", "signup")
	if err == nil || !strings.Contains(err.Error(), "gênese user") {
		t.Fatalf("archived ahead of user's genesis: %v\n%s", err, out)
	}
	if exists(filepath.Join(root, "specs", "user", "spec.md")) {
		t.Error("a refused archive wrote user's spec")
	}
}

// The MCP approve tool reads a genesis id as the command does.
func TestMCPApprovesAGenesisWithOtherDeltas(t *testing.T) {
	root := configuredProject(t)
	changeTemplates(t, root)
	userLiving(t, root)
	if out, err := runChange(t, root, "new", "signup", "--context", "signup", "--context", "user"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	dir := filepath.Join(root, "specs", "signup")
	writeFile(t, filepath.Join(dir, "proposal.md"), genesisProposal("signup, user"))
	writeFile(t, filepath.Join(dir, "deltas", "signup.md"), genesisDelta("signup", "REQ-01", true))
	writeFile(t, filepath.Join(dir, "deltas", "user.md"), genesisDelta("user", "REQ-02", false))
	writeFile(t, filepath.Join(dir, "design.md"), genesisDesign("signup", "| REQ-01 | Signup.Create |\n| REQ-02 | User.Refuse |"))
	cs := mcpSession(t, root)
	if out, isErr := callText(t, cs, "approve", map[string]any{"ref": "signup"}); isErr {
		t.Fatalf("approve: %s", out)
	}
	if !sealed(t, root, "signup") {
		t.Error("the genesis was not sealed")
	}
}

// A server that outlived an update says so instead of naming the id unknown.
func TestMCPSaysItIsStale(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "gofi")
	writeFile(t, exe, "v1")
	g := &gofiMCP{exe: exe, binary: binaryStamp(exe)}
	if g.stale() {
		t.Fatal("stale before any update")
	}
	writeFile(t, exe, "v2, a longer binary")
	if !g.stale() {
		t.Error("the update was not seen")
	}
}

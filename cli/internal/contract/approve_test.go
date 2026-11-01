package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const specPath = "specs/pedidos/sdd-pedidos.md"

// Approving writes the seal into the frontmatter, and what it seals is the
// body: approving again changes nothing, a frontmatter edit keeps the seal, a
// body edit or a new version breaks it.
func TestTheSealHoldsUntilTheBodyChanges(t *testing.T) {
	root := project(t)
	write(t, root, specPath, readySpec)

	if state, _ := SealOf(only(t, root)); state != Unsealed {
		t.Fatalf("state before approval = %v", state)
	}
	if wrote, err := Approve(root, only(t, root)); err != nil || !wrote {
		t.Fatalf("approve: wrote=%v err=%v", wrote, err)
	}
	doc := read(t, root, specPath)
	for _, want := range []string{"status: aprovado\n", "aprovacao_versao: \"1.0\"\n", "aprovacao_sha: \"sha256:"} {
		if !strings.Contains(doc, want) {
			t.Errorf("approved spec lacks %q:\n%s", want, doc)
		}
	}
	if state, _ := SealOf(only(t, root)); state != Sealed {
		t.Fatalf("state after approval = %v", state)
	}
	if wrote, err := Approve(root, only(t, root)); err != nil || wrote {
		t.Errorf("approving an unchanged spec again: wrote=%v err=%v", wrote, err)
	}

	write(t, root, specPath, strings.Replace(doc, "keywords: [pedido]", "keywords: [pedido, venda]", 1))
	if state, _ := SealOf(only(t, root)); state != Sealed {
		t.Errorf("a frontmatter edit broke the seal: %v", state)
	}

	write(t, root, specPath, strings.Replace(doc, "maior que zero", "maior ou igual a zero", 1))
	state, reason := SealOf(only(t, root))
	if state != Stale || !strings.Contains(reason, "corpo mudou") {
		t.Errorf("a body edit left state %v (%q)", state, reason)
	}
	if got := StaleSeals(root); len(got) != 1 || !got[0].Error || !strings.Contains(got[0].Message, "aprovação vencida") {
		t.Errorf("stale seals = %+v", got)
	}
	// A stale seal does not block approving again: that is how it is cleared.
	if got := Blocking(root, []Doc{only(t, root)}, only(t, root), mustPlaceholders(t, root)); len(got) != 0 {
		t.Errorf("blocking = %+v", got)
	}

	write(t, root, specPath, strings.Replace(doc, `versao: "1.0"`, `versao: "1.1"`, 1))
	if state, reason := SealOf(only(t, root)); state != Stale || !strings.Contains(reason, "versão 1.0") {
		t.Errorf("a new version left state %v (%q)", state, reason)
	}
}

// An approved status written by hand or by an agent, with no seal, is worth a
// warning in spec check — and nothing in the corpus check, where every spec
// from before sealing would trip it.
func TestAnApprovedStatusWithoutASeal(t *testing.T) {
	root := project(t)
	write(t, root, specPath, strings.Replace(readySpec, "status: proposto", "status: aprovado", 1))
	got := Check(root, corpus(t, root), only(t, root), mustPlaceholders(t, root))
	if len(got) != 1 || got[0].Error || !strings.Contains(got[0].Message, "sem selo") {
		t.Errorf("findings:\n%s", messages(got))
	}
	if got := StaleSeals(root); len(got) != 0 {
		t.Errorf("stale seals = %+v", got)
	}
}

// The frontmatter is rewritten in place: fields replaced on their line,
// missing ones added before the fence, comments and line endings kept.
func TestSetFieldsKeepsTheRestOfTheFile(t *testing.T) {
	doc := "---\r\ntipo: spec\r\nstatus: proposto   # ou em_revisao\r\nversao: \"1.0\"\r\n---\r\n# corpo # com cerquilha\r\n"
	got, err := setFields(doc, [][2]string{{"status", "aprovado"}, {"aprovacao_sha", `"x"`}})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\r\ntipo: spec\r\nstatus: aprovado\r\nversao: \"1.0\"\r\naprovacao_sha: \"x\"\r\n---\r\n# corpo # com cerquilha\r\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if _, err := setFields("# sem frontmatter\n", nil); err == nil {
		t.Error("a file with no frontmatter was rewritten")
	}
}

func TestSealIgnoresLineEndingsOnly(t *testing.T) {
	a := Seal([]string{"# x", "texto", "", ""})
	if b := Seal([]string{"# x", "texto"}); a != b {
		t.Error("trailing blank lines changed the seal")
	}
	if c := Seal([]string{"# x", "texto."}); a == c {
		t.Error("a changed byte kept the seal")
	}
}

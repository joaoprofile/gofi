package retrieval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joaoprofile/gofi/cli/internal/docs"
)

// Portuguese and English inflections of one root meet at the same stem, which
// is what lets a Portuguese question reach an English heading.
func TestStemFoldsInflectionsAcrossLanguages(t *testing.T) {
	for _, group := range [][]string{
		{"cancelar", "cancelado", "cancelados", "cancelled", "cancel"},
		{"validar", "validação", "validation", "validate"},
		{"faturar", "faturado", "faturamento"},
		{"logging", "logged", "log"},
	} {
		want := tokens(group[0])
		for _, w := range group[1:] {
			if got := tokens(w); strings.Join(got, " ") != strings.Join(want, " ") {
				t.Errorf("%s → %v, want %v like %s", w, got, want, group[0])
			}
		}
	}
	// Short words are left alone: cutting them merges words that only look alike.
	if got := stem("dado"); got != "dado" {
		t.Errorf("stem(dado) = %q", got)
	}
}

// A phrase in the question bridges as a whole, from the base lexicon or the
// project's, and a synonym counts less than the word asked.
func TestSynonymPhrasesBridgeTheQuestion(t *testing.T) {
	root := fixture(t, false)
	write(t, root, ".claude/lexicon/sinonimos.md", "| Termo | Sinônimos |\n|---|---|\n| billing run | rodada de faturamento |\n")
	e := open(t, root)

	terms := e.expand("como modelar a máquina de estados")
	for _, want := range tokens("state machine") {
		if w, ok := terms[want]; !ok || w >= 1 {
			t.Errorf("base lexicon: %q weight %v, want a synonym weight", want, w)
		}
	}
	if _, ok := e.expand("rodada de faturamento")[stem("run")]; !ok {
		t.Error("project lexicon phrase did not bridge")
	}
	if w := terms[stem("modelar")]; w != 1 {
		t.Errorf("a word asked counts %v, want 1", w)
	}
}

func TestFindsViaTheBaseLexicon(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".claude/knowledge/shared/events.md", "# Eventos\n\n## Contrato\n\nFormato do envelope.\n\n## Retry transient vs permanent\n\nReprocessar só o que pode passar numa segunda vez.\n")
	write(t, root, ".claude/knowledge/shared/other.md", "# Outro\n\n## Erro de negócio\n\nMensagem para o usuário.\n")
	if _, _, _, err := (&docs.Builder{Root: root}).Build(); err != nil {
		t.Fatal(err)
	}
	hits := open(t, root).Find(Query{Text: "erro temporário deve ser tentado de novo", Limit: 1})
	if len(hits) == 0 || hits[0].Title != "Retry transient vs permanent" {
		t.Fatalf("got %v", hits)
	}
}

func TestNamesThroughStemAndLexicon(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, filepath.FromSlash(docs.LexiconDir()), docs.LexSynonyms+".md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("| termo | sinônimos |\n|---|---|\n| order | pedido |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Engine{synonyms: loadSynonyms(root)}
	for _, c := range []struct {
		word, term string
		want       bool
	}{
		{"pedidos", "order", true},
		{"Pedido", "order", true},
		{"orders", "order", true},
		{"pricing", "pricing", true},
		{"sincronização", "order", false},
	} {
		if got := e.Names(c.word, c.term); got != c.want {
			t.Errorf("Names(%q, %q) = %v, want %v", c.word, c.term, got, c.want)
		}
	}
}

package learn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lessons = `{"lessons":[{"scope":"eng","name":"sem-float-em-dinheiro","title":"Dinheiro em centavos","body":"Valor monetário é inteiro em centavos.","keywords":["dinheiro"],"evidence":"F-1"},
{"scope":"qa","name":"x","title":"y","body":"z","keywords":[]}]}`

func TestALessonIsWrittenOnlyWhenAccepted(t *testing.T) {
	root := t.TempDir()
	l, err := Parse([]byte(lessons), "r-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(root, l); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "knowledge")); !os.IsNotExist(err) {
		t.Fatal("a proposal touched the knowledge")
	}
	rel, err := Accept(root, l, 0)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, rel))
	if !strings.Contains(rel, "knowledge/eng/sem-float-em-dinheiro.md") || !strings.Contains(string(body), "keywords: [dinheiro]") ||
		strings.Contains(string(body), "F-1") {
		t.Errorf("%s:\n%s", rel, body)
	}
	if err := Reject(root, l, 1); err != nil {
		t.Fatal(err)
	}
	all, err := All(root)
	if err != nil || len(all) != 1 || all[0].Lessons[0].Status != Accepted || all[0].Lessons[1].Status != Rejected {
		t.Errorf("all = %+v, %v", all, err)
	}
	if _, err := Accept(root, all[0], 0); err == nil {
		t.Error("an accepted lesson was accepted again")
	}
	for _, bad := range []string{`{"lessons":[{"scope":"sales","name":"a","title":"t","body":"b"}]}`, `{"lessons":[{"scope":"eng","name":"Not Kebab","title":"t","body":"b"}]}`} {
		if _, err := Parse([]byte(bad), "r"); err == nil {
			t.Errorf("Parse(%s) accepted", bad)
		}
	}
}

package review

import "testing"

func TestParseOrdersAndRefusesWhatItCannotRead(t *testing.T) {
	r, err := Parse([]byte(`{"findings":[{"severity":"minor","kind":"lacuna","where":"a","message":"m1","extra":1},{"severity":"Blocker","kind":"contradição","where":"b","message":"m2"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.Schema != Schema || r.Findings[0].Severity != "blocker" || len(r.Blocking()) != 1 {
		t.Errorf("review = %+v", r)
	}
	for _, bad := range []string{`{"findings":[{"severity":"grave","message":"m"}]}`, `{"findings":[{"severity":"major"}]}`, `{"schema":"x/v9"}`, `nope`} {
		if _, err := Parse([]byte(bad)); err == nil {
			t.Errorf("Parse(%s) accepted", bad)
		}
	}
}

func TestHashIsWhatWasRead(t *testing.T) {
	a := Hash(map[string][]string{"p.md": {"x", ""}, "d.md": {"y"}})
	if a != Hash(map[string][]string{"d.md": {"y"}, "p.md": {"x"}}) {
		t.Error("the same bodies hash differently")
	}
	if a == Hash(map[string][]string{"p.md": {"x"}, "d.md": {"z"}}) {
		t.Error("an edit kept the hash")
	}
}

func TestSaveAndLoad(t *testing.T) {
	root := t.TempDir()
	if r, err := Load(root, "x"); r != nil || err != nil {
		t.Fatalf("nothing kept: %v, %v", r, err)
	}
	if err := Save(root, &Review{Schema: Schema, Change: "x", Hash: "h", Findings: []Finding{}}); err != nil {
		t.Fatal(err)
	}
	if r, err := Load(root, "x"); err != nil || r.Hash != "h" {
		t.Errorf("loaded %+v, %v", r, err)
	}
}

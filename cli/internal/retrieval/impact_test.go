package retrieval

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImpactOfASymbolAndOfAFile(t *testing.T) {
	root := fixture(t, true)
	e := open(t, root)
	view, err := e.Impact([]string{"service.BillOrder"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	d := view.Data.(ImpactData)
	if len(d.Dependents) != 1 || d.Dependents[0].Symbol != "domain/order/handler.Bill" || d.Dependents[0].Context != "order" ||
		d.Dependents[0].File != "domain/order/handler/order_handler.go" || d.Dependents[0].Depth != 1 {
		t.Fatalf("dependents = %+v", d.Dependents)
	}
	if strings.Join(d.Contexts, ",") != "order" || !strings.Contains(view.Text, "1 dependentes") {
		t.Errorf("contexts %v, text:\n%s", d.Contexts, view.Text)
	}
	// A file stands for the symbols it declares.
	file := "domain/order/service/order_service.go"
	_ = os.MkdirAll(filepath.Join(root, filepath.Dir(file)), 0o755)
	_ = os.WriteFile(filepath.Join(root, file), []byte("package service\n"), 0o644)
	view, err = e.Impact([]string{file}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d := view.Data.(ImpactData); len(d.Targets) != 1 || len(d.Dependents) != 1 {
		t.Errorf("impact of %s = %+v", file, d)
	}
	if _, err := e.Impact([]string{"Nada"}, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown target: %v", err)
	}
}

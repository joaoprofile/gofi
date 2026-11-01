package contract

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// spec writes a ready spec for a context, with the dependencies and tables
// given.
func spec(t *testing.T, root, ctx, deps, tables string) {
	t.Helper()
	body := strings.Replace(readySpec, "contexto: pedidos", "contexto: "+ctx, 1)
	body = strings.Replace(body, "keywords: [pedido]", "depende_de: ["+deps+"]\nentidades: ["+tables+"]\nkeywords: [pedido]", 1)
	write(t, root, "specs/"+ctx+"/sdd-"+ctx+".md", body)
}

func corpus(t *testing.T, root string) []Spec {
	t.Helper()
	all, err := Specs(root)
	if err != nil {
		t.Fatal(err)
	}
	return all
}

// Each level depends only on the levels before it; within a level, by name.
func TestOrderPutsContextsInLevels(t *testing.T) {
	root := project(t)
	spec(t, root, "envio", "pedidos", "")
	spec(t, root, "pagamento", "pedidos, clientes", "")
	spec(t, root, "pedidos", "clientes, catalogo", "")
	spec(t, root, "catalogo", "", "")
	spec(t, root, "clientes", "", "")
	// A dependency on a context with no spec does not move the order.
	spec(t, root, "relatorios", "estoque", "")

	levels, err := Order(corpus(t, root))
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"catalogo", "clientes", "relatorios"}, {"pedidos"}, {"envio", "pagamento"}}
	if !reflect.DeepEqual(levels, want) {
		t.Errorf("levels = %v, want %v", levels, want)
	}
}

func TestACycleHasNoOrder(t *testing.T) {
	root := project(t)
	spec(t, root, "a", "b", "")
	spec(t, root, "b", "c", "")
	spec(t, root, "c", "a", "")
	spec(t, root, "d", "", "")

	_, err := Order(corpus(t, root))
	var cycle *CycleError
	if !errors.As(err, &cycle) || strings.Join(cycle.Path, ">") != "a>b>c>a" {
		t.Fatalf("err = %v", err)
	}
	all := corpus(t, root)
	got := messages(CheckAll(root, all, all))
	if strings.Count(got, "ciclo de dependência: a → b → c → a") != 3 {
		t.Errorf("each spec in the cycle should say so:\n%s", got)
	}
}

func TestCheckReportsWhatNoOrderCouldBuild(t *testing.T) {
	root := project(t)
	spec(t, root, "pedidos", "pedidos, estoque", "")
	all := corpus(t, root)
	got := messages(CheckAll(root, all, all))
	for _, want := range []string{
		"error depende_de: pedidos depende de si mesmo",
		`error depende_de: "estoque" não tem spec`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// The same errors keep it from being approved.
	if b := Blocking(root, all, all[0], mustPlaceholders(t, root)); len(b) != 2 {
		t.Errorf("blocking = %+v", b)
	}
}

// Two contexts describing one table, with no dependency either way, are
// worth a look: once per pair, on the first of them, naming every table.
func TestSharedTablesWithoutADependencyAreAWarning(t *testing.T) {
	root := project(t)
	spec(t, root, "clientes", "", "cliente_cliente, cliente_endereco")
	spec(t, root, "pedidos", "", "pedido_pedido, cliente_cliente, cliente_endereco")
	spec(t, root, "envio", "pedidos", "pedido_pedido")
	all := corpus(t, root)

	got := CheckAll(root, all, all)
	var shared []string
	for _, f := range got {
		if strings.Contains(f.Message, "sem dependência declarada") {
			if f.Error {
				t.Errorf("a shared table is an error: %+v", f)
			}
			shared = append(shared, f.Path+": "+f.Message)
		}
	}
	want := "specs/clientes/sdd-clientes.md: clientes e pedidos descrevem cliente_cliente, cliente_endereco sem dependência declarada — se um consome o outro, declare depende_de"
	if len(shared) != 1 || shared[0] != want {
		t.Errorf("shared = %q", shared)
	}
}

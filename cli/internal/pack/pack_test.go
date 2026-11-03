package pack

import "testing"

func TestGlob(t *testing.T) {
	for _, c := range []struct {
		pat, file string
		want      bool
	}{
		{"domain/*/handler/**", "domain/pedidos/handler/pedidos_handler.go", true},
		{"domain/*/handler/**", "domain/pedidos/handler/sub/x.go", true},
		{"domain/*/handler/**", "domain/pedidos/service/x.go", false},
		{"domain/*/*", "domain/pedidos/pontos.go", true},
		{"domain/*/*", "domain/pedidos/model/entity.go", false},
		{".migrations/**", ".migrations/0001_up.sql", true},
		{"**/x.go", "x.go", true},
		{"**/x.go", "a/b/x.go", true},
	} {
		if got := Glob(c.pat, c.file); got != c.want {
			t.Errorf("Glob(%q, %q) = %v, want %v", c.pat, c.file, got, c.want)
		}
	}
}

func TestGoLayersAndContexts(t *testing.T) {
	p, err := Load("go")
	if err != nil || p == nil {
		t.Fatalf("Load(go) = %v, %v", p, err)
	}
	for file, want := range map[string][2]string{
		"domain/pedidos/handler/pedidos_handler.go":       {"handler", "api"},
		"domain/pedidos/repository/pedidos_repository.go": {"repository", "data"},
		"domain/pedidos/service/pedidos_service.go":       {"service", "domain"},
		"domain/pedidos/model/entity.go":                  {"model", "data"},
		"domain/pedidos/adapter/iam_adapter.go":           {"adapter", "integration"},
		"domain/pontos/pontos.go":                         {"domain", "domain"},
		".migrations/0001_pedidos.up.sql":                 {"migration", "data"},
		"loja/main.go":                                    {"", ""},
	} {
		layer, plan := p.LayerOf(file)
		if layer != want[0] || plan != want[1] {
			t.Errorf("LayerOf(%s) = %s, %s; want %s, %s", file, layer, plan, want[0], want[1])
		}
	}
	for file, want := range map[string]string{
		"domain/pedidos/handler/x.go": "pedidos",
		"domain/pontos/pontos.go":     "pontos",
		"domain/pontos":               "",
		"loja/main.go":                "",
	} {
		if got := p.ContextOf(file); got != want {
			t.Errorf("ContextOf(%s) = %q, want %q", file, got, want)
		}
	}
}

func TestNestLayersAndContexts(t *testing.T) {
	p, err := Load("nodejs")
	if err != nil || p == nil {
		t.Fatalf("Load(nodejs) = %v, %v", p, err)
	}
	for file, want := range map[string][2]string{
		"src/pontos/pontos.controller.ts":      {"controller", "api"},
		"src/pontos/pontos.repository.ts":      {"repository", "data"},
		"src/pontos/dto/consulta.dto.ts":       {"model", "data"},
		"src/pontos/entities/pontos.entity.ts": {"model", "data"},
		"src/pontos/pontos.service.ts":         {"service", "domain"},
		"src/pontos/pontos.module.ts":          {"domain", "domain"},
		"src/main.ts":                          {"", ""},
	} {
		layer, plan := p.LayerOf(file)
		if layer != want[0] || plan != want[1] {
			t.Errorf("LayerOf(%s) = %s, %s; want %s, %s", file, layer, plan, want[0], want[1])
		}
	}
	if c := p.ContextOf("src/pontos/pontos.service.ts"); c != "pontos" {
		t.Errorf("ContextOf = %q", c)
	}
	if !p.IsTest("src/pontos/pontos.service.spec.ts") || p.IsTest("src/pontos/pontos.service.ts") {
		t.Error("IsTest")
	}
}

func TestFastAPILayersAndContexts(t *testing.T) {
	p, err := Load("python")
	if err != nil || p == nil {
		t.Fatalf("Load(python) = %v, %v", p, err)
	}
	for file, want := range map[string][2]string{
		"app/pontos/router.py":       {"router", "api"},
		"app/pontos/repository.py":   {"repository", "data"},
		"app/pontos/schemas.py":      {"model", "data"},
		"app/pontos/service.py":      {"service", "domain"},
		"app/pontos/test_service.py": {"domain", "domain"},
		"app/main.py":                {"", ""},
	} {
		layer, plan := p.LayerOf(file)
		if layer != want[0] || plan != want[1] {
			t.Errorf("LayerOf(%s) = %s, %s; want %s, %s", file, layer, plan, want[0], want[1])
		}
	}
	if c := p.ContextOf("app/pontos/service.py"); c != "pontos" {
		t.Errorf("ContextOf = %q", c)
	}
	if !p.IsTest("app/pontos/test_service.py") || p.IsTest("app/pontos/service.py") {
		t.Error("IsTest")
	}
}

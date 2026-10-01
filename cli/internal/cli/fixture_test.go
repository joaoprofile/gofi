package cli

import (
	"os"
	"testing"
)

// TestWriteFixture writes, at GOFI_FIXTURE_DIR, a project the way gofi init
// installs it — the bench's, with an implemented context (order) and a PRD
// with no spec yet (catalog) — for runs against a real model that must never
// touch a real project. Skipped unless the directory is given; the index is
// left to `gofi index`, run with the binary under test.
//
//	GOFI_FIXTURE_DIR=/path/proj go test ./internal/cli -run WriteFixture
func TestWriteFixture(t *testing.T) {
	dir := os.Getenv("GOFI_FIXTURE_DIR")
	if dir == "" {
		t.Skip("set GOFI_FIXTURE_DIR to write the fixture project")
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("%s exists: the fixture is written into a new folder only", dir)
	}
	for rel, body := range fixturePRD {
		benchContext[rel] = body
	}
	benchFixture(t, benchRepoRoot(t), dir)
}

// fixturePRD is a context with a PRD and nothing else: what a spec is written
// from. It leaves open, on purpose, what a spec has to decide — listing,
// cache, write profile — so an elicitation has something to ask.
var fixturePRD = map[string]string{
	"prd/catalog/prd-catalog.md": `---
tipo: prd
contexto: catalog
versao: "1.0"
status: aprovado
keywords: [catalogo, produto, preco, vitrine]
---
# PRD — Catálogo de produtos

## 1. Problema

As lojas cadastram produtos em planilhas e o time de vendas não tem uma fonte
única de preço e disponibilidade.

## 2. Objetivo

Um catálogo central de produtos, consultado pelas lojas e pelo time de vendas.

## 3. Atores

- Gestor de catálogo: cadastra e altera produtos.
- Loja: consulta produtos disponíveis.

## 5. Regras de negócio

### RN-01 — SKU único por loja

Dois produtos da mesma loja não podem ter o mesmo SKU.

### RN-02 — Preço positivo

Todo produto tem preço maior que zero, em reais.

### RN-03 — Produto inativo não aparece para a loja

Produto desativado continua no catálogo, mas a loja não o vê.

## 6. Requisitos funcionais

- RF-01: cadastrar, alterar e desativar produto.
- RF-02: a loja lista os produtos ativos dela, com busca por nome.

## 9. Critérios de aceite

- CA-01: cadastrar produto com SKU repetido na mesma loja é recusado.
- CA-02: produto desativado some da listagem da loja.
`,
	".claude/memory/contexts/catalog.md": `---
formato: memoria
contexto: catalog
versao: "1.0"
status: prd
keywords: [catalogo, produto]
---
# catalog

## Estado atual

PRD aprovado; spec a escrever.

## Histórico de versões

- 1.0 — PRD aprovado.
`,
}

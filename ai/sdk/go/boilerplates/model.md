---
name: boilerplate-model
description: Esqueleto do model Go — entidade com tags db, DTOs com validate, query_dto com sqln.FilterMapping
sdk: v0.8.2
keywords: [boilerplate, model, entity, dto, FilterMapping, db-tag, validator, Page]
---

# Boilerplate — Model

> **`//gofi:context {contexto}`** abre a cláusula `package` — é o elo entre o
> símbolo no grafo e `specs/{contexto}/` + `.claude/memory/contexts/{contexto}.md`.
> Basta em **um** arquivo do pacote (vale para todos os símbolos dele); use o
> mesmo nome de `specs/{contexto}/`, kebab-case. Detalhe em
> `.claude/expertise/harness-protocols/graph-retrieval.md`.

## entity.go — entidade de domínio

```go
//gofi:context {contexto}
package model

import "time"

type Entity struct {
	ID        string    `json:"id"        db:"id"`
	TenantID  string    `json:"-"         db:"tenant_id"`
	Name      string    `json:"name"      db:"name"`
	Email     string    `json:"email"     db:"email"`
	Status    string    `json:"status"    db:"status"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
```

- Tag `db` = nome (ou alias) da coluna no `SELECT`: o mapper casa **por
  nome** quando o resultado cobre todas as tags; ordem das colunas não importa
  (`.claude/sdk/go/knowledge/value-objects.md`).
- Listagem paginada devolve `*sqln.Page[Entity]` direto nas assinaturas — sem
  alias de tipo ponteiro.
- Campo calculado depois do scan: só `json`, **sem** tag `db` (nem `db:"-"`).

### Value Objects aninhados

```go
type Address struct {
	Street  string `json:"street"  db:"street"`
	City    string `json:"city"    db:"city"`
	ZipCode string `json:"zipCode" db:"zip_code"`
}

type Customer struct {
	ID      string  `json:"id"      db:"id"`
	Address Address `json:"address" db:"address"` // outer tag marks the VO; leaves map to columns
}
```

`SELECT c.id, c.street, c.city, c.zip_code ...` preenche `Customer.Address`.
Coluna JSON única = tipo com `sql.Scanner` + `driver.Valuer`. Regras e
armadilhas em `.claude/sdk/go/knowledge/value-objects.md`.

## dto.go — DTOs de entrada

```go
package model

import "github.com/gofi-labs/gofi-sdk-go/base/validator"

var v = validator.New()

type CreateEntityRequest struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

func (r CreateEntityRequest) Validate() error { return v.ValidateStruct(r) }

type UpdateEntityRequest struct {
	Name   string `json:"name"   validate:"required"`
	Email  string `json:"email"  validate:"required,email"`
	Status string `json:"status" validate:"required"`
}

func (r UpdateEntityRequest) Validate() error { return v.ValidateStruct(r) }

// EntityFilter is bound from the query string (netx.BindQueryParamsToStruct).
type EntityFilter struct {
	TenantID string   `form:"-"` // set from the auth context, never from the query
	Name     string   `form:"name"`
	Statuses []string `form:"status"`
	Page     uint16   `form:"page"`
	Limit    uint16   `form:"limit" validate:"omitempty,lte=200"`
}
```

## query_dto.go — filtro dinâmico (só quando necessário)

Arquivo separado de `dto.go`, criado **apenas** quando o contexto expõe
`/schemas` + `/query` (`.claude/sdk/go/knowledge/dynamic-filter.md`).

```go
package model

import (
	"github.com/gofi-labs/gofi-sdk-go/sqln"

	"<module>/common/enums"
)

// EntityFilterMapping is the allowlist of the dynamic query: API names bound
// to columns, accepted operators and sortable fields. Column is never serialized.
var EntityFilterMapping = sqln.FilterMapping{
	"name":    {Column: "e.name", Ops: sqln.Text, Sortable: true, Label: "NAME", FilterType: "text"},
	"email":   {Column: "e.email", Ops: sqln.Text, Label: "EMAIL", FilterType: "text"},
	"status":  {Column: "e.status", Ops: sqln.Equality, Label: "STATUS",
		FilterType: "search-multiple", SearchType: "embedded", Content: enums.EntityStatusMap},
	"created": {Column: "e.created_at", Ops: sqln.Range, Sortable: true, Label: "CREATED_AT"},
}
```

Read model próprio (`EntityQuery`) só quando a projeção da query dinâmica
difere da entidade; senão reuse `Entity`.

## Separação entity.go / dto.go / query_dto.go

| Arquivo | Responsabilidade | Tags | Dependência |
|---|---|---|---|
| `entity.go` | struct mapeada do banco | `db`, `json` | — |
| `dto.go` | entrada/saída da API | `validate`, `json`, `form` | `base/validator` |
| `query_dto.go` | allowlist do filtro dinâmico (+ read model) | `db`, `json` | `sqln` |

## Regras

- Validator como singleton de pacote (`var v = validator.New()`); `Validate()`
  em todo DTO de entrada.
- Tenancy nunca bindada do cliente: `form:"-"` / `json:"-"`, setada do auth.
- `query_dto.go` só com filtro dinâmico; o mapping é variável de pacote
  (serializada pelo `getSchema`).

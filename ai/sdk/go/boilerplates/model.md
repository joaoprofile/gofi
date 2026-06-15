# Boilerplate — Model

## entity.go — Entidade de domínio

```go
package model

import (
	"time"

	"github.com/joaoprofile/gofi/sqln"
)

// PersonResponse é o tipo da resposta paginada — alias para sqln.Page[Person]
type PersonResponse *sqln.Page[Person]

type Person struct {
	ID        string    `json:"id"        db:"id"`
	Name      string    `json:"name"      db:"name"`
	Email     string    `json:"email"     db:"email"`
	CPF       string    `json:"cpf"       db:"cpf"`
	Age       int       `json:"age"       db:"age"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
```

### Value Objects aninhados

Quando o domínio tem um value object que encapsula um ou mais atributos (`Pricing`, `Address`, `Money`), declare como struct aninhada com tag `db` no campo externo. O mapper do `sqln` desce recursivamente e liga os sub-campos `db` às colunas da query.

```go
type Pricing struct {
    Price float64 `json:"price" db:"price"`
}

type Product struct {
    ID    int64   `json:"id"      db:"id"`
    Name  string  `json:"name"    db:"name"`
    Price Pricing `json:"pricing" db:"price"`
}
```

Query: `SELECT id, name, price FROM product` → a coluna `price` escaneia em `Product.Price.Price`. A tag `db` externa é marcador de presença; a ordem dos sub-campos internos define o mapeamento posicional. `time.Time` e tipos que implementam `sql.Scanner` permanecem primitivos.

> Regras, armadilhas e estratégias de persistência (colunas separadas vs. coluna única JSON/bytes) em `.claude/knowledge/value-objects.md`.

## dto.go — DTOs de entrada

```go
package model

import "github.com/joaoprofile/gofi/base/validator"

var v = validator.New()

type CreatePersonRequest struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
	CPF   string `json:"cpf"   validate:"required"`
	Age   int    `json:"age"   validate:"required,min=1"`
}

func (r CreatePersonRequest) Validate() error {
	return v.ValidateStruct(r)
}

type UpdatePersonRequest struct {
	Name  string `json:"name"  validate:"required"`
	Email string `json:"email" validate:"required,email"`
	CPF   string `json:"cpf"   validate:"required"`
	Age   int    `json:"age"   validate:"required,min=1"`
}

func (r UpdatePersonRequest) Validate() error {
	return v.ValidateStruct(r)
}

type PersonFilter struct {
	Name  string `form:"name"`
	CPF   string `form:"cpf"`
	Page  uint16 `form:"page"`
	Limit uint16 `form:"limit"`
}
```

## query_dto.go — Filtro Dinâmico (quando necessário)

Arquivo separado de `dto.go`. Criado apenas quando o contexto expõe endpoints de filtro dinâmico.

```go
package model

import (
    "time"
    "github.com/joaoprofile/gofi/sqln"
)

func PersonQueryMapping() *sqln.QueryMapping {
    return &sqln.QueryMapping{
        AllowedSortingFields: map[string]string{
            "Name": "sortedBy",
            "Age":  "sortedBy",
        },
        AllowedFields: []sqln.FieldMapping{
            {Key: "p.name",  Label: "NAME",  FilterType: "text"},
            {Key: "p.email", Label: "EMAIL", FilterType: "text"},
            {Key: "p.cpf",   Label: "CPF",   FilterType: "text"},
        },
        Operators: map[string]string{
            sqln.Eq:       sqln.Eq,
            sqln.Contains: sqln.Contains,
        },
        LogicalOperators: map[string]string{
            sqln.And: sqln.And,
            sqln.Or:  sqln.Or,
        },
    }
}

// PersonQueryResponse é o tipo de retorno do endpoint de query dinâmica
type PersonQueryResponse = *sqln.Page[PersonQuery]

// PersonQuery é o read model da query dinâmica — usa tags db:, não gofi:
type PersonQuery struct {
    ID        string    `json:"id"        db:"id"`
    Name      string    `json:"name"      db:"name"`
    Email     string    `json:"email"     db:"email"`
    CPF       string    `json:"cpf"       db:"cpf"`
    Age       int       `json:"age"       db:"age"`
    CreatedAt time.Time `json:"createdAt" db:"created_at"`
}
```

## Separação entity.go / dto.go / query_dto.go

| Arquivo | Responsabilidade | Tags | Dependência |
|---------|-----------------|------|-------------|
| `entity.go` | Struct mapeado do banco (write model) | `db:"col"`, `json:` | `gofi/sqln` |
| `dto.go` | Input/output CRUD da API | `validate:`, `json:`, `form:` | `gofi/base/validator` |
| `query_dto.go` | Mapping de filtro dinâmico + read model | `db:"col"`, `json:` | `gofi/sqln` |

## Regras

- Validator como **singleton de pacote** — `var v = validator.New()`
- Método `Validate()` em todos os DTOs de entrada (Create, Update)
- `Filter` não precisa de `Validate()` — campos opcionais
- `PersonResponse` como type alias para `*sqln.Page[Person]` — aproveitado em assinaturas do service e repository
- Tags `db:"col_name"` mapeiam para nomes de coluna SQL
- Tags `form:"field"` mapeiam query params via `netx.BindQueryParamsToStruct`
- `query_dto.go` é criado **somente** quando o contexto tem endpoints de filtro dinâmico — não criar por padrão
- `{Context}Query` struct é o **read model** específico para a query dinâmica — pode ter campos calculados ou projeções diferentes da entidade principal

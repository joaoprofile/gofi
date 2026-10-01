---
name: validation
description: Validação de DTOs com base/validator — singleton por pacote, Validate() no DTO, chamada no service e detalhes por campo no 400
sdk: v0.8.2
keywords: [validator, ValidateStruct, Validate, DTO, required, oneof, gtfield, ValidationError, FieldError, WithDetails, 400]
---

# Conhecimento — Validação de DTOs (`base/validator`)

Import: `github.com/gofi-labs/gofi-sdk-go/base/validator` (tags do
`go-playground/validator/v10`). Referência: `.claude/sdk/go/api/base-validator.md`.

## Singleton de pacote

```go
// dto.go
var v = validator.New() // *validator.Validator
```

Instanciar **uma vez por pacote**, não por request. O validator compila reflection em cache na primeira chamada — instanciar por request desperdiça CPU.

## Método Validate() em DTOs de entrada

Todos os DTOs que chegam via HTTP (Create, Update) devem ter `Validate()`:

```go
func (r CreateEntityRequest) Validate() error {
    return v.ValidateStruct(r)
}
```

DTOs de filtro/paginação não precisam — seus campos são opcionais.

## Tags de validação mais usadas

```go
type CreateEntityRequest struct {
    Name     string `validate:"required"`             // não vazio
    Email    string `validate:"required,email"`       // não vazio + formato email
    Document string `validate:"required"`             // não vazio
    Age      int    `validate:"required,min=1"`       // não zero + mínimo 1
    Role     string `validate:"oneof=RoleA RoleB"`    // enum
    URL      string `validate:"url"`                  // URL válida
    ID       string `validate:"uuid"`                 // UUID
}
```

## Onde chamar Validate()

Sempre no **service**, antes de qualquer I/O:

```go
func (s *entityService) Create(ctx context.Context, req model.CreateEntityRequest) errs.AppError {
    if err := req.Validate(); err != nil {
        return ErrEntityValidation.WithDetails(err)  // detalhes chegam ao cliente
    }
    // ... apenas aqui acessa o banco
}
```

Nunca no handler — o handler não sabe nada sobre regras de negócio.

## Propagação de erros de validação ao cliente

`ValidateStruct` devolve `validator.ValidationError{Errors []FieldError}`.
`WithDetails(err)` + `netx.RespondError(w, r, appErr)` levam os campos ao body:

```json
HTTP 400
{
  "code": 400,
  "message": "invalid entity data",
  "errorCode": "ENTITY_VALIDATION",
  "kind": "VALIDATION",
  "details": {
    "errors": [
      {"field": "Email", "message": "Field 'Email' failed on the 'email' rule"},
      {"field": "Age", "message": "Field 'Age' failed on the 'min' rule"}
    ]
  }
}
```

`field` é o **nome do campo Go** (não a tag `json`) e a mensagem é fixa em
inglês (`Field '<campo>' failed on the '<tag>' rule`). Se o front precisa do
nome JSON ou de texto traduzido, traduza no cliente a partir de `field` + tag —
não reescreva o validator.

## Config que alimenta motor de decisão — Create exige payload completo

Quando o DTO cria/atualiza uma **config consumida por um motor/decider**
(qualquer engine que lê a config e decide um comportamento — pricing,
ranking, matching, scheduling), **todos** os campos que o motor lê são
**obrigatórios no Create**. Config incompleta não é estado válido: um campo
ausente (`nil`/zero) vira default silencioso e o motor decide errado sem
erro visível — bug muito mais caro que um `400` na borda.

Marcar como `required` no Create DTO **cada** campo que o motor consome:

```go
type Create{Ctx}ConfigRequest struct {
    Type   string  `validate:"required,oneof=PRESET_A PRESET_B"` // discriminador
    Min    float64 `validate:"required"`                         // limite inferior
    Max    float64 `validate:"required,gtfield=Min"`             // limite superior > inferior
    Status string  `validate:"required,oneof=ACTIVE INACTIVE"`   // estado operacional
    Margin float64 `validate:"required"`                         // parâmetro de negócio
}
```

Duas camadas, ambas obrigatórias:

1. **Presença** — `required` em cada campo lido pelo motor. Nenhum opcional
   "por conveniência do front": se o motor lê, o Create exige.
2. **Coerência cross-field** — limites relacionados validados entre si
   (`gtfield`/`gtefield`/`ltefield`): `Max > Min`, faixa não-vazia, etc.
   Um par `(Min, Max)` individualmente preenchido mas `Max < Min` é tão
   inválido quanto ausente.

**Update parcial é a exceção, não a regra.** Só aceitar payload parcial
(`PATCH`) se a spec declarar update incremental **e** o repo souber
preservar o que não veio. Na dúvida, `Update` também exige payload completo
(`PUT` semântico) — reenvia a config inteira, revalida tudo. Aceitar update
parcial de uma config de motor reabre a porta do estado incompleto.

**Anti-padrões:**
- Campo que o motor lê marcado opcional no Create "porque o front manda
  depois" → janela de config incompleta persistida.
- Validar só presença e não a coerência (`Max < Min` passa) → motor com
  faixa inválida.
- `oneof` desatualizado em relação ao enum real → valor "válido" pelo DTO
  mas sem destino no motor (ver `lookup-endpoints.md` / `enums`).

**Caminhos de ingestão (bulk/import) reusam a MESMA validação.** Se um
parser de planilha/CSV/batch monta o mesmo Create DTO e o valida, **todo**
campo `required` precisa vir do input — não só os do endpoint interativo. O
único campo legitimamente preenchido pelo backend é o que o backend
**deriva** (ex.: composição de grupo/relacionamento resolvida por lookup);
esse pode ser montado no parser. Os demais (limites, parâmetros do motor)
são colunas de entrada — se a planilha não as tem, ou ela ganha as colunas,
ou as linhas são rejeitadas. **Não** criar um `Validate()` mais frouxo só
para o bulk passar: isso reabre a porta do estado incompleto por uma rota
lateral.

> `required` em `float64`/`int` rejeita o **zero**. Se o zero for um valor
> de negócio legítimo (margem 0%, limite 0), `required` está errado — use
> ponteiro (`*float64` + `required`, distingue ausente de zero) ou valide o
> intervalo (`min=0`) explicitando que zero é aceito mas o campo é
> obrigatório. Decidir conscientemente: "zero é válido aqui?" antes de
> escolher entre `required` e `*T`.

## Separação entity.go / dto.go

| Arquivo | Tags de validação |
|---------|-------------------|
| `entity.go` | ❌ Nenhuma — entidade é dado persistido, não validado na chegada |
| `dto.go` | ✅ `validate:"..."` + método `Validate()` |

A entidade nunca deve ter tags `validate:` — ela representa dado já persistido e confiável.

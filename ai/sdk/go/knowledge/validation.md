# Conhecimento — Validação de DTOs (gofi/base/validator)

## Singleton de pacote

```go
// dto.go
var v = validator.New()
```

Instanciar **uma vez por pacote**, não por request. O validator compila reflection em cache na primeira chamada — instanciar por request desperdiça CPU.

## Método Validate() em DTOs de entrada

Todos os DTOs que chegam via HTTP (Create, Update) devem ter `Validate()`:

```go
func (r CreatePersonRequest) Validate() error {
    return v.ValidateStruct(r)
}
```

DTOs de filtro/paginação não precisam — seus campos são opcionais.

## Tags de validação mais usadas

```go
type CreatePersonRequest struct {
    Name  string `validate:"required"`           // não vazio
    Email string `validate:"required,email"`     // não vazio + formato email
    CPF   string `validate:"required"`           // não vazio
    Age   int    `validate:"required,min=1"`     // não zero + mínimo 1
    Role  string `validate:"oneof=admin user"`   // enum
    URL   string `validate:"url"`                // URL válida
    ID    string `validate:"uuid"`               // UUID v4
}
```

## Onde chamar Validate()

Sempre no **service**, antes de qualquer I/O:

```go
func (s *personService) Create(ctx context.Context, req model.CreatePersonRequest) errs.AppError {
    if err := req.Validate(); err != nil {
        return ErrPersonValidation.WithDetails(err)  // detalhes chegam ao cliente
    }
    // ... apenas aqui acessa o banco
}
```

Nunca no handler — o handler não sabe nada sobre regras de negócio.

## Propagação de erros de validação ao cliente

`WithDetails(err)` + `netx.RespondError` garantem que os erros de campo chegam no body:

```json
HTTP 400
{
  "code": "PERSON_VALIDATION",
  "message": "invalid person data",
  "details": {
    "errors": [
      {"field": "Email", "message": "Email must be a valid email address"},
      {"field": "Age", "message": "Age must be 1 or greater"}
    ]
  }
}
```

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

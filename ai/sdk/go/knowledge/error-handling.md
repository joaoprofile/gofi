---
name: error-handling
description: Fluxo de erro repository → service (errs.AppError) → handler (netx.RespondError) e o mapeamento Kind → HTTP
sdk: v0.8.2
keywords: [errs, AppError, RegisterNotFound, RegisterValidation, RegisterForbidden, RespondError, ErrorResponse, WithDetails, Wrap, status HTTP, not-found]
---

# Conhecimento — Tratamento de Erros (`base/errs` + `netx`)

Import: `github.com/joaoprofile/gofi-sdk-go/base/errs`. Referência completa:
`.claude/sdk/go/api/base-errs.md`.

## Fluxo de erro completo

```
repository (error puro) → service (errs.AppError) → handler (netx.RespondError(w, r, appErr)) → cliente (JSON)
```

## O service nunca retorna `error` puro

```go
// ❌ errado
func (s *entityService) Create(...) error { ... }

// ✅ correto
func (s *entityService) Create(...) errs.AppError { ... }
```

## AppError vazio = sem erro

```go
return errs.AppError{} // equivale a return nil
```

Checagem: `if appErr.Exists()` — `false` quando `Code == ""` e `Err == nil`.

## Registro — todos os erros de um contexto em `errors.go`

```go
// service/errors.go
var (
    ErrEntityNotFound   = errs.RegisterNotFound("ENTITY_NOT_FOUND", "entity not found [%s]")
    ErrEntityConflict   = errs.RegisterConflict("ENTITY_CONFLICT", "entity already exists")
    ErrEntityValidation = errs.RegisterValidation("ENTITY_VALIDATION", "invalid entity data")
    ErrEntityCreate     = errs.RegisterOperation("ENTITY_CREATE_FAILED", "error creating entity")
    // um erro por caso de uso
)
```

| Construtor | Uso |
|---|---|
| `ErrX.New(args...)` | cópia do registrado; `args` formatam a `Message` (`%s`, `%d`) |
| `ErrX.Wrap(err, args...)` | idem + guarda a causa em `Err` (I/O, driver, rede) |
| `ErrX.WithDetails(details)` | anexa `Details` (vai no body) — validação |

`errors.Is(appErr, ErrEntityNotFound)` funciona: `AppError.Is` compara pelo
`Code`, mesmo depois de `New`/`Wrap`. `Unwrap` expõe a causa ao `errors.Is/As`.

## Mapeamento Kind → HTTP

| `errs.Register*` | Kind | HTTP |
|---|---|---|
| `RegisterValidation` | `VALIDATION` | 400 |
| `RegisterUnauthorized` | `UNAUTHORIZED` | 401 |
| `RegisterForbidden` | `FORBIDDEN` | 403 |
| `RegisterNotFound` | `NOT_FOUND` | 404 |
| `RegisterConflict` | `CONFLICT` | 409 |
| `RegisterExternalError` | `EXTERNAL_ERROR` | 502 |
| `RegisterOperation` / `Register` | `OPERATION` / `UNKNOWN` | 500 |

Feito por `netx.RespondError(w, r, appErr)` — assinatura com `*http.Request`
(usa o contexto para logar com trace). Não há outro mapeador; não escreva
`switch` de status no handler.

## Corpo da resposta e a causa

`RespondError` escreve `netx.ErrorResponse`:

```json
{
  "code": 400,
  "message": "invalid entity data",
  "errorCode": "ENTITY_VALIDATION",
  "kind": "VALIDATION",
  "details": { "errors": [ { "field": "Email", "message": "Field 'Email' failed on the 'email' rule" } ] }
}
```

- `code` é o **status HTTP**; o código de negócio vem em `errorCode`.
- A causa (`appErr.Err`) é **sempre logada** e **nunca** vai no body, a menos
  que `netx.ExposeErrorCause = true` (só dev — carrega SQL/driver).
- 5xx loga em Error, 4xx em Warn. O handler **não** loga de novo.

## Propagação de detalhes de validação

```go
return ErrEntityValidation.WithDetails(validationErr) // detalhes chegam ao cliente
```

Não usar `.Wrap(err)` para validação — `Wrap` é para erro de I/O e a causa
não é exposta. Ver `validation.md`.

## Detecção de not found — `FindByID` no service, não `ErrNoRowsAffected`

O repository devolve `nil, nil` quando não acha; o service decide o not-found.
Em escrita, o service chama `FindByID` **antes** do `Update`/`Delete` — o
repository não checa `RowsAffected` (`repository-update-simple.md`):

```go
existing, err := s.repo.FindByID(ctx, id)
if err != nil {
    return ErrEntityUpdate.Wrap(err)
}
if existing == nil {
    return ErrEntityNotFound.New(id)
}
if err := s.repo.Update(ctx, id, req); err != nil {
    return ErrEntityUpdate.Wrap(err)
}
return errs.AppError{}
```

Única exceção: lock otimista (`RowsAffected == 0` = conflito de versão →
`RegisterConflict`).

## Erro fora do fluxo de service

| Situação no handler | Resposta |
|---|---|
| Body/query/path malformado (antes do service) | `netx.Error(w, http.StatusBadRequest, err)` |
| 400 com detalhes montados no handler | `netx.ErrorDetails(w, 400, "msg", details)` |
| Qualquer `errs.AppError` do service | `netx.RespondError(w, r, appErr)` |

## Anti-padrões

- ❌ `netx.RespondError(w, appErr)` — assinatura antiga; hoje é `(w, r, appErr)`.
- ❌ `netx.RespondJSON` / `netx.DecodeJSON` — não existem; use `netx.Response` / `netx.ParseRequestBody`.
- ❌ `errors.New` exportado no service como erro de negócio — registre com `errs.Register*`.
- ❌ Assert de teste por `Message` — compare `appErr.Code` (ou `errors.Is`).

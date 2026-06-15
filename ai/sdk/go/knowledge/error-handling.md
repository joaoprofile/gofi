# Conhecimento — Tratamento de Erros (gofi/base/errs)

## Fluxo de erro completo

```
repository (error puro) → service (AppError) → handler (netx.RespondError) → cliente (JSON)
```

## O service nunca retorna `error` puro

```go
// ❌ errado
func (s *personService) Create(...) error { ... }

// ✅ correto
func (s *personService) Create(...) errs.AppError { ... }
```

## AppError vazio = sem erro

```go
return errs.AppError{}  // equivalente a return nil em funções que retornam error
```

Checagem: `if appErr.Exists()` — retorna false para AppError zero-value.

## Propagação de detalhes de validação

`RespondError` propaga `appErr.Details` no body JSON quando não nil.  
Isso significa que erros de validação chegam ao cliente com os campos inválidos:

```json
{
  "code": "PERSON_VALIDATION",
  "message": "invalid person data",
  "details": {
    "errors": [
      {"field": "email", "message": "email must be a valid email address"}
    ]
  }
}
```

Para isso funcionar, usar `WithDetails(err)`:
```go
return ErrPersonValidation.WithDetails(validationErr)
```

Não usar `.Wrap(err)` para erros de validação — Wrap é para erros de I/O.

## Detecção de not found via ErrNoRowsAffected

O repository declara `ErrNoRowsAffected` e retorna em operações de escrita sem linhas afetadas.  
O service detecta com `errors.Is`:

```go
err := s.repo.Update(ctx, id, req)
if errors.Is(err, repository.ErrNoRowsAffected) {
    return ErrPersonNotFound.New(id)
}
return ErrPersonUpdate.Wrap(err)
```

Para leituras (`FindByID`), o padrão é retornar `nil, nil` quando não encontrado:
```go
if person == nil {
    return nil, ErrPersonNotFound.New(id)
}
```

## Mapeamento Kind → HTTP

| `errs.Register*` | Kind | HTTP Status |
|------------------|------|-------------|
| `RegisterNotFound` | NotFound | 404 |
| `RegisterConflict` | Conflict | 409 |
| `RegisterValidation` | Validation | 400 |
| `RegisterOperation` | Operation | 500 |

Mapeamento feito por `netx.RespondError` automaticamente.

## Todos os erros de um contexto em errors.go

```go
// service/errors.go
var (
    ErrPersonNotFound   = errs.RegisterNotFound(...)
    ErrPersonConflict   = errs.RegisterConflict(...)
    ErrPersonValidation = errs.RegisterValidation(...)
    ErrPersonCreate     = errs.RegisterOperation(...)
    // um erro por caso de uso
)
```

Centralizar permite:
- Referenciar em testes: `assert.Equal(t, ErrPersonCreate.Code, appErr.Code)`
- Documentar todos os casos de erro do contexto em um lugar

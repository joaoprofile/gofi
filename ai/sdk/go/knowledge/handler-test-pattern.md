# Handler Test — Padrão Simples

## Regra

Todo handler deve ter `{contexto}_handler_test.go` cobrindo o óbvio de cada endpoint.

## Stub vs Mock

Handler tests usam **stub** (campos de resultado fixos), não mock com funções como no service test.

```go
// Stub — simples, campos de resultado
type stubPersonService struct {
    createErr      errs.AppError
    getByIDResult  *model.Person
    getByIDErr     errs.AppError
    // um campo por retorno de cada método
}

func (s *stubPersonService) Create(_ context.Context, _ model.CreatePersonRequest) errs.AppError {
    return s.createErr
}
// ... demais métodos da interface
```

## O que cobrir por endpoint

| Tipo de endpoint | Casos obrigatórios |
|-----------------|-------------------|
| POST/PUT com body | happy path, body inválido (400), service error mapeado |
| GET com path param int64 | happy path, ID inválido (400), not found (404) |
| GET com query params | happy path, service error (500) |

**Happy path → verificar apenas o status code.** Não testar corpo da resposta.

## Infraestrutura do teste

```go
// Path params: req.SetPathValue("id", "1")  — Go 1.22+ net/http, compatível com netx.GetPathParam
req := httptest.NewRequest(http.MethodGet, "/api/v1/persons/1", nil)
req.SetPathValue("id", "1")
rr := httptest.NewRecorder()

h.getByID(rr, req)
assert.Equal(t, http.StatusOK, rr.Code)
```

## Mapeamento de status esperado

| AppError kind | Status HTTP |
|--------------|-------------|
| `ErrXxxNotFound.New()` | 404 |
| `ErrXxxConflict.New()` | 409 |
| `ErrXxxValidation.New()` | 400 |
| `ErrXxxCreate/Update/Query.New()` | 500 |

## O que NÃO testar no handler

- Corpo da resposta JSON (responsabilidade do service test)
- Regras de negócio (idem)
- Integração com banco (sem banco nos testes de handler)

---
name: handler-test-pattern
description: Teste de handler netx — stub de service com campos de resultado, httptest, só status code; claims injetadas pelo helper do middleware de auth
sdk: v0.8.2
keywords: [handler test, httptest, stub, SetPathValue, status code, WithClaims, RespondError]
---

# Handler Test — Padrão Simples

## Regra

Todo handler tem `{contexto}_handler_test.go` cobrindo o óbvio de cada endpoint.
Esqueleto completo: `.claude/sdk/go/boilerplates/handler-test.md`.

## Stub vs Mock

Handler test usa **stub** (campos de resultado fixos), não mock com funções
como no service test.

```go
type stubEntityService struct {
    createErr     errs.AppError
    getByIDResult *model.Entity
    getByIDErr    errs.AppError
    // um campo por retorno de cada método
}

func (s *stubEntityService) Create(_ context.Context, _ string, _ model.CreateEntityRequest) errs.AppError {
    return s.createErr
}
// ... demais métodos da interface
```

## O que cobrir por endpoint

| Tipo de endpoint | Casos obrigatórios |
|---|---|
| POST/PUT com body | happy path, body inválido (400), erro do service mapeado |
| GET com path param | happy path, param inválido (400) se o handler converte, not found (404) |
| GET com query params | happy path, erro do service (500) |
| Rota com gate de permissão | sem claims (401), permissão negada (403) |

**Happy path → verificar apenas o status code.** Não testar corpo da resposta.

## Infraestrutura do teste

Chame o método do handler direto — sem servidor, sem roteador:

```go
req := httptest.NewRequest(http.MethodGet, "/v1/entities/1", nil)
req.SetPathValue("id", "1") // netx.GetPathParam lê r.PathValue
rr := httptest.NewRecorder()

h.getByID(rr, req)
assert.Equal(t, http.StatusOK, rr.Code)
```

- `netx.RespondError(w, r, appErr)` precisa do `*http.Request` — passe o `req`
  do teste (nunca `nil` no código de produção).
- Chamar o método direto **não** passa pelo middleware de auth (`UseAuth` só
  embrulha rotas `netx.PrivateRoutes` no servidor). Handler que lê claims
  recebe-as pelo helper do middleware de auth:
  `req = req.WithContext(authhandler.WithClaims(req.Context(), &types.Claims{...}))`
  — ver `http-auth-middleware.md` §"Claims no contexto".
- O contrato de autenticação (401 sem token, token revogado) é testado **uma
  vez** no pacote do middleware, não em cada handler.

## Mapeamento de status esperado

| AppError (kind) | Status HTTP |
|---|---|
| `ErrXxxValidation.New()` | 400 |
| `ErrXxxUnauthorized.New()` | 401 |
| `ErrXxxForbidden.New()` | 403 |
| `ErrXxxNotFound.New()` | 404 |
| `ErrXxxConflict.New()` | 409 |
| `ErrXxxExternal.New()` | 502 |
| `ErrXxxCreate/Update/Query.New()` (operation) | 500 |

## O que NÃO testar no handler

- Corpo da resposta JSON (responsabilidade do service test)
- Regras de negócio (idem)
- Integração com banco (sem banco nos testes de handler)
- Validação de assinatura/sessão do token (é do middleware)

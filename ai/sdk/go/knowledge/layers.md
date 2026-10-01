---
name: layers
description: Responsabilidade de cada camada (handler, service, repository, adapter), fluxo de request e de erro entre camadas, testabilidade com mocks handcraft
sdk: v0.8.2
keywords: [camadas, handler, service, repository, adapter, errs.AppError, netx.RespondError, netx.Response, fluxo, mocks]
---

# Camadas de um Contexto — Go

## Responsabilidades

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Handler** | Parse request → chamar service → traduzir resposta (`netx.Response` / `netx.RespondError`) | conter lógica de negócio; conhecer SQL; acessar repository diretamente |
| **Service** | Validar DTO → operar via repository → retornar `errs.AppError` | conhecer `http.ResponseWriter`/`http.Request`; manipular SQL; retornar `error` puro |
| **Repository** | Executar SQL via `sqln` → retornar entidade ou `error` puro | depender de DTOs; conter regra de negócio |
| **Adapter** | Bridge entre SDK externo (IAM, mensageria, bucket) e domain model | conter regra de negócio do domínio |

Nenhuma camada lê ambiente, abre conexão ou encerra o processo: recebe tudo
pelo construtor, montado em `wire.go` depois do `Build`
(`service-bootstrap.md`).

## Fluxo típico de request

```
[Cliente HTTP]
    ↓
Handler.create(w, r)
    ↓ parse do body → CreateUserRequest
    ↓ req.Validate() (DTO)
    ↓ svc.Create(ctx, req)
        ↓ Service: regras de negócio (existência, invariantes)
        ↓ repo.Save(ctx, model.User)
            ↓ Repository: INSERT ... RETURNING
        ↑ retorna *model.User, error
    ↑ retorna *model.User, errs.AppError
    ↓ if appErr.Exists() → netx.RespondError(w, r, appErr)
    ↓ senão → netx.Response(w, http.StatusCreated, user)
```

## Erros entre camadas

- **Repository** retorna `error` puro (incluindo `nil, nil` para not-found em
  `FindByID`).
- **Service** converte `error` em `errs.AppError`:
  - `nil` → `errs.AppError{}` (sucesso)
  - `nil, nil` em `FindByID` → `ErrXxxNotFound.New(id)` no service
  - outros → `ErrXxxAction.Wrap(err)`
- **Handler** chama `appErr.Exists()` e responde com
  `netx.RespondError(w, r, appErr)`, que mapeia o `Kind` para o status
  (validação 400, não autorizado 401, proibido 403, não encontrado 404,
  conflito 409, externo 502, demais 500) e loga a causa. 401/403 nascem de
  erros registrados com `errs.RegisterUnauthorized`/`errs.RegisterForbidden`.
  Tabela completa: `error-handling.md`.
- `netx.Error(w, status, err)` só para erro que não é `AppError` (ex.:
  middleware de borda).
- Sucesso → `netx.Response(w, status, data)`.

## Testabilidade

- Service recebe **interface** de repository (não a struct concreta)
- Handler recebe **interface** de service
- Mocks são **handcraft** — sem frameworks de mock externos
  - Mock de repository: usa fn fields (`saveFn func(...)`)
  - Stub de service: usa campos de resultado (`createErr errs.AppError`)
- Mock implementa **todos** os métodos da interface, incluindo `Close() error { return nil }` quando o repository o expõe

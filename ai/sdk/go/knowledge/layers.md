# Camadas de um Contexto — Go

## Responsabilidades

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Handler** | Parse request → chamar service → traduzir resposta (`netx.Response` ou `netx.RespondError`/`netx.Error`) | conter lógica de negócio; conhecer SQL; acessar repository diretamente |
| **Service** | Validar DTO → operar via repository → retornar `errs.AppError` | conhecer `http.ResponseWriter`/`http.Request`; manipular SQL; retornar `error` puro |
| **Repository** | Executar SQL → retornar entidade ou `error` puro | depender de DTOs; conter regra de negócio |
| **Adapter** | Bridge entre SDK externo (IAM, mensageria) e domain model | conter regra de negócio do domínio |

## Fluxo típico de request

```
[Cliente HTTP]
    ↓
Handler.create(w, r)
    ↓ ParseRequestBody → CreateUserRequest
    ↓ req.Validate() (DTO)
    ↓ svc.Create(ctx, req)
        ↓ Service: regras de negócio (existência, invariantes)
        ↓ repo.Save(ctx, model.User)
            ↓ Repository: INSERT ... RETURNING
        ↑ retorna *model.User, error
    ↑ retorna *model.User, errs.AppError
    ↓ if appErr.Exists() → netx.RespondError ou netx.Error
    ↓ senão → netx.Response(w, 201, user)
```

## Erros entre camadas

- **Repository** retorna `error` puro (incluindo `nil, nil` para not-found em `FindByID`).
- **Service** converte `error` em `errs.AppError`:
  - `nil` → `errs.AppError{}` (sucesso)
  - `nil, nil` em `FindByID` → `ErrXxxNotFound.New(id)` no service
  - outros → `ErrXxxAction.Wrap(err)`
- **Handler** chama `appErr.Exists()` e roteia:
  - `IsNotFound()` / `IsConflict()` / `IsValidation()` → `netx.RespondError(w, appErr)` (mapeia automaticamente)
  - 401/403 → `netx.Error(w, http.StatusUnauthorized, err)` **sempre explícito**
  - sucesso → `netx.Response(w, status, data)`

## Testabilidade

- Service recebe **interface** de repository (não a struct concreta)
- Handler recebe **interface** de service
- Mocks são **handcraft** — sem frameworks de mock externos
  - Mock de repository: usa fn fields (`saveFn func(...)`)
  - Stub de service: usa campos de resultado (`createErr errs.AppError`)
- Mock implementa **todos** os métodos da interface, incluindo `Close() error { return nil }`

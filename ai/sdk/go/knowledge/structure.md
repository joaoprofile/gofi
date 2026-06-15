# Estrutura de Contexto — Go

Layout obrigatório para um contexto de domínio em projetos gofi/Go.

## Diretórios

```
{pathService}                 ← ex: ./src/   (raiz do módulo Go)
  go.mod
  go.work                      ← na raiz do projeto, aponta pra ./src
  .migrations/
  {projectName}/               ← ex: ./src/web-api/   (pathCmd — main.go aqui)
    main.go
  domain/
    {contexto}/                ← pathContext = ./src/domain/{contexto}/
      model/
        entity.go              — struct com tags db:"" e json:""
        dto.go                 — DTOs com tags validate:"" + Validate()
        query_dto.go           — apenas se filtro dinâmico (ver dynamic-filter.md)
      service/
        errors.go              — todos os errs.Register* do contexto
        {contexto}_service.go  — interface + implementação
        {contexto}_service_test.go
      repository/
        {contexto}_repository.go   ← arquivo ÚNICO: interface + SQL + implementação
      adapter/                 ← apenas quando integra com SDK externo (IAM, etc.)
        iam_adapter.go         — UserIAMAdapter, UserTenantAdapter, WithTenantID
      handler/
        middleware.go          — apenas se o contexto gerencia autenticação
        {contexto}_handler.go
        {contexto}_handler_test.go
        auth_handler.go        — apenas se o contexto gerencia autenticação
```

## Regras de posicionamento

| Arquivo | Onde fica | Onde NÃO fica |
|---------|-----------|---------------|
| `go.mod` | `pathService` (`./src/`) | em `pathCmd` |
| `go.work` | raiz do projeto, `use ./src` | apontando pra `./src/{projectName}` |
| `main.go` | `pathCmd` (`./src/{projectName}/`) | na raiz de `pathService` |
| `domain/` | `pathService` (`./src/domain/`) | dentro de `pathCmd` |
| `.migrations/` | `pathService` (`./src/.migrations/`) | em `pathCmd` |
| Replace paths no `go.mod` | `../gofi` | qualquer outro path |

## Regras de arquivo único

- `repository/` tem **um único arquivo** `{contexto}_repository.go` que contém:
  - Interface `{Contexto}Repository`
  - Constantes SQL
  - Struct concreta `{contexto}Repository`
  - Constructor `New{Contexto}Repository(ctx)`
  - Implementação dos métodos
- Adapters/factories que fazem bridge com SDKs externos vão em `adapter/`,
  **nunca** dentro de `repository/`.

## Variáveis de path lidas pelos agents

| Variável | Valor padrão |
|----------|--------------|
| `pathService` | `./src/` |
| `projectName` | nome do binário Go (ex: `web-api`) |
| `pathCmd` | `./src/{projectName}/` |
| `pathContext` | `./src/domain/{contexto}/` |
| `pathSpec` | `./specs/` |
| `pathPrd` | `./prd/` |

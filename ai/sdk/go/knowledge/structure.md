---
name: structure
description: Layout físico de um serviço Go com gofi — go.work com os módulos do SDK, pathCmd (main/config/wire/workers), domain/{contexto} por camada, arquivo único de repository e split de service CRUD/Auth
sdk: v0.8.2
keywords: [estrutura, layout, go.work, go.mod, pathService, pathCmd, pathContext, domain, repository, adapter, handler, main.go, config.go, wire.go, .migrations, split service]
---

# Estrutura de Contexto — Go

Layout obrigatório para um contexto de domínio em projetos gofi/Go.

## Diretórios

```
go.work                        ← raiz do projeto: use ./src + módulos do SDK em ./.gofi/gofi-sdk-go/...
.gofi/gofi-sdk-go/             ← checkout do SDK na versão fixada (não editar)
{pathService}                  ← ex: ./src/   (raiz do módulo Go)
  go.mod                       ← require github.com/gofi-labs/gofi-sdk-go/<módulo> (um por módulo usado)
  .env                         ← só desenvolvimento; nunca com segredo real no git
  .migrations/
  {projectName}/               ← pathCmd, ex: ./src/web-api/ — package main
    main.go                    — main + run(): config → componentes → Build → wiring → ListenAndServe
    config.go                  — Config + LoadConfig (único arquivo que lê ambiente)
    wire.go                    — build{Xxx}: repository → service → handler
    iam.go                     — apenas se o serviço tem IAM
    {worker}_cron.go           — um componente por worker (worker-bootstrap.md)
    {topic}_consumer.go        — um componente por consumer (consumer-bootstrap.md)
    config_test.go
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
        auth_service.go        — apenas no split CRUD/Auth (abaixo)
      repository/
        {contexto}_repository.go   ← arquivo ÚNICO: interface + SQL + implementação
      adapter/                 ← apenas quando integra com SDK externo (IAM, etc.)
        iam_adapter.go         — UserIAMAdapter, UserTenantAdapter, WithTenantID
      observability/           ← apenas se o contexto tem métricas próprias (observability-otel.md)
      handler/
        middleware.go          — apenas se o contexto gerencia autenticação
        {contexto}_handler.go
        {contexto}_handler_test.go
        auth_handler.go        — apenas se o contexto gerencia autenticação
```

## Módulos e imports do SDK

- O SDK é **multi-módulo**: cada biblioteca, provider e componente é importado
  pelo caminho `github.com/gofi-labs/gofi-sdk-go/<módulo>` (`gofi`,
  `gofi/component/database`, `sqln`, `netx`, `msq`, `msq/provider/kafka`,
  `obs/logging`, `base/errs`, `iam`…). Lista: `.claude/sdk/go/api/INDEX.md`.
- `go.work` na raiz aponta para `./src` e para cada módulo do checkout em
  `.gofi/gofi-sdk-go/` — mantido pelo gofi; não editar essas entradas à mão.
- Sem `replace` no `go.mod` apontando para cópia local do SDK.
- Driver SQL e provider de mensageria/bucket/segredo entram por **import em
  branco no `main.go`** — nunca em pacote de domínio.

## Regras de posicionamento

| Arquivo | Onde fica | Onde NÃO fica |
|---------|-----------|---------------|
| `go.mod` | `pathService` (`./src/`) | em `pathCmd` |
| `go.work` | raiz do projeto | dentro de `pathService` |
| `main.go`, `config.go`, `wire.go`, workers | `pathCmd` (`./src/{projectName}/`) | na raiz de `pathService`; em `internal/bootstrap/` |
| `domain/` | `pathService` (`./src/domain/`) | dentro de `pathCmd` |
| `.migrations/` | `pathService` | em `pathCmd` |

`DATABASE_MIGRATION=true` lê `.migrations` **relativo ao diretório de
trabalho** do processo, e o `.env` é procurado subindo a partir dele: rode o
binário (e o `go run`) a partir de `pathService` ou copie `.migrations` para o
`WORKDIR` da imagem.

## Regras de arquivo único

- `repository/` tem **um único arquivo** `{contexto}_repository.go` com
  interface `{Contexto}Repository`, constantes SQL, struct concreta
  `{contexto}Repository`, constructor `New{Contexto}Repository` e métodos.
- Adapters/factories que fazem bridge com SDKs externos vão em `adapter/`,
  **nunca** dentro de `repository/`.

## Split de service por responsabilidade

Contexto que mistura CRUD com auth (login, OAuth, refresh, logout,
change/reset password, GetMe):

```
service/
  errors.go                  — único para o contexto
  {contexto}_service.go      — interface única + struct + New{Contexto}Service + CRUD
  auth_service.go            — métodos auth no MESMO *{contexto}Service + IAMSession + AuthInfra + OAuth
  {contexto}_service_test.go — mocks compartilhados (package scope)
  auth_service_test.go
handler/
  {contexto}_handler.go
  auth_handler.go            — espelha auth_service.go
```

Regra completa: `service-bootstrap.md` § Service split. Esqueleto:
`boilerplates/service.md` § "Variante — Service com split CRUD + Auth/IAM".

## Variáveis de path lidas pelos agents

| Variável | Valor padrão |
|----------|--------------|
| `pathService` | `./src/` |
| `projectName` | nome do binário Go (ex: `web-api`) |
| `pathCmd` | `./src/{projectName}/` |
| `pathContext` | `./src/domain/{contexto}/` |
| `pathSpec` | `./specs/` |
| `pathPrd` | `./prd/` |

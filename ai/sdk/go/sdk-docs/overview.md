# gofi SDK — Overview

## Estrutura do Monorepo

```
gofi/
  base/      — validator, erros estruturados (AppError)
  netx/      — HTTP server, middleware, response helpers
  sqln/      — SQL: statements, criteria builder, paginação, migrations
  obs/       — observabilidade: logging OTLP, tracing, métricas
  iam/       — autenticação JWT, sessão, RBAC, multi-tenancy, IDPs sociais
  msq/       — mensageria: Kafka, RabbitMQ, SQS, OCI, Redis
  gofi/      — módulo raiz: builder de aplicação, wiring de todos os módulos
```

Todos os módulos são referenciados como `gofi/{modulo}`:
- `github.com/joaoprofile/gofi/base`
- `github.com/joaoprofile/gofi/netx`
- `github.com/joaoprofile/gofi/sqln`
- `github.com/joaoprofile/gofi/obs`
- `github.com/joaoprofile/gofi/iam`
- `github.com/joaoprofile/gofi/msq`

## Dependências Comuns por Camada

| Camada     | Imports gofi típicos |
|------------|----------------------|
| model/entity | `gofi/sqln` (Page[T]) |
| model/dto  | `gofi/base/validator` |
| service    | `gofi/base/errs` |
| repository | `gofi/sqln`, `gofi/sqln/criteria`, `gofi/obs/logging` |
| handler    | `gofi/netx`, `gofi/base/errs` |
| main       | `gofi/netx`, `gofi/sqln`, `gofi/obs/logging` |

## go.work

O monorepo usa `go.work`. Cada módulo tem seu próprio `go.mod`.  
Exemplos adicionam `replace` directives apontando para `../../{modulo}`.

## Arquivos de Referência

- SDK detalhado: `.claude/sdk/<lang>/sdk-docs/{modulo}.md`
- Boilerplates: `.claude/sdk/<lang>/boilerplates/`
- Exemplos completos: `.claude/sdk/<lang>/boilerplates/`

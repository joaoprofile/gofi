---
name: service-bootstrap
description: Composition root do serviço Go — ordem do main (config → componentes → Build → wiring → ListenAndServe), split main/config/wire/iam, zero os.Getenv fora de config.go, workers como componentes, split de service CRUD vs Auth
sdk: v0.8.2
keywords: [main.go, bootstrap, composition root, config.go, wire.go, iam.go, LoadConfig, run, Build, ListenAndServe, os.Getenv, workers, service split, auth_service]
---

# Regras — service e bootstrap do serviço

Mecânica do orquestrador: `gofi-orchestrator.md`. Ambiente e segredos:
`configuration.md`. Esqueleto pronto: `.claude/sdk/go/boilerplates/main.md`.

## Service split CRUD vs Auth/IAM (cross-language)

Quando o contexto mistura CRUD com auth (login, OAuth, refresh, logout,
change/reset password, GetMe), `service/` tem **dois arquivos**, **uma
interface** e **um struct/constructor**: `{contexto}_service.go` (interface +
struct + `New{Contexto}Service` + métodos CRUD) e `auth_service.go` (métodos
auth no **mesmo** `*{contexto}Service` + interface `IAMSession` + struct
`AuthInfra` + constantes/helpers de OAuth). `errors.go` continua **único**.
Testes espelham: `auth_service_test.go` reusa o mock de repositório de
`{contexto}_service_test.go` (package scope). O split do service **espelha** o
do handler (`auth_handler.go` ↔ `{contexto}_handler.go`) — um existe, o outro
também. Árvore em `structure.md` §"Split de service por responsabilidade";
esqueleto em `boilerplates/service.md` §"Variante — Service com split CRUD +
Auth/IAM".

## O `main` — ordem fixa

```
environment.Instance() → LoadConfig → declarar componentes → Build
    → wiring (repo → service → handler) → http.Handlers(...) → ListenAndServe
```

1. **Config primeiro.** `LoadConfig(ctx, environment.Instance())` monta o
   `Config` do serviço (variáveis próprias + helpers do SDK). Erro → sai antes
   de abrir qualquer coisa.
2. **Declarar componentes** com o que a config decide (`AllowedOrigins`,
   workers, consumers). Workers e consumers são componentes e entram aqui
   (`worker-bootstrap.md`, `consumer-bootstrap.md`).
3. **`Build`** abre tudo. Erro → sai (o gofi já desfez o que abriu).
4. **Wiring depois do `Build`** — banco global, broker e IAM só existem a
   partir daqui. Handlers entram com `http.Handlers(...)` (o servidor só sobe
   no `ListenAndServe`).
5. Erro no wiring → `svc.Shutdown` **antes** de sair.
6. `ListenAndServe` bloqueia até o sinal e fecha tudo.

**Só o `main` encerra o processo** — e só com `log.Fatal` sobre o erro
devolvido por `run()`. Nenhum outro arquivo (nem `config.go`, `wire.go`,
construtor de repositório, worker) chama `log.Fatal`, `logging.Fatal`,
`os.Exit` ou `panic` para erro de config/boot: devolve `error`.

```go
func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```

## Split do bootstrap — mesmo `package main`, arquivos por responsabilidade

| Arquivo em `pathCmd` | Conteúdo |
|---|---|
| `main.go` | `main` + `run()`: a ordem acima, nada mais |
| `config.go` | `type Config` + `LoadConfig(ctx, env) (Config, error)` — **único** arquivo do projeto que lê ambiente |
| `wire.go` | `build{Contexto}Handler(ctx, …)`, `build{Contexto}Service(ctx, …)`: repo → service → handler; recebe dependências por parâmetro, não lê env nem global |
| `iam.go` | quando há IAM: `newIdentity(cfg) *iam.Component` com ports/RBAC |
| `{worker}_cron.go` / `{topic}_consumer.go` | um componente por worker/consumer |
| `config_test.go` | env obrigatório, defaults e overrides (`t.Setenv` + `environment.ResetForTesting()`) |

Gatilho do split: providers de auth/OAuth, adapters de SDK, workers, ou
`run()` passando de ~40 linhas úteis. **Não** criar `internal/bootstrap/`.

## Zero `os.Getenv` fora de `config.go`

- Variável **do SDK** (`DATABASE_*`, `CACHE_*`, `MESSAGING_*`, `OTEL_*`,
  `JWT_*`…) é lida pelo componente/adapter — o projeto **não** a relê nem a
  redeclara no `Config`. Precisa do valor? Helper do `Environment`
  (`env.HTTP().AllowedOrigins`, `env.Auth().AccessTokenTTL`,
  `env.OAuth().Google`) dentro de `config.go`.
- Variável **do serviço** → campo com tag `env` no `Config`, preenchido por
  `common.ParseStructAnnotationFunc` (resolve `secret://`) — ver
  `configuration.md` § `Config` do projeto.
- Handler/service/repository/adapter/worker recebem valores prontos pelo
  construtor.
- Nomes: `env-vars-standard.md` (`OAUTH_GOOGLE_*`, não `GOOGLE_*`;
  `ALLOWED_ORIGINS`, não `CORS_*`).

## IAM no bootstrap

O componente `iam` (`github.com/gofi-labs/gofi-sdk-go/gofi/component/iam`)
lê `JWT_*`, `*_TOKEN_TTL`, `OAUTH_GOOGLE_*` e sessões Redis (`CACHE_TYPE=redis`)
e falha o `Build` sem `JWT_SECRET`. O projeto só fornece o que o ambiente não
tem: `User`, `Tenant`, `RBAC`, `OnEvent`.

```go
func newIdentity(cfg Config) *iam.Component {
    users := buildUserIAMAdapter() // repository constructors do not touch the database
    return iam.New(iam.Config{
        User:    users,
        Tenant:  users,
        RBAC:    roles.NewRBACProvider(roles.Config{Permissions: permissions}),
        OnEvent: logIAMEvent,
        // Configure adjusts what config.IAM read from the environment, inside Build.
        Configure: func(c *iamconfig.DefaultConfig) {
            c.Security.RequireTenantTicket = true
        },
    })
}
```

`identity.Service()` (depois do `Build`) alimenta middleware e handlers de
auth. Adapter de ports: `iam-adapter-pattern.md`.

## Workers e consumers

Todo trabalho de longa duração é **componente declarado no `With`**, dono do
próprio ciclo de vida — `main.go` não chama método dele. Cron/ticker/listener:
`worker-bootstrap.md` (`gofi.Runner`). Consumer: `consumer-bootstrap.md`.
Agendamento, fuso e réplicas: `cronjob.md`.

## Anti-padrões

- ❌ Construtor de repositório/serviço que acessa o banco (quebra quando
  montado antes do `Build`) — ver `database-connection.md`.
- ❌ `log.Fatal`/`logging.Fatal` entre `Build` e `ListenAndServe` — pula o
  `Shutdown` (recursos abertos, logs sem flush). Devolva o erro para `run()`.
- ❌ `os.Getenv` em `wire.go`, handler, service, repository, adapter ou worker.
- ❌ Redeclarar no `Config` campo que o SDK já lê (`JWTSecret`, `DatabaseHost`…).
- ❌ `go worker.Loop(ctx)` / `defer consumer.Close()` no `main`.
- ❌ Builder em `wire.go` que busca dependência global em vez de receber por
  parâmetro.

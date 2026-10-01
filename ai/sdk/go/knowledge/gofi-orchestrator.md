---
name: gofi-orchestrator
description: Orquestrador gofi — gofi.New/With/Build/ListenAndServe/Shutdown, componentes por estágio, Runner, Runtime e componente próprio
sdk: v0.8.2
keywords: [gofi.New, With, Build, ListenAndServe, Shutdown, Component, Runner, Stage, Runtime, OnClose, AddHealthCheck, Shared, componentes, ciclo de vida]
---

# Orquestrador `gofi` — declarar, construir, servir

API: `.claude/sdk/go/api/gofi.md` e `gofi-component-*.md`. Programas completos
em `.claude/sdk/go/api/examples.md`: `netx/api` (HTTP), `sqln/search` (job sem
HTTP), `obs` (Runners próprios), `msq/*/consumer` (consumer sem HTTP),
`iam/login` (IAM + rotas registradas depois do `Build`).

Import: `github.com/gofi-labs/gofi-sdk-go/gofi` e
`github.com/gofi-labs/gofi-sdk-go/gofi/component/<nome>`.

## Ciclo de vida

```go
svc, err := gofi.New("{servico}").
    With(database.New(), httpserver.New(":8080")).
    Build()
if err != nil {
    return err
}
return svc.ListenAndServe()
```

| Passo | O que faz |
|---|---|
| `gofi.New(name)` | Só cria o builder — **sem efeito colateral**. `name` sobrescreve `APP_NAME` (vira `service.name` em log e telemetria). |
| `With(c...)` | Só declara. A ordem **não importa**: `Build` ordena por `Stage`. Componente `nil` vira erro no `Build`. |
| `Build()` | Carrega o ambiente (`environment.Instance()`), aplica `TIMEZONE` (vazio = UTC), inicia o logging (`LOG_LEVEL`, `APP_ENVIRONMENT`) e `TLS_INSECURE_SKIP_VERIFY`; depois dá `Start` em cada componente por estágio. Devolve **todos** os erros de declaração/ambiente juntos; se um `Start` falha, fecha o que já abriu e devolve `Service` nil. Chamar duas vezes é erro. |
| `ListenAndServe()` | Roda os `Runner` (servidor HTTP e os seus) e bloqueia até SIGINT/SIGTERM, `Shutdown` ou o retorno de qualquer Runner. Para todos os Runners, fecha os recursos em ordem **reversa** (timeout 30s) e faz flush dos logs. Sem Runner, só espera o sinal. `nil` em parada limpa. |
| `Shutdown(ctx)` | Para um `ListenAndServe` em curso; sem ele (job), fecha os recursos direto. |

**Orquestrador e componentes não chamam `log.Fatal`/`os.Exit`** — todo erro
volta para o `main`, único lugar que encerra o processo (ver
`service-bootstrap.md`). Exceções do SDK a conhecer: `logging.Fatal` faz
`os.Exit(1)` (o projeto não usa) e `cronjob.ScheduleJob` panica com config
inválida (`cronjob.md`).

## Componentes

| Pacote `gofi/component/…` | Pelo ambiente (gofi abre e fecha) | Injetado (chamador é dono) | Handle | `Stage` |
|---|---|---|---|---|
| `observability` | `observability.New()` (`OTEL_*`) | `observability.FromTelemetry(t)` | `Telemetry()` | `StageObservability` 100 |
| `database` | `database.New()` + `_ ".../sqln/driver/<nome>"` | `database.FromDB(db)` | `DB()`, `ReadDB()` | `StageDatabase` 200 |
| `cache` | `cache.New()` (`CACHE_*`) | `cache.FromClient(rdb)` | `Client()` | `StageCache` 300 |
| `session` | `session.New(cfg...)` | — | `session.Instance()` (`base/session`) | `StageSession` 400 |
| `messaging` | `messaging.New(msq.Config...)` + `_ ".../msq/provider/<nome>"` | `messaging.FromBroker(b)` | `Broker()` | `StageMessaging` 500 |
| `iam` | `iam.New(iam.Config...)` (`JWT_*`, `*_TOKEN_TTL`, `OAUTH_GOOGLE_*`) | `iam.FromService(svc)` | `Service()` | `StageIAM` 600 |
| `httpserver` | `httpserver.New(port, cfg...)` | `httpserver.FromServer(s)` | `Server()`; `.Handlers/.Use/.UseAuth` | `StageServer` 900 |

Regras:

- **Handle é `nil` antes do `Build`.** `db.DB()`, `mq.Broker()`,
  `identity.Service()`, `rdb.Client()` só depois do `Build` — ou dentro do
  `Start` de um componente de estágio maior.
- **Só entra no binário o que é importado.** Sem `observability` não há gRPC
  nem SDK OpenTelemetry; sem `database`, nenhum driver SQL. Driver e provider
  de mensageria são registrados por import em branco no `main`; sem o import,
  `Build` falha nomeando o que falta.
- **`From*` = o chamador é dono.** gofi usa, registra health check/métricas,
  mas não fecha.
- `cache.New()` com Redis inacessível **falha o `Build`** (não degrada em
  silêncio).
- `httpserver`: o servidor vive em memória até `ListenAndServe`, então
  `Handlers`/`Use`/`UseAuth` podem ser chamados **depois** do `Build` (rotas que
  dependem de handle já iniciado). `UseAuth` antes de `Handlers`: a rota pega o
  middleware de auth no momento em que é adicionada.
- Health: `httpserver.New(":8080", &netx.WSConfig{Health: &netx.HealthConfig{}})`
  serve `/livez` e `/readyz` com os checks registrados pelos componentes
  iniciados **antes** do servidor (database, réplica, cache).

## Job (sem HTTP)

Sem `ListenAndServe`: `Build` → trabalho → `Shutdown` (fecha banco/broker e
faz flush dos logs). Ver `examples/sqln/search/main.go`.

```go
svc, err := gofi.New("{job}").With(database.New()).Build()
if err != nil {
    return err
}
runErr := run(ctx)
return errors.Join(runErr, svc.Shutdown(context.Background()))
```

## Componente próprio

Qualquer tipo com `Name() string`, `Stage() gofi.Stage` e
`Start(ctx, *gofi.Runtime) error` é um `gofi.Component`. Em `Start`:

- `rt.OnClose(fn)` — registra o que abriu; roda no `Shutdown` **ou** quando um
  componente posterior falha no `Build`. Ordem reversa de registro.
- `rt.AddHealthCheck(name, fn)` — entra no `/readyz` se o componente iniciar
  antes do `httpserver` (estágio < `gofi.StageServer`).
- `rt.Env()` — o `*environment.Environment` carregado pelo `Build`.
- `rt.Shared(key, open)` — um recurso compartilhado por vários componentes
  (`cache.Shared(rt)` devolve o Redis do Build).

`Stage` é `int`: um valor entre as constantes (ex.: `gofi.StageMessaging + 10`)
posiciona o componente depois do que ele usa. Empate = ordem de declaração.

## Runner — trabalho de longa duração

`gofi.Runner` = `Component` + `Run() error` (bloqueia até parar; `nil` em
parada limpa) + `Stop(ctx) error` (faz `Run` retornar; idempotente).
`ListenAndServe` roda todos em paralelo e **para todos quando qualquer um
retorna** — `Run` que retorna cedo derruba o serviço. Adapter canônico
(`examples/obs/runner.go`):

```go
type runner struct {
    name   string
    loop   func(context.Context)
    ctx    context.Context
    cancel context.CancelFunc
    done   chan struct{}
}

func newRunner(name string, loop func(context.Context)) *runner {
    ctx, cancel := context.WithCancel(context.Background())
    return &runner{name: name, loop: loop, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

func (r *runner) Name() string                               { return r.name }
func (r *runner) Stage() gofi.Stage                          { return gofi.StageServer }
func (r *runner) Start(context.Context, *gofi.Runtime) error { return nil }

func (r *runner) Run() error {
    defer close(r.done)
    r.loop(r.ctx)
    return nil
}

func (r *runner) Stop(ctx context.Context) error {
    r.cancel()
    select {
    case <-r.done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

Os Runners param **antes** de os recursos fecharem: o loop termina com banco,
cache e broker ainda abertos. Padrão de worker do projeto em
`worker-bootstrap.md`; cron em `cronjob.md`.

## Anti-padrões

- ❌ Usar handle (`DB()`, `Broker()`, `Service()`) antes do `Build`.
- ❌ `log.Fatal` entre `Build` e `ListenAndServe` sem `Shutdown` — pula o
  fechamento e o flush de logs/telemetria.
- ❌ `Runner.Run` que devolve na hora (dispara goroutine e retorna) — encerra o
  serviço inteiro.
- ❌ Fechar à mão o que o gofi abriu (`database.New`, `messaging.New`…) ou
  esperar que ele feche o que entrou por `From*`.
- ❌ Esquecer o import em branco do driver SQL / provider de mensageria.

---
name: worker-bootstrap
description: Background worker (cron, ticker, listener) como gofi.Runner dono do próprio ciclo de vida — main só declara no With; Start monta dependências, Run bloqueia, Stop drena
sdk: v0.8.2
keywords: [worker, cron, cronjob, Runner, gofi.Runner, background, ciclo de vida, Start, Run, Stop, feature flag, multi-worker, timezone, tzdata]
---

# Background worker bootstrap — o worker é um `gofi.Runner`

> **Quando aplicar:** qualquer estrutura em `pathCmd` que represente trabalho
> de longa duração: cron, loop com ticker, listener, watcher. Consumer de
> mensageria tem página própria (`consumer-bootstrap.md`). Mecânica do
> orquestrador: `gofi-orchestrator.md` § Runner. API de agendamento:
> `cronjob.md`. Exemplo executável: `examples/obs` (`runner.go`,
> `job/archive.go`) — `.claude/sdk/go/api/examples.md`.

## Regra inviolável

**O wrapper do worker é dono do próprio ciclo de vida e implementa
`gofi.Runner`.** O gofi chama `Start` no `Build` (depois de banco, cache e
broker), roda `Run` no `ListenAndServe` e chama `Stop` no shutdown —
**antes** de fechar os recursos. O `main.go` só **declara** o worker:

```go
svc, err := gofi.New("{servico}").
    With(
        database.New(),
        newReportCron(cfg),   // the whole worker is this line
        httpserver.New(":8080"),
    ).
    Build()
```

Nada de `go worker.Loop(ctx)`, `worker.Schedule(...)`, `worker.Start(...)`,
`defer worker.Close()` no `main`. **Cada chamada a método do worker no
`main.go` é red flag** — o ciclo de vida vazou e o shutdown não espera o
worker (banco fecha com job no meio).

| Método | Responsabilidade |
|---|---|
| `Start(ctx, rt)` | Monta dependências (banco já aberto), valida config (agenda, fuso), pega recursos compartilhados (`cache.Shared(rt)`). Erro aqui falha o `Build`. **Não** executa trabalho longo — bloqueia o boot. |
| `Run()` | Feature flag, primeira execução síncrona (se o domínio exige), agendamento; **bloqueia até `Stop`**. Retornar cedo derruba o serviço. |
| `Stop(ctx)` | Cancela e espera a execução em curso terminar (limitado por `ctx`). Idempotente. |

---

## Template canônico — cron

### Wrapper (`pathCmd/{worker}_cron.go`)

```go
package main

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/joaoprofile/gofi-sdk-go/base/cronjob"
    "github.com/joaoprofile/gofi-sdk-go/gofi"
    "github.com/joaoprofile/gofi-sdk-go/obs/logging"

    reportsvc "<module>/domain/{contexto}/service"
)

const reportRunTimeout = 10 * time.Minute

type reportCron struct {
    cfg    Config
    svc    reportsvc.ReportService
    ctx    context.Context
    cancel context.CancelFunc
    done   chan struct{}
}

func newReportCron(cfg Config) *reportCron {
    ctx, cancel := context.WithCancel(context.Background())
    return &reportCron{cfg: cfg, ctx: ctx, cancel: cancel, done: make(chan struct{})}
}

func (w *reportCron) Name() string      { return "{contexto} report cron" }
func (w *reportCron) Stage() gofi.Stage { return gofi.StageServer }

func (w *reportCron) schedule() cronjob.ScheduleConfig {
    return cronjob.ScheduleConfig{
        Mode:         cronjob.Fixed,
        Hour:         w.cfg.ReportHour,
        Minute:       w.cfg.ReportMinute,
        LocationName: w.cfg.ReportLocation,
    }
}

// Start runs during Build, after database and cache: wire and validate here.
func (w *reportCron) Start(_ context.Context, _ *gofi.Runtime) error {
    if err := validSchedule(w.schedule()); err != nil { // ScheduleJob panics on bad config
        return err
    }
    w.svc = buildReportService()
    return nil
}

// Run blocks until Stop; a disabled worker just waits.
func (w *reportCron) Run() error {
    defer close(w.done)
    if !w.cfg.ReportEnabled {
        logging.Debug("{contexto} report cron: disabled by config")
        <-w.ctx.Done()
        return nil
    }
    var running sync.Mutex
    cronjob.ScheduleJob(w.ctx, w.schedule(), func() {
        running.Lock()
        defer running.Unlock()
        if w.ctx.Err() != nil {
            return // late tick after Stop
        }
        w.runOnce()
    })
    <-w.ctx.Done()
    running.Lock() // wait for the run in progress before gofi closes the database
    defer running.Unlock()
    return nil
}

func (w *reportCron) Stop(ctx context.Context) error {
    w.cancel()
    select {
    case <-w.done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (w *reportCron) runOnce() {
    ctx, cancel := context.WithTimeout(context.WithoutCancel(w.ctx), reportRunTimeout)
    defer cancel()
    if err := w.svc.Generate(ctx); err != nil {
        logging.FromContext(ctx).Error("{contexto} report cron: failed", slog.Any("error", err))
    }
}
```

`validSchedule`: `cronjob.md` § Config inválida.

### `wire.go` — builder recebe o que precisa por parâmetro

```go
func buildReportService() reportsvc.ReportService {
    return reportsvc.NewReportService(reportrepo.NewReportRepository())
}
```

> Service e repo consumidos só pelo worker são montados **dentro** do builder
> — sem `buildFooService` exposto só para alimentar o wrapper. Builder
> separado só para dependência realmente compartilhada (mesmo repo no worker e
> no handler HTTP).

### Loop com ticker, listener, watcher

Mesma forma. `Run` executa o loop com `w.ctx` e retorna quando ele é
cancelado (`examples/obs/job/archive.go`: `time.NewTicker` + `select` em
`ctx.Done()`). Quando não há `Start` a fazer, o adapter genérico `newRunner(name,
loop)` de `gofi-orchestrator.md` § Runner basta.

### Várias réplicas

Job que não pode duplicar: no `Start`, `rdb, err := cache.Shared(rt)` e
`Locker: cronjob.RedisLocker{Client: rdb}` + `Name` estável no
`ScheduleConfig` (`cronjob.md` § Várias réplicas).

---

## Multi-worker no mesmo serviço

Cada worker é um Runner próprio, uma linha no `With`:

```go
With(
    database.New(),
    newPartitionCron(cfg),
    newArchiveCron(cfg),
    newOrderConsumer(mq, cfg), // consumer-bootstrap.md
    httpserver.New(":8080"),
)
```

Worker desligado por config (`cfg.XxxEnabled == false`) continua declarado:
`Run` só espera o `Stop`. Mantém o `main` uniforme em todos os ambientes.

---

## Anti-padrões

### ❌ Ciclo de vida fora do gofi

```go
// ANTI-PATTERN
cron := buildPartitionCron(ctx, cfg)
defer cron.Close()          // runs after gofi shutdown: database already closed
go cron.Loop(ctx)           // nobody waits for the loop to finish
```

### ❌ `Bootstrap` / `Schedule` / `Start` públicos chamados do `main`

`main` passa a carregar ordem de inicialização do worker; esquecer
`Schedule` deixa o cron inerte em silêncio; passo novo de inicialização
obriga editar o `main`.

### ❌ `Run` que retorna na hora

```go
// ANTI-PATTERN
func (w *reportCron) Run() error {
    cronjob.ScheduleJob(w.ctx, w.schedule(), w.job)
    return nil // ListenAndServe reads "runner finished" and stops the service
}
```

### ❌ Trabalho longo no `Start`

Primeira execução, warm-up pesado ou carga inicial no `Start` seguram o
`Build` (e o `/readyz`). Vão no começo do `Run`.

### ❌ Feature flag no `main`

```go
// ANTI-PATTERN
if cfg.PartitionEnabled {
    components = append(components, newPartitionCron(cfg))
}
```

A decisão "está habilitado?" pertence ao worker.

### ❌ Handle do SDK exposto

`*cronjob.JobHandle`, ticker, manager: campo privado do wrapper, nunca
retornado ao `main`.

---

## Cron com horário fixo — tz de negócio explícito + `tzdata` embutido

1. **Fuso é decisão de negócio, não do container.** "À noite" / "meia-noite"
   significa no fuso de operação. `LocationName` explícito (IANA, por env) em
   todo `cronjob.Fixed`; sem ele vale `time.Local`, que no gofi é UTC por
   padrão — `Hour: 0` vira meia-noite UTC.
2. **`tzdata` já vem embutido.** `base/timezone` (importado pelo `Build`)
   embute o banco IANA, então `LoadLocation` não falha por imagem slim.
   Binário que agenda com fuso **sem** usar `gofi.New` importa
   `_ "time/tzdata"` no `main`.
3. **Nome inválido panica em `ScheduleJob`** — valide no `Start`/`LoadConfig`
   (`cronjob.md`).
4. **Escalonar horários** quando há N workers do mesmo tipo à mesma hora:
   minutos distintos por env.
5. **Runner genérico com modo fixed sem quebrar intervalo:** campos opcionais
   (`Daily bool` + `Hour`/`Minute`/`Location`) no `Config`; `Daily` monta
   `cronjob.Fixed`, senão `cronjob.Interval` (default).

---

## Checklist (gofi-eng)

- [ ] Worker implementa `gofi.Runner` (`Name`, `Stage`, `Start`, `Run`, `Stop`) e entra no `With` — **zero** chamada a método do worker no `main.go`
- [ ] `Start` monta dependências e valida config (agenda, fuso); nada de trabalho longo
- [ ] `Run` bloqueia até `Stop` (inclusive desligado por feature flag)
- [ ] `Stop` cancela e espera a execução em curso (`done` + `ctx` do `Stop`)
- [ ] Execução usa ctx próprio com timeout (`context.WithoutCancel` + `WithTimeout`), nunca `context.Background()` solto
- [ ] Handle do SDK (`*cronjob.JobHandle`, ticker) é campo privado
- [ ] Service/repo consumidos só pelo worker montados dentro do builder
- [ ] Um Runner por worker; feature flag decidida dentro dele
- [ ] Cron com horário fixo: `LocationName` IANA explícito vindo de config e validado; `Locker` quando não pode duplicar entre réplicas

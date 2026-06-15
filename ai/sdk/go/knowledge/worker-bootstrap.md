# Background worker bootstrap — wrapper owns lifecycle

> **Quando aplicar:** qualquer estrutura em `pathCmd` que represente um
> **trabalho de longa duração** com ciclo de vida (start + stop):
> consumer Kafka, cron scheduler, worker tick, listener TCP, watcher
> de filesystem, etc. Para o caso específico de consumer Kafka (que tem
> `*msq.ConsumerManager` e ordenação `Register → Dispatcher`), ver também
> `consumer-bootstrap.md`.

## Regra inviolável

**O wrapper do worker é dono do próprio ciclo de vida.** O constructor
(`New{Worker}` ou o builder de `wire.go`) **executa toda a inicialização
em si mesmo** — registro, scheduling, warm-up, primeira execução
(`Bootstrap`/`runOnce`), tudo. O wrapper expõe **apenas** `Close()`
para shutdown.

`main.go` enxerga **exclusivamente** o par:

```go
worker := build{Worker}(ctx, …)
defer worker.Close()
```

Nada de `worker.Schedule(...)`, `worker.Bootstrap(...)`, `worker.Start(...)`,
`worker.Register(...)` em `main`. **Cada método público chamado do `main.go`
além de `Close()` é red flag** — significa que o ciclo de vida vazou.

---

## Template canônico

### Wrapper (`pathCmd/{worker}.go`)

```go
package main

import (
    "context"

    "github.com/joaoprofile/gofi/base/cronjob"

    fooSvcPkg "{module}/services/domain/{ctx}/service"
)

type {Worker}Cron struct {
    svc    fooSvcPkg.FooService
    handle *cronjob.JobHandle  // ou *msq.ConsumerManager, *time.Ticker, etc.
}

func New{Worker}Cron(ctx context.Context, svc fooSvcPkg.FooService, cfg Config) *{Worker}Cron {
    w := &{Worker}Cron{svc: svc}

    w.runOnce(ctx) // bootstrap: primeira execução síncrona, se o domínio exige

    if !cfg.{Worker}CronEnabled {
        logging.Info("{worker} cron: disabled by config")
        return w
    }

    w.handle = cronjob.ScheduleJob(ctx, cronjob.ScheduleConfig{
        Mode:   cronjob.Fixed,
        Hour:   cfg.{Worker}CronHour,
        Minute: cfg.{Worker}CronMinute,
    }, func() {
        w.runOnce(context.Background())
    })

    return w
}

func (w *{Worker}Cron) Close() {
    if w.handle != nil {
        w.handle.Stop()
    }
}

func (w *{Worker}Cron) runOnce(parent context.Context) {
    ctx, cancel := context.WithTimeout(parent, runTimeout)
    defer cancel()
    w.svc.Run(ctx)
}
```

### `wire.go` — builder recebe `cfg` por valor

```go
func build{Worker}Cron(ctx context.Context, cfg Config) *{Worker}Cron {
    repo := fooRepoPkg.NewFooRepository()
    svc  := fooSvcPkg.NewFooService(repo)
    return New{Worker}Cron(ctx, svc, cfg)
}
```

> Construção do service e do repo são detalhes internos do builder.
> Não há `buildFooService` separado consumido apenas como entrada do
> wrapper — inline a construção (menos uma indireção, menos uma função
> exportada por engano).

### `main.go` — duas linhas por worker

```go
{worker}Cron := build{Worker}Cron(ctx, cfg)
defer {worker}Cron.Close()
```

---

## Multi-worker no mesmo serviço

Cada worker tem seu próprio handle e seu próprio par no `main.go`:

```go
partitionCron := buildPartitionCron(ctx, cfg)
defer partitionCron.Close()

archiveCron := buildArchiveCron(ctx, cfg)
defer archiveCron.Close()

bulkConsumer := buildBulkConsumer(ctx, service.Messaging(), cfg.BulkConcurrency)
defer bulkConsumer.Close()
```

Cron desabilitado por config (`cfg.XxxCronEnabled == false`) ainda
retorna wrapper válido — só com `handle == nil`. `Close()` é no-op
nesse caso. Isso mantém o par `build → defer Close` válido em todos
os cenários (com/sem feature flag).

---

## Anti-padrões

### ❌ `Bootstrap` / `Schedule` / `Start` públicos chamados de `main`

```go
// ANTI-PADRÃO
cron := NewPartitionCron(svc)
cron.Bootstrap(ctx)        // método público só pro main chamar
cron.Schedule(ctx, cfg)    // idem
```

Problemas:
- `main.go` carrega ordem de inicialização (`Bootstrap` antes de `Schedule`?) — detalhe que pertence ao worker.
- Esquecer de chamar `Schedule` deixa o cron silenciosamente inerte.
- Adicionar passo de inicialização (warm-up de cache, registro em service registry, etc.) obriga editar `main.go` em vez do wrapper.
- Acopla o `main.go` ao ciclo de vida específico desse worker.

### ❌ Builder retorna pares de objetos

```go
// ANTI-PADRÃO
svc := buildPartitionService(ctx)        // exposto só pra montar o cron
cron := NewPartitionCron(svc)
```

Service só consumido pelo wrapper não precisa de função builder dedicada.
Inline dentro de `build{Worker}Cron`. Builders separados são para
**dependências realmente compartilhadas** (mesmo repo usado por 2
workers + handler HTTP, por exemplo).

### ❌ `handle` exposto / retornado de `Schedule`

```go
// ANTI-PADRÃO
handle := cron.Schedule(ctx, cfg)
defer handle.Stop()  // main lida com o tipo da SDK direto
```

`*cronjob.JobHandle` (ou `*msq.ConsumerManager`) é detalhe de
implementação do worker. `main.go` opera no tipo do wrapper, não no
tipo da SDK. Wrapper encapsula `Stop()` dentro de `Close()`.

### ❌ Feature-flag verificada em `main`

```go
// ANTI-PADRÃO
if cfg.PartitionCronEnabled {
    cron := buildPartitionCron(ctx, cfg)
    defer cron.Close()
}
```

Decisão "está habilitado?" pertence ao wrapper. `main.go` sempre chama
`build → defer Close`; quando desabilitado, o wrapper retorna no-op
(handle nil, Close vazio). Isso preserva uniformidade e simetria com
outros workers.

---

## Cron com horário fixo — tz de negócio explícito + `tzdata` embutido

Quando um worker roda em **horário fixo diário** (modo `cronjob.Fixed` com
`Hour`/`Minute`, vs `cronjob.Interval`), duas armadilhas valem regra:

1. **Fuso é decisão de negócio, não do container.** "À noite" / "meia-noite"
   significa meia-noite no fuso de operação (ex.: o fuso de negócio do
   produto), **não** no fuso do container — que em produção quase sempre é
   **UTC**. Use `LocationName` explícito (IANA, configurável por env) no
   `ScheduleConfig`; **nunca** confie em `time.Local`/tz default do host. Um
   `Hour: 0` interpretado em UTC dispara às 21h do dia anterior no horário de
   um fuso `-03`, silenciosamente.

2. **`time.LoadLocation` com nome IANA exige `tzdata` disponível — embuta no
   binário.** `cronjob.ScheduleJob` faz `time.LoadLocation(cfg.LocationName)`
   e **PANICA no boot** se o tz database não estiver presente. Imagens
   slim/`scratch`/`distroless` normalmente **não têm** tzdata. Solução
   robusta e portável: o `main.go` do binário cron importa em branco:

   ```go
   import _ "time/tzdata" // embute o tz database no binário; LoadLocation(IANA) nunca panica por falta de tzdata no container
   ```

   Custo ~450KB no binário, zero dependência de tzdata do SO. Preferível a
   depender de pacote `tzdata` instalado na imagem.

3. **Escalonar horários** quando há N workers do mesmo tipo (um por dimensão)
   que disparam "à noite": minutos distintos (ex.: `:05`, `:20`, `:35`) por
   env, para não saturar broker/DB no mesmo instante.

4. **Runner genérico ganha modo fixed sem quebrar callers de intervalo:**
   adicione campos opcionais (`Daily bool` + `Hour`/`Minute`/`Location`) ao
   `Config`; quando `Daily`, monta `cronjob.Fixed`, senão mantém
   `cronjob.Interval` (default). Callers existentes que só setam `Interval`
   seguem inalterados (backward-compatible).

---

## Checklist (gofi-eng)

- [ ] Wrapper `{Worker}` em `pathCmd` tem campo opaco (`handle *cronjob.JobHandle`, `manager *msq.ConsumerManager`, etc.) — nunca exposto
- [ ] Constructor faz **toda** a inicialização (bootstrap síncrono, schedule, registro, dispatcher) — nada vaza pro `main`
- [ ] Feature flag (`cfg.XxxEnabled`) tratada dentro do constructor, com no-op quando desabilitado
- [ ] Wrapper expõe **apenas** `Close()` para shutdown — `Bootstrap`/`Schedule`/`Start`/`Register` são privados ou inexistentes
- [ ] `wire.go`/`build{Worker}(ctx, …)` inline construção de service+repo quando consumidos só pelo wrapper
- [ ] `main.go` tem o par `{worker} := build{Worker}(ctx, …)` + `defer {worker}.Close()` — **uma chamada de método pública pós-`build` é red flag**
- [ ] Cada worker no serviço tem seu próprio par (1:1)
- [ ] Cron com horário fixo: `LocationName` IANA explícito (fuso de negócio, não tz do container) + binário importa `_ "time/tzdata"` (senão `LoadLocation` panica no boot em imagem slim)

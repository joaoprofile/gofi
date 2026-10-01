---
name: cronjob
description: Agendamento com base/cronjob — ScheduleJob por intervalo ou horário fixo, fuso explícito, panics de config, Locker para rodar em uma réplica, WorkerPool
sdk: v0.8.2
keywords: [cronjob, ScheduleJob, ScheduleConfig, Fixed, Interval, LocationName, Weekdays, Locker, RedisLocker, JobHandle, WorkerPool, JobGenerator, cron, agendamento, réplica]
---

# Agendamento — `base/cronjob`

API: `.claude/sdk/go/api/base-cronjob.md`. Como o job vive dentro do serviço
(Runner, ciclo de vida, `main.go`): `worker-bootstrap.md`.

## `ScheduleJob` — o contrato

```go
handle := cronjob.ScheduleJob(ctx, cronjob.ScheduleConfig{
    Mode:         cronjob.Fixed,        // or cronjob.Interval + Interval: 5 * time.Minute
    Hour:         2,
    Minute:       30,
    Weekdays:     []time.Weekday{time.Monday, time.Friday}, // empty = every day
    LocationName: cfg.ReportLocation,   // business timezone, from config
    Name:         "{contexto}-report",  // required with Locker
    Locker:       cronjob.RedisLocker{Client: rdb},
}, func() { /* one run */ })
```

- Roda em goroutine própria até `ctx` ser cancelado ou `handle.Stop()`.
- **Execução sequencial**: o próximo disparo espera o job atual terminar.
- `job` é `func()` — **sem ctx**: feche sobre o ctx do dono e derive um
  timeout por execução.
- Panic dentro do job é recuperado, logado e marca o handle como
  `cronjob.JobFailed` — o agendamento continua.
- `LocationName` vazio = `time.Local` (UTC por padrão no gofi — ver
  `configuration.md` § Timezone).

## Config inválida = panic — valide antes

`ScheduleJob` **panica** com modo inválido, `Interval <= 0`, hora fora de
0–23, minuto fora de 0–59, `LocationName` desconhecido ou `Locker` sem
`Name`. Nenhum desses erros pode chegar lá:

```go
func validSchedule(c cronjob.ScheduleConfig) error {
    var errs []error
    if c.Hour < 0 || c.Hour > 23 || c.Minute < 0 || c.Minute > 59 {
        errs = append(errs, fmt.Errorf("invalid schedule time %02d:%02d", c.Hour, c.Minute))
    }
    if c.LocationName != "" {
        if _, err := time.LoadLocation(c.LocationName); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

Chame em `LoadConfig` ou no `Start` do Runner — o erro volta pelo `Build` em
vez de derrubar o processo.

## Horário fixo — fuso é decisão de negócio

"Meia-noite", "fechamento do dia" = horário **no fuso de operação**, não no
do container. `LocationName` explícito (IANA, configurável por env) em todo
`cronjob.Fixed`; `Hour: 0` sem fuso roda à meia-noite UTC. O tz database está
embutido em todo binário gofi, então `LoadLocation` funciona em imagem slim.

N jobs do mesmo tipo à mesma hora (um por dimensão): minutos escalonados
por config para não saturar banco/broker no mesmo instante.

## Várias réplicas — `Locker`

Sem `Locker`, **cada réplica roda o job**. Job que não pode duplicar usa
`cronjob.RedisLocker{Client: rdb}` + `Name` estável: cada disparo é disputado
com `SET NX` e só uma réplica executa. Falha no lock **pula** a execução
(perder um disparo é mais seguro que duplicar). O Redis vem do Build:
`cache.Shared(rt)` dentro do `Start` do Runner.

## Lote em paralelo — `WorkerPool`

Dentro de uma execução, para processar muitos itens com limite de
concorrência: `pool := cronjob.NewPool(n)` → `pool.Start()` →
`pool.EnqueueJobBatch(batch)` (ou `cronjob.NewJobGenerator(items, batchSize,
fn).RunWithPool(pool)`) → `pool.Close()` (espera tudo e fecha). Panic de item
é recuperado. O `error` devolvido por `fn` no `JobGenerator` só é impresso com
`log.Println` — trate e registre o erro **dentro** de `fn`. `n` sai de config
(`APP_MAX_PARALLEL_WORKERS` ou variável do contexto).

## Anti-padrões

- ❌ `ScheduleJob` com valor de config não validado (panic no boot).
- ❌ `cronjob.Fixed` sem `LocationName` para regra de horário de negócio.
- ❌ Job que não pode duplicar rodando em N réplicas sem `Locker`.
- ❌ `context.Background()` sem timeout dentro do job.
- ❌ `cronjob.CheckAllJobsHealth` como health check (só imprime com
  `log.Println`); status do handle via `handle.GetStatus()`.

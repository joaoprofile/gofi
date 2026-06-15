# Logging — Níveis e Disciplina (Go)

Infra já resolvida pelo SDK (`gofi/obs/logging`, `obs/logging/logging.go`):
o nível default é **Info** e só vira **Debug** quando `LOG_LEVEL=debug`. Em
prod, `logging.Debug(...)` é filtrado no handler — **não chega no Loki**. Logs
fazem Tee `console + OTLP→Loki`, JSON fora de `dev`, e `FromContext` injeta
`trace_id`/`span_id`. O problema nunca é a infra, é **escolher o nível certo** e
**não logar dentro de loop**.

## A regra (única, auditável)

| Nível | Quando | Cardinalidade |
|------|--------|---------------|
| **INFO** | **Só início e fim de fluxo de negócio importante.** O *fim* carrega o resultado agregado (contadores, outcome). | ~1–2 por request/job. **Nunca dentro de loop.** |
| **DEBUG** | Todo o resto diagnóstico: por-item, por-stage, por-página, payloads, `message received`, timings/profiling. | Livre — some em prod automaticamente. |
| **WARN** | Degradação tolerada (fallback usado, best-effort que falhou mas seguiu). | Baixa. |
| **ERROR** | Falha que precisa de atenção. Sempre com `slog.Any("error", err)`. | Por falha real. |
| **FATAL** | **Só no bootstrap** (`prepare` de stmt, wiring, `main`). Nunca em request/consumer/processor. | Boot. |

> "Info que não diz nada" = qualquer Info que dispara por iteração, por página,
> por mensagem recebida, ou por etapa de profiling. **Rebaixa pra Debug.** Se
> em prod o Loki recebe a mesma linha N vezes por job, está errado.

## O que é "fluxo importante"

- **Consumer Kafka** (`wb_*`, `*_consumer.go`): start/end da **mensagem de
  negócio** — não por retry/parse/filtro descartado.
- **Scheduler processor** (`domain/*/scheduler/processor`): start/end do **run
  inteiro** (tick) com totais. Cada página = Debug. Se o tick não produziu
  trabalho (`published == 0`), o resumo também é Debug.
- **Application / Orchestrator** (`manage`, `pricing apply`, `synchronization
  apply`): start/end da operação com outcome.
- **Bootstrap** (`main.go`/`wire.go`): `service started` / `consumer
  registered` — 1 linha, não por-runner.

## Padrão de implementação

```go
func (c *consumer) Handle(ctx context.Context, msg Message) error {
    log := logging.FromContext(ctx) // correlaciona o log com o trace

    log.Debug("pricing apply: message received", slog.String("key", msg.Key)) // diagnóstico
    if !relevant(msg) {
        log.Debug("pricing apply: skipped (filter)", slog.String("source", msg.Source))
        return nil // descartado != fluxo; nunca Info
    }

    log.Info("pricing apply: started", // START do fluxo real
        slog.String("source", msg.Source), slog.String("accountId", msg.AccountID))

    n, err := c.app.Apply(ctx, msg)
    if err != nil {
        log.Error("pricing apply: failed", slog.Any("error", err))
        return err
    }

    log.Info("pricing apply: completed", // END com resultado agregado
        slog.Int("itemsUpdated", n), slog.String("outcome", "success"))
    return nil
}
```

Dentro de loop de paginação → **sempre** `log.Debug("...: page processed", ...)`.
O resumo (`total_read`, `published`) vai num único Info no fim do run.

## Convenções

- **Mensagem:** `"<contexto>: <evento>"` minúsculo, estável (vira chave de busca
  no Loki). Ex.: `"meli order consumer: started"`.
- **Sempre `logging.FromContext(ctx)`** nos pontos de fluxo — sem isso o log não
  correlaciona com o trace (`trace_id`/`span_id`). Os shortcuts globais
  (`logging.Info` direto) só pra bootstrap, onde não há `ctx` de request.
- **Erro sempre** em `slog.Any("error", err)` — nunca interpolar no `msg`.
- **IDs e valores free-form** vão pelo log (`slog.String`), **nunca** pela
  métrica (cardinalidade — ver `observability-otel.md`).
- **Lifecycle de infra** (abrir/fechar conexão, pool stats, cache connected) é
  **Debug**, não Info. Se só interessa quando quebra, logue **só no erro**.

## Anti-padrões (rejeitados em review)

- ❌ `logging.Info` dentro de `for {}` de paginação → use `Debug`.
- ❌ `logging.Info("auth_perf", "stage", ..., "elapsed", ...)` por etapa →
  profiling é `Debug` (ou métrica de duração).
- ❌ `logging.Info` em todo `message received` / `skipped (filter)` → `Debug`.
- ❌ `logging.Info` de "conexão fechada / cache connected" no shutdown/boot →
  `Debug`, ou só `Error` se falhar.
- ❌ `logging.Fatal` fora de bootstrap (em consumer/handler/processor).

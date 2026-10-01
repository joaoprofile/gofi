---
name: logging
description: Logging com obs/logging — o que o Build já configura, níveis (Info só início/fim de fluxo), FromContext para correlação com trace, Fatal só no main e armadilhas de logger derivado cedo
sdk: v0.8.2
keywords: [logging, slog, obs/logging, LOG_LEVEL, Info, Debug, Warn, Error, Fatal, FromContext, trace_id, níveis, loop]
---

# Logging — Níveis e Disciplina (Go)

API: `.claude/sdk/go/api/obs-logging.md`. Import:
`github.com/joaoprofile/gofi-sdk-go/obs/logging`.

## O que o SDK já resolve

- **`Build` inicializa o logger global** (`config.InitLogging`): nível de
  `LOG_LEVEL` (`debug`\|`info`\|`warn`\|`error`; vazio = Info), texto quando
  `APP_ENVIRONMENT=dev`, JSON em qualquer outro, `service` = nome do
  `gofi.New`.
- Com `observability.New()`, os logs vão também para o collector (OTLP),
  **no mesmo nível** do console: `Debug` filtrado não sai de lugar nenhum.
- `logging.FromContext(ctx)` devolve `*slog.Logger` com `trace_id`/`span_id`
  do span em `ctx`.
- `ListenAndServe`/`Shutdown` fazem flush (`logging.Shutdown`) no fim.
- Antes do `Build`, `logging.*` cai no `slog.Default()` (com aviso) — não
  quebra, mas sai sem formato nem export.

O problema nunca é a infra: é **escolher o nível certo** e **não logar dentro
de loop**.

## A regra (única, auditável)

| Nível | Quando | Cardinalidade |
|------|--------|---------------|
| **INFO** | **Só início e fim de fluxo de negócio importante.** O *fim* carrega o resultado agregado (contadores, outcome). | ~1–2 por request/job. **Nunca dentro de loop.** |
| **DEBUG** | Todo o resto diagnóstico: por item, por etapa, por página, payloads, `message received`, timings. | Livre — some fora de `LOG_LEVEL=debug`. |
| **WARN** | Degradação tolerada (fallback usado, best-effort que falhou mas seguiu). | Baixa. |
| **ERROR** | Falha que precisa de atenção. Sempre com `slog.Any("error", err)`. | Por falha real. |
| **FATAL** | **Nunca** fora do `main`. `logging.Fatal` faz `os.Exit(1)` **sem** flush — perde os logs pendentes do OTLP e pula o fechamento dos recursos. | — |

> "Info que não diz nada" = qualquer Info que dispara por iteração, por página,
> por mensagem recebida ou por etapa de profiling. **Rebaixa para Debug.**

## O que é "fluxo importante"

- **Consumer**: start/end da **mensagem de negócio** — não por retry, parse
  ou filtro descartado.
- **Job / cron**: start/end da **execução inteira** com totais. Cada página =
  Debug. Execução que não produziu trabalho: o resumo também é Debug.
- **Operação de aplicação/orquestração**: start/end com outcome.
- **Bootstrap**: o SDK já loga `service started` / `shutdown started` — não
  repetir; no máximo 1 linha própria de configuração relevante.

## Padrão de implementação

```go
func (c *orderConsumer) handle(ctx context.Context, msg *msq.Message) (msq.Result, error) {
    log := logging.FromContext(ctx) // correlates with the process span

    log.Debug("{contexto} consumer: message received", slog.String("key", msg.Key))
    order, err := msq.UnpackMessage[OrderCreated](msg)
    if err != nil {
        log.Debug("{contexto} consumer: skipped (malformed)", slog.Any("error", err))
        return msq.Ignore, err
    }

    log.Info("{contexto} consumer: started", slog.String("orderId", order.ID))
    n, err := c.svc.Apply(ctx, *order)
    if err != nil {
        return msq.Nack, err // the msq pipeline logs the nack/dead-letter with the error
    }
    log.Info("{contexto} consumer: completed", slog.Int("itemsUpdated", n), slog.String("outcome", "success"))
    return msq.Ack, nil
}
```

Loop de paginação → `log.Debug("...: page processed", ...)`; o resumo
(`total_read`, `published`) vai num único Info no fim.

## Convenções

- **Mensagem:** `"<contexto>: <evento>"` minúsculo e estável (é chave de
  busca).
- **Sempre `logging.FromContext(ctx)`** nos pontos de fluxo. Os atalhos
  globais (`logging.Info` direto) só onde não há `ctx` de request/mensagem.
- **Derive o logger na chamada, não na construção.** Logger derivado (`With`,
  `FromContext`) **antes** do `Build` não recebe o export OTLP anexado depois.
  Nada de `log *slog.Logger` guardado em struct montada antes do `Build`.
- **Erro sempre** em `slog.Any("error", err)` — nunca interpolado no `msg`.
- **IDs e valores livres** vão no log (`slog.String`), **nunca** em atributo de
  métrica (`observability-otel.md`).
- **Lifecycle de infra** (conexão aberta/fechada, cache conectado) é do SDK;
  código do projeto não loga isso.
- **Erro já logado pelo SDK não é relogado:** `netx.RespondError` loga a causa
  do `AppError`; o componente de mensageria loga `Nack` (Warn), dead-letter e
  erro de consumer/producer.
- `fmt.Println`/`log.Printf` em código de produção: proibido (exceção: `main`
  reportando erro do `Build`/`ListenAndServe`, antes/depois do logger).

## Anti-padrões (rejeitados em review)

- ❌ `logging.Info` dentro de `for` de paginação → `Debug`.
- ❌ `logging.Info` por etapa de profiling → `Debug` ou métrica de duração.
- ❌ `logging.Info` em todo `message received` / `skipped` → `Debug`.
- ❌ `logging.Fatal` / `os.Exit` / `log.Fatal` fora do `main` — em
  construtor, repository, handler, consumer, job: devolva `error`.
- ❌ `slog.Logger` derivado e guardado antes do `Build`.
- ❌ Logar de novo o erro que `netx.RespondError` ou o pipeline do `msq` já
  logam.

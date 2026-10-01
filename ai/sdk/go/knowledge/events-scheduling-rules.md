---
name: events-scheduling-rules
description: Regras de eventos, scheduler e motor de decisão em Go — loader, processor no domínio, decider/executor sobre msq, Type* do envelope, consumer naming, rollup
sdk: v0.8.2
keywords: [loader, scheduler, processor, decider, executor, event-driven, msq, kafka, type-naming, consumer-group, dead-letter, partition-key, rollup]
---

# Regras — eventos, scheduler e motor de decisão

Índice; detalhe no arquivo apontado. Mensageria do SDK (producer, consumer,
retry/DLQ, key, group): `messaging-msq.md` + `.claude/sdk/go/api/msq*.md`.

- **Loader pattern — snapshot consistente para motores de decisão.**
  Decider que lê 3+ tabelas que precisam ser consistentes entre si durante a
  avaliação ganha pacote `services/domain/{ctx}/loader/` (irmão de `service/`,
  `repository/`, `application/`):
  ```
  services/domain/{ctx}/loader/
    contract.go   — interface Loader + struct {Ctx}EvaluationContext
    loader.go     — prepared stmts no constructor + Load + List* + Close
    queries.go    — SQL bruto (constantes string)
    snapshot.go   — DTOs de snapshot
    errors.go     — erros próprios do loader
    loader_test.go
  ```
  Interface: `Load(ctx, entityID) (*EvaluationContext, errs.AppError)` +
  `ListBy{Dimension}(ctx, dim) ([]int64, errs.AppError)` + `Close() error`.
  1 query monolítica (JOINs 1:1) + N sub-queries paralelas via
  `errgroup.WithContext` (N:1); cascata de config resolvida no SQL
  (`COALESCE`), não chamando `XxxService.GetEffective`. Application/processor
  recebem `Loader`; testes mockam **1** interface em vez de 5+ repositories.
  `ListBy{Dimension}` mora no Loader — **não** criar `scheduler/repository/`
  paralelo. Detalhe: `loader-pattern.md`.
- **Processor scheduler-driven mora no DOMÍNIO, não no binário cron.**
  Contexto acionado por cron (em vez de consumer reativo): o `Processor`
  (implementa `scheduler.Processor` do pacote comum de scheduler do projeto)
  que itera entidades, chama o use case e emite evento **mora no domínio**:
  ```
  services/domain/{ctx}/scheduler/
    model/       — DTOs do scheduler (ex.: EligibleEntity)
    processor/   — {ctx}_processor.go (NewXxxProcessor + Process(ctx, emit))
    repository/  — {ctx}_pending_repository.go (pendências paginadas)
  ```
  O binário cron é **só composition root** (runner + Processor do domínio, um
  runner por dimensão). `*_processor.go` no binário cron = refatorar. Com mais
  de uma réplica, o slot roda em uma só (`cronjob.Locker` do SDK, ex.:
  `cronjob.RedisLocker`). Detalhe: `.claude/expertise/event-driven/executor-pattern.md`
  §"Processor (scheduler-driven) mora no DOMÍNIO"; ciclo de vida do wrapper em
  `worker-bootstrap.md`.
- **Event-driven decider/executor — split por evento entre dois workers.**
  Decidir (estado local, pode ser função pura) e aplicar em sistema externo
  (I/O com latência e falha) são dois agents: o decider publica evento com
  `decision_id` (UUID v7); o executor consome e aplica via bridge/adapter.
  - **Idempotência por banco:** tabela `{ctx}_execution` com `decision_id`
    UNIQUE; `INSERT ... ON CONFLICT DO NOTHING` **antes** de qualquer chamada
    externa (o broker entrega at-least-once).
  - **Re-validação obrigatória** antes de aplicar (alvo ainda elegível + guard
    rails passam) → senão `STALE` (terminal; decider reavalia no próximo ciclo).
  - **Transient vs permanent** do externo: transient (timeout/5xx/429/rede) →
    handler devolve `msq.Nack` e o **pipeline do msq** retenta
    (`ConsumeConfig.MaxRetries` + `RetryBackoff`, exponencial com jitter) e,
    esgotado, manda para `DeadLetterTopic`; permanent (4xx exceto 429) →
    `FAILED` + `msq.Ack` (sem retry); 401 → renova token + 1 nova tentativa.
    **Sem retry manual no handler.**
  - **Status terminais:** `APPLIED` / `FAILED` / `STALE` (trabalho: `PENDING`).
    Materialização atômica: `status=APPLIED` + INSERT/DELETE na junction local
    na **mesma transação**.
  - **Partition key = entity_id:** o decider publica com
    `msg.WithKey(entityID)` — preserva ordem por entidade (Kafka: partição;
    SQS FIFO: grupo).
  - **Cutoff de idade** (default 24h entre `decided_at` e consumo) → `STALE` sem
    re-validar (proteção contra burst após incidente).
  - **Fire-and-forget:** executor não emite confirmação; decider reavalia.
  Detalhe: `.claude/expertise/event-driven/executor-pattern.md`;
  bridge com escrita em `bridge-factory-adapter-pattern.md`.
- **Types canônicos do envelope — `kafka.Type*` (pacote do projeto) segue a
  direção do evento.** Inbound (dado factual observado de fora) → substantivo
  singular (`Type{Snapshot}`); outbound (comando nosso para fora) →
  gerúndio/ação (`Type{Acting}`). Mesma dimensão com os dois fluxos → **dois
  types** com comentário de direção, consumidos por groups distintos;
  **anti-padrão MAJOR:** 1 type + discriminador `source` para direções opostas
  (força dispatch no consumer). As constantes são do pacote comum de Kafka do
  projeto, não do SDK; o `msq.Message.Type` (atributo CloudEvents) é outro
  campo. Detalhe: `.claude/expertise/event-driven/kafka-type-naming.md`.
- **Consumer group — passar SÓ o prefix.** Os helpers do projeto
  `kafka.SyncConsumer(prefix)` / `kafka.LifecycleConsumer(prefix)` montam o
  `msq.ConsumeConfig` e adicionam `-sync-cg` / `-lifecycle-cg`; duplicar o
  sufixo no caller é bug latente. Detalhe: `kafka-consumer-naming.md`.
- **Consumer Kafka sempre com `DeadLetterTopic`.** No Kafka, `Nack` final sem
  DLQ **avança o offset e descarta** a mensagem. `InitialOffset` explícito
  (`msq.OffsetResetEarliest` para comando/evento que não pode sumir). Detalhe:
  `messaging-msq.md` § Retry e dead-letter.
- **Recompute de rollup denormalizado — owner por coluna, best-effort,
  set-based.** Contexto que calcula compilado de janela móvel numa tabela de
  **outro** contexto é escritor exclusivo daquelas colunas; recompute roda
  **após** persistir o raw, **best-effort** (WARN + métrica, nunca derruba o
  pipeline), e **sempre** (roll-off temporal independe de fato novo); um
  `UPDATE ... FROM (CTE)` recompila o escopo inteiro (roll-off via `LEFT JOIN`
  + filtro, guard de divisão por zero). Detalhe:
  `.claude/expertise/ddd-architecture/application-vs-domain-service.md` §"Recompute de
  agregado derivado".

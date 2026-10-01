---
name: kafka-consumer-naming
description: Nome de consumer group Kafka via helpers do projeto (SyncConsumer/LifecycleConsumer) — passar só o prefix
sdk: v0.8.2
keywords: [kafka, consumer-group, group-id, SyncConsumer, LifecycleConsumer, naming, offset]
---

# Kafka consumer — naming convention + armadilhas dos helpers

Convenção para nomes de **consumer groups** quando o projeto usa
`kafka.SyncConsumer(prefix)` / `kafka.LifecycleConsumer(prefix)`.

> **Esses helpers são do PROJETO**, não do SDK: vivem no pacote comum de
> Kafka do projeto (`services/common/kafka`, junto dos `Topic*`/`Type*`).
> O provider do SDK (`msq/provider/kafka`) não tem helper de consumer. O
> helper só monta um `msq.ConsumeConfig` (SDK v0.8.2) com group, tópico e DLQ
> pré-cabeados. Mecânica do consumer (retry, DLQ, offset, concorrência):
> `messaging-msq.md`.

## Regra principal — **passar SÓ o prefix**

O helper **adiciona internamente** o sufixo `-sync-cg` / `-lifecycle-cg` ao
prefix recebido:

```go
// services/common/kafka — helper do projeto sobre o msq
func SyncConsumer(groupPrefix string) msq.ConsumeConfig {
    cfg := msq.DefaultConsumeConfig(TopicSync)
    cfg.GroupID = groupPrefix + "-sync-cg" // ← SUFIXO AUTOMÁTICO
    cfg.DeadLetterTopic = TopicSyncDLQ    // Kafka sem DLQ descarta o Nack final
    cfg.InitialOffset = msq.OffsetResetEarliest
    return cfg
}
```

**Caller passa só o prefix.** O groupId final é montado pelo helper.

| ✅ Correto | ❌ Errado (sufixo duplicado) |
|---|---|
| `kafka.SyncConsumer("ctx-a-typeb")` → `ctx-a-typeb-sync-cg` | `kafka.SyncConsumer("ctx-a-typeb-sync-cg")` → `ctx-a-typeb-sync-cg-sync-cg` |
| `kafka.SyncConsumer("ctx-a")` → `ctx-a-sync-cg` | `kafka.SyncConsumer("ctx-a-cg")` → `ctx-a-cg-sync-cg` |
| `kafka.LifecycleConsumer("ctx-a")` → `ctx-a-lifecycle-cg` | `kafka.LifecycleConsumer("ctx-a-lifecycle")` → `ctx-a-lifecycle-lifecycle-cg` |

**Bug latente:** group duplicado **funciona** no Kafka (é um group ID válido,
só feio) → não quebra teste local nem build → vai para produção e fica
**permanentemente** com o nome ruim. Detectar via QA + revisão do catálogo da
spec de topologia.

## Convenção de prefix

`{escopo}` é o discriminador semântico do consumer:

| Cenário | Prefix recomendado | Group final |
|---|---|---|
| 1 consumer por dimensão polimórfica, todos os types | `{dim-slug}` | `{dim-a}-sync-cg`, `{dim-b}-sync-cg` |
| Consumer split por type (ver `worker-bootstrap.md` § split por type) | `{dim-slug}-{type}` | `{dim-a}-typeA-sync-cg`, `{dim-a}-typeB-sync-cg` |
| Cross-dimensão (filtra por type, não por dim) | `{slug-do-modulo}` | `{module-x}-sync-cg` |
| Lifecycle (eventos de controle) | `{dim-slug}` | `{dim-a}-lifecycle-cg` |

**Nunca** colocar `-cg` / `-sync` / `-lifecycle` no prefix. **Nunca** PascalCase
ou underscore — Kafka aceita, mas a convenção é kebab-case lowercase.

## Onde declarar os constants do prefix

No arquivo do consumer em `pathCmd/{binary}/`. Constantes locais ao binário,
**sem `-sync-cg` no nome nem no valor**:

```go
const (
    typeAConsumerGroupPrefix = "ctx-a-typeA"
    typeBConsumerGroupPrefix = "ctx-a-typeB"
)

// usage — dentro do constructor do wrapper (consumer-bootstrap.md)
mgr := msq.NewConsumerManager(broker)
if err := mgr.Register(kafka.SyncConsumer(typeAConsumerGroupPrefix), c.handle).Start(); err != nil {
    return nil, err
}
```

Sufixar a const com `Prefix` torna a regra **óbvia no callsite**.

> **Concorrência não é parâmetro do nome nem do `Dispatcher`.** No Kafka o
> paralelismo é por partição (o provider ignora `Concurrency`), e
> `Dispatcher(n)` não sobrescreve o `Concurrency = 20` que
> `DefaultConsumeConfig` já preenche. Escala = partições + réplicas do grupo.

## Catálogo de consumer groups vive na spec de topologia

A spec de topologia Kafka do projeto mantém o **catálogo completo** dos
consumer groups (§ "Consumer Groups"). Toda mudança de naming ou criação de
group novo atualiza esse catálogo — é a fonte da verdade do plano de
partitions/scale.

## QA — check no contexto

Busca literal (`--in code` exclui os documentos; cada hit vem sob a função que
registra o consumer):

```bash
gofi find --regex 'SyncConsumer\(.*-sync-cg' --in code
gofi find --regex 'LifecycleConsumer\(.*-lifecycle-cg' --in code
# Esperado: zero hits (sufixo só no helper, nunca no caller).
```

Hit é **MAJOR** — groupId em prod fica divergente do catálogo. Corrigir antes
do merge.

## Mudança de groupId em produção — atenção

Renomear consumer group **perde o offset**: o group novo começa pelo
`InitialOffset` do `ConsumeConfig` (`msq.OffsetResetEarliest` reprocessa o
tópico inteiro retido; `msq.OffsetResetLatest` — e o default do provider Kafka —
pula o backlog, e o provider loga WARN "existing backlog SKIPPED").

**Quando renomear:**
1. Decidir conscientemente o `InitialOffset` do group novo (reprocessar vs pular)
   e garantir idempotência do handler se for reprocessar.
2. Deploy do código novo → group novo aparece, group antigo fica órfão.
3. Group órfão é coletado pela retenção de offsets do broker.
4. Documentar a transição no PR + no catálogo da spec de topologia.

## Anti-padrões

- ❌ Duplicar sufixo: `kafka.SyncConsumer("x-sync-cg")`
- ❌ PascalCase / underscore no prefix: `"CtxATypeA"`, `"ctx_a_typeA"`
- ❌ Não documentar group novo no catálogo da spec de topologia
- ❌ Hard-code do groupId final no caller (`cfg.GroupID = "ctx-a-typeA-sync-cg"`)
  contornando o helper — perde a convenção centralizada do projeto
- ❌ Helper sem `DeadLetterTopic` — no Kafka o `Nack` final sem DLQ é descartado

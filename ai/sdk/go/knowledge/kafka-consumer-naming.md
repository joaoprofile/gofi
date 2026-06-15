# Kafka consumer — naming convention + armadilhas dos helpers

Convenção pra nomes de **consumer groups** quando o projeto usa
`kafka.SyncConsumer(prefix)` / `kafka.LifecycleConsumer(prefix)` (helpers do
SDK que constroem `msq.ConsumeConfig` com group ID, topic e DLQ pré-cabeados).

## Regra principal — **passar SÓ o prefix**

Os helpers `SyncConsumer`/`LifecycleConsumer` **adicionam internamente** o
sufixo `-sync-cg` / `-lifecycle-cg` ao prefix fornecido:

```go
func SyncConsumer(groupId string) msq.ConsumeConfig {
    cfg := msq.DefaultConsumeConfig(TopicSync)
    cfg.GroupID = groupId + "-sync-cg"        // ← SUFIXO AUTOMÁTICO
    cfg.DeadLetterTopic = TopicSyncDLQ
    return cfg
}
```

**Caller passa só o prefix.** O groupId final é montado pelo helper.

| ✅ Correto | ❌ Errado (sufixo duplicado) |
|---|---|
| `kafka.SyncConsumer("ctx-a-typeb")` → `ctx-a-typeb-sync-cg` | `kafka.SyncConsumer("ctx-a-typeb-sync-cg")` → `ctx-a-typeb-sync-cg-sync-cg` |
| `kafka.SyncConsumer("ctx-a")` → `ctx-a-sync-cg` | `kafka.SyncConsumer("ctx-a-cg")` → `ctx-a-cg-sync-cg` |
| `kafka.LifecycleConsumer("ctx-a")` → `ctx-a-lifecycle-cg` | `kafka.LifecycleConsumer("ctx-a-lifecycle")` → `ctx-a-lifecycle-lifecycle-cg` |

**Bug latente:** group duplicado **funciona** em Kafka (é um group ID válido,
só feio) → não quebra teste local nem build → vai pra produção e fica
**permanentemente** com o nome ruim no Kafka. Detectar via QA + revisão do
catálogo da spec de topologia (cross-spec do projeto).

## Convenção de prefix

`{escopo}` é o discriminador semântico do consumer. Padrão:

| Cenário | Prefix recomendado | Group final |
|---|---|---|
| 1 consumer por dimensão polimórfica, todos os types | `{dim-slug}` | `{dim-a}-sync-cg`, `{dim-b}-sync-cg` |
| Consumer split por type (ver `worker-bootstrap.md` § split por type) | `{dim-slug}-{type}` | `{dim-a}-typeA-sync-cg`, `{dim-a}-typeB-sync-cg` |
| Cross-dimensão (filtra por type, não por dim) | `{slug-do-modulo}` | `{module-x}-sync-cg` |
| Lifecycle (eventos de controle) | `{dim-slug}` | `{dim-a}-lifecycle-cg` |

**Nunca** colocar `-cg` / `-sync` / `-lifecycle` no prefix. **Nunca** usar
PascalCase ou underscores — Kafka aceita, mas a convenção do SDK é
kebab-case lowercase.

## Onde declarar os constants do prefix

Arquivo do consumer Kafka no `pathCmd/{binary}/`. Constantes locais ao
binário, **sem `-sync-cg` no nome nem no valor**:

```go
// ✅ Correto
const (
    typeAConsumerGroupPrefix = "ctx-a-typeA"
    typeBConsumerGroupPrefix = "ctx-a-typeB"
    typeCConsumerGroupPrefix = "ctx-a-typeC"
)

// usage
manager.Register(kafka.SyncConsumer(typeAConsumerGroupPrefix), c.handle).Dispatcher(N)
```

Sufixar o nome da const com `Prefix` torna a regra **óbvia no callsite** —
quem ler `kafka.SyncConsumer(typeAConsumerGroupPrefix)` entende que o sufixo
não está duplicado.

## Catálogo de consumer groups vive na spec de topologia

A spec de topologia Kafka do projeto mantém o **catálogo completo** dos
consumer groups (§ "Consumer Groups"). Toda mudança de naming ou criação
de group novo passa pela atualização desse catálogo — é a fonte da verdade
do plano de partitions/scale.

## QA — check no contexto

Auditar via grep simples:

```bash
# Procurar duplicação acidental do sufixo
grep -rE "SyncConsumer\(.*-sync-cg" services/
grep -rE "LifecycleConsumer\(.*-lifecycle-cg" services/

# Esperado: zero hits (sufixo só nos helpers, nunca no caller).
```

Se houver hit, é **MAJOR** — groupId em prod fica feio e divergente do que
o catálogo de topologia documenta. Corrigir antes do merge.

## Mudança de groupId em produção — atenção

Renomear consumer group em prod **perde o offset** — Kafka cria um group
novo do zero (rebobina pra `auto.offset.reset` configurado: `latest` por
default no projeto). Se rebobinar pra `earliest` por engano, consumer
re-processa toda a história.

**Quando renomear:**
1. Confirmar `auto.offset.reset=latest` no consumer.
2. Deploy do código novo → group novo aparece, group antigo fica órfão.
3. Group órfão é coletado por retention automática de offset (default 7d).
4. Documentar a transição no PR + no catálogo da spec de topologia.

## Anti-padrões

- ❌ Duplicar sufixo: `kafka.SyncConsumer("x-sync-cg")`
- ❌ PascalCase / underscores no prefix: `"CtxATypeA"`, `"ctx_a_typeA"`
- ❌ Não documentar group novo no catálogo da spec de topologia
- ❌ Hard-code do groupId final no caller (`cfg.GroupID = "ctx-a-typeA-sync-cg"`)
  bypassando o helper — perde a convenção centralizada do SDK

---
name: messaging-msq
description: Mensageria com gofi/msq — provider por import, producer, consumer, retry/DLQ, key e consumer group, componente messaging, msq/worker e testes
sdk: v0.8.2
keywords: [msq, messaging, kafka, rabbitmq, sqs, nats, redis, oci-queue, producer, consumer, dead-letter, retry, consumer-group, partition-key, msqtest, worker]
---

# Mensageria — `msq` de ponta a ponta

Referência gerada: `.claude/sdk/go/api/msq.md` (fachada), `msq-types.md`
(`Message`, `ConsumeConfig`, `Result`), `msq-core.md` (pipeline e
`ConsumerManager`), `msq-port.md`, `msq-provider-<p>.md`,
`gofi-component-messaging.md`. Programas completos (producer + consumer com
retry e DLQ, um por broker): `.claude/sdk/go/api/examples.md` § Kafka / RabbitMQ /
Amazon SQS.

Imports: `github.com/gofi-labs/gofi-sdk-go/msq` (tudo que o código de aplicação
usa é re-exportado aqui — `msq.Message`, `msq.ConsumeConfig`, `msq.Ack`…) e
`github.com/gofi-labs/gofi-sdk-go/gofi/component/messaging`.

---

## Providers suportados — trocar de broker

Cada provider é um **módulo próprio** que se registra no `init()` via
`msq.Register`. Só o provider importado entra no binário.

| `MESSAGING_PROVIDER` | Import (blank) | Observação |
|---|---|---|
| `kafka` | `_ ".../msq/provider/kafka"` | CloudEvents binário; SASL PLAIN/SCRAM |
| `rabbitmq` | `_ ".../msq/provider/rabbitmq"` | exchange via `msq.Config{Exchange}` |
| `sqs` | `_ ".../msq/provider/sqs"` | credenciais pela cadeia AWS (`base/cloud/aws`) |
| `oci` | `_ ".../msq/provider/oci"` | OCI Queue; `ConsumeConfig.QueueID` obrigatório |
| `redis` | `_ ".../msq/provider/redis"` | reusa `CACHE_*`; `MESSAGING_REDIS_MODE=pubsub\|streams` |
| `nats` | `_ ".../msq/provider/nats"` | JetStream |

**Regra:** o provider é escolhido por **import + env**, nunca por `switch` no
código. Trocar de broker = trocar o blank import **e** `MESSAGING_PROVIDER`;
producer/consumer não mudam. Env sem o import correspondente falha no `Build`
(`msq.Open`: provider não registrado).

---

## Bootstrap — componente `messaging`

```go
import (
    "github.com/gofi-labs/gofi-sdk-go/gofi"
    "github.com/gofi-labs/gofi-sdk-go/gofi/component/messaging"
    "github.com/gofi-labs/gofi-sdk-go/msq"
    _ "github.com/gofi-labs/gofi-sdk-go/msq/provider/kafka" // MESSAGING_PROVIDER=kafka
)

mq := messaging.New() // MESSAGING_* → broker
svc, err := gofi.New("{service}").With(mq /*, database.New(), ... */).Build()
if err != nil { log.Fatal(err) }

broker := mq.Broker() // msq.Broker já envolvido no pipeline; nil antes do Build
```

- `messaging.New()` lê `MESSAGING_PROVIDER`; `messaging.New(msq.Config{BrokerType: msq.BrokerKafka})`
  fixa o tipo; `messaging.New(msq.Config{Broker: b})` usa broker montado à mão
  (ex.: `kafka.New(kafka.Config{Topics: ...})` para criar tópicos no `Setup`).
- `messaging.FromBroker(b)` injeta broker do caller **sem** pipeline, setup nem
  close — só para quem já tem o broker vivo.
- O componente chama `Setup` (`port.BrokerSetup`) no `Build` e, no shutdown,
  **drena os consumers antes** de fechar broker, cache e banco.
- Standalone (sem builder): `b, err := msq.Open(ctx, messaging.ConfigFromEnv(env))`
  + `svc, err := msq.New(msq.Config{Broker: b})`.

**Não existe** `msq.Messaging` nem `service.Messaging()`: o handle é
`mq.Broker()` (tipo `msq.Broker`), passado por parâmetro aos builders do
`wire.go`.

---

## Producer

```go
producer, err := broker.NewProducer() // erro não pode ser ignorado
if err != nil { return err }
defer producer.Close()

msg, err := msq.NewMessageWithTopic(topic, payload) // JSON; erro de encoding volta aqui
if err != nil { return err }
msg.Type = "{entity}.{event}" // atributo CloudEvents (opcional)
msg.WithKey(entityID)         // partição (Kafka) / MessageGroupId (SQS FIFO)
return producer.SendMessage(ctx, msg)
```

- `SendMessagesBatch(ctx, msgs)` usa batch nativo onde existe (Kafka, SQS).
- `Producer` **não** é seguro para uso concorrente sem sincronização: um por
  goroutine/wrapper, ou batch.
- Headers transversais (`msg.WithHeader(k, v)`); trace context W3C já é
  injetado pelo pipeline — não propagar trace à mão.

### Key — ordem por entidade

`Message.Key` define a partição no Kafka (hash) e o grupo FIFO no SQS. **Toda
mensagem cuja ordem importa por entidade leva `Key = <id da entidade>`.** Sem
key: distribuição arbitrária, sem ordem entre eventos da mesma entidade.

---

## Consumer

```go
cfg := msq.DefaultConsumeConfig(topic) // Concurrency=20, PollInterval=10s
cfg.GroupID = "{grupo}"                 // instâncias do mesmo grupo dividem o trabalho
cfg.InitialOffset = msq.OffsetResetEarliest
cfg.MaxRetries = 3
cfg.RetryBackoff = time.Second
cfg.DeadLetterTopic = topic + "-dlq"
cfg.HandlerTimeout = 30 * time.Second

mgr := msq.NewConsumerManager(broker) // broker vindo de mq.Broker()
if err := mgr.Register(cfg, handle).Start(); err != nil {
    return err // erros de criação dos consumers, juntados
}
defer mgr.Close() // para e espera os handlers em voo
```

- `Register(cfg, func(ctx, *msq.Message) (msq.Result, error))` ou
  `RegisterHandler(cfg, port.MessageHandler)`.
- `Start()` usa `cfg.Concurrency` de cada entry; `Dispatcher(n)` aplica `n`
  **só às entries com `Concurrency <= 0`** e então inicia. Ambos retornam
  `error` — **checar**. Registrar depois de iniciar exige nova chamada de
  `Start`/`Dispatcher` (só as entries novas sobem).
- **Armadilha:** `DefaultConsumeConfig` já seta `Concurrency = 20`, então
  `Dispatcher(n)` não muda nada para essa config. Concorrência se define em
  `cfg.Concurrency`.
- **Kafka ignora `Concurrency`**: paralelismo = partições atribuídas (uma
  goroutine por partição, sequencial dentro dela). Escala-se com partições +
  réplicas do grupo.
- Manager criado a partir de `mq.Broker()` é drenado pelo shutdown do serviço;
  `Close()` é idempotente.
- Projeto costuma encapsular o manager num wrapper dono do ciclo de vida
  (`consumer-bootstrap.md`, `worker-bootstrap.md`).

### Result — o que devolver

| Retorno | Significado |
|---|---|
| `msq.Ack, nil` | processado; commit/delete |
| `msq.Nack, err` | falha transitória; entra no retry do pipeline |
| `msq.Ignore, err\|nil` | descarte proposital (payload inválido, evento que não é deste consumer); **sem** retry |

Payload malformado → `Ignore` (retry não conserta). Decodificação:
`order, err := msq.UnpackMessage[T](msg)` ou `msg.DecodeMessage(&v)`.

### Retry e dead-letter (pipeline comum a todos os providers)

Todo consumer criado via `msq.New`/componente passa pelo mesmo pipeline:

1. Panic no handler vira `Nack`.
2. `Nack` → até `MaxRetries` novas tentativas **no processo**, backoff
   exponencial com jitter a partir de `RetryBackoff` (default 1s, teto 30s).
3. Esgotou e há `DeadLetterTopic` → publica cópia com headers
   `msq.HeaderDLQOriginalTopic` / `msq.HeaderDLQError` / `msq.HeaderDLQAttempts`
   e **dá Ack** no original.
4. Esgotou **sem** DLQ → `Nack` volta ao provider, e o efeito depende do broker:

| Broker | `Nack` final sem DLQ |
|---|---|
| Kafka | **offset avança, mensagem descartada** (só um log de erro) |
| RabbitMQ, SQS, OCI, NATS, Redis Streams | reentrega (requeue / visibility timeout / AckWait / ClaimIdle) |
| Redis Pub/Sub | perdida (at-most-once) |

**Regra:** consumer Kafka de evento/comando que não pode sumir **sempre** tem
`DeadLetterTopic`. DLQ é um tópico comum; consumí-lo (log/alerta/replay) é
outro `Register`.

`HandlerTimeout` limita cada tentativa. O handler roda em contexto que o
shutdown **não** cancela — trabalho em voo termina; o que para é a espera de
retry.

### Consumer group e offset inicial

- `GroupID` vazio → Kafka usa o nome do tópico; NATS o subject; Redis Streams o
  tópico. **Sempre nomear explicitamente** (convenção de nomes do projeto:
  `kafka-consumer-naming.md`).
- `InitialOffset` vale só na **primeira** execução do grupo (sem offset
  comitado). Default Kafka = `latest` — backlog existente é pulado.
  Tópico de comando/evento que não pode ser perdido → `msq.OffsetResetEarliest`.
- Renomear grupo em produção = grupo novo sem offset → aplica `InitialOffset`.

---

## Observabilidade

O pipeline já emite spans `send`/`process`, métricas `messaging.*` e propaga
W3C trace context pelos headers. `msq.Config{OnEvent: fn}` recebe
`msq.BrokerEvent` (hot path — barato); o componente usa por default um logger
dos eventos. Não duplicar spans/logs de envio/consumo no handler.

---

## `msq/worker` — não confundir com worker do projeto

`github.com/gofi-labs/gofi-sdk-go/msq/worker` é peça de baixo nível usada
pelos providers: `worker.New(n)` (pool limitado: `Enqueue`/`Wait`/`Close`),
`worker.Backoff{Min, Max}` (`Next`/`Reset`), `worker.Gate`
(`Pause`/`Resume`/`Wait`) e `worker.Sleep(ctx, d)`. Use-os ao escrever loop de
polling ou provider próprio. **Não** são o caminho para concorrência de
consumer (isso é `ConsumeConfig.Concurrency`) nem substituem o wrapper de
background worker do projeto (`worker-bootstrap.md`).

---

## Testes

- **Handler:** função pura sobre `*msq.Message` — monte a mensagem com
  `msq.NewMessageWithTopic(topic, payload)` e chame o handler direto; assert
  sobre `msq.Result` e sobre o mock do service. Sem broker.
- **Quem publica:** dependa de `msq.Producer` (interface) e injete fake
  handcraft que grava as mensagens.
- **`msq/msqtest`:** `msqtest.Run(t, msqtest.Target{Broker, Caps, Topic})` é o
  **contrato de provider** — usar só se o projeto implementar um `port.Broker`
  próprio.

---

## Anti-padrões

- ❌ `switch` de provider no código ou import de provider que o serviço não usa.
- ❌ Ignorar o `error` de `NewProducer`, `NewMessageWithTopic`, `Start`/`Dispatcher`.
- ❌ Consumer Kafka sem `DeadLetterTopic` para evento que não pode sumir.
- ❌ `Nack` para payload malformado (retry inútil) — use `Ignore`.
- ❌ Esperar que `Dispatcher(n)` mude a concorrência de config vinda de
  `DefaultConsumeConfig`.
- ❌ Retry manual (loop/sleep) dentro do handler — o pipeline já faz.
- ❌ Evento com ordem por entidade publicado sem `Key`.

---
name: consumer-bootstrap
description: Consumer de mensageria como gofi.Component dono do próprio ConsumerManager — Start monta dependências, registra e inicia; o gofi drena no shutdown; main só declara
sdk: v0.8.2
keywords: [consumer, ConsumerManager, NewConsumerManager, Register, Start, Dispatcher, Concurrency, messaging, msq, Component, drain, shutdown, wire]
---

# Consumer bootstrap — o consumer é um componente gofi

> **Especialização** de `worker-bootstrap.md` (worker dono do ciclo de vida;
> `main.go` só declara). Aqui o dono é um `gofi.Component` que possui o
> próprio `*msq.ConsumerManager`. Pipeline, retry, DLQ, `Result`, key e group:
> `messaging-msq.md`. Nomes de grupo: `kafka-consumer-naming.md`. Exemplo
> executável: `examples/msq/kafka/consumer` (e `rabbitmq`, `sqs`) —
> `.claude/sdk/go/api/examples.md`.

## Regra inviolável

**O consumer é um componente declarado no `With` e cria o próprio
`ConsumerManager` no `Start`.** O `main.go` não declara manager, não chama
`Register`/`Start`/`Dispatcher`/`Close`:

```go
mq := messaging.New() // MESSAGING_* + import _ ".../msq/provider/<nome>"

svc, err := gofi.New("{servico}").
    With(database.New(), mq, newOrderConsumer(mq, cfg)).
    Build()
```

Por que componente e não código depois do `Build`:

- `Start` roda no `Build` **depois** de banco e broker (estágio maior): o
  repositório e `mq.Broker()` já existem.
- Falha ao criar o consumer **falha o `Build`**, com rollback do que abriu.
- Manager criado de `mq.Broker()` é **drenado pelo gofi** no shutdown
  (espera os handlers em voo) **antes** de fechar broker, cache e banco. Não
  existe `Close` do consumer.
- Serviço só-consumer usa `svc.ListenAndServe()` normalmente: sem Runner, ele
  espera o sinal e depois drena.

**Um consumer = um manager.** Não compartilhar manager entre consumers:
concorrência e back-pressure são por workload.

---

## Template canônico

### Wrapper (`pathCmd/{topic}_consumer.go`)

```go
package main

import (
    "context"
    "time"

    "github.com/joaoprofile/gofi-sdk-go/gofi"
    "github.com/joaoprofile/gofi-sdk-go/gofi/component/messaging"
    "github.com/joaoprofile/gofi-sdk-go/msq"

    ordersvc "<module>/domain/{contexto}/service"
)

const (
    orderTopic = "{topic}"
    orderGroup = "{grupo}" // kafka-consumer-naming.md
)

type orderConsumer struct {
    mq  *messaging.Component
    cfg Config
    svc ordersvc.OrderService
}

func newOrderConsumer(mq *messaging.Component, cfg Config) *orderConsumer {
    return &orderConsumer{mq: mq, cfg: cfg}
}

func (c *orderConsumer) Name() string      { return "{contexto} consumer" }
func (c *orderConsumer) Stage() gofi.Stage { return gofi.StageServer }

// Start runs during Build, after database and messaging.
func (c *orderConsumer) Start(_ context.Context, _ *gofi.Runtime) error {
    if !c.cfg.OrderConsumerEnabled {
        return nil
    }
    c.svc = buildOrderService()

    cc := msq.DefaultConsumeConfig(orderTopic)
    cc.GroupID = orderGroup
    cc.Concurrency = c.cfg.OrderConcurrency
    cc.MaxRetries = 3
    cc.RetryBackoff = time.Second
    cc.DeadLetterTopic = orderTopic + "-dlq"

    // A manager built from the service broker is drained by gofi on shutdown.
    return msq.NewConsumerManager(c.mq.Broker()).Register(cc, c.handle).Start()
}

func (c *orderConsumer) handle(ctx context.Context, msg *msq.Message) (msq.Result, error) {
    // decode -> call the service -> classify the error (Ack/Nack/Ignore): messaging-msq.md
}
```

### `wire.go` — builder recebe o que precisa por parâmetro

```go
func buildOrderService() ordersvc.OrderService {
    return ordersvc.NewOrderService(orderrepo.NewOrderRepository())
}
```

> O wrapper recebe o `*messaging.Component` (não o broker): o broker só existe
> depois do `Start` do componente de mensageria. `main.go` continua sendo o
> único lugar que conhece todos os componentes.

---

## Multi-consumer no mesmo serviço

Um componente por consumer, uma linha no `With` cada:

```go
With(database.New(), mq, newOrderConsumer(mq, cfg), newPaymentConsumer(mq, cfg))
```

Cada um tem manager e concorrência próprios — um workload saturado não
segura o outro. DLQ consumido para log/alerta/replay é outro `Register` (no
mesmo componente do tópico de origem ou num próprio).

---

## Concorrência (`Concurrency` / `Dispatcher`)

- `ConsumeConfig.Concurrency` define os handlers em paralelo daquela entry;
  `Start()` usa esse valor. **`msq.DefaultConsumeConfig` já seta 20.**
- `Dispatcher(n)` só aplica `n` a entries com `Concurrency <= 0` — com
  `DefaultConsumeConfig` não muda nada. Prefira `Concurrency` + `Start()`.
- Kafka ignora `Concurrency`: paralelismo = partições atribuídas.
- Valor vem do `Config` do serviço (`{CONTEXTO}_CONSUMER_CONCURRENCY`, ver
  `env-vars-standard.md`). A spec só fixa default quando é decisão de domínio;
  senão o eng escolhe (1–4 para trabalho pesado em banco) e expõe por env.
- `Start()`/`Dispatcher()` devolvem os erros de criação juntos — **sempre**
  retornar do `Start` do componente.

---

## Shutdown

No SIGINT/SIGTERM o gofi para os Runners e fecha os recursos em ordem
reversa: o componente de mensageria **drena** todo manager criado do seu
broker (handlers em voo terminam; `HandlerTimeout` limita cada tentativa) e só
então fecha o broker; banco e cache fecham depois. Handler **nunca** fecha
nada — só decide `msq.Ack` / `msq.Nack` / `msq.Ignore`.

---

## Anti-padrões

### ❌ Manager no `main.go`

```go
// ANTI-PATTERN
mgr := msq.NewConsumerManager(mq.Broker())
mgr.Register(cfg, handler.Handle)
if err := mgr.Start(); err != nil { log.Fatal(err) } // skips Shutdown
defer mgr.Close()
```

Ciclo de vida dividido; `log.Fatal` depois do `Build` pula o fechamento; o
`main` carrega detalhe de workload (tópico, grupo, concorrência).

### ❌ Consumer criado com `mq.Broker()` antes do `Build`

`Broker()` é `nil` até o componente de mensageria iniciar.

### ❌ Manager sobre broker próprio fora do gofi

`msq.NewConsumerManager(kafka.New(...))` num serviço gofi cria um pipeline que
o shutdown não drena. Use o broker de `messaging.New(...)`. Com
`messaging.FromBroker(b)` o broker entra sem o pipeline e sem ser fechado pelo
gofi — o chamador drena os managers (`mgr.Close()`) e fecha o broker.

### ❌ `Register` público no wrapper / um manager para N consumers

Registro é interno ao `Start`, uma vez. Manager compartilhado mistura
concorrência e shutdown de workloads diferentes.

### ❌ Ignorar o erro de `Start()`

Consumer que não subiu passa despercebido até o backlog crescer.

---

## Checklist (gofi-eng)

- [ ] `{topic}_consumer.go` implementa `gofi.Component` (`Name`, `Stage`, `Start`) e entra no `With` depois do `messaging.New()`
- [ ] `Start` monta o service (via `wire.go`), cria `msq.NewConsumerManager(mq.Broker())`, `Register(...)` e `Start()` — e devolve o erro
- [ ] `main.go` sem `ConsumerManager`, sem `Register`/`Start`/`Dispatcher`/`Close`
- [ ] Concorrência em `ConsumeConfig.Concurrency`, vinda de config
- [ ] `GroupID` explícito e, em evento que não pode sumir, `DeadLetterTopic` (`messaging-msq.md`)
- [ ] Um manager por consumer
- [ ] Teste de `handle` monta `&orderConsumer{svc: fakeService}` direto (sem broker) e chama `handle` com `msq.NewMessageWithTopic`

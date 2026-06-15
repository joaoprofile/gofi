# Kafka consumer bootstrap — wrapper owns ConsumerManager lifecycle

> **Especialização** do princípio geral de
> `.claude/sdk/go/knowledge/worker-bootstrap.md` (wrapper de background worker
> é dono do ciclo de vida; `main.go` só vê `build → defer Close`). Esta
> página cobre o caso específico de consumer Kafka, que tem
> `*msq.ConsumerManager` e exige ordem `Register → Dispatcher` no
> constructor — esses dois detalhes não generalizam para outros workers.
>
> **Quando aplicar:** o serviço (`pathCmd`) tem **um ou mais** consumers Kafka
> (via `gofi/msq`). Esta regra define como o `wire.go` / `main.go` montam o
> consumer e quem possui o `ConsumerManager`.

## Regra inviolável

**O struct consumer possui o próprio `*msq.ConsumerManager`.** O `main.go` **não**
declara `consumerManager` em variável local nem chama `Dispatcher` / `Close` no
manager diretamente. O ciclo de vida do manager (criação + `Register(...)` +
`Dispatcher(n)` + `Close`) vive **dentro** do wrapper consumer; `main.go`
enxerga apenas `buildXxxConsumer(...)` e `defer xxx.Close()`.

**Cada consumer tem seu próprio manager** (1 wrapper = 1 manager dedicado).
Não compartilhar um `ConsumerManager` entre múltiplos consumers — concorrência
(`Dispatcher(n)`) é decisão por workload, e Close compartilhado borra a
fronteira de propriedade.

> **Atenção à ordem `Register` → `Dispatcher`**: `Dispatcher(n)` aplica a
> concorrência nas entries **já registradas** e em seguida **chama
> `start()` internamente**. Registrar depois de `Dispatcher` significa
> consumer iniciado sem entries. Por isso o constructor sempre faz
> `Register(...)` antes de `Dispatcher(n)`, e **não há método `Start`
> separado** — o start é implícito.

---

## Template canônico

### Wrapper consumer (`pathCmd/{topic}_consumer.go`)

```go
package main

import (
    "context"

    "github.com/joaoprofile/gofi/msq"

    "{module}/services/common/kafka"
    fooSvcPkg "{module}/services/domain/{ctx}/service"
)

const {topic}ConsumerGroup = "{ctx}"

type {Topic}Consumer struct {
    fooSvc  fooSvcPkg.FooService
    manager *msq.ConsumerManager
}

func New{Topic}Consumer(broker msq.Messaging, concurrency int, fooSvc fooSvcPkg.FooService) *{Topic}Consumer {
    mgr := msq.NewConsumerManager(broker)
    c := &{Topic}Consumer{fooSvc: fooSvc, manager: mgr}
    mgr.Register(kafka.SyncConsumer({topic}ConsumerGroup), c.handle)
    mgr.Dispatcher(concurrency)
    return c
}

func (c *{Topic}Consumer) Close() { c.manager.Close() }

func (c *{Topic}Consumer) handle(ctx context.Context, msg *msq.Message) (msq.Result, error) {
    // decodificar → despachar p/ service → classificar erro (Ack/Nack) — ver msq.md
}
```

### `wire.go` — builder recebe broker + cfg

```go
func build{Topic}Consumer(ctx context.Context, broker msq.Messaging, concurrency int) *{Topic}Consumer {
    fooRepo := fooRepoPkg.NewFooRepository(ctx)
    fooSvc  := fooSvcPkg.NewFooService(fooRepo)
    return New{Topic}Consumer(broker, concurrency, fooSvc)
}
```

> O builder **recebe** `broker` e `concurrency` por parâmetro — não lê
> `service.Messaging()` nem `cfg.XxxConcurrency` do escopo do `main.go`.
> Mantém `wire.go` testável e o `main.go` como único composition root.

### `main.go` — duas linhas por consumer

```go
{topic}Consumer := build{Topic}Consumer(ctx, service.Messaging(), cfg.{Topic}Concurrency)
defer {topic}Consumer.Close()
```

**Não existe** `consumerManager := msq.NewConsumerManager(...)` em `main.go`.
**Não existe** `consumerManager.Dispatcher(...)` em `main.go`. **Não existe**
`xxxConsumer.Register(consumerManager)` em `main.go` — registro é interno ao
constructor. Como `Dispatcher(n)` chama `start()` internamente, o consumer
já está consumindo ao retornar de `buildXxxConsumer`; nenhum `Start`
adicional é necessário.

---

## Multi-consumer no mesmo serviço

Cada consumer tem seu próprio manager. `main.go` repete o par
`build → defer Close` por consumer:

```go
bulkConsumer := buildBulkIngestConsumer(ctx, service.Messaging(), cfg.BulkConcurrency)
defer bulkConsumer.Close()

decisionConsumer := buildDecisionExecutorConsumer(ctx, service.Messaging(), cfg.DecisionConcurrency)
defer decisionConsumer.Close()
```

Vantagem: cada workload tem dispatcher próprio (concorrência independente,
back-pressure isolada, falha de um não bloqueia o outro no shutdown).

---

## Anti-padrões

### ❌ `ConsumerManager` em variável de `main.go`

```go
// ANTI-PADRÃO
consumerManager := msq.NewConsumerManager(service.Messaging())
consumerManager.Dispatcher(cfg.XxxConcurrency)
defer consumerManager.Close()

xxxConsumer := buildXxxConsumer(ctx)
xxxConsumer.Register(consumerManager)   // wrapper depende de manager externo
defer xxxConsumer.Close()               // duplo defer, ordem frágil
```

Problemas:
- Lifecycle dividido: `main` cria/fecha manager, wrapper cria/fecha o resto — ordem de `defer` importa.
- Concorrência (`Dispatcher(n)`) é decisão do workload, não do bootstrap genérico — vaza pro `main`.
- Adicionar um segundo consumer obriga compartilhar o manager OU duplicar a cerimônia — ambos ruins.
- Em código real esse layout induz bug clássico: usar `consumerManager` antes de declará-lo (Go bloqueia, mas o pattern convida ao erro).

### ❌ Builder que lê `service.Messaging()` global

```go
// ANTI-PADRÃO — wire.go chamando service global
func buildXxxConsumer(ctx context.Context) *XxxConsumer {
    broker := globalService.Messaging() // dependência implícita
    ...
}
```

Builder deve receber `broker` por parâmetro. Único lugar que conhece o
`service` é `main.go`.

### ❌ `Register(manager *msq.ConsumerManager)` público no wrapper

```go
// ANTI-PADRÃO
func (c *XxxConsumer) Register(manager *msq.ConsumerManager) {
    manager.Register(kafka.SyncConsumer(group), c.handle)
}
```

Expõe ao `main.go` o detalhe de que existe um manager. O registro é
**responsabilidade interna do constructor** — feito uma única vez quando
o wrapper é criado. O wrapper não deve poder ser "registrado" em mais de um
manager nem re-registrado.

### ❌ Um `ConsumerManager` para N consumers

```go
// ANTI-PADRÃO
mgr := msq.NewConsumerManager(broker)
mgr.Dispatcher(cfg.SharedConcurrency)
mgr.Register(topicA, consumerA.handle)
mgr.Register(topicB, consumerB.handle)
defer mgr.Close()
```

Concorrência fica compartilhada (workload A satura → B sofre); shutdown
compartilhado (Close de um implica Close do outro). Manter 1:1 wrapper↔manager.

---

## Concorrência (`Dispatcher`)

`Dispatcher(n)` define quantas mensagens são processadas em paralelo dentro
desse manager. Valor vem de `cfg.{Topic}Concurrency` (env var do serviço,
ver `env-vars-standard.md`). Spec só declara o **default** quando é decisão
de domínio (ex: "bulk import precisa de N=8 para throughput X"); caso contrário,
o eng escolhe um default razoável (1–4) e expõe via env.

---

## Shutdown

`defer xxxConsumer.Close()` no `main.go` garante que ao receber sinal de
término o manager dreina mensagens em voo e fecha as conexões com o broker.
**Não chamar `Close` no `handle`** — Close é responsabilidade do owner
(main, via defer). Handler só decide `Ack` / `Nack` / `Ignore`.

---

## Checklist (gofi-eng)

- [ ] `{topic}_consumer.go` tem campo `manager *msq.ConsumerManager`
- [ ] Constructor cria `msq.NewConsumerManager(broker)` + `Register(...)` + `Dispatcher(concurrency)` — **nessa ordem**, tudo dentro
- [ ] `Close()` delega para `c.manager.Close()`
- [ ] `wire.go` recebe `broker msq.Messaging` e `concurrency int` por parâmetro
- [ ] `main.go` tem o par `build → defer Close` por consumer — **zero** `consumerManager` em scope, **zero** chamada a `Start` separada (Dispatcher já inicia)
- [ ] Cada consumer no serviço tem manager próprio (1:1)
- [ ] Testes que invocam só `handle` constroem `&{Topic}Consumer{...}` direto (struct literal no `package main` de teste), sem passar pelo constructor — evita dependência de broker real no test

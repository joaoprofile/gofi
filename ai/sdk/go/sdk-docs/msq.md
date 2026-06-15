# gofi/msq — Mensageria

## Variáveis de Ambiente

| Variável | Descrição |
|----------|-----------|
| `MESSAGING_PROVIDER` | `rabbitmq`, `kafka`, `sqs`, `oci`, `redis` |
| `MESSAGING_USER` | Usuário do broker |
| `MESSAGING_PASSWORD` | Senha do broker |
| `MESSAGING_HOST` | Host do broker |
| `MESSAGING_PORT` | Porta (RabbitMQ: `5672`, Kafka: `9092`) |

## Brokers Suportados

```go
import "github.com/joaoprofile/gofi/msq"

msq.BrokerKafka    // "kafka"
msq.BrokerRabbitMQ // "rabbitmq"
msq.BrokerSQS      // "sqs"
msq.BrokerOCI      // "oci"
msq.BrokerRedis    // "redis"
```

## Configuração e Build

```go
// Via BrokerType (lê env vars automaticamente)
cfg := msq.Config{
    BrokerType: msq.BrokerKafka,
}

// Via instância explícita do broker
cfg := msq.Config{
    Broker: myCustomBroker,
}

// Via env var MESSAGING_PROVIDER (auto-detect)
cfg := msq.Config{}
```

## Producer

```go
broker, err := cfg.Factory.Build(ctx) // ou via Builder gofi
producer := broker.NewProducer()
defer producer.Close()

msg := msq.NewMessage(myStruct)       // serializa como JSON
msg.Topic = "persons.created"

// Com topic inline
msg = msq.NewMessageWithTopic("persons.created", myStruct)

err = producer.Send(ctx, msg)
```

## Consumer

```go
broker, _ := cfg.Factory.Build(ctx)

consumer := broker.NewConsumer(msq.ConsumeConfig{
    Topic:       "persons.created",
    Group:       "my-service",
    Concurrency: 3,
})
defer consumer.Close()
```

## ConsumerManager

```go
mgr := msq.NewConsumerManager(broker)

mgr.Register(msq.ConsumeConfig{Topic: "persons.created"}, func(ctx context.Context, msg *msq.Message) msq.Result {
    order, err := msq.UnpackMessage[PersonCreatedEvent](msg)
    if err != nil {
        return msq.Nack
    }
    if err := processEvent(ctx, order); err != nil {
        return msq.Nack
    }
    return msq.Ack
})

// Iniciar todos os consumers
mgr.Start(ctx)

// Ou como dispatcher (blocks até ctx cancelar)
mgr.Dispatcher(ctx)
```

## Result Constants

```go
msq.Ack    // mensagem processada com sucesso — remove da fila
msq.Nack   // falha no processamento — requeue ou dead-letter
msq.Ignore // ignora a mensagem — sem requeue
```

## UnpackMessage

```go
type PersonCreatedEvent struct {
    PersonID string `json:"personId"`
    Name     string `json:"name"`
}

event, err := msq.UnpackMessage[PersonCreatedEvent](msg)
if err != nil {
    return msq.Nack
}
// event é *PersonCreatedEvent
```

## Interfaces

```go
// port.Producer
type Producer interface {
    Send(ctx context.Context, msg *Message) error
    Close() error
}

// port.Consumer
type Consumer interface {
    Consume(ctx context.Context, handler MessageHandler) error
    Close() error
}

// port.MessageHandler
type MessageHandler interface {
    Handle(ctx context.Context, msg *Message) Result
}

// port.MessageHandlerFunc (adapta func)
type MessageHandlerFunc func(ctx context.Context, msg *Message) Result
```

## Setup de Infraestrutura (RabbitMQ, etc.)

```go
// Brokers que precisam declarar exchange/tópicos antes de usar
if setup, ok := broker.(msq.BrokerSetup); ok {
    if err := setup.Setup(ctx); err != nil {
        logging.Fatal("broker setup failed", slog.Any("error", err))
    }
}
```

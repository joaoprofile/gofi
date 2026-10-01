# msq/port

`import "github.com/joaoprofile/gofi-sdk-go/msq/port"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## port.Broker

```go
type Broker interface {
	// NewProducer returns a ready-to-use Producer.
	// It returns a non-nil error when the producer cannot be created
	// (e.g. the broker is unreachable); callers must not ignore it.
	// Callers must call Producer.Close() when done.
	NewProducer() (Producer, error)

	// NewConsumer returns a Consumer configured for the given ConsumeConfig.
	// Callers must call Consumer.Close() when done.
	NewConsumer(cfg types.ConsumeConfig) (Consumer, error)
}
```

Broker is the central port that every messaging provider must implement.
It is the only interface callers need to hold in order to create producers and consumers.

## port.BrokerSetup

```go
type BrokerSetup interface {
	Setup(ctx context.Context) error
}
```

BrokerSetup is implemented by brokers that require infrastructure setup before use
(e.g. declaring a RabbitMQ exchange, or creating a Kafka topic).
Call Setup once during application bootstrap.

## port.Consumer

```go
type Consumer interface {
	// Consume blocks and delivers messages to handler until ctx is cancelled.
	// Returns a non-nil error only for unrecoverable failures; cancellation returns nil.
	Consume(ctx context.Context, handler MessageHandler) error

	// Close stops message delivery and releases all resources.
	Close() error

	// Pause temporarily halts message delivery without closing the consumer.
	// Useful for backpressure and rate-limiting scenarios.
	Pause() error

	// Resume restores message delivery after a Pause.
	Resume() error
}
```

Consumer receives and processes messages from a broker topic or queue.

## port.MessageHandler

```go
type MessageHandler interface {
	Handle(ctx context.Context, msg *types.Message) (types.Result, error)
}
```

MessageHandler is the interface for processing broker messages.

Return semantics:
  - (Ack, nil)    — message processed successfully; commit offset or delete from queue
  - (Nack, err)   — processing failed; requeue if the broker supports it
  - (Ignore, nil) — message discarded on purpose; removed without requeue

## port.MessageHandlerFunc

```go
type MessageHandlerFunc func(ctx context.Context, msg *types.Message) (types.Result, error)
```

MessageHandlerFunc is a func type that implements MessageHandler.
Allows existing handler functions to satisfy the interface with zero wrapping.

Example:

	manager.Register(cfg, msq.MessageHandlerFunc(myHandlerFunc))

### MessageHandlerFunc.Handle

```go
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *types.Message) (types.Result, error)
```

## port.Producer

```go
type Producer interface {
	// SendMessage sends a single message. msg.Topic must be set.
	SendMessage(ctx context.Context, msg *types.Message) error

	// SendMessagesBatch sends multiple messages.
	// Providers that support native batching (SQS, Kafka) will use it;
	// others will iterate sequentially.
	SendMessagesBatch(ctx context.Context, msgs []*types.Message) error

	// Close releases all resources held by this Producer.
	Close() error
}
```

Producer sends messages to a broker topic or queue.
A single Producer instance should not be shared across goroutines without
external synchronization; prefer creating one Producer per goroutine or use
the batch API for bulk operations.


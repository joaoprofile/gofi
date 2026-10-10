# msq/core

`import "github.com/joaoprofile/gofi-sdk-go/msq/core"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package core implements the orchestration layer of the msq messaging system.
It depends only on port interfaces and types — never on provider implementations.

## Constantes

```go
const (
	HeaderDLQOriginalTopic = "x-gofi-dlq-original-topic"
	HeaderDLQError         = "x-gofi-dlq-error"
	HeaderDLQAttempts      = "x-gofi-dlq-attempts"
	// HeaderDLQDeliveries is the broker delivery count of the dead message.
	HeaderDLQDeliveries = "x-gofi-dlq-deliveries"
)
```

Dead-letter headers added to the copy published to ConsumeConfig.DeadLetterTopic.

```go
const DefaultHealthGrace = 30 * time.Second
```

DefaultHealthGrace is how long a consumer may be down (restarting) before
Healthy reports it.

## Variáveis

```go
var (
	// ErrBrokerRequired is returned when New() is called without a Broker provider.
	ErrBrokerRequired = errors.New("msq: broker provider is required")

	// ErrTopicRequired is returned when a ConsumeConfig has an empty Topic.
	ErrTopicRequired = errors.New("msq: ConsumeConfig.Topic must not be empty")

	// ErrConsumerFailed is returned when a consumer exits with an unrecoverable error.
	ErrConsumerFailed = errors.New("msq: consumer failed")
)
```

```go
var (
	// ErrManagerClosed is returned by Start and Healthy after Close.
	ErrManagerClosed = errors.New("msq: consumer manager closed")

	// ErrConsumerDown is reported by Healthy for a consumer that stays down.
	ErrConsumerDown = errors.New("msq: consumer down")
)
```

## core.BrokerService

```go
type BrokerService struct {
	// contains filtered or unexported fields
}
```

BrokerService is the central facade of the msq package, obtained via msq.New.

Producers and consumers it creates share one pipeline: trace propagation,
OpenTelemetry spans and metrics, OnEvent events, handler panic recovery,
retries, dead-lettering and a shutdown that lets in-flight handlers finish.

### core.NewService

```go
func NewService(cfg ServiceConfig) *BrokerService
```

NewService builds a BrokerService from validated configuration.
Use msq.New instead of calling this directly.

### BrokerService.Close

```go
func (s *BrokerService) Close() error
```

Close drains every ConsumerManager created from this service, waiting for
in-flight handlers. It does not close the underlying broker. Prefer
CloseContext to bound the wait.

### BrokerService.CloseContext

```go
func (s *BrokerService) CloseContext(ctx context.Context) error
```

CloseContext drains every ConsumerManager created from this service; gofi's
builder calls it with the shutdown context before closing the broker and
the database/cache pools. When ctx ends first it returns an error wrapping
ctx.Err(). It does not close the underlying broker.

### BrokerService.Healthy

```go
func (s *BrokerService) Healthy() error
```

Healthy returns nil while every consumer of every ConsumerManager created
from this service is running (see ConsumerManager.Healthy). Wire it into
the readiness check.

### BrokerService.NewConsumer

```go
func (s *BrokerService) NewConsumer(cfg types.ConsumeConfig) (port.Consumer, error)
```

NewConsumer returns a Consumer whose handler runs through the pipeline.
With DeadLetterTopic set it also owns a producer for dead letters.

### BrokerService.NewConsumerManager

```go
func (s *BrokerService) NewConsumerManager() *ConsumerManager
```

NewConsumerManager returns a ConsumerManager backed by this service; Close
drains it.

### BrokerService.NewProducer

```go
func (s *BrokerService) NewProducer() (port.Producer, error)
```

NewProducer returns an instrumented Producer from the underlying broker.

## core.ConsumeDefaults

```go
type ConsumeDefaults struct {
	MaxDeliveries  int
	HandlerTimeout time.Duration
}
```

ConsumeDefaults are service-wide values for ConsumeConfig fields a consumer
leaves at zero (e.g. from MESSAGING_* variables).

## core.ConsumerManager

```go
type ConsumerManager struct {
	// contains filtered or unexported fields
}
```

ConsumerManager orchestrates multiple consumers against a single BrokerService.
Register all consumers before calling Start or Dispatcher.

A consumer whose Consume fails (or returns before shutdown) is recreated
and restarted with exponential backoff until the manager closes; Healthy
reports consumers that stay down longer than the health grace.

### core.NewConsumerManager

```go
func NewConsumerManager(broker port.Broker) *ConsumerManager
```

NewConsumerManager creates a ConsumerManager backed by the given Broker.
A plain provider broker is wrapped in a BrokerService so every consumer
runs through the pipeline.

### ConsumerManager.Close

```go
func (m *ConsumerManager) Close()
```

Close signals all consumers to stop and waits until in-flight handlers
return. It is idempotent: a second caller (e.g. BrokerService.Close) blocks
until the drain finishes, which keeps DB/cache pools alive meanwhile.
Handlers are bounded by ConsumeConfig.HandlerTimeout; use CloseContext to
bound the wait itself.

### ConsumerManager.CloseContext

```go
func (m *ConsumerManager) CloseContext(ctx context.Context) error
```

CloseContext is Close bounded by ctx: when ctx ends before the drain
finishes it returns an error wrapping ctx.Err() while consumers keep
draining in the background. Calling it again waits for the same drain.

### ConsumerManager.Dispatcher

```go
func (m *ConsumerManager) Dispatcher(concurrency int) error
```

Dispatcher sets the default concurrency for entries without one and starts
the consumers registered since the last start. It returns the joined errors
of consumers that could not be created; those are retried in the
background and the others keep running.

### ConsumerManager.Healthy

```go
func (m *ConsumerManager) Healthy() error
```

Healthy returns nil while every started consumer is running. A consumer
down (failed and restarting) for longer than the health grace, or a
closed manager, is reported. Wire it into the readiness check.

### ConsumerManager.Register

```go
func (m *ConsumerManager) Register(cfg types.ConsumeConfig, handler func(ctx context.Context, msg *types.Message) (types.Result, error)) *ConsumerManager
```

Register adds a consumer for the given topic configuration.
The handler func is automatically adapted to the MessageHandler interface.

### ConsumerManager.RegisterHandler

```go
func (m *ConsumerManager) RegisterHandler(cfg types.ConsumeConfig, handler port.MessageHandler) *ConsumerManager
```

RegisterHandler adds a consumer using the full MessageHandler interface.
Use when your handler is a struct that implements port.MessageHandler.

### ConsumerManager.SetHealthGrace

```go
func (m *ConsumerManager) SetHealthGrace(d time.Duration) *ConsumerManager
```

SetHealthGrace sets how long a consumer may be down before Healthy reports
it (default DefaultHealthGrace).

### ConsumerManager.Shutdown

```go
func (m *ConsumerManager) Shutdown()
```

Shutdown is an alias for Close.

### ConsumerManager.Start

```go
func (m *ConsumerManager) Start() error
```

Start launches the consumers registered since the last start, using the
concurrency in each ConsumeConfig. It returns the joined creation errors;
failed consumers are retried in the background. After Close it returns
ErrManagerClosed.

## core.ServiceConfig

```go
type ServiceConfig struct {
	Broker port.Broker
	// System is the messaging.system attribute (kafka, rabbitmq, aws_sqs, ...).
	System  string
	OnEvent func(ctx context.Context, event types.BrokerEvent)
	// Defaults fill ConsumeConfig fields left at zero.
	Defaults ConsumeDefaults
}
```

ServiceConfig holds the internal parameters for building a BrokerService.


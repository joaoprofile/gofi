# gofi/component/messaging

`import "github.com/gofi-labs/gofi-sdk-go/gofi/component/messaging"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package messaging is the gofi component for the message broker (msq).

## Funções

### messaging.ConfigFromEnv

```go
func ConfigFromEnv(env *environment.Environment) msq.ProviderConfig
```

ConfigFromEnv builds an msq.ProviderConfig from the MESSAGING_* variables; the
Redis provider reuses the CACHE_* connection. Open it with msq.Open after
importing the provider package (msq/provider/<type>).

## messaging.Component

```go
type Component struct {
	// contains filtered or unexported fields
}
```

Component opens the broker and wraps it in the msq pipeline.

### messaging.FromBroker

```go
func FromBroker(b msq.Broker) *Component
```

FromBroker uses a broker built and owned by the caller as is: it is not
wrapped, set up or closed by gofi.

### messaging.New

```go
func New(cfg ...msq.Config) *Component
```

New registers a messaging provider (RabbitMQ, Kafka, SQS, OCI, Redis, NATS).

Three calling patterns, in order of ergonomics:

	// 1 — zero-config: reads MESSAGING_PROVIDER from env
	messaging.New()

	// 2 — explicit type, credentials from env
	messaging.New(msq.Config{BrokerType: msq.BrokerKafka})

	// 3 — explicit broker instance (full control / custom config)
	messaging.New(msq.Config{Broker: myBroker})

Patterns 1 and 2 need the provider package imported, which keeps unused
broker SDKs out of the binary:

	import _ "github.com/gofi-labs/gofi-sdk-go/msq/provider/kafka"

If the broker implements port.BrokerSetup, Setup is called during Build so
that exchanges, topics or queues are declared before the first producer or
consumer.

### Component.Broker

```go
func (c *Component) Broker() msq.Broker
```

Broker returns the broker to create producers and consumers; nil before Build.

### Component.Name

```go
func (c *Component) Name() string
```

### Component.Stage

```go
func (c *Component) Stage() gofi.Stage
```

### Component.Start

```go
func (c *Component) Start(ctx context.Context, rt *gofi.Runtime) error
```


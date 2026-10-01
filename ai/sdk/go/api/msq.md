# msq

`import "github.com/gofi-labs/gofi-sdk-go/msq"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	EventMessageSent         = types.EventMessageSent
	EventMessageReceived     = types.EventMessageReceived
	EventMessageAcked        = types.EventMessageAcked
	EventMessageNacked       = types.EventMessageNacked
	EventMessageDeadLettered = types.EventMessageDeadLettered
	EventConsumerStarted     = types.EventConsumerStarted
	EventConsumerStopped     = types.EventConsumerStopped
	EventProducerError       = types.EventProducerError
	EventConsumerError       = types.EventConsumerError
)
```

Event types re-exported at package level.

```go
const (
	HeaderDLQOriginalTopic = core.HeaderDLQOriginalTopic
	HeaderDLQError         = core.HeaderDLQError
	HeaderDLQAttempts      = core.HeaderDLQAttempts
)
```

Dead-letter headers set on the copy published to ConsumeConfig.DeadLetterTopic.

```go
const (
	EncodingEnvelope    = types.EncodingEnvelope
	EncodingCloudEvents = types.EncodingCloudEvents
)
```

Wire formats re-exported at package level.

```go
const (
	Ack    = types.Ack
	Nack   = types.Nack
	Ignore = types.Ignore
)
```

Result constants re-exported at package level.

```go
const (
	DefaultConcurrency  = types.DefaultConcurrency
	DefaultPollInterval = types.DefaultPollInterval
)
```

Consumer concurrency / polling defaults re-exported at package level.

```go
const (
	OffsetResetDefault  = types.OffsetResetDefault
	OffsetResetEarliest = types.OffsetResetEarliest
	OffsetResetLatest   = types.OffsetResetLatest
)
```

OffsetReset values re-exported so callers import only this package.

## Funções

### msq.New

```go
func New(cfg Config) (*core.BrokerService, error)
```

New builds a BrokerService. Broker must be non-nil.

### msq.NewConsumerManager

```go
func NewConsumerManager(broker port.Broker) *core.ConsumerManager
```

NewConsumerManager creates a ConsumerManager backed by the given Broker.
Register consumers with Register, then call Start or Dispatcher.

### msq.Register

```go
func Register(t BrokerType, o Opener)
```

Register makes a provider available to Open. Provider packages call it from
init, so importing one (e.g. _ ".../msq/provider/kafka") enables it and only
imported providers are linked into the binary.

### msq.UnpackMessage

```go
func UnpackMessage[T any](message *Message) (*T, error)
```

UnpackMessage decodes the message Value into T using type inference.

	order, err := msq.UnpackMessage[Order](msg)

## msq.Broker

```go
type Broker = port.Broker
```

Broker is the central port every messaging provider must implement.

### msq.Open

```go
func Open(ctx context.Context, cfg ProviderConfig) (Broker, error)
```

Open builds the broker for cfg.Type. It fails when the type is not set or
its provider package was not imported.

## msq.BrokerEvent

```go
type BrokerEvent = types.BrokerEvent
```

BrokerEvent carries observability payloads emitted during broker lifecycle.

## msq.BrokerEventType

```go
type BrokerEventType = types.BrokerEventType
```

BrokerEventType identifies the kind of BrokerEvent.

## msq.BrokerSetup

```go
type BrokerSetup = port.BrokerSetup
```

BrokerSetup is implemented by brokers that require infrastructure setup
before producing or consuming (e.g. RabbitMQ exchange declaration).

## msq.BrokerType

```go
type BrokerType string
```

BrokerType identifies a messaging provider so that AddMessaging can build
the broker automatically from environment variables (see Register).
Values are intentionally lowercase strings to match MESSAGING_PROVIDER env values.

```go
const (
	BrokerKafka    BrokerType = "kafka"
	BrokerRabbitMQ BrokerType = "rabbitmq"
	BrokerSQS      BrokerType = "sqs"
	BrokerOCI      BrokerType = "oci"
	BrokerRedis    BrokerType = "redis"
	BrokerNATS     BrokerType = "nats"
)
```

## msq.Config

```go
type Config struct {
	// BrokerType instructs AddMessaging to build the broker from env vars.
	// Ignored when Broker is set explicitly.
	BrokerType BrokerType

	// Exchange is the AMQP exchange name used by the RabbitMQ provider.
	// Ignored by all other providers. Defaults to "" (AMQP default exchange).
	Exchange string

	// Broker is an explicit provider instance.
	// Use when you need configuration beyond what environment variables provide.
	Broker port.Broker

	// System is the OpenTelemetry messaging.system attribute; defaults to the
	// semantic-convention value of BrokerType.
	System string

	// OnEvent is called for every message and consumer lifecycle event. It runs
	// on the hot path, so keep it cheap. Optional.
	OnEvent func(ctx context.Context, event types.BrokerEvent)
}
```

Config configures a messaging provider for use with the GOFI builder.

There are three ways to supply the broker, in order of precedence:

 1. Broker — explicit port.Broker instance (full control / custom config).
 2. BrokerType — builds the broker from environment variables automatically.
 3. Neither — AddMessaging reads MESSAGING_PROVIDER from env and auto-selects.

## msq.ConsumeConfig

```go
type ConsumeConfig = types.ConsumeConfig
```

ConsumeConfig configures a consumer, regardless of broker.

### msq.DefaultConsumeConfig

```go
func DefaultConsumeConfig(topic string) ConsumeConfig
```

DefaultConsumeConfig returns a ConsumeConfig with sensible defaults for the
given topic.

## msq.Consumer

```go
type Consumer = port.Consumer
```

Consumer receives and processes messages from a broker.

## msq.ConsumerManager

```go
type ConsumerManager = core.ConsumerManager
```

ConsumerManager orchestrates multiple consumers against a single broker.
Obtain via NewConsumerManager or BrokerService.NewConsumerManager().

## msq.Encoding

```go
type Encoding = types.Encoding
```

Encoding selects the wire format of providers that support more than one.

## msq.Message

```go
type Message = types.Message
```

Message is the universal broker envelope.

### msq.NewMessage

```go
func NewMessage(value any) (*Message, error)
```

NewMessage creates a Message with the payload serialized as JSON.
Set Topic via WithTopic or directly before sending.

### msq.NewMessageWithTopic

```go
func NewMessageWithTopic(topic string, value any) (*Message, error)
```

NewMessageWithTopic creates a Message with topic and payload already set.

## msq.MessageHandler

```go
type MessageHandler = port.MessageHandler
```

MessageHandler processes a single broker message.

## msq.MessageHandlerFunc

```go
type MessageHandlerFunc = port.MessageHandlerFunc
```

MessageHandlerFunc adapts a plain function to MessageHandler.

## msq.OCICredentials

```go
type OCICredentials struct {
	AuthMode    string
	Region      string
	TenancyID   string
	UserID      string
	Fingerprint string
	PrivateKey  string
}
```

OCICredentials configures the OCI Queue provider; see base/cloud/oci.

## msq.OffsetReset

```go
type OffsetReset = types.OffsetReset
```

OffsetReset controls where a consumer group starts with no committed offset.

## msq.Opener

```go
type Opener func(ctx context.Context, cfg ProviderConfig) (Broker, error)
```

Opener builds a Broker from a ProviderConfig.

## msq.Producer

```go
type Producer = port.Producer
```

Producer sends messages to a broker.

## msq.ProviderConfig

```go
type ProviderConfig struct {
	// Type selects the registered provider.
	Type BrokerType

	// Addr is the broker host:port (Kafka seed, AMQP or NATS server).
	Addr     string
	User     string
	Password string
	UseTLS   bool

	// ClientName identifies the connection on the server (NATS).
	ClientName string
	// SASLMechanism selects Kafka SASL: PLAIN (default), SCRAM-SHA-256 or SCRAM-SHA-512.
	SASLMechanism string
	// Exchange is the AMQP exchange (RabbitMQ).
	Exchange string
	// Encoding is the wire format (RabbitMQ); empty means envelope.
	Encoding Encoding

	Redis RedisConnection
	OCI   OCICredentials
}
```

ProviderConfig is the provider-neutral connection that Open hands to a
registered provider. The root config package fills it from the MESSAGING_*
variables; each provider reads only its own fields.

## msq.RedisConnection

```go
type RedisConnection struct {
	Addr     string
	Password string
	UseTLS   bool
	// Mode is pubsub (default) or streams.
	Mode string
}
```

RedisConnection configures the Redis provider.

## msq.Result

```go
type Result = types.Result
```

Result signals the broker how to handle a processed message.


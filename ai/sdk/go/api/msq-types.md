# msq/types

`import "github.com/gofi-labs/gofi-sdk-go/msq/types"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	DefaultConcurrency  = 20
	DefaultPollInterval = 10 * time.Second
)
```

```go
const (
	DefaultEventType = "gofi.message"
)
```

Default CloudEvents attributes for messages that do not set them.

## Variáveis

```go
var (
	// KafkaBinding follows the CloudEvents Kafka protocol binding.
	KafkaBinding = Binding{Prefix: "ce_", ContentType: "content-type"}
	// AMQPBinding follows the CloudEvents AMQP binding; the content type goes
	// in the AMQP property instead of a header.
	AMQPBinding = Binding{Prefix: "cloudEvents_", KeyInHeaders: true}
	// NATSBinding follows the CloudEvents NATS binding (headers like HTTP).
	NATSBinding = Binding{Prefix: "ce-", ContentType: "content-type", KeyInHeaders: true}
)
```

CloudEvents binary-mode bindings.

## Funções

### types.UnpackMessage

```go
func UnpackMessage[T any](message *Message) (*T, error)
```

UnpackMessage decodes Value into T using type inference.

## types.Binding

```go
type Binding struct {
	// Prefix of the attribute headers (ce_, cloudEvents_).
	Prefix string
	// ContentType is the header carrying datacontenttype; empty when the
	// transport has a native property for it.
	ContentType string
	// KeyInHeaders writes Message.Key as the partitionkey extension.
	KeyInHeaders bool
}
```

Binding maps a Message to CloudEvents binary mode for one transport.

### Binding.Decode

```go
func (b Binding) Decode(body []byte, headers map[string]string) Message
```

Decode rebuilds a Message from a binary-mode body and headers. CloudEvents
attributes are removed from Headers; a non-UUID id maps to a stable UUID.

### Binding.Encode

```go
func (b Binding) Encode(m *Message) ([]byte, map[string]string)
```

Encode returns the body and headers of m. User headers are kept as is.

### Binding.IsBinary

```go
func (b Binding) IsBinary(headers map[string]string) bool
```

IsBinary reports whether headers carry a CloudEvents binary-mode message.

## types.BrokerEvent

```go
type BrokerEvent struct {
	Type      BrokerEventType
	Topic     string
	MessageID string
	Error     error
	Timestamp time.Time
}
```

BrokerEvent represents an observable event within the messaging lifecycle.
Passed to the OnEvent callback configured in msq.Config.

## types.BrokerEventType

```go
type BrokerEventType string
```

BrokerEventType identifies the kind of messaging event.

```go
const (
	EventMessageSent         BrokerEventType = "message_sent"
	EventMessageReceived     BrokerEventType = "message_received"
	EventMessageAcked        BrokerEventType = "message_acked"
	EventMessageNacked       BrokerEventType = "message_nacked"
	EventConsumerStarted     BrokerEventType = "consumer_started"
	EventConsumerStopped     BrokerEventType = "consumer_stopped"
	EventProducerError       BrokerEventType = "producer_error"
	EventConsumerError       BrokerEventType = "consumer_error"
	EventMessageDeadLettered BrokerEventType = "message_dead_lettered"
)
```

## types.ConsumeConfig

```go
type ConsumeConfig struct {
	GroupID         string        // consumer group identifier (Kafka, SQS)
	Topic           string        // topic / queue name / Redis channel
	RoutingKey      string        // AMQP routing key (RabbitMQ only)
	QueueID         string        // provider-assigned queue ID (OCI only)
	Concurrency     int           // number of parallel goroutines
	PollInterval    time.Duration // polling interval for pull-based brokers
	MaxRetries      int           // in-process retries after a Nack (0 = none)
	RetryBackoff    time.Duration // first retry wait, doubled with jitter up to 30s (default 1s)
	DeadLetterTopic string        // receives a copy once retries are exhausted; the original is then acked
	HandlerTimeout  time.Duration // per-attempt handler deadline (0 = none)
	InitialOffset   OffsetReset   // where to start when the group has no committed offset (Kafka)
}
```

ConsumeConfig holds all configuration for a consumer, regardless of broker.
Unused fields are silently ignored by providers that do not support them.

### types.DefaultConsumeConfig

```go
func DefaultConsumeConfig(topic string) ConsumeConfig
```

DefaultConsumeConfig returns a ConsumeConfig with sensible defaults.

## types.Encoding

```go
type Encoding string
```

Encoding selects how a provider writes messages on the wire.

```go
const (
	// EncodingEnvelope writes the whole Message as JSON in the body. It is the
	// default and what every gofi version reads.
	EncodingEnvelope Encoding = "envelope"
	// EncodingCloudEvents writes CloudEvents 1.0 binary mode: the body is Value
	// and the attributes travel in native headers, readable by any CloudEvents
	// SDK. Enable it once every consumer runs gofi v0.5 or later.
	EncodingCloudEvents Encoding = "cloudevents"
)
```

## types.Message

```go
type Message struct {
	Id        uuid.UUID         `json:"id"`
	Topic     string            `json:"topic,omitempty"`
	Type      string            `json:"type,omitempty"`   // CloudEvents type, e.g. "order.created"
	Source    string            `json:"source,omitempty"` // CloudEvents source; defaults to /topics/<topic>
	Key       string            `json:"key,omitempty"`
	Value     json.RawMessage   `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Headers   map[string]string `json:"headers,omitempty"`
}
```

Message is the universal envelope for all broker messages.

Value holds the business payload as raw JSON, making it wire-compatible with
any broker format and allowing type-safe extraction via UnpackMessage.
Headers carries cross-cutting metadata (trace-id, account_info, etc.)
without polluting the business payload.

### types.DecodeEnvelope

```go
func DecodeEnvelope(body []byte) Message
```

DecodeEnvelope reads a JSON envelope. Bodies that are not an envelope
(foreign producers) are delivered as the raw Value.

### types.NewMessage

```go
func NewMessage(value any) (*Message, error)
```

NewMessage creates a Message with the payload serialized as JSON.
Set Topic via WithTopic or directly before sending.

### types.NewMessageWithTopic

```go
func NewMessageWithTopic(topic string, value any) (*Message, error)
```

NewMessageWithTopic creates a Message with topic and payload already set.

### Message.DecodeMessage

```go
func (m *Message) DecodeMessage(model any) error
```

DecodeMessage decodes Value into the given model pointer.

### Message.String

```go
func (m *Message) String() string
```

String returns the JSON representation of the message.

### Message.WithHeader

```go
func (m *Message) WithHeader(key, val string) *Message
```

WithHeader adds a metadata header and returns the message for chaining.

### Message.WithKey

```go
func (m *Message) WithKey(key string) *Message
```

WithKey sets the partition or routing key and returns the message for chaining.

### Message.WithTopic

```go
func (m *Message) WithTopic(topic string) *Message
```

WithTopic sets the routing topic and returns the message for chaining.

## types.OffsetReset

```go
type OffsetReset string
```

OffsetReset controls where a consumer group starts reading when it has no
committed offset (its first run). It is ignored once the group has committed
offsets, and ignored by brokers without the concept (RabbitMQ, Redis).

```go
const (
	// OffsetResetDefault defers to the provider default (Kafka: latest).
	OffsetResetDefault OffsetReset = ""
	// OffsetResetEarliest starts from the beginning of the topic on first run —
	// use for command/event topics that must not be lost.
	OffsetResetEarliest OffsetReset = "earliest"
	// OffsetResetLatest starts from the end of the topic on first run — use for
	// live-tail consumers that should ignore backlog.
	OffsetResetLatest OffsetReset = "latest"
)
```

## types.Result

```go
type Result int
```

Result signals the broker how to handle a processed message.

```go
const (
	Ack    Result = iota // message processed: commit offset / delete from queue
	Nack                 // processing failed: requeue / retry
	Ignore               // discarded on purpose: removed from the queue, no requeue
)
```


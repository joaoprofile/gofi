# msq/types

`import "github.com/joaoprofile/gofi-sdk-go/msq/types"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	DefaultConcurrency  = 20
	DefaultPollInterval = 10 * time.Second
	// DefaultVisibilityTimeout is the lease a received message gets on
	// SQS, OCI Queue and NATS when ConsumeConfig.VisibilityTimeout is zero.
	DefaultVisibilityTimeout = 30 * time.Second
	// DefaultMaxDeliveries bounds how often the broker hands the same message
	// to a handler that keeps nacking it (poison message).
	DefaultMaxDeliveries = 10
	// DefaultHandlerTimeout bounds one handler attempt when
	// ConsumeConfig.HandlerTimeout is zero. The lease is renewed meanwhile, so
	// it only has to outlast the slowest legitimate handler; its job is to
	// keep a stuck handler from blocking shutdown forever.
	DefaultHandlerTimeout = 5 * time.Minute
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

### types.StableID

```go
func StableID(s string) uuid.UUID
```

StableID maps a transport message id (SQS MessageId, AMQP message-id,
Kafka topic/partition/offset, ...) to a UUID: a UUID string is kept, any
other value maps to the same name-based UUID every time.

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
	// EventMessageRejected: the delivery limit was reached with no
	// DeadLetterTopic, so the message was rejected (dropped or broker DLX).
	EventMessageRejected BrokerEventType = "message_rejected"
	// EventConsumerRestarting: a consumer failed and is restarted with backoff.
	EventConsumerRestarting BrokerEventType = "consumer_restarting"
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
	HandlerTimeout  time.Duration // per-attempt handler deadline (0 = DefaultHandlerTimeout, negative = none)
	InitialOffset   OffsetReset   // where to start when the group has no committed offset (Kafka)

	// VisibilityTimeout is how long a received message stays leased to this
	// consumer before the broker redelivers it (SQS and OCI visibility, NATS
	// AckWait, Redis Streams claim idle). The lease is renewed every third of
	// it while the message waits for and runs in its handler, so it bounds
	// crash recovery, not handler time. 0 = DefaultVisibilityTimeout (Redis
	// Streams: Config.ClaimIdle). NATS applies it when it creates the durable.
	VisibilityTimeout time.Duration

	// MaxDeliveries is how many times the broker may deliver a message that
	// keeps failing. On the last one, a message still nacked goes to
	// DeadLetterTopic when set; otherwise it is rejected (dropped, or routed
	// by the broker's own dead-letter setup such as a RabbitMQ DLX) with an
	// EventMessageRejected. Each delivery runs up to 1+MaxRetries attempts.
	// 0 = DefaultMaxDeliveries with a DeadLetterTopic and unlimited without
	// one, so a message is never dropped unless asked; negative = unlimited
	// (defer to the broker, e.g. an SQS redrive policy).
	MaxDeliveries int
}
```

ConsumeConfig holds all configuration for a consumer, regardless of broker.
Unused fields are silently ignored by providers that do not support them.

### types.DefaultConsumeConfig

```go
func DefaultConsumeConfig(topic string) ConsumeConfig
```

DefaultConsumeConfig returns a ConsumeConfig with sensible defaults.

### ConsumeConfig.DeliveryLimit

```go
func (c ConsumeConfig) DeliveryLimit() int
```

DeliveryLimit returns the effective MaxDeliveries; 0 means unlimited.

### ConsumeConfig.DeliveryLimitReached

```go
func (c ConsumeConfig) DeliveryLimitReached(count int) bool
```

DeliveryLimitReached reports whether a message delivered count times has
used its last delivery. An unknown count (0) never reaches the limit.

### ConsumeConfig.EffectiveHandlerTimeout

```go
func (c ConsumeConfig) EffectiveHandlerTimeout() time.Duration
```

EffectiveHandlerTimeout returns the handler deadline; 0 means none.

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

	// DeliveryCount is how many times the broker has delivered this message,
	// this delivery included (1 = first). It is set by the consumer from the
	// broker's own counter and is 0 when the broker does not track it. It
	// never travels on the wire.
	DeliveryCount int `json:"-"`
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
(foreign producers) are delivered as the raw Value with a zero Id: the
provider sets one from the transport (see StableID) so that it stays the
same across redeliveries.

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
	// Reject gives up on a message that cannot be processed: it is removed
	// without requeue and, where the broker has its own dead-letter route
	// (RabbitMQ DLX, NATS terminate advisory), handed to it. The pipeline
	// returns it once MaxDeliveries is reached without a DeadLetterTopic.
	Reject
)
```


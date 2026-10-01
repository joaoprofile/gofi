# msq/msqtest

`import "github.com/gofi-labs/gofi-sdk-go/msq/msqtest"` · SDK v0.8.2

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

Package msqtest is the contract every msq provider must satisfy. Provider
modules run it against in-process fakes and, in integration runs, against
real brokers.

## Funções

### msqtest.Run

```go
func Run(t *testing.T, tg Target)
```

Run exercises the target.

## msqtest.Capabilities

```go
type Capabilities struct {
	// Redelivery: a Nack is delivered again (false for Redis Pub/Sub).
	Redelivery bool
}
```

Capabilities describes provider semantics the contract adapts to.

## msqtest.Target

```go
type Target struct {
	Broker port.Broker
	Caps   Capabilities
	// Topic returns a fresh, empty topic/queue for each case.
	Topic func(t *testing.T) string
	// Config adapts the consume config (e.g. QueueID for OCI).
	Config func(cfg types.ConsumeConfig) types.ConsumeConfig
	// Timeout bounds each case (default 20s).
	Timeout time.Duration
}
```

Target is a broker under test.


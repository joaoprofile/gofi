# msq/worker

`import "github.com/joaoprofile/gofi-sdk-go/msq/worker"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	ReceiveBackoffMin = 200 * time.Millisecond
	ReceiveBackoffMax = 30 * time.Second
)
```

Default receive-error backoff bounds for polling consumers.

## Funções

### worker.KeepAlive

```go
func KeepAlive(ctx context.Context, interval time.Duration, renew func(context.Context)) (stop func())
```

KeepAlive calls renew every interval until the returned stop is called.
Renewal ignores ctx cancellation: a shutdown drains in-flight handlers, and
their leases must outlive it. Each renew call is bounded by interval. stop
waits for a running renew, so no lease is renewed after the message is
acked or deleted.

### worker.RedeliveryDelay

```go
func RedeliveryDelay(base, maxDelay time.Duration, n int) time.Duration
```

RedeliveryDelay is the wait before delivery n+1 of a message nacked on
delivery n (1-based): Backoff from base, capped at maxDelay.

### worker.RenewEvery

```go
func RenewEvery(lease time.Duration) time.Duration
```

RenewEvery is the renewal period for a lease: a third of it, so two renewals
can fail before the broker hands the message to another consumer.

### worker.Sleep

```go
func Sleep(ctx context.Context, d time.Duration) error
```

Sleep waits for d or until ctx is done.

## worker.Backoff

```go
type Backoff struct {
	Min, Max time.Duration
	// contains filtered or unexported fields
}
```

Backoff yields exponential delays with jitter between Min and Max.

### Backoff.Next

```go
func (b *Backoff) Next() time.Duration
```

Next returns the delay for the current attempt and advances it.

### Backoff.Reset

```go
func (b *Backoff) Reset()
```

Reset starts the sequence again after a success.

## worker.Gate

```go
type Gate struct {
	// contains filtered or unexported fields
}
```

Gate pauses consumers without busy waiting. The zero value is open.

### Gate.Pause

```go
func (g *Gate) Pause()
```

Pause closes the gate; it is idempotent.

### Gate.Paused

```go
func (g *Gate) Paused() bool
```

Paused reports whether the gate is closed.

### Gate.Resume

```go
func (g *Gate) Resume()
```

Resume opens the gate and wakes every waiter; it is idempotent.

### Gate.Wait

```go
func (g *Gate) Wait(ctx context.Context) error
```

Wait blocks while the gate is paused or until ctx is done.

## worker.Pool

```go
type Pool struct {
	// contains filtered or unexported fields
}
```

Pool is a bounded goroutine pool. Jobs are dispatched through a channel;
exactly N goroutines consume from that channel in parallel.

### worker.New

```go
func New(n int) *Pool
```

New creates a Pool of n goroutines and starts them immediately.
Call Close to drain pending jobs and stop all goroutines.

### Pool.Close

```go
func (p *Pool) Close()
```

Close drains pending jobs, waits for completion, then stops all goroutines.

### Pool.Enqueue

```go
func (p *Pool) Enqueue(job func())
```

Enqueue submits a job to the pool. Blocks if all workers are busy.

### Pool.Wait

```go
func (p *Pool) Wait()
```

Wait blocks until all enqueued jobs complete.

## worker.Slots

```go
type Slots struct {
	// contains filtered or unexported fields
}
```

Slots counts free handler slots so a poller receives only what can start
now; a message waiting in a local buffer would burn its lease unseen.

### worker.NewSlots

```go
func NewSlots(n int) *Slots
```

NewSlots returns n free slots (at least one).

### Slots.Acquire

```go
func (s *Slots) Acquire(ctx context.Context, limit int) (int, error)
```

Acquire blocks until a slot is free, then takes up to limit of them
without blocking. It returns how many it took, or ctx's error.

### Slots.Release

```go
func (s *Slots) Release(n int)
```

Release returns n slots; slots beyond the capacity are dropped, so a
broker that delivers more than asked cannot deadlock the poller.


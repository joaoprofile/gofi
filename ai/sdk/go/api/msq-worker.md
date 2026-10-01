# msq/worker

`import "github.com/gofi-labs/gofi-sdk-go/msq/worker"` · SDK v0.8.2

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


# base/cronjob

`import "github.com/joaoprofile/gofi-sdk-go/base/cronjob"` · SDK gofi/v0.2.4 (310ba1a)

> Gerado do código-fonte do SDK — não edite. Regenere com `gofi update sdk`.

## Constantes

```go
const (
	// DefaultLockTTL is the lease ttl when ScheduleConfig.LockTTL is zero.
	DefaultLockTTL = 30 * time.Second
)
```

## Variáveis

```go
var ErrPoolClosed = errors.New("cronjob: worker pool is closed")
```

ErrPoolClosed is returned when a batch is enqueued after Close.

## Funções

### cronjob.CheckAllJobsHealth

```go
func CheckAllJobsHealth(jobHandles []*JobHandle)
```

### cronjob.FencingToken

```go
func FencingToken(ctx context.Context) (int64, bool)
```

FencingToken returns the token of the lease held by the running job. Pass
it to the systems the job writes to so they can reject a holder whose lease
already expired (e.g. UPDATE ... WHERE fence < $token). ok is false when
the job has no Locker.

## cronjob.JobGenerator

```go
type JobGenerator[T any] struct {
	GenericObject  []T
	ProcessJobFunc func(T) error
	BatchSize      int
	// OnError receives each item whose ProcessJobFunc failed; nil logs the
	// error with slog.
	OnError func(item T, err error)
}
```

### cronjob.NewJobGenerator

```go
func NewJobGenerator[T any](genericObject []T, batchSize int, processJobFunc func(T) error) *JobGenerator[T]
```

### JobGenerator.GenerateJobs

```go
func (j *JobGenerator[T]) GenerateJobs() [][]func()
```

### JobGenerator.RunWithPool

```go
func (j *JobGenerator[T]) RunWithPool(pool *WorkerPool) error
```

RunWithPool enqueues every batch; it stops at ErrPoolClosed.

## cronjob.JobHandle

```go
type JobHandle struct {
	// contains filtered or unexported fields
}
```

### cronjob.ScheduleJob

```go
func ScheduleJob(ctx context.Context, cfg ScheduleConfig, job func()) *JobHandle
```

ScheduleJob is ScheduleJobCtx for a job that takes no context and cannot fail.

### cronjob.ScheduleJobCtx

```go
func ScheduleJobCtx(ctx context.Context, cfg ScheduleConfig, job func(context.Context) error) *JobHandle
```

ScheduleJobCtx starts a background job according to cfg and returns a
handle to control it. Each run receives a context cancelled by Stop, by the
parent ctx or when its lease is lost; an error or panic marks the handle
JobFailed until a later run succeeds.
Panics if cfg is invalid (invalid mode, non-positive interval, out-of-range
hour/minute, unknown location, or a Locker without Name).

### JobHandle.GetStatus

```go
func (j *JobHandle) GetStatus() JobStatus
```

GetStatus returns JobRunning while scheduled and the last run succeeded,
JobFailed when the last run returned an error or panicked, and JobStopped
after Stop or the parent context ends.

### JobHandle.Stop

```go
func (j *JobHandle) Stop(ctx context.Context) error
```

Stop cancels the schedule and the context of the run in progress, then
waits for that run to return. It returns ctx.Err() if ctx ends first; the
run keeps going in the background. Calling it again is safe.

## cronjob.JobStatus

```go
type JobStatus string
```

```go
const (
	JobRunning JobStatus = "running"
	JobStopped JobStatus = "stopped"
	JobFailed  JobStatus = "failed"
)
```

## cronjob.Locker

```go
type Locker interface {
	// Acquire takes key for ttl. ok is false when another holder has it. The
	// token identifies this holder and grows with every successful Acquire of
	// the job's keys, so it doubles as a fencing token.
	Acquire(ctx context.Context, key string, ttl time.Duration) (token int64, ok bool, err error)
	// Renew extends key to ttl while token still holds it; false means the
	// key was lost (expired or taken by another holder).
	Renew(ctx context.Context, key string, token int64, ttl time.Duration) (bool, error)
	// Release frees key if token still holds it.
	Release(ctx context.Context, key string, token int64) error
}
```

Locker coordinates replicas running the same job: a key is held by one
holder at a time until its ttl expires or it is released.

## cronjob.RedisLocker

```go
type RedisLocker struct {
	Client goredis.UniversalClient
}
```

RedisLocker implements Locker on Redis. Tokens come from a per-job counter
(gofi:cron:{name}:fence) that is never reset, so they keep growing across
leases and replicas.

### RedisLocker.Acquire

```go
func (l RedisLocker) Acquire(ctx context.Context, key string, ttl time.Duration) (int64, bool, error)
```

Acquire implements Locker.

### RedisLocker.Release

```go
func (l RedisLocker) Release(ctx context.Context, key string, token int64) error
```

Release implements Locker.

### RedisLocker.Renew

```go
func (l RedisLocker) Renew(ctx context.Context, key string, token int64, ttl time.Duration) (bool, error)
```

Renew implements Locker.

## cronjob.ScheduleConfig

```go
type ScheduleConfig struct {
	Mode         ScheduleMode   // Scheduling mode
	Interval     time.Duration  // Used in 'interval' mode
	Hour         int            // Used in 'fixed' mode
	Minute       int            // Used in 'fixed' mode
	Weekdays     []time.Weekday // If empty, runs every day
	LocationName string         // Optional, if empty, uses the system's timezone

	// Name identifies the job across replicas; required with Locker.
	Name string
	// Locker makes each scheduled run execute on one replica only and keeps
	// runs from overlapping across replicas (see RedisLocker). Nil runs the
	// job on every replica.
	Locker Locker
	// LockTTL is the lease ttl with a Locker, renewed every third of it while
	// the job runs. Zero uses DefaultLockTTL.
	LockTTL time.Duration
}
```

## cronjob.ScheduleMode

```go
type ScheduleMode string
```

```go
const (
	Interval ScheduleMode = "interval"
	Fixed    ScheduleMode = "fixed"
)
```

## cronjob.WorkerPool

```go
type WorkerPool struct {
	Workers int
	Jobs    chan []func()
	// contains filtered or unexported fields
}
```

### cronjob.NewPool

```go
func NewPool(workers int) *WorkerPool
```

NewPool creates a pool of workers goroutines (started by Start); workers
below 1 uses GOMAXPROCS, as a pool without workers would block forever.

### WorkerPool.Close

```go
func (p *WorkerPool) Close()
```

Close stops accepting batches, waits for the queued ones and stops the
workers. Calling it again is a no-op.

### WorkerPool.EnqueueJobBatch

```go
func (p *WorkerPool) EnqueueJobBatch(batch []func()) error
```

EnqueueJobBatch queues batch, blocking while the queue is full. It returns
ErrPoolClosed after Close.

### WorkerPool.Start

```go
func (p *WorkerPool) Start()
```

### WorkerPool.Wait

```go
func (p *WorkerPool) Wait()
```

